package dns

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func NewResolver(address string) (Resolver, error) {
	if strings.HasPrefix(address, "[/") {
		parts := strings.SplitN(address, "/]", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid domain-specific format")
		}
		address = parts[1]
	}

	// 解析协议
	if strings.Contains(address, "://") {
		u, err := url.Parse(address)
		if err != nil {
			return nil, err
		}

		switch u.Scheme {
		case "udp":
			return &UDPResolver{server: u.Host}, nil
		case "tcp":
			return &TCPResolver{server: u.Host}, nil
		case "tls":
			return &TLSResolver{server: u.Host}, nil
		case "https":
			if u.Host == "" {
				return nil, fmt.Errorf("invalid DoH endpoint: missing host")
			}
			return &HTTPSResolver{endpoint: u.String()}, nil
		default:
			return nil, fmt.Errorf("unsupported protocol: %s", u.Scheme)
		}
	}

	// 默认 UDP
	return &UDPResolver{server: address}, nil
}

func (r *UDPResolver) Resolve(ctx context.Context, domain string, timeout time.Duration, options ResolveOptions) DNSResult {
	c := dns.Client{
		Timeout: timeout,
	}
	return exchangeQueries(ctx, &c, r.server, "53", ProtocolUDP, domain, options)
}

func (r *TCPResolver) Resolve(ctx context.Context, domain string, timeout time.Duration, options ResolveOptions) DNSResult {
	c := dns.Client{
		Net:     "tcp",
		Timeout: timeout,
	}
	return exchangeQueries(ctx, &c, r.server, "53", ProtocolTCP, domain, options)
}

func (r *TLSResolver) Resolve(ctx context.Context, domain string, timeout time.Duration, options ResolveOptions) DNSResult {
	c := dns.Client{
		Net:     "tcp-tls",
		Timeout: timeout,
	}
	return exchangeQueries(ctx, &c, r.server, "853", ProtocolTLS, domain, options)
}

var newDoHHTTPClient = func(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func (r *HTTPSResolver) Resolve(ctx context.Context, domain string, timeout time.Duration, options ResolveOptions) DNSResult {
	result := DNSResult{
		Server:   r.endpoint,
		Domain:   domain,
		Protocol: ProtocolHTTPS,
		Answers:  make([]Answer, 0),
	}

	client := newDoHHTTPClient(timeout)
	recordTypes := normalizeRecordTypes(options.RecordTypes)
	successfulQueries := 0
	var lastErr error
	for _, recordType := range recordTypes {
		if err := ctx.Err(); err != nil {
			result.ResolutionError = err
			result.QueryErrors = append(result.QueryErrors, err.Error())
			return result
		}

		queryType, err := dnsQueryType(recordType)
		if err != nil {
			result.ResolutionError = err
			return result
		}

		msg := dns.Msg{}
		msg.SetQuestion(dns.Fqdn(domain), queryType)
		payload, err := msg.Pack()
		if err != nil {
			result.ResolutionError = err
			result.QueryErrors = append(result.QueryErrors, fmt.Sprintf("%s: pack DNS query: %v", recordType, err))
			return result
		}

		resp, duration, err := exchangeHTTPSMessage(ctx, client, r.endpoint, payload)
		result.ResponseTime += duration
		if err != nil {
			result.QueryErrors = append(result.QueryErrors, fmt.Sprintf("%s: %v", recordType, err))
			lastErr = err
			continue
		}

		result.ResponseCodes = append(result.ResponseCodes, ResponseCode{
			RecordType: string(recordType),
			Code:       resp.Rcode,
			Name:       dns.RcodeToString[resp.Rcode],
		})
		if resp.Rcode != dns.RcodeSuccess {
			err := fmt.Errorf("%s: DNS response code %s", recordType, dns.RcodeToString[resp.Rcode])
			result.QueryErrors = append(result.QueryErrors, err.Error())
			lastErr = err
			continue
		}

		successfulQueries++
		result.Answers = append(result.Answers, ParseAnswers(resp.Answer, recordType)...)
	}

	if len(recordTypes) > 0 {
		result.ResponseTime /= time.Duration(len(recordTypes))
	}
	if successfulQueries == 0 && lastErr != nil {
		result.ResolutionError = lastErr
	}
	if successfulQueries > 0 {
		result.NoAnswer = !hasAddressAnswer(result.Answers)
	}
	return result
}

func exchangeHTTPSMessage(ctx context.Context, client *http.Client, endpoint string, payload []byte) (*dns.Msg, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("Content-Type", "application/dns-message")

	start := time.Now()
	httpResp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return nil, duration, fmt.Errorf("HTTPS exchange: %w", err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))
		if len(body) > 0 {
			return nil, duration, fmt.Errorf("HTTP status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil, duration, fmt.Errorf("HTTP status %d", httpResp.StatusCode)
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, duration, fmt.Errorf("read HTTPS DNS response: %w", err)
	}
	msg := dns.Msg{}
	if err := msg.Unpack(body); err != nil {
		return nil, duration, fmt.Errorf("decode HTTPS DNS response: %w", err)
	}
	return &msg, duration, nil
}

func exchangeQueries(ctx context.Context, c *dns.Client, server, defaultPort string, protocol Protocol, domain string, options ResolveOptions) DNSResult {
	result := DNSResult{
		Server:   server,
		Domain:   domain,
		Protocol: protocol,
		Answers:  make([]Answer, 0),
	}

	address := serverAddress(server, defaultPort)
	recordTypes := normalizeRecordTypes(options.RecordTypes)
	successfulQueries := 0
	var lastErr error
	for _, recordType := range recordTypes {
		if err := ctx.Err(); err != nil {
			result.ResolutionError = err
			result.QueryErrors = append(result.QueryErrors, err.Error())
			return result
		}

		queryType, err := dnsQueryType(recordType)
		if err != nil {
			result.ResolutionError = err
			return result
		}

		msg := dns.Msg{}
		msg.SetQuestion(dns.Fqdn(domain), queryType)

		resp, duration, err := c.ExchangeContext(ctx, &msg, address)
		result.ResponseTime += duration
		if err != nil {
			result.QueryErrors = append(result.QueryErrors, fmt.Sprintf("%s: %v", recordType, err))
			lastErr = err
			continue
		}
		if resp == nil {
			err := fmt.Errorf("%s: empty DNS response", recordType)
			result.QueryErrors = append(result.QueryErrors, err.Error())
			lastErr = err
			continue
		}

		result.ResponseCodes = append(result.ResponseCodes, ResponseCode{
			RecordType: string(recordType),
			Code:       resp.Rcode,
			Name:       dns.RcodeToString[resp.Rcode],
		})
		if resp.Rcode != dns.RcodeSuccess {
			err := fmt.Errorf("%s: DNS response code %s", recordType, dns.RcodeToString[resp.Rcode])
			result.QueryErrors = append(result.QueryErrors, err.Error())
			lastErr = err
			continue
		}

		successfulQueries++
		result.Answers = append(result.Answers, ParseAnswers(resp.Answer, recordType)...)
	}

	if len(recordTypes) > 0 {
		result.ResponseTime /= time.Duration(len(recordTypes))
	}
	if successfulQueries == 0 && lastErr != nil {
		result.ResolutionError = lastErr
	}
	if successfulQueries > 0 {
		result.NoAnswer = !hasAddressAnswer(result.Answers)
	}

	return result
}

func serverAddress(server, defaultPort string) string {
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(strings.Trim(server, "[]"), defaultPort)
}

func normalizeRecordTypes(recordTypes []RecordType) []RecordType {
	if len(recordTypes) == 0 {
		return []RecordType{RecordTypeA}
	}

	normalized := make([]RecordType, 0, len(recordTypes))
	seen := make(map[RecordType]bool, len(recordTypes))
	for _, recordType := range recordTypes {
		recordType = RecordType(strings.ToUpper(strings.TrimSpace(string(recordType))))
		if recordType == "" || seen[recordType] {
			continue
		}
		seen[recordType] = true
		normalized = append(normalized, recordType)
	}
	if len(normalized) == 0 {
		return []RecordType{RecordTypeA}
	}
	return normalized
}

func dnsQueryType(recordType RecordType) (uint16, error) {
	switch recordType {
	case RecordTypeA:
		return dns.TypeA, nil
	case RecordTypeAAAA:
		return dns.TypeAAAA, nil
	default:
		return 0, fmt.Errorf("unsupported DNS record type %q", recordType)
	}
}

func ParseAnswers(records []dns.RR, queryType RecordType) []Answer {
	answers := make([]Answer, 0, len(records))
	for _, record := range records {
		header := record.Header()
		answer := Answer{
			QueryType: string(queryType),
			Name:      strings.TrimSuffix(header.Name, "."),
			Type:      dns.TypeToString[header.Rrtype],
			TTL:       header.Ttl,
		}

		switch value := record.(type) {
		case *dns.A:
			answer.Value = value.A.String()
			answer.Family = "ipv4"
		case *dns.AAAA:
			answer.Value = value.AAAA.String()
			answer.Family = "ipv6"
		case *dns.CNAME:
			answer.Value = strings.TrimSuffix(value.Target, ".")
		case *dns.MX:
			answer.Value = strings.TrimSuffix(value.Mx, ".")
			answer.Priority = int(value.Preference)
		default:
			answer.Value = record.String()
		}

		answers = append(answers, answer)
	}
	return answers
}

func hasAddressAnswer(answers []Answer) bool {
	for _, answer := range answers {
		switch answer.Type {
		case "A", "AAAA":
			if answer.Value != "" {
				return true
			}
		}
	}
	return false
}

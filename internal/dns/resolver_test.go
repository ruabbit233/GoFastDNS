package dns

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	miekgdns "github.com/miekg/dns"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestParseAnswersPreservesStructuredRecords(t *testing.T) {
	records := []miekgdns.RR{
		&miekgdns.CNAME{
			Hdr:    miekgdns.RR_Header{Name: "www.example.com.", Rrtype: miekgdns.TypeCNAME, Class: miekgdns.ClassINET, Ttl: 60},
			Target: "edge.example.com.",
		},
		&miekgdns.A{
			Hdr: miekgdns.RR_Header{Name: "edge.example.com.", Rrtype: miekgdns.TypeA, Class: miekgdns.ClassINET, Ttl: 300},
			A:   net.ParseIP("192.0.2.1"),
		},
		&miekgdns.AAAA{
			Hdr:  miekgdns.RR_Header{Name: "edge.example.com.", Rrtype: miekgdns.TypeAAAA, Class: miekgdns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP("2001:db8::1"),
		},
	}

	answers := ParseAnswers(records, RecordTypeA)
	if len(answers) != 3 {
		t.Fatalf("expected 3 answers, got %#v", answers)
	}
	if answers[0].Type != "CNAME" || answers[0].Value != "edge.example.com" || answers[0].TTL != 60 {
		t.Fatalf("unexpected CNAME answer: %#v", answers[0])
	}
	if answers[1].Type != "A" || answers[1].Value != "192.0.2.1" || answers[1].Family != "ipv4" {
		t.Fatalf("unexpected A answer: %#v", answers[1])
	}
	if answers[2].Type != "AAAA" || answers[2].Value != "2001:db8::1" || answers[2].Family != "ipv6" {
		t.Fatalf("unexpected AAAA answer: %#v", answers[2])
	}
}

func TestHasAddressAnswer(t *testing.T) {
	if hasAddressAnswer([]Answer{{Type: "CNAME", Value: "edge.example.com"}}) {
		t.Fatal("expected CNAME-only answer set to have no address answer")
	}
	if !hasAddressAnswer([]Answer{{Type: "AAAA", Value: "2001:db8::1"}}) {
		t.Fatal("expected AAAA answer to count as address answer")
	}
}

func TestServerAddressPreservesExplicitPort(t *testing.T) {
	tests := []struct {
		name        string
		server      string
		defaultPort string
		want        string
	}{
		{name: "plain host", server: "8.8.8.8", defaultPort: "53", want: "8.8.8.8:53"},
		{name: "host with port", server: "8.8.8.8:5353", defaultPort: "53", want: "8.8.8.8:5353"},
		{name: "ipv6 host", server: "[2001:4860:4860::8888]", defaultPort: "53", want: "[2001:4860:4860::8888]:53"},
		{name: "ipv6 with port", server: "[2001:4860:4860::8888]:5353", defaultPort: "53", want: "[2001:4860:4860::8888]:5353"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverAddress(tt.server, tt.defaultPort); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveDNSWithOptionsUnsupportedResolverInitializesSlices(t *testing.T) {
	result := ResolveDNSWithOptions("bad://resolver", "example.com", 0, time.Second, ResolveOptions{})
	if result.ResolutionError == nil {
		t.Fatal("expected unsupported resolver error")
	}
	if result.Answers == nil {
		t.Fatal("expected answers to be an empty slice")
	}
	if len(result.QueryErrors) != 1 {
		t.Fatalf("expected query error, got %#v", result.QueryErrors)
	}
}

func TestHTTPSResolverResolve(t *testing.T) {
	restore := replaceDoHHTTPClientForTest(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.String() != "https://resolver.example/dns-query" {
			t.Fatalf("unexpected endpoint URL: %s", r.URL.String())
		}
		if got := r.Header.Get("Content-Type"); got != "application/dns-message" {
			t.Fatalf("expected application/dns-message content type, got %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var query miekgdns.Msg
		if err := query.Unpack(body); err != nil {
			t.Fatalf("unpack query: %v", err)
		}
		if len(query.Question) != 1 || query.Question[0].Qtype != miekgdns.TypeA {
			t.Fatalf("unexpected DNS question: %#v", query.Question)
		}

		response := miekgdns.Msg{}
		response.SetReply(&query)
		response.Answer = []miekgdns.RR{
			&miekgdns.A{
				Hdr: miekgdns.RR_Header{Name: "example.com.", Rrtype: miekgdns.TypeA, Class: miekgdns.ClassINET, Ttl: 60},
				A:   net.ParseIP("192.0.2.10"),
			},
		}
		payload, err := response.Pack()
		if err != nil {
			t.Fatalf("pack response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	}))
	defer restore()

	resolver := HTTPSResolver{endpoint: "https://resolver.example/dns-query"}
	result := resolver.Resolve(context.Background(), "example.com", time.Second, ResolveOptions{RecordTypes: []RecordType{RecordTypeA}})

	if result.ResolutionError != nil {
		t.Fatalf("unexpected resolution error: %v", result.ResolutionError)
	}
	if result.Protocol != ProtocolHTTPS {
		t.Fatalf("expected HTTPS protocol, got %s", result.Protocol)
	}
	if len(result.Answers) != 1 || result.Answers[0].Value != "192.0.2.10" {
		t.Fatalf("unexpected answers: %#v", result.Answers)
	}
	if len(result.ResponseCodes) != 1 || result.ResponseCodes[0].Name != "NOERROR" {
		t.Fatalf("unexpected response codes: %#v", result.ResponseCodes)
	}
}

func TestHTTPSResolverHTTPStatusError(t *testing.T) {
	restore := replaceDoHHTTPClientForTest(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("upstream unavailable")),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	}))
	defer restore()

	resolver := HTTPSResolver{endpoint: "https://resolver.example/dns-query"}
	result := resolver.Resolve(context.Background(), "example.com", time.Second, ResolveOptions{RecordTypes: []RecordType{RecordTypeA}})

	if result.ResolutionError == nil {
		t.Fatal("expected HTTP status to become resolution error")
	}
	if len(result.QueryErrors) != 1 || !strings.Contains(result.QueryErrors[0], "HTTP status 503") {
		t.Fatalf("expected HTTP status query error, got %#v", result.QueryErrors)
	}
}

func replaceDoHHTTPClientForTest(t *testing.T, transport http.RoundTripper) func() {
	t.Helper()
	original := newDoHHTTPClient
	newDoHHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout:   timeout,
			Transport: transport,
		}
	}
	return func() {
		newDoHHTTPClient = original
	}
}

func TestNewResolverAcceptsHTTPS(t *testing.T) {
	resolver, err := NewResolver("https://dns.example/dns-query")
	if err != nil {
		t.Fatalf("expected https resolver to be accepted: %v", err)
	}
	httpsResolver, ok := resolver.(*HTTPSResolver)
	if !ok {
		t.Fatalf("expected HTTPSResolver, got %T", resolver)
	}
	if _, err := url.Parse(httpsResolver.endpoint); err != nil {
		t.Fatalf("expected valid endpoint URL: %v", err)
	}
}

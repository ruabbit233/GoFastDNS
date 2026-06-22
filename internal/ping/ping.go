package ping

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"GoFastDNS/internal/dns"

	probing "github.com/prometheus-community/pro-bing"
)

const (
	defaultCount    = 3
	defaultInterval = time.Millisecond * 100
	defaultTimeout  = time.Second * 2
)

var tcpDialContext = func(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	return dialer.DialContext(ctx, "tcp", address)
}

func PingIP(ip string) PingResult {
	return PingIPWithOptions(ip, Options{})
}

func PingIPWithOptions(ip string, options Options) PingResult {
	return PingIPWithOptionsContext(context.Background(), ip, options)
}

func PingIPWithOptionsContext(ctx context.Context, ip string, options Options) PingResult {
	options = applyDefaults(options)

	switch options.Method {
	case "tcp":
		return pingTCP(ctx, ip, options)
	default:
		return pingICMP(ctx, ip, options)
	}
}

func pingICMP(ctx context.Context, ip string, options Options) PingResult {
	pinger, err := probing.NewPinger(ip)
	if err != nil {
		return PingResult{
			IP:    ip,
			Error: err,
		}
	}
	pinger.SetPrivileged(options.Privileged)
	pinger.Count = options.Count
	pinger.Interval = options.Interval
	pinger.Timeout = options.Timeout

	err = pinger.RunWithContext(ctx)
	if err != nil {
		return PingResult{
			IP:    ip,
			Error: err,
		}
	}

	stats := pinger.Statistics()
	result := PingResult{
		IP:          ip,
		RTT:         stats.AvgRtt,
		PacketLoss:  stats.PacketLoss,
		PacketsSent: stats.PacketsSent,
		PacketsRecv: stats.PacketsRecv,
	}
	if err := result.FailureError(); err != nil {
		result.Error = err
	}
	return result
}

func pingTCP(ctx context.Context, ip string, options Options) PingResult {
	address := net.JoinHostPort(ip, fmt.Sprintf("%d", options.TCPPort))
	result := PingResult{
		IP: ip,
	}
	var totalRTT time.Duration
	var lastErr error

	for i := 0; i < options.Count; i++ {
		if err := ctx.Err(); err != nil {
			result.Error = err
			break
		}

		start := time.Now()
		result.PacketsSent++
		conn, err := tcpDialContext(ctx, address, options.Timeout)
		if err != nil {
			lastErr = err
		} else {
			totalRTT += time.Since(start)
			result.PacketsRecv++
			_ = conn.Close()
		}

		if i < options.Count-1 {
			timer := time.NewTimer(options.Interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				result.Error = ctx.Err()
				i = options.Count
			case <-timer.C:
			}
		}
	}

	failed := result.PacketsSent - result.PacketsRecv
	if result.PacketsSent > 0 {
		result.PacketLoss = float64(failed) / float64(result.PacketsSent) * 100
	}
	if result.PacketsRecv > 0 {
		result.RTT = totalRTT / time.Duration(result.PacketsRecv)
		return result
	}
	if result.Error == nil {
		result.Error = fmt.Errorf("tcp ping %s failed: %w", address, lastErr)
	}
	return result
}

func PingDNSResult(result dns.DNSResult) DNSPingResult {
	return PingDNSResultWithOptions(result, Options{})
}

func PingDNSResultWithOptions(result dns.DNSResult, options Options) DNSPingResult {
	return PingDNSResultWithOptionsContext(context.Background(), result, options)
}

func PingDNSResultWithOptionsContext(ctx context.Context, result dns.DNSResult, options Options) DNSPingResult {
	return PingDNSResultWithOptionsAndRunner(ctx, result, options, PingIPWithOptionsContext)
}

func PingDNSResultWithOptionsAndRunner(ctx context.Context, result dns.DNSResult, options Options, runner func(context.Context, string, Options) PingResult) DNSPingResult {
	if result.ResolutionError != nil {
		return DNSPingResult{
			Domain:    result.Domain,
			DNSServer: result.Server,
			Error:     result.ResolutionError,
		}
	}

	targets := SelectTargets(result.Answers, options.IPSelection, options.IPFamily)
	pingResults := make([]PingResult, 0, len(targets))
	var totalRTT time.Duration
	successfulPings := 0

	for _, ip := range targets {
		if err := ctx.Err(); err != nil {
			return DNSPingResult{
				Domain:      result.Domain,
				DNSServer:   result.Server,
				PingResults: pingResults,
				AvgRTT:      averageRTT(totalRTT, successfulPings),
				Error:       err,
			}
		}
		pingResult := runner(ctx, ip, options)
		if err := pingResult.FailureError(); err != nil {
			pingResult.Error = err
		}
		pingResults = append(pingResults, pingResult)

		// 只计算成功的 ping 结果
		if pingResult.Successful() {
			totalRTT += pingResult.RTT
			successfulPings++
		}
	}

	// 计算平均 RTT
	avgRTT := averageRTT(totalRTT, successfulPings)

	dnsPingResult := DNSPingResult{
		Domain:      result.Domain,
		DNSServer:   result.Server,
		PingResults: pingResults,
		AvgRTT:      avgRTT,
	}
	if len(pingResults) > 0 && successfulPings == 0 {
		dnsPingResult.Error = pingResults[0].FailureError()
	}
	return dnsPingResult
}

func (r PingResult) Successful() bool {
	return r.FailureError() == nil
}

func (r PingResult) FailureError() error {
	if r.Error != nil {
		return r.Error
	}
	if r.PacketsSent > 0 && r.PacketsRecv == 0 && r.PacketLoss >= 100 {
		return fmt.Errorf("no ping replies received from %s (%d packets sent, %.1f%% packet loss)", r.IP, r.PacketsSent, r.PacketLoss)
	}
	if r.RTT <= 0 {
		if r.IP == "" {
			return fmt.Errorf("no successful ping RTT")
		}
		return fmt.Errorf("no successful ping RTT for %s", r.IP)
	}
	return nil
}

func averageRTT(totalRTT time.Duration, successfulPings int) time.Duration {
	if successfulPings == 0 {
		return 0
	}
	return totalRTT / time.Duration(successfulPings)
}

func applyDefaults(options Options) Options {
	if options.Count <= 0 {
		options.Count = defaultCount
	}
	if options.Interval <= 0 {
		options.Interval = defaultInterval
	}
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if options.Method == "" {
		options.Method = "icmp"
	}
	options.Method = strings.ToLower(options.Method)
	if options.TCPPort <= 0 {
		options.TCPPort = 443
	}
	if options.IPSelection == "" {
		options.IPSelection = "all"
	}
	options.IPSelection = strings.ToLower(options.IPSelection)
	if options.IPFamily == "" {
		options.IPFamily = "ipv4"
	}
	options.IPFamily = strings.ToLower(options.IPFamily)
	return options
}

func SelectTargets(answers []dns.Answer, selection, family string) []string {
	selection = strings.ToLower(selection)
	family = strings.ToLower(family)

	targets := make([]string, 0, len(answers))
	for _, answer := range answers {
		if answer.Type != "A" && answer.Type != "AAAA" {
			continue
		}
		if answer.Value == "" {
			continue
		}

		ip := net.ParseIP(answer.Value)
		if ip == nil {
			continue
		}
		answerFamily := "ipv6"
		if ip.To4() != nil {
			answerFamily = "ipv4"
		}
		if family != "dual" && family != "" && family != answerFamily {
			continue
		}

		targets = append(targets, answer.Value)
	}

	if strings.ToLower(selection) == "first" {
		if len(targets) == 0 {
			return nil
		}
		return targets[:1]
	}
	return targets
}

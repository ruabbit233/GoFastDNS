package ping

import (
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

func PingIP(ip string) PingResult {
	return PingIPWithOptions(ip, Options{})
}

func PingIPWithOptions(ip string, options Options) PingResult {
	options = applyDefaults(options)

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

	err = pinger.Run()
	if err != nil {
		return PingResult{
			IP:    ip,
			Error: err,
		}
	}

	stats := pinger.Statistics()
	return PingResult{
		IP:          ip,
		RTT:         stats.AvgRtt,
		PacketLoss:  stats.PacketLoss,
		PacketsSent: stats.PacketsSent,
	}
}

func PingDNSResult(result dns.DNSResult) DNSPingResult {
	return PingDNSResultWithOptions(result, Options{})
}

func PingDNSResultWithOptions(result dns.DNSResult, options Options) DNSPingResult {
	if result.ResolutionError != nil {
		return DNSPingResult{
			Domain:    result.Domain,
			DNSServer: result.Server,
			Error:     result.ResolutionError,
		}
	}

	pingResults := make([]PingResult, 0, len(result.Answers))
	var totalRTT time.Duration
	successfulPings := 0

	for _, ip := range selectPingTargets(result.Answers, options.IPSelection) {
		pingResult := PingIPWithOptions(ip, options)
		pingResults = append(pingResults, pingResult)

		// 只计算成功的 ping 结果
		if pingResult.Error == nil {
			totalRTT += pingResult.RTT
			successfulPings++
		}
	}

	// 计算平均 RTT
	var avgRTT time.Duration
	if successfulPings > 0 {
		avgRTT = totalRTT / time.Duration(successfulPings)
	}

	dnsPingResult := DNSPingResult{
		Domain:      result.Domain,
		DNSServer:   result.Server,
		PingResults: pingResults,
		AvgRTT:      avgRTT,
	}
	if len(pingResults) > 0 && successfulPings == 0 {
		dnsPingResult.Error = pingResults[0].Error
	}
	return dnsPingResult
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
	if options.IPSelection == "" {
		options.IPSelection = "all"
	}
	options.IPSelection = strings.ToLower(options.IPSelection)
	return options
}

func selectPingTargets(answers []string, selection string) []string {
	if len(answers) == 0 {
		return nil
	}
	if strings.ToLower(selection) == "first" {
		return answers[:1]
	}
	return answers
}

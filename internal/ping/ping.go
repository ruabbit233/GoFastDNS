package ping

import (
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

	targets := SelectTargets(result.Answers, options.IPSelection, options.IPFamily)
	pingResults := make([]PingResult, 0, len(targets))
	var totalRTT time.Duration
	successfulPings := 0

	for _, ip := range targets {
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

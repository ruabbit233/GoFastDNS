package benchmark

import (
	"GoFastDNS/internal/config"
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/ping"
	"fmt"
	"log"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

func Run(cfg config.Config, logger *log.Logger) (string, error) {
	outputFormat := strings.ToLower(cfg.Output.Format)

	switch cfg.Mode {
	case config.ModeResolvePing:
		results := RunResolvePing(cfg, logger)
		SortResolvePingResults(results)
		switch outputFormat {
		case "excel":
			return SaveResultsToExcel(cfg.DNSServers, results, cfg.Output.Path)
		case "html":
			return SaveResultsToHTML(results, cfg.Output.Path)
		case "json":
			return SaveResolvePingResultsToJSON(results, cfg.Output.Path)
		default:
			return "", fmt.Errorf("unsupported output format %q", cfg.Output.Format)
		}
	case config.ModeDNSPing:
		results := RunDNSPing(cfg, logger)
		SortDNSPingResults(results)
		switch outputFormat {
		case "excel":
			return SaveDNSPingResultsToExcel(results, cfg.Output.Path)
		case "html":
			return SaveDNSPingResultsToHTML(results, cfg.Output.Path)
		case "json":
			return SaveDNSPingResultsToJSON(results, cfg.Output.Path)
		default:
			return "", fmt.Errorf("unsupported output format %q", cfg.Output.Format)
		}
	default:
		return "", fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

func RunBenchmark(servers []string, domains []string, attempts int, timeout time.Duration) []BenchmarkResult {
	cfg := config.DefaultConfig()
	cfg.DNSServers = servers
	cfg.Domains = domains
	cfg.Attempts = attempts
	cfg.Timeout = timeout
	return RunResolvePing(cfg, log.Default())
}

func RunResolvePing(cfg config.Config, logger *log.Logger) []BenchmarkResult {
	var results []BenchmarkResult
	var wg sync.WaitGroup
	mu := &sync.Mutex{}
	pingOptions := ping.Options{
		Count:       cfg.Ping.Count,
		Interval:    cfg.Ping.Interval,
		Timeout:     cfg.Ping.Timeout,
		Privileged:  cfg.Ping.Privileged,
		IPSelection: cfg.Ping.IPSelection,
		IPFamily:    cfg.Ping.IPFamily,
	}
	resolveOptions := dns.ResolveOptions{
		RecordTypes: dnsRecordTypes(cfg.DNS.RecordTypes),
	}

	for _, server := range cfg.DNSServers {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			var total time.Duration
			var pingTotal time.Duration
			domainResults := make([]DomainResult, 0, len(cfg.Domains))
			successCount := 0
			pingSuccessCount := 0
			totalQueries := len(cfg.Domains)
			totalRetries := 0

			for _, domain := range cfg.Domains {
				// DNS 解析
				result := dns.ResolveDNSWithOptions(s, domain, cfg.Attempts, cfg.Timeout, resolveOptions)

				// 执行 Ping 测试
				dnsPingResult := ping.PingDNSResultWithOptions(result, pingOptions)

				domainResult := DomainResult{
					Domain:         domain,
					Answers:        result.Answers,
					ResponseCodes:  result.ResponseCodes,
					ResponseTime:   result.ResponseTime,
					Error:          result.ResolutionError,
					QueryErrors:    result.QueryErrors,
					NoAnswer:       result.NoAnswer,
					RetryCount:     result.RetryCount,
					DnsPingResults: dnsPingResult, // 添加 Ping 结果
				}

				domainResults = append(domainResults, domainResult)
				totalRetries += result.RetryCount

				if result.ResolutionError == nil {
					total += result.ResponseTime
					successCount++

					// 记录 Ping 结果
					if len(dnsPingResult.PingResults) > 0 {
						if dnsPingResult.Error != nil {
							logger.Printf("DNS服务器：%s，域名：%s，解析IP：%v，Ping失败：%v\n",
								s, domain, result.Answers, dnsPingResult.Error)
						} else {
							logger.Printf("DNS服务器：%s，域名：%s，解析IP：%v，平均延迟：%v\n",
								s, domain, result.Answers, dnsPingResult.AvgRTT)
						}
					}
					if dnsPingResult.AvgRTT > 0 {
						pingTotal += dnsPingResult.AvgRTT
						pingSuccessCount++
					}
				}
			}

			actualTotalQueries := totalQueries + totalRetries
			successRate := float64(successCount) / float64(actualTotalQueries)

			var avgResponseTime time.Duration
			if successCount > 0 {
				avgResponseTime = total / time.Duration(successCount)
			}

			var avgPingRTT time.Duration
			if pingSuccessCount > 0 {
				avgPingRTT = pingTotal / time.Duration(pingSuccessCount)
			}

			mu.Lock()
			results = append(results, BenchmarkResult{
				Server:          s,
				AvgResponseTime: avgResponseTime,
				DomainResults:   domainResults,
				SuccessRate:     successRate,
				TotalRetries:    totalRetries,
				AvgPingRTT:      avgPingRTT,
			})
			mu.Unlock()
			logger.Printf("DNS服务器：%s，平均DNS响应时间：%s，平均解析IP延迟：%s，成功率：%.2f%%，总重试次数：%d\n",
				s, avgResponseTime, avgPingRTT, successRate*100, totalRetries)
		}(server)
	}
	wg.Wait()
	return results
}

func RunDNSPing(cfg config.Config, logger *log.Logger) []DNSPingBenchmarkResult {
	results := make([]DNSPingBenchmarkResult, 0, len(cfg.DNSServers))
	var wg sync.WaitGroup
	mu := &sync.Mutex{}
	pingOptions := ping.Options{
		Count:       cfg.Ping.Count,
		Interval:    cfg.Ping.Interval,
		Timeout:     cfg.Ping.Timeout,
		Privileged:  cfg.Ping.Privileged,
		IPSelection: cfg.Ping.IPSelection,
		IPFamily:    cfg.Ping.IPFamily,
	}

	for _, server := range cfg.DNSServers {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			target, err := pingTargetFromServer(s)
			result := DNSPingBenchmarkResult{
				Server: s,
				Target: target,
			}
			if err != nil {
				result.Error = err
			} else {
				pingResult := ping.PingIPWithOptions(target, pingOptions)
				result.RTT = pingResult.RTT
				result.PacketLoss = pingResult.PacketLoss
				result.PacketsSent = pingResult.PacketsSent
				result.Error = pingResult.Error
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if result.Error != nil {
				logger.Printf("DNS服务器：%s，Ping目标：%s，错误：%v\n", s, target, result.Error)
				return
			}
			logger.Printf("DNS服务器：%s，Ping目标：%s，平均延迟：%s，丢包率：%.2f%%\n",
				s, target, result.RTT, result.PacketLoss)
		}(server)
	}

	wg.Wait()
	return results
}

func SortResolvePingResults(results []BenchmarkResult) {
	sort.SliceStable(results, func(i, j int) bool {
		left := results[i]
		right := results[j]
		if left.AvgPingRTT != right.AvgPingRTT {
			if left.AvgPingRTT == 0 {
				return false
			}
			if right.AvgPingRTT == 0 {
				return true
			}
			return left.AvgPingRTT < right.AvgPingRTT
		}
		if left.AvgResponseTime != right.AvgResponseTime {
			return left.AvgResponseTime < right.AvgResponseTime
		}
		return left.SuccessRate > right.SuccessRate
	})
}

func SortDNSPingResults(results []DNSPingBenchmarkResult) {
	sort.SliceStable(results, func(i, j int) bool {
		left := results[i]
		right := results[j]
		if (left.Error == nil) != (right.Error == nil) {
			return left.Error == nil
		}
		if left.RTT != right.RTT {
			if left.RTT == 0 {
				return false
			}
			if right.RTT == 0 {
				return true
			}
			return left.RTT < right.RTT
		}
		return left.PacketLoss < right.PacketLoss
	})
}

func pingTargetFromServer(server string) (string, error) {
	address := server
	if strings.HasPrefix(address, "[/") {
		parts := strings.SplitN(address, "/]", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid domain-specific format")
		}
		address = parts[1]
	}

	if strings.Contains(address, "://") {
		parsed, err := url.Parse(address)
		if err != nil {
			return "", err
		}
		address = parsed.Host
	}

	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host, nil
	}

	if strings.Contains(err.Error(), "missing port in address") {
		return strings.Trim(address, "[]"), nil
	}

	return "", err
}

func dnsRecordTypes(recordTypes []string) []dns.RecordType {
	values := make([]dns.RecordType, 0, len(recordTypes))
	for _, recordType := range recordTypes {
		recordType = strings.ToUpper(strings.TrimSpace(recordType))
		if recordType == "" {
			continue
		}
		values = append(values, dns.RecordType(recordType))
	}
	return values
}

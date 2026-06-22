package benchmark

import (
	"GoFastDNS/internal/config"
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/geoip"
	"GoFastDNS/internal/ping"
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type resolveRunner func(context.Context, string, string, int, time.Duration, dns.ResolveOptions) dns.DNSResult
type ipPingRunner func(context.Context, string, ping.Options) ping.PingResult

func Run(cfg config.Config, logger *log.Logger) (string, error) {
	return RunContext(context.Background(), cfg, logger)
}

func RunContext(ctx context.Context, cfg config.Config, logger *log.Logger) (string, error) {
	config.ApplyDefaults(&cfg)
	outputFormat := strings.ToLower(cfg.Output.Format)

	switch cfg.Mode {
	case config.ModeResolvePing:
		results, err := RunResolvePingContext(ctx, cfg, logger)
		if err != nil {
			return "", err
		}
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
		results, err := RunDNSPingContext(ctx, cfg, logger)
		if err != nil {
			return "", err
		}
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
	results, _ := RunResolvePingContext(context.Background(), cfg, logger)
	return results
}

func RunResolvePingContext(ctx context.Context, cfg config.Config, logger *log.Logger) ([]BenchmarkResult, error) {
	return runResolvePingWithRunners(ctx, cfg, logger, dns.ResolveDNSWithOptionsContext, ping.PingIPWithOptionsContext)
}

func runResolvePingWithRunners(ctx context.Context, cfg config.Config, logger *log.Logger, resolver resolveRunner, pinger ipPingRunner) ([]BenchmarkResult, error) {
	config.ApplyDefaults(&cfg)
	lookup, closeLookup, err := openConfiguredGeoIPLookup(cfg.GeoIP)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeLookup()
	}()
	return runResolvePingWithRunnersAndGeoIP(ctx, cfg, logger, resolver, pinger, lookup)
}

func runResolvePingWithRunnersAndGeoIP(ctx context.Context, cfg config.Config, logger *log.Logger, resolver resolveRunner, pinger ipPingRunner, lookup geoip.Lookup) ([]BenchmarkResult, error) {
	config.ApplyDefaults(&cfg)
	var results []BenchmarkResult
	var wg sync.WaitGroup
	mu := &sync.Mutex{}
	geoAnnotator := newGeoIPAnnotator(lookup)
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
	pingSem := make(chan struct{}, cfg.Concurrency.Pings)
	serverSem := make(chan struct{}, cfg.Concurrency.Servers)
	errCh := make(chan error, 1)
	var loopErr error

	for _, server := range cfg.DNSServers {
		if err := ctx.Err(); err != nil {
			loopErr = err
			break
		}
		if err := acquire(ctx, serverSem); err != nil {
			loopErr = err
			break
		}
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			defer release(serverSem)

			domainResults := make([]DomainResult, 0, len(cfg.Domains)*cfg.Benchmark.Rounds)
			totalRetries := 0
			totalMeasured := len(cfg.Domains) * cfg.Benchmark.Rounds
			totalRuns := cfg.Benchmark.Warmup + cfg.Benchmark.Rounds

			for round := 1; round <= totalRuns; round++ {
				roundResults, err := runResolveRound(ctx, cfg, s, round, pingOptions, resolveOptions, resolver, pinger, pingSem, geoAnnotator, logger)
				if round <= cfg.Benchmark.Warmup {
					if err != nil {
						recordError(errCh, err)
						return
					}
					continue
				}
				domainResults = append(domainResults, roundResults...)
				for _, result := range roundResults {
					totalRetries += result.RetryCount
				}
				if err != nil {
					recordError(errCh, err)
					return
				}
			}

			result := buildBenchmarkResult(s, cfg.Benchmark.Rounds, cfg.Benchmark.Warmup, totalMeasured, domainResults, totalRetries)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
			logger.Printf("DNS服务器：%s，平均DNS响应时间：%s，平均解析IP延迟：%s，成功率：%.2f%%，总重试次数：%d\n",
				s, result.AvgResponseTime, result.AvgPingRTT, result.SuccessRate*100, totalRetries)
		}(server)
	}
	wg.Wait()
	applyResolveScores(results, cfg.Benchmark.Score)
	if loopErr != nil {
		return results, loopErr
	}
	return results, firstError(errCh, ctx.Err())
}

func RunDNSPing(cfg config.Config, logger *log.Logger) []DNSPingBenchmarkResult {
	results, _ := RunDNSPingContext(context.Background(), cfg, logger)
	return results
}

func RunDNSPingContext(ctx context.Context, cfg config.Config, logger *log.Logger) ([]DNSPingBenchmarkResult, error) {
	return runDNSPingWithRunner(ctx, cfg, logger, ping.PingIPWithOptionsContext)
}

func runDNSPingWithRunner(ctx context.Context, cfg config.Config, logger *log.Logger, pinger ipPingRunner) ([]DNSPingBenchmarkResult, error) {
	config.ApplyDefaults(&cfg)
	lookup, closeLookup, err := openConfiguredGeoIPLookup(cfg.GeoIP)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeLookup()
	}()
	return runDNSPingWithRunnerAndGeoIP(ctx, cfg, logger, pinger, lookup)
}

func runDNSPingWithRunnerAndGeoIP(ctx context.Context, cfg config.Config, logger *log.Logger, pinger ipPingRunner, lookup geoip.Lookup) ([]DNSPingBenchmarkResult, error) {
	config.ApplyDefaults(&cfg)
	results := make([]DNSPingBenchmarkResult, 0, len(cfg.DNSServers))
	var wg sync.WaitGroup
	mu := &sync.Mutex{}
	geoAnnotator := newGeoIPAnnotator(lookup)
	pingOptions := ping.Options{
		Count:       cfg.Ping.Count,
		Interval:    cfg.Ping.Interval,
		Timeout:     cfg.Ping.Timeout,
		Privileged:  cfg.Ping.Privileged,
		IPSelection: cfg.Ping.IPSelection,
		IPFamily:    cfg.Ping.IPFamily,
	}
	pingSem := make(chan struct{}, cfg.Concurrency.Pings)
	serverSem := make(chan struct{}, cfg.Concurrency.Servers)
	errCh := make(chan error, 1)
	var loopErr error

	for _, server := range cfg.DNSServers {
		if err := ctx.Err(); err != nil {
			loopErr = err
			break
		}
		if err := acquire(ctx, serverSem); err != nil {
			loopErr = err
			break
		}
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			defer release(serverSem)

			target, err := pingTargetFromServer(s)
			result := DNSPingBenchmarkResult{
				Server: s,
				Target: target,
				Rounds: cfg.Benchmark.Rounds,
				Warmup: cfg.Benchmark.Warmup,
			}
			if err != nil {
				result.Error = err
			} else {
				result = runDNSPingTarget(ctx, result, cfg.Benchmark.Rounds, cfg.Benchmark.Warmup, target, pingOptions, pinger, pingSem)
			}
			result.TargetGeoIP = geoAnnotator.lookupIP(target)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if result.Error != nil {
				logger.Printf("DNS服务器：%s，Ping目标：%s，错误：%v\n", s, target, result.Error)
				if err := ctx.Err(); err != nil {
					recordError(errCh, err)
				}
				return
			}
			logger.Printf("DNS服务器：%s，Ping目标：%s，平均延迟：%s，丢包率：%.2f%%\n",
				s, target, result.RTT, result.PacketLoss)
		}(server)
	}

	wg.Wait()
	applyDNSPingScores(results, cfg.Benchmark.Score)
	if loopErr != nil {
		return results, loopErr
	}
	return results, firstError(errCh, ctx.Err())
}

func runResolveRound(ctx context.Context, cfg config.Config, server string, round int, pingOptions ping.Options, resolveOptions dns.ResolveOptions, resolver resolveRunner, pinger ipPingRunner, pingSem chan struct{}, geoAnnotator *geoIPAnnotator, logger *log.Logger) ([]DomainResult, error) {
	results := make([]DomainResult, 0, len(cfg.Domains))
	var wg sync.WaitGroup
	mu := &sync.Mutex{}
	domainSem := make(chan struct{}, cfg.Concurrency.Domains)
	errCh := make(chan error, 1)
	var loopErr error

	for _, domain := range cfg.Domains {
		if err := ctx.Err(); err != nil {
			loopErr = err
			break
		}
		if err := acquire(ctx, domainSem); err != nil {
			loopErr = err
			break
		}

		wg.Add(1)
		go func(domain string) {
			defer wg.Done()
			defer release(domainSem)

			result := resolver(ctx, server, domain, cfg.Attempts, cfg.Timeout, resolveOptions)
			result.Answers = geoAnnotator.annotateAnswers(result.Answers)
			dnsPingResult := ping.PingDNSResultWithOptionsAndRunner(ctx, result, pingOptions, func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
				if err := acquire(ctx, pingSem); err != nil {
					return ping.PingResult{IP: ip, Error: err}
				}
				defer release(pingSem)
				return pinger(ctx, ip, options)
			})
			dnsPingResult.PingResults = geoAnnotator.annotatePingResults(dnsPingResult.PingResults)

			domainResult := DomainResult{
				Round:          round - cfg.Benchmark.Warmup,
				Domain:         domain,
				Answers:        result.Answers,
				ResponseCodes:  result.ResponseCodes,
				ResponseTime:   result.ResponseTime,
				Error:          result.ResolutionError,
				QueryErrors:    result.QueryErrors,
				NoAnswer:       result.NoAnswer,
				RetryCount:     result.RetryCount,
				DnsPingResults: dnsPingResult,
			}

			if len(dnsPingResult.PingResults) > 0 {
				if dnsPingResult.Error != nil {
					logger.Printf("DNS服务器：%s，域名：%s，第%d轮，解析IP：%v，Ping失败：%v\n",
						server, domain, round, result.Answers, dnsPingResult.Error)
				} else {
					logger.Printf("DNS服务器：%s，域名：%s，第%d轮，解析IP：%v，平均延迟：%v\n",
						server, domain, round, result.Answers, dnsPingResult.AvgRTT)
				}
			}
			if err := ctx.Err(); err != nil {
				recordError(errCh, err)
			}

			mu.Lock()
			results = append(results, domainResult)
			mu.Unlock()
		}(domain)
	}

	wg.Wait()
	if loopErr != nil {
		return results, loopErr
	}
	return results, firstError(errCh, ctx.Err())
}

func buildBenchmarkResult(server string, rounds, warmup, totalMeasured int, domainResults []DomainResult, totalRetries int) BenchmarkResult {
	dnsDurations := make([]time.Duration, 0, len(domainResults))
	pingDurations := make([]time.Duration, 0, len(domainResults))
	dnsSuccessCount := 0
	pingSuccessCount := 0

	for _, result := range domainResults {
		if result.Error == nil {
			dnsSuccessCount++
			if result.ResponseTime > 0 {
				dnsDurations = append(dnsDurations, result.ResponseTime)
			}
		}
		if result.DnsPingResults.AvgRTT > 0 {
			pingSuccessCount++
			pingDurations = append(pingDurations, result.DnsPingResults.AvgRTT)
		}
	}

	dnsStats := calculateDurationStats(dnsDurations)
	pingStats := calculateDurationStats(pingDurations)
	dnsSuccessRate := ratio(dnsSuccessCount, totalMeasured)
	pingSuccessRate := ratio(pingSuccessCount, totalMeasured)
	return BenchmarkResult{
		Server:          server,
		Rounds:          rounds,
		Warmup:          warmup,
		AvgResponseTime: dnsStats.Avg,
		DomainResults:   domainResults,
		SuccessRate:     dnsSuccessRate,
		DNSSuccessRate:  dnsSuccessRate,
		PingSuccessRate: pingSuccessRate,
		TotalRetries:    totalRetries,
		AvgPingRTT:      pingStats.Avg,
		DNSStats:        dnsStats,
		PingStats:       pingStats,
	}
}

func runDNSPingTarget(ctx context.Context, result DNSPingBenchmarkResult, rounds, warmup int, target string, options ping.Options, pinger ipPingRunner, pingSem chan struct{}) DNSPingBenchmarkResult {
	totalRuns := warmup + rounds
	roundResults := make([]DNSPingRoundResult, 0, rounds)
	rtts := make([]time.Duration, 0, rounds)
	successCount := 0
	var totalLoss float64
	var totalPackets int
	var lastErr error

	for round := 1; round <= totalRuns; round++ {
		if err := acquire(ctx, pingSem); err != nil {
			result.Error = err
			return result
		}
		pingResult := pinger(ctx, target, options)
		release(pingSem)
		if round <= warmup {
			if err := ctx.Err(); err != nil {
				result.Error = err
				return result
			}
			continue
		}

		roundResult := DNSPingRoundResult{
			Round:       round - warmup,
			RTT:         pingResult.RTT,
			PacketLoss:  pingResult.PacketLoss,
			PacketsSent: pingResult.PacketsSent,
			Error:       pingResult.Error,
		}
		roundResults = append(roundResults, roundResult)
		if pingResult.Error != nil {
			lastErr = pingResult.Error
			if err := ctx.Err(); err != nil {
				result.RoundResults = roundResults
				result.Error = err
				return result
			}
			continue
		}

		successCount++
		rtts = append(rtts, pingResult.RTT)
		totalLoss += pingResult.PacketLoss
		totalPackets += pingResult.PacketsSent
	}

	stats := calculateDurationStats(rtts)
	result.RTT = stats.Avg
	result.Stats = stats
	result.RoundResults = roundResults
	result.SuccessRate = ratio(successCount, rounds)
	if successCount > 0 {
		result.PacketLoss = totalLoss / float64(successCount)
		result.PacketsSent = totalPackets / successCount
	} else if lastErr != nil {
		result.Error = lastErr
	}
	return result
}

func acquire(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func release(sem chan struct{}) {
	<-sem
}

func ratio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func firstError(errCh <-chan error, fallback error) error {
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	default:
	}
	return fallback
}

func recordError(errCh chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case errCh <- err:
	default:
	}
}

func SortResolvePingResults(results []BenchmarkResult) {
	sort.SliceStable(results, func(i, j int) bool {
		left := results[i]
		right := results[j]
		if (left.PingSuccessRate > 0) != (right.PingSuccessRate > 0) {
			return left.PingSuccessRate > 0
		}
		if (left.DNSSuccessRate > 0) != (right.DNSSuccessRate > 0) {
			return left.DNSSuccessRate > 0
		}
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.PingSuccessRate != right.PingSuccessRate {
			return left.PingSuccessRate > right.PingSuccessRate
		}
		if left.DNSSuccessRate != right.DNSSuccessRate {
			return left.DNSSuccessRate > right.DNSSuccessRate
		}
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
		if (left.SuccessRate > 0) != (right.SuccessRate > 0) {
			return left.SuccessRate > 0
		}
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.SuccessRate != right.SuccessRate {
			return left.SuccessRate > right.SuccessRate
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

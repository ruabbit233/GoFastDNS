package benchmark

import (
	"GoFastDNS/internal/config"
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/geoip"
	"GoFastDNS/internal/ping"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeGeoLookup struct {
	calls atomic.Int64
}

func (f *fakeGeoLookup) Lookup(ip string) (*geoip.Info, error) {
	f.calls.Add(1)
	switch ip {
	case "192.0.2.1":
		return &geoip.Info{
			IP:          ip,
			Provider:    "fake",
			CountryName: "Testland",
			Region:      "Test Region",
			City:        "Test City",
			ASN:         "64500",
			ASName:      "Example AS",
			ISP:         "Example ISP",
		}, nil
	default:
		return &geoip.Info{IP: ip, Provider: "fake"}, nil
	}
}

func logDiscard() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func updateMax(max *atomic.Int64, value int64) {
	for {
		current := max.Load()
		if value <= current {
			return
		}
		if max.CompareAndSwap(current, value) {
			return
		}
	}
}

func TestPingTargetFromServer(t *testing.T) {
	tests := []struct {
		name   string
		server string
		want   string
	}{
		{
			name:   "udp scheme",
			server: "udp://8.8.8.8",
			want:   "8.8.8.8",
		},
		{
			name:   "tls hostname",
			server: "tls://dns.google",
			want:   "dns.google",
		},
		{
			name:   "host port",
			server: "1.1.1.1:5353",
			want:   "1.1.1.1",
		},
		{
			name:   "plain host",
			server: "223.5.5.5",
			want:   "223.5.5.5",
		},
		{
			name:   "ipv6 host",
			server: "[2001:4860:4860::8888]",
			want:   "2001:4860:4860::8888",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pingTargetFromServer(tt.server)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestSortDNSPingResults(t *testing.T) {
	results := []DNSPingBenchmarkResult{
		{Server: "failed", Error: errors.New("failed")},
		{Server: "slow", RTT: 20 * time.Millisecond},
		{Server: "fast", RTT: 10 * time.Millisecond},
	}

	SortDNSPingResults(results)

	if results[0].Server != "fast" {
		t.Fatalf("expected fast first, got %q", results[0].Server)
	}
	if results[len(results)-1].Server != "failed" {
		t.Fatalf("expected failed last, got %q", results[len(results)-1].Server)
	}
}

func TestSortResolvePingResults(t *testing.T) {
	results := []BenchmarkResult{
		{Server: "no-ping", AvgResponseTime: 1 * time.Millisecond, SuccessRate: 1},
		{Server: "slow-cdn", AvgPingRTT: 30 * time.Millisecond, AvgResponseTime: 1 * time.Millisecond, SuccessRate: 1},
		{Server: "fast-cdn", AvgPingRTT: 10 * time.Millisecond, AvgResponseTime: 2 * time.Millisecond, SuccessRate: 1},
	}

	SortResolvePingResults(results)

	if results[0].Server != "fast-cdn" {
		t.Fatalf("expected fast-cdn first, got %q", results[0].Server)
	}
	if results[len(results)-1].Server != "no-ping" {
		t.Fatalf("expected no-ping last, got %q", results[len(results)-1].Server)
	}
}

func TestCalculateDurationStats(t *testing.T) {
	stats := calculateDurationStats([]time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		80 * time.Millisecond,
	})

	if stats.Min != 10*time.Millisecond {
		t.Fatalf("expected min 10ms, got %s", stats.Min)
	}
	if stats.Max != 80*time.Millisecond {
		t.Fatalf("expected max 80ms, got %s", stats.Max)
	}
	if stats.Avg != 37*time.Millisecond+500*time.Microsecond {
		t.Fatalf("expected avg 37.5ms, got %s", stats.Avg)
	}
	if stats.Median != 30*time.Millisecond {
		t.Fatalf("expected median 30ms, got %s", stats.Median)
	}
	if stats.P95 != 80*time.Millisecond {
		t.Fatalf("expected p95 80ms, got %s", stats.P95)
	}
	if stats.Jitter != 22*time.Millisecond+500*time.Microsecond {
		t.Fatalf("expected jitter average deviation, got %s", stats.Jitter)
	}
}

func TestRunResolvePingMultiRoundStats(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DNSServers = []string{"udp://resolver"}
	cfg.Domains = []string{"example.com", "example.org"}
	cfg.Benchmark.Rounds = 2
	cfg.Benchmark.Warmup = 1
	cfg.Concurrency.Servers = 1
	cfg.Concurrency.Domains = 2
	cfg.Concurrency.Pings = 2

	var calls atomic.Int64
	resolver := func(ctx context.Context, server, domain string, attempts int, timeout time.Duration, options dns.ResolveOptions) dns.DNSResult {
		call := calls.Add(1)
		return dns.DNSResult{
			Server:       server,
			Domain:       domain,
			ResponseTime: time.Duration(call) * time.Millisecond,
			Answers: []dns.Answer{
				{Type: "A", Value: "192.0.2.1"},
			},
		}
	}
	pinger := func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
		return ping.PingResult{IP: ip, RTT: 10 * time.Millisecond, PacketsSent: 1}
	}

	results, err := runResolvePingWithRunners(context.Background(), cfg, logDiscard(), resolver, pinger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one server result, got %d", len(results))
	}
	result := results[0]
	if len(result.DomainResults) != 4 {
		t.Fatalf("expected warmup to be excluded from 4 measured domain results, got %d", len(result.DomainResults))
	}
	if calls.Load() != 6 {
		t.Fatalf("expected 6 resolver calls including warmup, got %d", calls.Load())
	}
	if result.Rounds != 2 || result.Warmup != 1 {
		t.Fatalf("unexpected rounds/warmup: rounds=%d warmup=%d", result.Rounds, result.Warmup)
	}
	if result.DNSSuccessRate != 1 || result.PingSuccessRate != 1 {
		t.Fatalf("expected full success rates, got dns=%.2f ping=%.2f", result.DNSSuccessRate, result.PingSuccessRate)
	}
	if result.DNSStats.Count != 4 || result.PingStats.Count != 4 {
		t.Fatalf("unexpected stats counts: dns=%d ping=%d", result.DNSStats.Count, result.PingStats.Count)
	}
	if result.AvgPingRTT != 10*time.Millisecond {
		t.Fatalf("expected avg ping 10ms, got %s", result.AvgPingRTT)
	}
}

func TestRunResolvePingTreatsZeroRTTAsPingFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DNSServers = []string{"udp://resolver"}
	cfg.Domains = []string{"example.com"}
	cfg.Benchmark.Rounds = 1
	cfg.Benchmark.Warmup = 0
	cfg.Concurrency.Servers = 1
	cfg.Concurrency.Domains = 1
	cfg.Concurrency.Pings = 1

	resolver := func(ctx context.Context, server, domain string, attempts int, timeout time.Duration, options dns.ResolveOptions) dns.DNSResult {
		return dns.DNSResult{
			Server:       server,
			Domain:       domain,
			ResponseTime: time.Millisecond,
			Answers:      []dns.Answer{{Type: "A", Value: "192.0.2.1"}},
		}
	}
	pinger := func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
		return ping.PingResult{IP: ip, PacketLoss: 100, PacketsSent: 3}
	}

	results, err := runResolvePingWithRunners(context.Background(), cfg, logDiscard(), resolver, pinger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || len(results[0].DomainResults) != 1 {
		t.Fatalf("unexpected results: %#v", results)
	}
	result := results[0]
	if result.PingSuccessRate != 0 {
		t.Fatalf("expected ping success rate 0, got %.2f", result.PingSuccessRate)
	}
	if result.AvgPingRTT != 0 || result.PingStats.Count != 0 {
		t.Fatalf("expected failed ping to be excluded from latency stats, avg=%s count=%d", result.AvgPingRTT, result.PingStats.Count)
	}
	domainPing := result.DomainResults[0].DnsPingResults
	if domainPing.Error == nil {
		t.Fatal("expected domain ping error for zero RTT")
	}
	if len(domainPing.PingResults) != 1 || domainPing.PingResults[0].Error == nil {
		t.Fatalf("expected ping target error, got %#v", domainPing.PingResults)
	}
}

func TestRunResolvePingKeepsResolverTimeoutAsResultError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DNSServers = []string{"tls://dns.google"}
	cfg.Domains = []string{"example.com"}
	cfg.Benchmark.Rounds = 1
	cfg.Benchmark.Warmup = 0
	cfg.Concurrency.Servers = 1
	cfg.Concurrency.Domains = 1
	cfg.Concurrency.Pings = 1

	timeoutErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: os.ErrDeadlineExceeded,
	}
	resolver := func(ctx context.Context, server, domain string, attempts int, timeout time.Duration, options dns.ResolveOptions) dns.DNSResult {
		return dns.DNSResult{
			Server:          server,
			Domain:          domain,
			ResolutionError: timeoutErr,
			QueryErrors:     []string{timeoutErr.Error()},
			Answers:         []dns.Answer{},
		}
	}
	pinger := func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
		t.Fatalf("pinger should not run when resolution fails")
		return ping.PingResult{}
	}

	results, err := runResolvePingWithRunners(context.Background(), cfg, logDiscard(), resolver, pinger)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one server result, got %d", len(results))
	}
	if len(results[0].DomainResults) != 1 {
		t.Fatalf("expected one domain result, got %d", len(results[0].DomainResults))
	}
	if !errors.Is(results[0].DomainResults[0].Error, os.ErrDeadlineExceeded) {
		t.Fatalf("expected resolver timeout to stay on domain result, got %v", results[0].DomainResults[0].Error)
	}
	if results[0].DNSSuccessRate != 0 {
		t.Fatalf("expected DNS success rate 0, got %.2f", results[0].DNSSuccessRate)
	}
}

func TestRunDNSPingTargetTreatsZeroRTTAsFailure(t *testing.T) {
	result := DNSPingBenchmarkResult{
		Server: "udp://192.0.2.1",
		Target: "192.0.2.1",
		Rounds: 1,
	}
	pingSem := make(chan struct{}, 1)
	pinger := func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
		return ping.PingResult{IP: ip, PacketLoss: 100, PacketsSent: 3}
	}

	result = runDNSPingTarget(context.Background(), result, 1, 0, "192.0.2.1", ping.Options{}, pinger, pingSem)

	if result.Error == nil {
		t.Fatal("expected dns-ping target error for zero RTT")
	}
	if result.SuccessRate != 0 {
		t.Fatalf("expected success rate 0, got %.2f", result.SuccessRate)
	}
	if result.RTT != 0 || result.Stats.Count != 0 {
		t.Fatalf("expected failed ping to be excluded from latency stats, rtt=%s count=%d", result.RTT, result.Stats.Count)
	}
	if result.PacketLoss != 100 || result.PacketsSent != 3 {
		t.Fatalf("expected failed ping packet stats to be preserved, loss=%.1f sent=%d", result.PacketLoss, result.PacketsSent)
	}
	if len(result.RoundResults) != 1 || result.RoundResults[0].Error == nil {
		t.Fatalf("expected round error, got %#v", result.RoundResults)
	}
}

func TestRunResolvePingHonorsConcurrencyLimits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DNSServers = []string{"udp://one", "udp://two"}
	cfg.Domains = []string{"a.example", "b.example", "c.example"}
	cfg.Benchmark.Rounds = 1
	cfg.Concurrency.Servers = 1
	cfg.Concurrency.Domains = 2
	cfg.Concurrency.Pings = 1

	var activeResolvers atomic.Int64
	var maxResolvers atomic.Int64
	var activePings atomic.Int64
	var maxPings atomic.Int64
	resolver := func(ctx context.Context, server, domain string, attempts int, timeout time.Duration, options dns.ResolveOptions) dns.DNSResult {
		current := activeResolvers.Add(1)
		updateMax(&maxResolvers, current)
		time.Sleep(5 * time.Millisecond)
		activeResolvers.Add(-1)
		return dns.DNSResult{
			Server:       server,
			Domain:       domain,
			ResponseTime: time.Millisecond,
			Answers:      []dns.Answer{{Type: "A", Value: "192.0.2.1"}},
		}
	}
	pinger := func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
		current := activePings.Add(1)
		updateMax(&maxPings, current)
		time.Sleep(5 * time.Millisecond)
		activePings.Add(-1)
		return ping.PingResult{IP: ip, RTT: time.Millisecond, PacketsSent: 1}
	}

	results, err := runResolvePingWithRunners(context.Background(), cfg, logDiscard(), resolver, pinger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected two server results, got %d", len(results))
	}
	if got := maxResolvers.Load(); got > 2 {
		t.Fatalf("expected domain concurrency <= 2, got %d", got)
	}
	if got := maxPings.Load(); got > 1 {
		t.Fatalf("expected ping concurrency <= 1, got %d", got)
	}
}

func TestRunResolvePingAnnotatesGeoIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DNSServers = []string{"udp://resolver"}
	cfg.Domains = []string{"example.com"}
	cfg.Benchmark.Rounds = 2
	cfg.Concurrency.Servers = 1
	cfg.Concurrency.Domains = 1
	cfg.Concurrency.Pings = 1

	resolver := func(ctx context.Context, server, domain string, attempts int, timeout time.Duration, options dns.ResolveOptions) dns.DNSResult {
		return dns.DNSResult{
			Server:       server,
			Domain:       domain,
			ResponseTime: time.Millisecond,
			Answers:      []dns.Answer{{Name: domain, Type: "A", Value: "192.0.2.1", TTL: 60, Family: "ipv4"}},
		}
	}
	pinger := func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
		return ping.PingResult{IP: ip, RTT: time.Millisecond, PacketsSent: 1}
	}
	lookup := &fakeGeoLookup{}

	results, err := runResolvePingWithRunnersAndGeoIP(context.Background(), cfg, logDiscard(), resolver, pinger, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || len(results[0].DomainResults) != 2 {
		t.Fatalf("unexpected results: %#v", results)
	}
	for _, domain := range results[0].DomainResults {
		if domain.Answers[0].GeoIP == nil || domain.Answers[0].GeoIP.ASN != "64500" {
			t.Fatalf("expected answer GeoIP annotation, got %#v", domain.Answers[0].GeoIP)
		}
		if domain.DnsPingResults.PingResults[0].GeoIP == nil || domain.DnsPingResults.PingResults[0].GeoIP.CountryName != "Testland" {
			t.Fatalf("expected ping GeoIP annotation, got %#v", domain.DnsPingResults.PingResults[0].GeoIP)
		}
	}
	if lookup.calls.Load() != 1 {
		t.Fatalf("expected GeoIP lookup to be cached, got %d calls", lookup.calls.Load())
	}
}

func TestRunDNSPingAnnotatesTargetGeoIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeDNSPing
	cfg.DNSServers = []string{"udp://192.0.2.1"}
	cfg.Benchmark.Rounds = 1
	cfg.Concurrency.Servers = 1
	cfg.Concurrency.Pings = 1

	pinger := func(ctx context.Context, ip string, options ping.Options) ping.PingResult {
		return ping.PingResult{IP: ip, RTT: time.Millisecond, PacketsSent: 1}
	}
	lookup := &fakeGeoLookup{}

	results, err := runDNSPingWithRunnerAndGeoIP(context.Background(), cfg, logDiscard(), pinger, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0].TargetGeoIP == nil || results[0].TargetGeoIP.ASName != "Example AS" {
		t.Fatalf("expected target GeoIP annotation, got %#v", results[0].TargetGeoIP)
	}
}

func TestSaveResultsToHTML(t *testing.T) {
	results := []BenchmarkResult{
		{
			Server:          "udp://8.8.8.8",
			AvgResponseTime: 12 * time.Millisecond,
			SuccessRate:     1,
			AvgPingRTT:      34 * time.Millisecond,
			DomainResults: []DomainResult{
				{
					Domain: "example.com",
					Answers: []dns.Answer{
						{Name: "example.com", Type: "A", Value: "93.184.216.34", TTL: 300, Family: "ipv4"},
					},
					ResponseTime: 12 * time.Millisecond,
					DnsPingResults: ping.DNSPingResult{
						AvgRTT: 34 * time.Millisecond,
						PingResults: []ping.PingResult{
							{IP: "93.184.216.34", RTT: 34 * time.Millisecond},
						},
					},
				},
			},
		},
		{
			Server:          "<script>alert(1)</script>",
			AvgResponseTime: 50 * time.Millisecond,
			SuccessRate:     0.5,
			AvgPingRTT:      80 * time.Millisecond,
			TotalRetries:    1,
		},
	}

	filename, err := SaveResultsToHTML(results, t.TempDir())
	if err != nil {
		t.Fatalf("SaveResultsToHTML returned error: %v", err)
	}
	if filepath.Ext(filename) != ".html" {
		t.Fatalf("expected .html output, got %q", filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read html output: %v", err)
	}
	html := string(content)
	if !strings.Contains(html, "DNS 解析与 CDN 延迟排行") {
		t.Fatal("expected resolve-ping summary section")
	}
	if !strings.Contains(html, "example.com") {
		t.Fatal("expected domain details")
	}
	if !strings.Contains(html, "A 93.184.216.34 TTL=300") {
		t.Fatal("expected structured DNS answer label")
	}
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatal("expected server value to be HTML-escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("expected escaped server value")
	}
	if strings.Contains(html, "ZgotmplZ") {
		t.Fatal("expected CSS bar widths to render without template filtering")
	}
}

func TestSaveDNSPingResultsToHTMLUsesExplicitFilePath(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "dns-report.html")
	results := []DNSPingBenchmarkResult{
		{
			Server:      "udp://1.1.1.1",
			Target:      "1.1.1.1",
			RTT:         10 * time.Millisecond,
			PacketLoss:  25,
			PacketsSent: 4,
		},
	}

	filename, err := SaveDNSPingResultsToHTML(results, outputPath)
	if err != nil {
		t.Fatalf("SaveDNSPingResultsToHTML returned error: %v", err)
	}
	if filename != outputPath {
		t.Fatalf("expected explicit output path %q, got %q", outputPath, filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read html output: %v", err)
	}
	html := string(content)
	if !strings.Contains(html, "DNS 节点 Ping 排行") {
		t.Fatal("expected dns-ping summary section")
	}
	if !strings.Contains(html, "25.0%") {
		t.Fatal("expected packet loss percentage")
	}
}

func TestSaveResolvePingResultsToJSON(t *testing.T) {
	results := []BenchmarkResult{
		{
			Server:          "<script>",
			AvgResponseTime: 12*time.Millisecond + 500*time.Microsecond,
			SuccessRate:     1,
			AvgPingRTT:      34 * time.Millisecond,
			DomainResults: []DomainResult{
				{
					Domain:       "example.com",
					ResponseTime: 12*time.Millisecond + 500*time.Microsecond,
					Answers: []dns.Answer{
						{Name: "example.com", Type: "CNAME", Value: "edge.example.com", TTL: 60},
						{Name: "edge.example.com", Type: "A", Value: "93.184.216.34", TTL: 300, Family: "ipv4"},
					},
					ResponseCodes: []dns.ResponseCode{{RecordType: "A", Code: 0, Name: "NOERROR"}},
					DnsPingResults: ping.DNSPingResult{
						AvgRTT: 34 * time.Millisecond,
						PingResults: []ping.PingResult{
							{IP: "93.184.216.34", RTT: 34 * time.Millisecond, PacketsSent: 3},
						},
					},
				},
				{
					Domain:      "failed.example",
					Error:       errors.New("escaped <error>"),
					QueryErrors: []string{"escaped <error>"},
				},
				{
					Domain:   "cname-only.example",
					NoAnswer: true,
					Answers: []dns.Answer{
						{Name: "cname-only.example", Type: "CNAME", Value: "edge.example.com", TTL: 30},
					},
				},
			},
		},
	}

	filename, err := SaveResolvePingResultsToJSON(results, t.TempDir())
	if err != nil {
		t.Fatalf("SaveResolvePingResultsToJSON returned error: %v", err)
	}
	if filepath.Ext(filename) != ".json" {
		t.Fatalf("expected .json output, got %q", filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read json output: %v", err)
	}
	if !json.Valid(content) {
		t.Fatalf("expected valid JSON, got %s", content)
	}
	jsonText := string(content)
	for _, want := range []string{
		`"mode": "resolve-ping"`,
		`"avg_response_time_ms": 12.5`,
		`"dns_success_rate"`,
		`"ping_success_rate"`,
		`"dns_stats"`,
		`"p95_ms"`,
		`"type": "CNAME"`,
		`"ping_targets"`,
		`"no_answer": true`,
		`"escaped \u003cerror\u003e"`,
		`"answers": []`,
	} {
		if !strings.Contains(jsonText, want) {
			t.Fatalf("expected JSON to contain %s\n%s", want, jsonText)
		}
	}
}

func TestSaveDNSPingResultsToJSONUsesExplicitFilePath(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "dns-report.json")
	results := []DNSPingBenchmarkResult{
		{
			Server:      "udp://1.1.1.1",
			Target:      "1.1.1.1",
			RTT:         10 * time.Millisecond,
			PacketLoss:  25,
			PacketsSent: 4,
		},
		{
			Server: "bad",
			Error:  errors.New("bad <target>"),
		},
	}

	filename, err := SaveDNSPingResultsToJSON(results, outputPath)
	if err != nil {
		t.Fatalf("SaveDNSPingResultsToJSON returned error: %v", err)
	}
	if filename != outputPath {
		t.Fatalf("expected explicit output path %q, got %q", outputPath, filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read json output: %v", err)
	}
	jsonText := string(content)
	if !strings.Contains(jsonText, `"mode": "dns-ping"`) {
		t.Fatal("expected dns-ping mode")
	}
	if !strings.Contains(jsonText, `"successful_count": 1`) {
		t.Fatal("expected successful count summary")
	}
	if !strings.Contains(jsonText, `bad \u003ctarget\u003e`) {
		t.Fatal("expected JSON escaped error text")
	}
}

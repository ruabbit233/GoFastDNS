package benchmark

import (
	"GoFastDNS/internal/config"
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/ping"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

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

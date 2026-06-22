package benchmark

import (
	"GoFastDNS/internal/ping"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestSaveResultsToHTML(t *testing.T) {
	results := []BenchmarkResult{
		{
			Server:          "udp://8.8.8.8",
			AvgResponseTime: 12 * time.Millisecond,
			SuccessRate:     1,
			AvgPingRTT:      34 * time.Millisecond,
			DomainResults: []DomainResult{
				{
					Domain:       "example.com",
					Answers:      []string{"93.184.216.34"},
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

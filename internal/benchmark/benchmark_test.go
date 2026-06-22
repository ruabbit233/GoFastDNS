package benchmark

import (
	"errors"
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

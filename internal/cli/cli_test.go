package cli

import (
	"GoFastDNS/internal/config"
	"strings"
	"testing"
	"time"
)

func TestApplyFlagOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	opts := flagOptions{
		mode:        "dns-ping",
		dnsServers:  "udp://8.8.8.8, tls://dns.google",
		domains:     "example.com, example.org",
		recordTypes: "A, AAAA",
		attempts:    4,
		timeout:     1500 * time.Millisecond,
		pingCount:   5,
		pingIntv:    200 * time.Millisecond,
		pingTime:    3 * time.Second,
		pingPriv:    true,
		pingMethod:  "tcp",
		tcpPort:     8443,
		ipSelect:    "first",
		ipFamily:    "dual",
		rounds:      3,
		warmup:      1,
		dnsWeight:   0.2,
		pingWeight:  0.7,
		succWeight:  0.1,
		concServers: 2,
		concDomains: 8,
		concPings:   12,
		geoEnabled:  true,
		geoProvider: "ip2location",
		geoDB:       "./geo.bin",
		geoASNDB:    "./asn.bin",
		rawFlags: []string{
			"-ping-privileged",
			"-ping-method=tcp",
			"-tcp-port=8443",
			"-rounds=3",
			"-warmup=1",
			"-score-dns-weight=0.2",
			"-score-ping-weight=0.7",
			"-score-success-weight=0.1",
			"-concurrency-servers=2",
			"-concurrency-domains=8",
			"-concurrency-pings=12",
			"-geoip-enabled",
		},
		outputPath: "results",
		outputFmt:  "json",
	}

	applyFlagOverrides(&cfg, opts)

	if cfg.Mode != config.ModeDNSPing {
		t.Fatalf("expected dns-ping mode, got %q", cfg.Mode)
	}
	if len(cfg.DNSServers) != 2 || cfg.DNSServers[1] != "tls://dns.google" {
		t.Fatalf("unexpected dns servers: %#v", cfg.DNSServers)
	}
	if len(cfg.Domains) != 2 || cfg.Domains[0] != "example.com" {
		t.Fatalf("unexpected domains: %#v", cfg.Domains)
	}
	if len(cfg.DNS.RecordTypes) != 2 || cfg.DNS.RecordTypes[1] != "AAAA" {
		t.Fatalf("unexpected record types: %#v", cfg.DNS.RecordTypes)
	}
	if cfg.Attempts != 4 {
		t.Fatalf("expected attempts=4, got %d", cfg.Attempts)
	}
	if cfg.Timeout != 1500*time.Millisecond {
		t.Fatalf("expected timeout=1500ms, got %s", cfg.Timeout)
	}
	if cfg.Ping.Count != 5 {
		t.Fatalf("expected ping count=5, got %d", cfg.Ping.Count)
	}
	if cfg.Ping.Interval != 200*time.Millisecond {
		t.Fatalf("expected ping interval=200ms, got %s", cfg.Ping.Interval)
	}
	if cfg.Ping.Timeout != 3*time.Second {
		t.Fatalf("expected ping timeout=3s, got %s", cfg.Ping.Timeout)
	}
	if !cfg.Ping.Privileged {
		t.Fatal("expected privileged ping to be enabled")
	}
	if cfg.Ping.Method != "tcp" {
		t.Fatalf("expected ping method=tcp, got %q", cfg.Ping.Method)
	}
	if cfg.Ping.TCPPort != 8443 {
		t.Fatalf("expected tcp port=8443, got %d", cfg.Ping.TCPPort)
	}
	if cfg.Ping.IPSelection != "first" {
		t.Fatalf("expected ip selection=first, got %q", cfg.Ping.IPSelection)
	}
	if cfg.Ping.IPFamily != "dual" {
		t.Fatalf("expected ip family=dual, got %q", cfg.Ping.IPFamily)
	}
	if cfg.Benchmark.Rounds != 3 || cfg.Benchmark.Warmup != 1 {
		t.Fatalf("unexpected benchmark rounds/warmup: %#v", cfg.Benchmark)
	}
	if cfg.Benchmark.Score.DNSWeight != 0.2 || cfg.Benchmark.Score.PingWeight != 0.7 || cfg.Benchmark.Score.SuccessWeight != 0.1 {
		t.Fatalf("unexpected score weights: %#v", cfg.Benchmark.Score)
	}
	if cfg.Concurrency.Servers != 2 || cfg.Concurrency.Domains != 8 || cfg.Concurrency.Pings != 12 {
		t.Fatalf("unexpected concurrency: %#v", cfg.Concurrency)
	}
	if !cfg.GeoIP.Enabled || cfg.GeoIP.Provider != "ip2location" || cfg.GeoIP.DatabasePath != "./geo.bin" || cfg.GeoIP.ASNDatabasePath != "./asn.bin" {
		t.Fatalf("unexpected geoip config: %#v", cfg.GeoIP)
	}
	if cfg.Output.Path != "results" {
		t.Fatalf("expected output path results, got %q", cfg.Output.Path)
	}
	if cfg.Output.Format != "json" {
		t.Fatalf("expected output format json, got %q", cfg.Output.Format)
	}
}

func TestRunRejectsInvalidRoundFlag(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder

	code := RunWithWriters([]string{
		"-mode", "dns-ping",
		"-dns", "udp://8.8.8.8",
		"-rounds=0",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected config error exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "benchmark.rounds") {
		t.Fatalf("expected benchmark.rounds error, got %q", stderr.String())
	}
}

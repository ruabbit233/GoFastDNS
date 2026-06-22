package cli

import (
	"GoFastDNS/internal/config"
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
		ipSelect:    "first",
		ipFamily:    "dual",
		rawFlags:    []string{"-ping-privileged"},
		outputPath:  "results",
		outputFmt:   "json",
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
	if cfg.Ping.IPSelection != "first" {
		t.Fatalf("expected ip selection=first, got %q", cfg.Ping.IPSelection)
	}
	if cfg.Ping.IPFamily != "dual" {
		t.Fatalf("expected ip family=dual, got %q", cfg.Ping.IPFamily)
	}
	if cfg.Output.Path != "results" {
		t.Fatalf("expected output path results, got %q", cfg.Output.Path)
	}
	if cfg.Output.Format != "json" {
		t.Fatalf("expected output format json, got %q", cfg.Output.Format)
	}
}

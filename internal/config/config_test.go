package config

import (
	"testing"
	"time"
)

func TestApplyDefaults(t *testing.T) {
	cfg := Config{}

	ApplyDefaults(&cfg)

	if cfg.Mode != ModeResolvePing {
		t.Fatalf("expected default mode %q, got %q", ModeResolvePing, cfg.Mode)
	}
	if cfg.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", cfg.Attempts)
	}
	if cfg.Timeout != 2*time.Second {
		t.Fatalf("expected timeout=2s, got %s", cfg.Timeout)
	}
	if cfg.Ping.Count != 3 {
		t.Fatalf("expected ping count=3, got %d", cfg.Ping.Count)
	}
	if cfg.Ping.Interval != 100*time.Millisecond {
		t.Fatalf("expected ping interval=100ms, got %s", cfg.Ping.Interval)
	}
	if cfg.Ping.Timeout != 2*time.Second {
		t.Fatalf("expected ping timeout=2s, got %s", cfg.Ping.Timeout)
	}
	if cfg.Ping.IPSelection != "all" {
		t.Fatalf("expected ip selection=all, got %q", cfg.Ping.IPSelection)
	}
}

func TestValidateRejectsUnknownIPSelection(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DNSServers = []string{"udp://8.8.8.8"}
	cfg.Domains = []string{"example.com"}
	cfg.Ping.IPSelection = "random"

	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid ip selection to be rejected")
	}
}

func TestValidateRequiresDomainsOnlyForResolvePing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DNSServers = []string{"udp://8.8.8.8"}
	cfg.Mode = ModeDNSPing

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected dns-ping without domains to be valid: %v", err)
	}

	cfg.Mode = ModeResolvePing
	if err := Validate(cfg); err == nil {
		t.Fatal("expected resolve-ping without domains to be invalid")
	}
}

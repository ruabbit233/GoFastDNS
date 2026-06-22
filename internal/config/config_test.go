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
	if cfg.Ping.IPFamily != "ipv4" {
		t.Fatalf("expected ip family=ipv4, got %q", cfg.Ping.IPFamily)
	}
	if len(cfg.DNS.RecordTypes) != 1 || cfg.DNS.RecordTypes[0] != "A" {
		t.Fatalf("expected default record types=[A], got %#v", cfg.DNS.RecordTypes)
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

func TestValidateAcceptsHTMLOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DNSServers = []string{"udp://8.8.8.8"}
	cfg.Domains = []string{"example.com"}
	cfg.Output.Format = "html"

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected html output to be valid: %v", err)
	}
}

func TestValidateAcceptsJSONOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DNSServers = []string{"udp://8.8.8.8"}
	cfg.Domains = []string{"example.com"}
	cfg.Output.Format = "json"

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected json output to be valid: %v", err)
	}
}

func TestValidateRejectsUnknownOutputFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DNSServers = []string{"udp://8.8.8.8"}
	cfg.Domains = []string{"example.com"}
	cfg.Output.Format = "xml"

	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid output format to be rejected")
	}
}

func TestValidateRecordTypesAndIPFamily(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DNSServers = []string{"udp://8.8.8.8"}
	cfg.Domains = []string{"example.com"}
	cfg.DNS.RecordTypes = []string{"A", "AAAA"}
	cfg.Ping.IPFamily = "dual"

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected A/AAAA dual stack config to be valid: %v", err)
	}

	cfg.DNS.RecordTypes = []string{"MX"}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected unsupported record type to be rejected")
	}

	cfg.DNS.RecordTypes = []string{"A"}
	cfg.Ping.IPFamily = "ipx"
	if err := Validate(cfg); err == nil {
		t.Fatal("expected unsupported ip family to be rejected")
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

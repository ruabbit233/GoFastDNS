package ping

import (
	"GoFastDNS/internal/dns"
	"context"
	"testing"
	"time"
)

func TestSelectPingTargets(t *testing.T) {
	answers := []dns.Answer{
		{Type: "CNAME", Value: "edge.example.com"},
		{Type: "A", Value: "192.0.2.1"},
		{Type: "AAAA", Value: "2001:db8::1"},
		{Type: "A", Value: "192.0.2.2"},
	}

	first := SelectTargets(answers, "first", "dual")
	if len(first) != 1 || first[0] != "192.0.2.1" {
		t.Fatalf("expected only the first target, got %#v", first)
	}

	all := SelectTargets(answers, "all", "dual")
	if len(all) != 3 {
		t.Fatalf("expected all targets, got %#v", all)
	}
}

func TestSelectTargetsFiltersIPFamily(t *testing.T) {
	answers := []dns.Answer{
		{Type: "A", Value: "192.0.2.1"},
		{Type: "AAAA", Value: "2001:db8::1"},
	}

	ipv4 := SelectTargets(answers, "all", "ipv4")
	if len(ipv4) != 1 || ipv4[0] != "192.0.2.1" {
		t.Fatalf("expected only IPv4 target, got %#v", ipv4)
	}

	ipv6 := SelectTargets(answers, "all", "ipv6")
	if len(ipv6) != 1 || ipv6[0] != "2001:db8::1" {
		t.Fatalf("expected only IPv6 target, got %#v", ipv6)
	}
}

func TestPingDNSResultIgnoresZeroRTTWithoutError(t *testing.T) {
	dnsResult := dns.DNSResult{
		Server: "udp://resolver",
		Domain: "example.com",
		Answers: []dns.Answer{
			{Type: "A", Value: "192.0.2.1"},
			{Type: "A", Value: "192.0.2.2"},
		},
	}

	result := PingDNSResultWithOptionsAndRunner(context.Background(), dnsResult, Options{}, func(ctx context.Context, ip string, options Options) PingResult {
		if ip == "192.0.2.1" {
			return PingResult{IP: ip, PacketLoss: 100, PacketsSent: 3}
		}
		return PingResult{IP: ip, RTT: 20 * time.Millisecond, PacketsSent: 3}
	})

	if result.Error != nil {
		t.Fatalf("expected one successful target to keep domain ping successful, got %v", result.Error)
	}
	if result.AvgRTT != 20*time.Millisecond {
		t.Fatalf("expected avg RTT from successful target only, got %s", result.AvgRTT)
	}
	if result.PingResults[0].Error == nil {
		t.Fatal("expected zero-RTT target to be marked as failed")
	}
}

func TestPingDNSResultFailsWhenAllTargetsHaveZeroRTT(t *testing.T) {
	dnsResult := dns.DNSResult{
		Server:  "udp://resolver",
		Domain:  "example.com",
		Answers: []dns.Answer{{Type: "A", Value: "192.0.2.1"}},
	}

	result := PingDNSResultWithOptionsAndRunner(context.Background(), dnsResult, Options{}, func(ctx context.Context, ip string, options Options) PingResult {
		return PingResult{IP: ip, PacketLoss: 100, PacketsSent: 3}
	})

	if result.Error == nil {
		t.Fatal("expected zero-RTT target to fail the DNS ping result")
	}
	if result.AvgRTT != 0 {
		t.Fatalf("expected no average RTT for failed ping, got %s", result.AvgRTT)
	}
}

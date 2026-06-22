package ping

import (
	"GoFastDNS/internal/dns"
	"testing"
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

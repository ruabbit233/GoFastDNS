//go:build integration

package ping_test

import (
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/ping"
	"testing"
	"time"
)

func TestPingIP(t *testing.T) {
	ip := "8.8.8.8"
	result := ping.PingIP(ip)
	if result.Error != nil {
		t.Fatalf("Ping %s failed: %v", ip, result.Error)
	}
	t.Logf("Ping %s: RTT=%v, PacketLoss=%.2f%%, PacketsSent=%d",

		ip, result.RTT, result.PacketLoss*100, result.PacketsSent)
}

func TestPingDNSResult(t *testing.T) {
	server := "8.8.8.8"
	domain := "www.google.com"
	result := dns.ResolveDNS(server, domain, 3, time.Second*2)
	pingResult := ping.PingDNSResult(result)
	if pingResult.Error != nil {
		t.Fatalf("Ping %s failed: %v", domain, pingResult.Error)
	}
	t.Logf("Ping %s: RTT=%v, PacketLoss=%.2f%%, PacketsSent=%d",
		domain, pingResult.PingResults[0].RTT, pingResult.PingResults[0].PacketLoss*100, pingResult.PingResults[0].PacketsSent)
}

package benchmark

import (
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/geoip"
	"GoFastDNS/internal/ping"
	"time"
)

type DomainResult struct {
	Round          int
	Domain         string
	Answers        []dns.Answer
	ResponseCodes  []dns.ResponseCode
	ResponseTime   time.Duration
	Error          error
	QueryErrors    []string
	NoAnswer       bool
	RetryCount     int
	DnsPingResults ping.DNSPingResult // 新增字段
}

type DurationStats struct {
	Count  int
	Min    time.Duration
	Max    time.Duration
	Avg    time.Duration
	Median time.Duration
	P95    time.Duration
	Jitter time.Duration
}

type BenchmarkResult struct {
	Server          string
	Rounds          int
	Warmup          int
	AvgResponseTime time.Duration
	DomainResults   []DomainResult
	SuccessRate     float64
	DNSSuccessRate  float64
	PingSuccessRate float64
	TotalRetries    int
	AvgPingRTT      time.Duration
	DNSStats        DurationStats
	PingStats       DurationStats
	Score           float64
}

type DNSPingRoundResult struct {
	Round       int
	RTT         time.Duration
	PacketLoss  float64
	PacketsSent int
	Error       error
}

type DNSPingBenchmarkResult struct {
	Server       string
	Target       string
	TargetGeoIP  *geoip.Info
	Rounds       int
	Warmup       int
	RTT          time.Duration
	PacketLoss   float64
	PacketsSent  int
	Stats        DurationStats
	SuccessRate  float64
	Score        float64
	RoundResults []DNSPingRoundResult
	Error        error
}

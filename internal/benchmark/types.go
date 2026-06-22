package benchmark

import (
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/ping"
	"time"
)

type DomainResult struct {
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

type BenchmarkResult struct {
	Server          string
	AvgResponseTime time.Duration
	DomainResults   []DomainResult
	SuccessRate     float64
	TotalRetries    int
	AvgPingRTT      time.Duration
}

type DNSPingBenchmarkResult struct {
	Server      string
	Target      string
	RTT         time.Duration
	PacketLoss  float64
	PacketsSent int
	Error       error
}

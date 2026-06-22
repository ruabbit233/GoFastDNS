package benchmark

import (
	"GoFastDNS/internal/ping"
	"time"
)

type DomainResult struct {
	Domain         string
	Answers        []string
	ResponseTime   time.Duration
	Error          error
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

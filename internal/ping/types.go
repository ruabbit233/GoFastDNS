package ping

import "time"

type Options struct {
	Count       int
	Interval    time.Duration
	Timeout     time.Duration
	Privileged  bool
	IPSelection string
}

type PingResult struct {
	IP          string
	RTT         time.Duration
	Error       error
	PacketLoss  float64
	PacketsSent int
}

type DNSPingResult struct {
	Domain      string
	DNSServer   string
	PingResults []PingResult
	AvgRTT      time.Duration
	Error       error
}

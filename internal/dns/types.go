package dns

import (
	"time"
)

type Protocol string

const (
	ProtocolUDP Protocol = "UDP"
	ProtocolTCP Protocol = "TCP"
	ProtocolTLS Protocol = "TLS"
)

type RecordType string

const (
	RecordTypeA    RecordType = "A"
	RecordTypeAAAA RecordType = "AAAA"
)

type ResolveOptions struct {
	RecordTypes []RecordType
}

type Answer struct {
	QueryType string `json:"query_type,omitempty"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	TTL       uint32 `json:"ttl"`
	Family    string `json:"family,omitempty"`
	Priority  int    `json:"priority,omitempty"`
}

type ResponseCode struct {
	RecordType string `json:"record_type"`
	Code       int    `json:"code"`
	Name       string `json:"name"`
}

type DNSResult struct {
	Server          string
	Domain          string
	Protocol        Protocol
	ResponseTime    time.Duration
	ResolutionError error
	RetryCount      int
	Answers         []Answer
	ResponseCodes   []ResponseCode
	QueryErrors     []string
	NoAnswer        bool
}

type Resolver interface {
	Resolve(domain string, timeout time.Duration, options ResolveOptions) DNSResult
}

type UDPResolver struct {
	server string
}

type TCPResolver struct {
	server string
}

type TLSResolver struct {
	server string
}

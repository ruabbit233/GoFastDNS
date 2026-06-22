package dns

import (
	"net"
	"testing"
	"time"

	miekgdns "github.com/miekg/dns"
)

func TestParseAnswersPreservesStructuredRecords(t *testing.T) {
	records := []miekgdns.RR{
		&miekgdns.CNAME{
			Hdr:    miekgdns.RR_Header{Name: "www.example.com.", Rrtype: miekgdns.TypeCNAME, Class: miekgdns.ClassINET, Ttl: 60},
			Target: "edge.example.com.",
		},
		&miekgdns.A{
			Hdr: miekgdns.RR_Header{Name: "edge.example.com.", Rrtype: miekgdns.TypeA, Class: miekgdns.ClassINET, Ttl: 300},
			A:   net.ParseIP("192.0.2.1"),
		},
		&miekgdns.AAAA{
			Hdr:  miekgdns.RR_Header{Name: "edge.example.com.", Rrtype: miekgdns.TypeAAAA, Class: miekgdns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP("2001:db8::1"),
		},
	}

	answers := ParseAnswers(records, RecordTypeA)
	if len(answers) != 3 {
		t.Fatalf("expected 3 answers, got %#v", answers)
	}
	if answers[0].Type != "CNAME" || answers[0].Value != "edge.example.com" || answers[0].TTL != 60 {
		t.Fatalf("unexpected CNAME answer: %#v", answers[0])
	}
	if answers[1].Type != "A" || answers[1].Value != "192.0.2.1" || answers[1].Family != "ipv4" {
		t.Fatalf("unexpected A answer: %#v", answers[1])
	}
	if answers[2].Type != "AAAA" || answers[2].Value != "2001:db8::1" || answers[2].Family != "ipv6" {
		t.Fatalf("unexpected AAAA answer: %#v", answers[2])
	}
}

func TestHasAddressAnswer(t *testing.T) {
	if hasAddressAnswer([]Answer{{Type: "CNAME", Value: "edge.example.com"}}) {
		t.Fatal("expected CNAME-only answer set to have no address answer")
	}
	if !hasAddressAnswer([]Answer{{Type: "AAAA", Value: "2001:db8::1"}}) {
		t.Fatal("expected AAAA answer to count as address answer")
	}
}

func TestServerAddressPreservesExplicitPort(t *testing.T) {
	tests := []struct {
		name        string
		server      string
		defaultPort string
		want        string
	}{
		{name: "plain host", server: "8.8.8.8", defaultPort: "53", want: "8.8.8.8:53"},
		{name: "host with port", server: "8.8.8.8:5353", defaultPort: "53", want: "8.8.8.8:5353"},
		{name: "ipv6 host", server: "[2001:4860:4860::8888]", defaultPort: "53", want: "[2001:4860:4860::8888]:53"},
		{name: "ipv6 with port", server: "[2001:4860:4860::8888]:5353", defaultPort: "53", want: "[2001:4860:4860::8888]:5353"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverAddress(tt.server, tt.defaultPort); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveDNSWithOptionsUnsupportedResolverInitializesSlices(t *testing.T) {
	result := ResolveDNSWithOptions("bad://resolver", "example.com", 0, time.Second, ResolveOptions{})
	if result.ResolutionError == nil {
		t.Fatal("expected unsupported resolver error")
	}
	if result.Answers == nil {
		t.Fatal("expected answers to be an empty slice")
	}
	if len(result.QueryErrors) != 1 {
		t.Fatalf("expected query error, got %#v", result.QueryErrors)
	}
}

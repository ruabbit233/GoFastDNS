package benchmark

import (
	"GoFastDNS/internal/config"
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/geoip"
	"GoFastDNS/internal/ping"
	"fmt"
	"net"
	"strings"
	"sync"
)

func openConfiguredGeoIPLookup(cfg config.GeoIPConfig) (geoip.Lookup, func() error, error) {
	if !cfg.Enabled {
		return nil, func() error { return nil }, nil
	}

	switch cfg.Provider {
	case "ip2location":
	default:
		return nil, nil, fmt.Errorf("unsupported geoip provider %q", cfg.Provider)
	}

	lookup, err := geoip.OpenIP2Location(cfg.DatabasePath, cfg.ASNDatabasePath)
	if err != nil {
		return nil, nil, err
	}
	return lookup, lookup.Close, nil
}

type geoIPAnnotator struct {
	lookup geoip.Lookup
	mu     sync.Mutex
	cache  map[string]*geoip.Info
}

func newGeoIPAnnotator(lookup geoip.Lookup) *geoIPAnnotator {
	if lookup == nil {
		return nil
	}
	return &geoIPAnnotator{
		lookup: lookup,
		cache:  make(map[string]*geoip.Info),
	}
}

func (a *geoIPAnnotator) annotateAnswers(answers []dns.Answer) []dns.Answer {
	if a == nil {
		return answers
	}
	for i := range answers {
		switch answers[i].Type {
		case "A", "AAAA":
			answers[i].GeoIP = a.lookupIP(answers[i].Value)
		}
	}
	return answers
}

func (a *geoIPAnnotator) annotatePingResults(results []ping.PingResult) []ping.PingResult {
	if a == nil {
		return results
	}
	for i := range results {
		results[i].GeoIP = a.lookupIP(results[i].IP)
	}
	return results
}

func (a *geoIPAnnotator) lookupIP(ip string) *geoip.Info {
	if a == nil {
		return nil
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil
	}
	key := parsed.String()

	a.mu.Lock()
	defer a.mu.Unlock()
	if cached, ok := a.cache[key]; ok {
		return cached
	}

	info, err := a.lookup.Lookup(key)
	if err != nil {
		info = &geoip.Info{IP: key, Error: err.Error()}
	}
	if info != nil && info.IP == "" {
		info.IP = key
	}
	a.cache[key] = info
	return info
}

func geoSummary(info *geoip.Info) string {
	if info == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	location := compactLocation(info)
	if location != "" {
		parts = append(parts, location)
	}
	if info.ASN != "" {
		if info.ASName != "" {
			parts = append(parts, "AS"+info.ASN+" "+info.ASName)
		} else {
			parts = append(parts, "AS"+info.ASN)
		}
	} else if info.ASName != "" {
		parts = append(parts, info.ASName)
	}
	if info.ISP != "" {
		parts = append(parts, info.ISP)
	}
	if info.Error != "" {
		parts = append(parts, "GeoIP error: "+info.Error)
	}
	return strings.Join(parts, " / ")
}

func compactLocation(info *geoip.Info) string {
	values := []string{info.CountryName, info.Region, info.City}
	parts := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		parts = append(parts, value)
	}
	return strings.Join(parts, ", ")
}

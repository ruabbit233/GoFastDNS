package geoip

import (
	"errors"
	"fmt"
	"net"
	"strings"

	ip2location "github.com/ip2location/ip2location-go/v9"
)

type Info struct {
	IP          string  `json:"ip,omitempty"`
	Provider    string  `json:"provider,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	CountryName string  `json:"country_name,omitempty"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	ISP         string  `json:"isp,omitempty"`
	ASN         string  `json:"asn,omitempty"`
	ASName      string  `json:"as_name,omitempty"`
	ASDomain    string  `json:"as_domain,omitempty"`
	ASCIDR      string  `json:"as_cidr,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type Lookup interface {
	Lookup(ip string) (*Info, error)
}

type IP2LocationLookup struct {
	geoDB *ip2location.DB
	asnDB *ip2location.DB
}

func OpenIP2Location(databasePath, asnDatabasePath string) (*IP2LocationLookup, error) {
	if strings.TrimSpace(databasePath) == "" && strings.TrimSpace(asnDatabasePath) == "" {
		return nil, errors.New("at least one IP2Location database path is required")
	}

	lookup := &IP2LocationLookup{}
	var err error
	if strings.TrimSpace(databasePath) != "" {
		lookup.geoDB, err = ip2location.OpenDB(databasePath)
		if err != nil {
			return nil, fmt.Errorf("open geoip database %q: %w", databasePath, err)
		}
	}
	if strings.TrimSpace(asnDatabasePath) != "" {
		lookup.asnDB, err = ip2location.OpenDB(asnDatabasePath)
		if err != nil {
			lookup.Close()
			return nil, fmt.Errorf("open geoip ASN database %q: %w", asnDatabasePath, err)
		}
	}
	return lookup, nil
}

func (l *IP2LocationLookup) Lookup(ip string) (*Info, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("invalid IP address %q", ip)
	}

	info := &Info{
		IP:       parsed.String(),
		Provider: "ip2location",
	}
	if l.geoDB != nil {
		record, err := l.geoDB.Get_all(parsed.String())
		if err != nil {
			return nil, fmt.Errorf("lookup geoip for %s: %w", parsed.String(), err)
		}
		mergeGeoRecord(info, record)
	}
	if l.asnDB != nil {
		record, err := l.asnDB.Get_all(parsed.String())
		if err != nil {
			return nil, fmt.Errorf("lookup ASN for %s: %w", parsed.String(), err)
		}
		mergeASNRecord(info, record)
	}
	return info, nil
}

func (l *IP2LocationLookup) Close() error {
	if l.geoDB != nil {
		l.geoDB.Close()
	}
	if l.asnDB != nil {
		l.asnDB.Close()
	}
	return nil
}

func mergeGeoRecord(info *Info, record ip2location.IP2Locationrecord) {
	info.CountryCode = cleanRecordValue(record.Country_short)
	info.CountryName = cleanRecordValue(record.Country_long)
	info.Region = cleanRecordValue(record.Region)
	info.City = cleanRecordValue(record.City)
	info.ISP = cleanRecordValue(record.Isp)
	if record.Latitude != 0 {
		info.Latitude = float64(record.Latitude)
	}
	if record.Longitude != 0 {
		info.Longitude = float64(record.Longitude)
	}
}

func mergeASNRecord(info *Info, record ip2location.IP2Locationrecord) {
	if value := cleanRecordValue(record.Asn); value != "" {
		info.ASN = value
	}
	if value := cleanRecordValue(record.As); value != "" {
		info.ASName = value
	}
	if value := cleanRecordValue(record.Asdomain); value != "" {
		info.ASDomain = value
	}
	if value := cleanRecordValue(record.Ascidr); value != "" {
		info.ASCIDR = value
	}
	if info.ISP == "" {
		info.ISP = cleanRecordValue(record.Isp)
	}
}

func cleanRecordValue(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "", "-", "N/A":
		return ""
	}
	if strings.Contains(value, "This parameter is unavailable") {
		return ""
	}
	return value
}

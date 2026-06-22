//go:build geoip_smoke

package geoip

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIP2LocationRootDatabasesSmoke(t *testing.T) {
	geoDB := filepath.Join("..", "..", "IP2LOCATION-LITE-DB11.BIN")
	asnDB := filepath.Join("..", "..", "IP2LOCATION-LITE-ASN.BIN")
	for _, path := range []string{geoDB, asnDB} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("GeoIP database %s is not available: %v", path, err)
		}
	}

	lookup, err := OpenIP2Location(geoDB, asnDB)
	if err != nil {
		t.Fatalf("open IP2Location databases: %v", err)
	}
	defer func() {
		_ = lookup.Close()
	}()

	info, err := lookup.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("lookup 8.8.8.8: %v", err)
	}
	if info == nil || info.IP != "8.8.8.8" {
		t.Fatalf("unexpected lookup result: %#v", info)
	}
}

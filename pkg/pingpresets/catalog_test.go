package pingpresets

import (
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestCatalogCoverageAndFamilies(t *testing.T) {
	regions := map[string]map[string]bool{}
	overseas := map[string]bool{}
	for _, p := range Catalog.Entries {
		if _, err := url.ParseRequestURI(p.SourceURL); err != nil {
			t.Fatal("missing source", p.ID)
		}
		if p.Group == "mainland" {
			if p.Type != "tcp" || !strings.HasSuffix(p.Target, ".ip.zstaticcdn.com:80") {
				t.Fatal("mainland must use provider TCP endpoint", p.ID)
			}
			if regions[p.Region] == nil {
				regions[p.Region] = map[string]bool{}
			}
			regions[p.Region][p.Carrier+"|"+p.Family] = true
		} else if p.Group == "overseas" {
			overseas[p.Region] = true
		}
		if p.Type == "icmp" {
			ip := net.ParseIP(p.Target)
			if ip == nil || ((ip.To4() != nil) != (p.Family == "ipv4")) {
				t.Fatal("incorrect ICMP family", p.ID)
			}
		}
	}
	if len(regions) != 31 || len(overseas) != 11 {
		t.Fatalf("coverage mainland=%d overseas=%d", len(regions), len(overseas))
	}
	for region, entries := range regions {
		if len(entries) != 6 {
			t.Fatal("incomplete carrier/family matrix", region)
		}
	}
	for id, target := range map[string]string{"global-google-ipv4": "8.8.8.8", "global-cloudflare-ipv4": "1.1.1.1"} {
		p, ok := Find(id)
		if !ok || p.Target != target || p.Type != "icmp" {
			t.Fatal("DNS preset mismatch", id)
		}
	}
}

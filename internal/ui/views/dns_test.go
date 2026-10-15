package views

import (
	"reflect"
	"slices"
	"testing"
)

const (
	dnsType  = "microsoft.network/dnszones"
	pdnsType = "microsoft.network/privatednszones"
)

func TestDNSOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Zone type       Public", "Location      global", 50),
		twoCol("Record sets     37 of 5000", "Subscription  Prod", 50),
		twoCol("Resource group  rg-dns", "", 50),
		"",
		"Name servers",
	}
	want = append(want, wantTable([]int{0},
		[]string{"Name server"},
		[]string{"ns1-01.azure-dns.com."},
		[]string{"ns2-01.azure-dns.net."},
		[]string{"ns3-01.azure-dns.org."},
		[]string{"ns4-01.azure-dns.info."},
	)...)
	want = append(want, "", "Tags", "env = prod", "")
	wantLines(t, bodyOf(t, "dns", "", 110), want)
}

func TestPrivateDNSOverview(t *testing.T) {
	want := []string{
		"Essentials",
		twoCol("Status                   ● Succeeded", "Resource group  rg-dns", 50),
		twoCol("Record sets              7 of 25000", "Location        global", 50),
		twoCol("Virtual network links    3 of 1000", "Subscription    Prod", 50),
		twoCol("Links with registration  2 of 100", "", 50),
		"",
		"Tags",
		"env = prod",
		"",
	}
	wantLines(t, bodyOf(t, "pdns", "", 110), want)
}

func TestDNSNameServers(t *testing.T) {
	for _, props := range []string{`{}`, `{"nameServers":[]}`, `{"nameServers":null}`} {
		got := overviewBody(t, typedRow(t, dnsType, props), 110)
		wantLines(t, got[slices.Index(got, "Name servers"):][:2], []string{"Name servers", "none"})
	}
	got := overviewBody(t, typedRow(t, dnsType, `{"nameServers":["b.example.","a.example."]}`), 110)
	wantLines(t, got[slices.Index(got, "Name servers"):][:4], append([]string{"Name servers"}, wantTable([]int{0}, []string{"Name server"}, []string{"b.example."}, []string{"a.example."})...))
}

func TestPrivateDNSHasNoNameServers(t *testing.T) {
	if got := overviewBody(t, typedRow(t, pdnsType, `{"nameServers":["x."]}`), 110); contains(got, "Name servers") {
		t.Errorf("a private zone shows name servers:\n%v", got)
	}
}

func TestCountOfMax(t *testing.T) {
	tests := []struct {
		name, props, want string
	}{
		{"count and maximum", `{"n":12,"m":10000}`, "12 of 10000"},
		{"count without maximum", `{"n":12}`, "12"},
		{"count with null maximum", `{"n":12,"m":null}`, "12"},
		{"zero count", `{"n":0,"m":50}`, "0 of 50"},
		{"no count", `{"m":10000}`, "—"},
		{"nothing", `{}`, "—"},
		{"null count", `{"n":null,"m":5}`, "—"},
		{"count that is no number", `{"n":"12","m":5}`, "—"},
		{"maximum that is no number", `{"n":12,"m":"5"}`, "12"},
		{"whole numbers from JSON", `{"n":12.0,"m":1e4}`, "12 of 10000"},
		{"fractional numbers", `{"n":12.6,"m":10000.4}`, "13 of 10000"},
		{"large numbers", `{"n":2000000,"m":30000000}`, "2000000 of 30000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := typedRow(t, dnsType, tt.props)
			if got := countOfMax("n", "m")(row); got != tt.want {
				t.Errorf("countOfMax = %q, want %q", got, tt.want)
			}
		})
	}
}

const noLinks = "▲ the zone is linked to no virtual network"

func TestDNSAttention(t *testing.T) {
	tests := []struct {
		name, props string
		want        []string
	}{
		{"nothing", `{}`, nil},
		{"far below", `{"numberOfRecordSets":100,"maxNumberOfRecordSets":10000}`, nil},
		{"exactly 90%", `{"numberOfRecordSets":9000,"maxNumberOfRecordSets":10000}`, nil},
		{"just above 90%", `{"numberOfRecordSets":9001,"maxNumberOfRecordSets":10000}`, []string{"▲ record sets use 9001 of 10000"}},
		{"full", `{"numberOfRecordSets":10000,"maxNumberOfRecordSets":10000}`, []string{"▲ record sets use 10000 of 10000"}},
		{"no maximum", `{"numberOfRecordSets":99999}`, nil},
		{"null maximum", `{"numberOfRecordSets":99999,"maxNumberOfRecordSets":null}`, nil},
		{"no count", `{"maxNumberOfRecordSets":10000}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(dnsAttention(typedRow(t, dnsType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrivateDNSAttention(t *testing.T) {
	const full = `"numberOfRecordSets":9001,"maxNumberOfRecordSets":10000`
	tests := []struct {
		name, props string
		want        []string
	}{
		{"linked", `{"numberOfVirtualNetworkLinks":3}`, nil},
		{"one link", `{"numberOfVirtualNetworkLinks":1}`, nil},
		{"no links", `{"numberOfVirtualNetworkLinks":0}`, []string{noLinks}},
		{"links missing", `{}`, []string{noLinks}},
		{"links null", `{"numberOfVirtualNetworkLinks":null}`, []string{noLinks}},
		{"registration links do not count", `{"numberOfVirtualNetworkLinksWithRegistration":2}`, []string{noLinks}},
		{"linked, record sets above 90%", `{"numberOfVirtualNetworkLinks":3,` + full + `}`, []string{"▲ record sets use 9001 of 10000"}},
		{"linked, record sets at 90%", `{"numberOfVirtualNetworkLinks":3,"numberOfRecordSets":9000,"maxNumberOfRecordSets":10000}`, nil},
		{"linked, no maximum", `{"numberOfVirtualNetworkLinks":3,"numberOfRecordSets":99999}`, nil},
		{"both, links first", `{"numberOfVirtualNetworkLinks":0,` + full + `}`, []string{noLinks, "▲ record sets use 9001 of 10000"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(pdnsAttention(typedRow(t, pdnsType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDNSAttentionIsWarning(t *testing.T) {
	for _, a := range append(dnsAttention(typedRow(t, dnsType, `{"numberOfRecordSets":10,"maxNumberOfRecordSets":10}`)), pdnsAttention(nil)...) {
		if a.severe {
			t.Errorf("%q is an error", a.text)
		}
	}
}

func TestDNSAttentionShownInOverview(t *testing.T) {
	got := overviewBody(t, typedRow(t, pdnsType, `{}`), 110)
	if !contains(got, "Attention") || !contains(got, noLinks) {
		t.Errorf("no Attention box:\n%v", got)
	}
	if contains(bodyOf(t, "dns", "", 110), "Attention") {
		t.Error("Attention shown without alerts")
	}
	got = overviewBody(t, typedRow(t, dnsType, `{"numberOfRecordSets":9500,"maxNumberOfRecordSets":10000}`), 110)
	if !contains(got, "Attention") || !contains(got, "▲ record sets use 9500 of 10000") {
		t.Errorf("no Attention box for a public zone:\n%v", got)
	}
}

func TestDNSTabs(t *testing.T) {
	for _, typ := range []string{dnsType, pdnsType} {
		if got := Tabs(typ); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
			t.Errorf("Tabs(%q) = %v", typ, got)
		}
	}
}

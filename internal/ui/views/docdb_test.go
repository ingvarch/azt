package views

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

const docdbType = "microsoft.documentdb/mongoclusters"

// docdbProbed is a cluster as the tenant returns it: old API shape, public access on, no high availability.
const docdbProbed = `{
	"resourceGroup": "rg-db", "location": "swedencentral", "subscription": "Dev",
	"identity": null, "sku": null, "zones": null, "tags": {"env": "test"},
	"properties": {
		"clusterStatus": "Ready", "provisioningState": "Succeeded", "serverVersion": "6.0",
		"administratorLogin": "rootdb", "infrastructureVersion": "1.0",
		"connectionString": "mongodb+srv://appuser:hunter2@dc-dev.mongocluster.cosmos.azure.com/?tls=true&retrywrites=false",
		"earliestRestoreTime": "2026-10-08T06:12:44Z",
		"nodeGroupSpecs": [{"kind": "Shard", "sku": "M10", "diskSizeGB": 32, "enableHa": false, "nodeCount": 1, "name": ""}],
		"privateEndpointConnections": [], "publicNetworkAccess": "Enabled",
		"replica": {"role": "Primary"}
	}
}`

// docdbNewer is a cluster of a newer API version: no nodeGroupSpecs.
const docdbNewer = `{
	"properties": {
		"compute": {"tier": "M40"}, "storage": {"sizeGb": 256},
		"highAvailability": {"targetMode": "SameZone"}, "sharding": {"shardCount": 4}
	}
}`

func TestDocumentDBOverview(t *testing.T) {
	// Inner width 124: two columns of 60 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status          ● Ready · Succeeded", "Earliest restore  2026-10-02 04:15 UTC", 60),
		twoCol("Server version  7.0", "Resource group    rg-orders", 60),
		twoCol("Admin           mongoroot", "Location          northeurope", 60),
		twoCol("Host            dc-orders.mongocluster.cosmos.azure.com", "Subscription      Prod", 60),
		twoCol("Replica role    Primary", "", 60),
		"",
		"Node groups",
		"Kind    SKU  Disk size  High availability  Nodes",
		"Shard   M30  128 GB     on                 3",
		"Config  M10  32 GB      off                2",
		"",
		"Network",
		"Public network access  off",
		"Private endpoints      1",
		"",
		"Attention",
		"▲ High availability is off",
		"",
		"Tags",
		"team = orders",
		"",
	}
	wantLines(t, bodyOf(t, "docdb", "", 130), want)
}

func TestDocumentDBOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status            ● Ready · Succeeded",
		"Server version    6.0",
		"Admin             rootdb",
		"Host              dc-dev.mongocluster.cosmos.azure.com",
		"Replica role      Primary",
		"Earliest restore  2026-10-08 06:12 UTC",
		"Resource group    rg-db",
		"Location          swedencentral",
		"Subscription      Dev",
		"",
		"Node groups",
		"Kind   SKU  Disk size  High availability  Nodes",
		"Shard  M10  32 GB      off                1",
		"",
		"Network",
		"Public network access  on",
		"Private endpoints      0",
		"",
		"Attention",
		"▲ public network access is enabled",
		"▲ High availability is off",
		"",
		"Tags",
		"env = test",
		"",
	}
	wantLines(t, overviewBody(t, bodyRow(t, docdbType, docdbProbed), 80), want)
}

func TestDocumentDBNewerShape(t *testing.T) {
	want := []string{
		"Kind   SKU  Disk size  High availability  Nodes",
		"Shard  M40  256 GB     SameZone           4",
	}
	got := overviewBody(t, bodyRow(t, docdbType, docdbNewer), 80)
	i := slices.Index(got, want[0])
	if i < 0 || !reflect.DeepEqual(got[i:i+2], want) {
		t.Errorf("node groups:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, c := range []struct{ name, body, want string }{
		{"empty specs", `{"properties":{"nodeGroupSpecs":[],"compute":{"tier":"M40"},"storage":{"sizeGb":256},"highAvailability":{"targetMode":"SameZone"},"sharding":{"shardCount":4}}}`, "Shard  M40  256 GB     SameZone           4"},
		{"tier only", `{"properties":{"compute":{"tier":"M50"}}}`, "Shard  M50  —          —                  —"},
		{"size only", `{"properties":{"storage":{"sizeGb":512}}}`, "Shard  —    512 GB     —                  —"},
		{"shards only", `{"properties":{"sharding":{"shardCount":6}}}`, "Shard  —    —          —                  6"},
		{"mode only", `{"properties":{"highAvailability":{"targetMode":"ZoneRedundantPreferred"}}}`, "Shard  —    —          ZoneRedundantPreferred  —"},
	} {
		if lines := overviewBody(t, bodyRow(t, docdbType, c.body), 80); !contains(lines, c.want) {
			t.Errorf("%s: no line %q:\n%s", c.name, c.want, strings.Join(lines, "\n"))
		}
	}
	off := overviewBody(t, bodyRow(t, docdbType, `{"properties":{"highAvailability":{"targetMode":"Disabled"}}}`), 80)
	if !contains(off, "Shard  —    —          off                —") {
		t.Errorf("Disabled is not off:\n%s", strings.Join(off, "\n"))
	}
}

func TestDocumentDBMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Status            —",
		"Server version    —",
		"Admin             —",
		"Host              —",
		"Replica role      —",
		"Earliest restore  —",
		"Resource group    —",
		"Location          —",
		"Subscription      —",
		"",
		"Node groups",
		"none",
		"",
		"Network",
		"Public network access  on",
		"Private endpoints      0",
		"",
		"Attention",
		"▲ public network access is enabled",
		"▲ High availability is off",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": docdbType}, 80), want)
}

func TestDocumentDBHost(t *testing.T) {
	for conn, want := range map[string]string{
		"mongodb+srv://u:p@h1.example.com/?tls=true": "h1.example.com",
		"mongodb+srv://u:p@h2.example.com":           "h2.example.com",
		"mongodb+srv://u:p@h3.example.com?tls=true":  "h3.example.com",
		"mongodb+srv://u:p@h4.example.com/db":        "h4.example.com",
		"mongodb+srv://u:p%40q@h5.example.com/?a=b":  "h5.example.com",
		"mongodb+srv://u:p@x@h6.example.com/?a=b":    "h6.example.com",
		"mongodb://h7.example.com/?tls=true":         "h7.example.com",
		"mongodb://h8.example.com":                   "h8.example.com",
		"mongodb://h9.example.com?tls=true":          "h9.example.com",
		"":                                           "—",
		"mongodb+srv://u:p@/?tls=true":               "—",
	} {
		row := map[string]any{"type": docdbType, "properties": map[string]any{"connectionString": conn}}
		if got := valueOf(overviewBody(t, row, 80), "Host"); got != want {
			t.Errorf("%q: Host = %q, want %q", conn, got, want)
		}
	}
}

func TestDocumentDBHidesCredentials(t *testing.T) {
	for _, row := range []map[string]any{fixture(t, "docdb"), bodyRow(t, docdbType, docdbProbed)} {
		for _, tab := range Tabs(docdbType) {
			if tab == "JSON" {
				continue
			}
			body := strings.Join(bodyOfRow(t, row, tab, 130), "\n")
			for _, secret := range []string{"connuser9", "s3cr3t", "appuser", "hunter2", "mongodb+srv", "SCRAM"} {
				if strings.Contains(body, secret) {
					t.Errorf("%s shows %q:\n%s", tab, secret, body)
				}
			}
		}
	}
	json := strings.Join(bodyOfRow(t, fixture(t, "docdb"), "JSON", 130), "\n")
	if !strings.Contains(json, "connuser9") {
		t.Error("JSON lacks the connection string")
	}
}

func TestDocumentDBAttention(t *testing.T) {
	const (
		public = "▲ public network access is enabled"
		ha     = "▲ High availability is off"
	)
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"nothing", `{"properties":{"publicNetworkAccess":"Disabled","nodeGroupSpecs":[{"enableHa":true}]}}`, nil},
		{"public missing", `{"properties":{"nodeGroupSpecs":[{"enableHa":true}]}}`, []string{public}},
		{"public in another case", `{"properties":{"publicNetworkAccess":"enabled","nodeGroupSpecs":[{"enableHa":true}]}}`, []string{public}},
		{"one group without HA", `{"properties":{"publicNetworkAccess":"Disabled","nodeGroupSpecs":[{"enableHa":true},{"enableHa":false}]}}`, []string{ha}},
		{"group not saying", `{"properties":{"publicNetworkAccess":"Disabled","nodeGroupSpecs":[{"enableHa":true},{}]}}`, nil},
		{"newer disabled", `{"properties":{"publicNetworkAccess":"Disabled","highAvailability":{"targetMode":"Disabled"}}}`, []string{ha}},
		{"newer same zone", `{"properties":{"publicNetworkAccess":"Disabled","highAvailability":{"targetMode":"SameZone"}}}`, nil},
		{"newer zone redundant", `{"properties":{"publicNetworkAccess":"Disabled","highAvailability":{"targetMode":"ZoneRedundantPreferred"}}}`, nil},
		{"neither shape", `{"properties":{"publicNetworkAccess":"Disabled"}}`, []string{ha}},
		{"empty specs, newer on", `{"properties":{"publicNetworkAccess":"Disabled","nodeGroupSpecs":[],"highAvailability":{"targetMode":"SameZone"}}}`, nil},
		{"empty specs, newer off", `{"properties":{"publicNetworkAccess":"Disabled","nodeGroupSpecs":[],"highAvailability":{"targetMode":"Disabled"}}}`, []string{ha}},
		{"group without any shape", `{"properties":{"publicNetworkAccess":"Disabled","nodeGroupSpecs":[{"kind":"Shard"}]}}`, []string{ha}},
		{"newer disabled in lower case", `{"properties":{"publicNetworkAccess":"Disabled","highAvailability":{"targetMode":"disabled"}}}`, []string{ha}},
		{"specs win over newer", `{"properties":{"publicNetworkAccess":"Disabled","nodeGroupSpecs":[{"enableHa":false}],"highAvailability":{"targetMode":"SameZone"}}}`, []string{ha}},
		{"both", `{"properties":{"nodeGroupSpecs":[{"enableHa":false}]}}`, []string{public, ha}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(docdbAttention(bodyRow(t, docdbType, tt.body)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDocumentDBTabs(t *testing.T) {
	if got := Tabs(docdbType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if k := kindOf("Microsoft.DocumentDB/mongoClusters"); k.command != "docdb" || k.attention == nil {
		t.Errorf("kindOf in another case = %+v, want docdb with attention", k)
	}
}

func TestDocumentDBColors(t *testing.T) {
	v := deliver(t, openedAtRow(t, "").Resize(130, 80), fixture(t, "docdb"), nil)
	wantFg(t, styledLine(t, v, "Status"), "Ready", fgGood)
	wantFg(t, styledLine(t, v, "Public network access"), "off", fgMuted)
	wantFg(t, styledLine(t, v, "Config"), "off", fgMuted)
	wantFg(t, styledLine(t, v, "Shard"), "on", fgGood)
}

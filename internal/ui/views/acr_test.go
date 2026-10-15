package views

import (
	"reflect"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
)

const acrType = "microsoft.containerregistry/registries"

func TestContainerRegistryOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status        ● Succeeded", "Zone redundancy  off", 50),
		twoCol("Login server  acrdemo.azurecr.io", "Resource group   rg-reg", 50),
		twoCol("SKU           Premium", "Location         swedencentral", 50),
		twoCol("Created       2024-03-06", "Subscription     Dev", 50),
		"",
		"Access",
		"Admin user               on",
		"Anonymous pull           off",
		"Role assignment mode     AbacRepositoryPermissions",
		"Entra ID tokens for ARM  on",
		"",
		"Network",
		"Public network access  Enabled",
		"Network rule bypass    AzureServices",
		"Default action         Deny",
		"IP rules               2",
		"Data endpoint          on",
		"Private endpoints      3",
		"",
		"Policies",
		"Retention    on · 7 days",
		"Soft delete  on · 14 days",
		"Trust        off",
		"Quarantine   on",
		"Export       off",
		"Encryption   customer-managed key",
		"",
		"Attention",
		"▲ the admin user is enabled",
		"",
		"Tags",
		"team = platform",
		"",
	}
	wantLines(t, bodyOf(t, "acr", "", 110), want)
}

func TestContainerRegistryMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Status           —",
		"Login server     —",
		"SKU              —",
		"Created          —",
		"Zone redundancy  —",
		"Resource group   —",
		"Location         —",
		"Subscription     —",
		"",
		"Access",
		"Admin user               —",
		"Anonymous pull           —",
		"Role assignment mode     —",
		"Entra ID tokens for ARM  —",
		"",
		"Network",
		"Public network access  —",
		"Network rule bypass    —",
		"Default action         —",
		"IP rules               0",
		"Data endpoint          —",
		"Private endpoints      0",
		"",
		"Policies",
		"Retention    —",
		"Soft delete  —",
		"Trust        —",
		"Quarantine   —",
		"Export       —",
		"Encryption   —",
		"",
		"Attention",
		"▲ the registry accepts connections from any network",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": acrType}, 80), want)
}

// acrRow is a registry row with the properties from the JSON text.

func TestContainerRegistryValues(t *testing.T) {
	for _, c := range []struct{ props, label, want string }{
		{`{"policies":{"retentionPolicy":{"status":"enabled","days":7}}}`, "Retention", "on · 7 days"},
		{`{"policies":{"retentionPolicy":{"status":"enabled"}}}`, "Retention", "on"},
		{`{"policies":{"retentionPolicy":{"status":"disabled","days":7}}}`, "Retention", "off"},
		{`{"policies":{"retentionPolicy":{}}}`, "Retention", "—"},
		{`{}`, "Retention", "—"},
		{`{"policies":{"softDeletePolicy":{"status":"enabled","retentionDays":14}}}`, "Soft delete", "on · 14 days"},
		{`{"policies":{"softDeletePolicy":{"status":"Enabled"}}}`, "Soft delete", "on"},
		{`{"policies":{"softDeletePolicy":{"status":"disabled","retentionDays":14}}}`, "Soft delete", "off"},
		{`{}`, "Soft delete", "—"},
		{`{"encryption":{"status":"enabled"}}`, "Encryption", "customer-managed key"},
		{`{"encryption":{"status":"disabled"}}`, "Encryption", "Microsoft-managed key"},
		{`{"encryption":{}}`, "Encryption", "—"},
		{`{}`, "Encryption", "—"},
		{`{"networkRuleSet":{"defaultAction":"Allow"}}`, "Default action", "Allow"},
		{`{"networkRuleSet":{}}`, "Default action", "—"},
		{`{"networkRuleSet":{"ipRules":[{},{},{}]}}`, "IP rules", "3"},
		{`{"creationDate":"soon"}`, "Created", "soon"},
		{`{"zoneRedundancy":"Enabled"}`, "Zone redundancy", "on"},
		{`{"privateEndpointConnections":[{},{}]}`, "Private endpoints", "2"},
	} {
		got := valueOf(overviewBody(t, typedRow(t, acrType, c.props), 80), c.label)
		if got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.props, c.label, got, c.want)
		}
	}
}

func TestContainerRegistryFlagsReadTheirOwnProperty(t *testing.T) {
	flags := []struct{ label, path, on string }{
		{"Admin user", `"adminUserEnabled"`, `true`},
		{"Anonymous pull", `"anonymousPullEnabled"`, `true`},
		{"Data endpoint", `"dataEndpointEnabled"`, `true`},
		{"Zone redundancy", `"zoneRedundancy"`, `"Enabled"`},
		{"Entra ID tokens for ARM", `"policies":{"azureADAuthenticationAsArmPolicy"`, `{"status":"enabled"}}`},
		{"Trust", `"policies":{"trustPolicy"`, `{"status":"enabled"}}`},
		{"Quarantine", `"policies":{"quarantinePolicy"`, `{"status":"enabled"}}`},
		{"Export", `"policies":{"exportPolicy"`, `{"status":"enabled"}}`},
	}
	for _, f := range flags {
		props := "{" + f.path + ":" + f.on + "}"
		lines := overviewBody(t, typedRow(t, acrType, props), 80)
		for _, other := range flags {
			want := "—"
			if other.label == f.label {
				want = "on"
			}
			if got := valueOf(lines, other.label); got != want {
				t.Errorf("only %s on: %s = %q, want %q", f.label, other.label, got, want)
			}
		}
	}
}

func TestContainerRegistryColors(t *testing.T) {
	v := openedAt(t, "acr", "", 110, 80)
	wantFg(t, styledLine(t, v, "Status"), "Succeeded", fgGood)
	wantFg(t, styledLine(t, v, "Admin user"), "on", fgGood)
	wantFg(t, styledLine(t, v, "Anonymous pull"), "off", fgMuted)
	wantFg(t, styledLine(t, v, "Soft delete"), "on", fgGood)
	wantFg(t, styledLine(t, v, "Soft delete"), "14 days", "")
	wantFg(t, styledLine(t, v, "Quarantine"), "on", fgGood)
	wantFg(t, styledLine(t, v, "Export"), "off", fgMuted)

	plain := deliver(t, NewDetails("s1", azure.Resource{ID: "/r/x", Name: "x"}).Resize(110, 80),
		typedRow(t, acrType, `{"policies":{"retentionPolicy":{"status":"disabled"},"softDeletePolicy":{"status":"enabled"}}}`), nil)
	wantFg(t, styledLine(t, plain, "Retention"), "off", fgMuted)
	wantFg(t, styledLine(t, plain, "Soft delete"), "on", fgGood)
}

func TestContainerRegistryAttention(t *testing.T) {
	const (
		admin = "▲ the admin user is enabled"
		anon  = "▲ anonymous pull is enabled"
		open  = "▲ the registry accepts connections from any network"
	)
	tests := []struct {
		name  string
		props string
		want  []string
	}{
		{"nothing to report", `{"adminUserEnabled":false,"anonymousPullEnabled":false,"publicNetworkAccess":"Disabled"}`, nil},
		{"admin user", `{"adminUserEnabled":true,"publicNetworkAccess":"Disabled"}`, []string{admin}},
		{"anonymous pull", `{"anonymousPullEnabled":true,"publicNetworkAccess":"Disabled"}`, []string{anon}},
		{"public access of a Basic row", `{"publicNetworkAccess":"Enabled"}`, []string{open}},
		{"public access missing", `{}`, []string{open}},
		{"public access disabled in another case", `{"publicNetworkAccess":"disabled"}`, nil},
		{"Premium row with the default action Deny", `{"publicNetworkAccess":"Enabled","networkRuleSet":{"defaultAction":"Deny"}}`, nil},
		{"default action deny in another case", `{"networkRuleSet":{"defaultAction":"deny"}}`, nil},
		{"Premium row with the default action Allow", `{"publicNetworkAccess":"Enabled","networkRuleSet":{"defaultAction":"Allow"}}`, []string{open}},
		{"all three", `{"adminUserEnabled":true,"anonymousPullEnabled":true}`, []string{admin, anon, open}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(acrAttention(typedRow(t, acrType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContainerRegistryAttentionIsWarning(t *testing.T) {
	for _, a := range acrAttention(nil) {
		if a.severe {
			t.Errorf("%q is an error", a.text)
		}
	}
	if k := kindOf(acrType); k.attention == nil {
		t.Error("the kind has no attention rules")
	}
}

func TestContainerRegistryTabs(t *testing.T) {
	if got := Tabs(acrType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if k := kindOf("Microsoft.ContainerRegistry/Registries"); k.command != "acr" {
		t.Errorf("kindOf in another case = %q, want acr", k.command)
	}
}

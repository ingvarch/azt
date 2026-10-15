package views

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
)

const (
	kvType = "microsoft.keyvault/vaults"
	miType = "microsoft.managedidentity/userassignedidentities"
)

func TestManagedIdentityOverview(t *testing.T) {
	// Inner width 124: two columns of 60 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Client ID        11111111-1111-1111-1111-111111111111", "Resource group  rg-app", 60),
		twoCol("Principal ID     22222222-2222-2222-2222-222222222222", "Location        northeurope", 60),
		twoCol("Tenant ID        33333333-3333-3333-3333-333333333333", "Subscription    Dev", 60),
		twoCol("Isolation scope  None", "", 60),
		"",
		"Tags",
		"team = app",
		"",
	}
	wantLines(t, bodyOf(t, "mi", "", 130), want)
}

func TestManagedIdentityOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Client ID        11111111-1111-1111-1111-111111111111",
		"Principal ID     22222222-2222-2222-2222-222222222222",
		"Tenant ID        33333333-3333-3333-3333-333333333333",
		"Isolation scope  None",
		"Resource group   rg-app",
		"Location         northeurope",
		"Subscription     Dev",
		"",
		"Tags",
		"team = app",
		"",
	}
	wantLines(t, bodyOf(t, "mi", "", 80), want)
}

func TestManagedIdentityMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Client ID        —",
		"Principal ID     —",
		"Tenant ID        —",
		"Isolation scope  —",
		"Resource group   —",
		"Location         —",
		"Subscription     —",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": miType}, 80), want)
}

func TestManagedIdentityTabs(t *testing.T) {
	if got := Tabs(miType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if k := kindOf(miType); k.attention != nil {
		t.Error("a managed identity has attention rules")
	}
}

// overviewBody is the unboxed Overview of a row at a pane width.
func overviewBody(t *testing.T, row map[string]any, width int) []string {
	t.Helper()
	return bodyOfRow(t, row, "", width)
}

// bodyOfRow is the body of a details view showing row on tab, width cells wide.
func bodyOfRow(t *testing.T, row map[string]any, tab string, width int) []string {
	t.Helper()
	v := deliver(t, openedAtRow(t, tab).Resize(width, 200), row, nil)
	return renderBody(v)
}

func TestKeyVaultOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status     ● Succeeded", "Resource group  rg-sec", 50),
		twoCol("Vault URI  https://kv-demo.vault.azure.net/", "Location        westeurope", 50),
		twoCol("SKU        standard", "Subscription    Dev", 50),
		twoCol("Tenant ID  33333333-3333-3333-3333-333333333333", "", 50),
		"",
		"Access",
		"Permission model     Access policies (2)",
		"Deployment           off",
		"Disk encryption      on",
		"Template deployment  off",
		"",
		"Protection",
		"Soft delete       on · 90 days",
		"Purge protection  on",
		"",
		"Network",
		"Public network access  Enabled",
		"Default action         Deny",
		"IP rules               1",
		"Virtual network rules  2",
		"Private endpoints      3",
		"",
		"Tags",
		"env = dev",
		"",
	}
	wantLines(t, bodyOf(t, "kv", "", 110), want)
}

func TestKeyVaultMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Status          —",
		"Vault URI       —",
		"SKU             —",
		"Tenant ID       —",
		"Resource group  —",
		"Location        —",
		"Subscription    —",
		"",
		"Access",
		"Permission model     Access policies (0)",
		"Deployment           —",
		"Disk encryption      —",
		"Template deployment  —",
		"",
		"Protection",
		"Soft delete       —",
		"Purge protection  off",
		"",
		"Network",
		"Public network access  Enabled",
		"Default action         Allow",
		"IP rules               0",
		"Virtual network rules  0",
		"Private endpoints      0",
		"",
		"Attention",
		"▲ purge protection is off",
		"▲ the vault accepts connections from any network",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": kvType}, 80), want)
}

// kvRow is a vault row with the properties from the JSON text.
// typedRow is a row of resource type typ with the given properties (JSON).
func typedRow(t *testing.T, typ, props string) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal([]byte(props), &p); err != nil {
		t.Fatal(err)
	}
	return map[string]any{"type": typ, "properties": p}
}

func TestKeyVaultValues(t *testing.T) {
	for _, c := range []struct{ props, want string }{
		{`{"enableRbacAuthorization":true,"accessPolicies":[{}]}`, "Permission model     Azure RBAC"},
		{`{"enableRbacAuthorization":false,"accessPolicies":[{},{},{}]}`, "Permission model     Access policies (3)"},
		{`{"accessPolicies":[]}`, "Permission model     Access policies (0)"},
		{`{"enableSoftDelete":true,"softDeleteRetentionInDays":7}`, "Soft delete       on · 7 days"},
		{`{"enableSoftDelete":true}`, "Soft delete       on"},
		{`{"enableSoftDelete":false,"softDeleteRetentionInDays":90}`, "Soft delete       off"},
		{`{}`, "Soft delete       —"},
		{`{"enablePurgeProtection":true}`, "Purge protection  on"},
		{`{"enablePurgeProtection":false}`, "Purge protection  off"},
		{`{}`, "Purge protection  off"},
		{`{"publicNetworkAccess":"Disabled"}`, "Public network access  Disabled"},
		{`{"publicNetworkAccess":"SecuredByPerimeter"}`, "Public network access  SecuredByPerimeter"},
		{`{}`, "Public network access  Enabled"},
		{`{"networkAcls":{"defaultAction":"Deny"}}`, "Default action         Deny"},
		{`{"networkAcls":{"defaultAction":"Allow"}}`, "Default action         Allow"},
		{`{"networkAcls":{}}`, "Default action         Allow"},
		{`{}`, "Default action         Allow"},
	} {
		if got := overviewBody(t, typedRow(t, kvType, c.props), 80); !contains(got, c.want) {
			t.Errorf("%s: lacks %q:\n%s", c.props, c.want, strings.Join(got, "\n"))
		}
	}
}

// valueOf is the value on the line that starts with label, spaces trimmed.
func valueOf(lines []string, label string) string {
	for _, l := range lines {
		if rest, ok := strings.CutPrefix(l, label+" "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func TestKeyVaultFlagsReadTheirOwnProperty(t *testing.T) {
	flags := map[string]string{
		"Deployment":          "enabledForDeployment",
		"Disk encryption":     "enabledForDiskEncryption",
		"Template deployment": "enabledForTemplateDeployment",
	}
	for label, prop := range flags {
		lines := overviewBody(t, typedRow(t, kvType, `{"`+prop+`":true}`), 80)
		for other := range flags {
			want := "—"
			if other == label {
				want = "on"
			}
			if got := valueOf(lines, other); got != want {
				t.Errorf("only %s on: %s = %q, want %q", prop, other, got, want)
			}
		}
	}
}

func TestKeyVaultColors(t *testing.T) {
	v := openedAt(t, "kv", "", 100, 80)
	wantFg(t, styledLine(t, v, "Status"), "Succeeded", fgGood)
	if a := lastAttrsOf(t, styledLine(t, v, "Disk encryption"), "on"); a.fg != fgGood {
		t.Errorf("disk encryption on = %+v, want good", a)
	}
	wantFg(t, styledLine(t, v, "Deployment"), "off", fgMuted)
	wantFg(t, styledLine(t, v, "Soft delete"), "on", fgGood)
	wantFg(t, styledLine(t, v, "Soft delete"), "90 days", "")
	if a := lastAttrsOf(t, styledLine(t, v, "Purge protection"), "on"); a.fg != fgGood {
		t.Errorf("purge protection on = %+v, want good", a)
	}
}

func TestKeyVaultAttention(t *testing.T) {
	const (
		noPurge = "▲ purge protection is off"
		open    = "▲ the vault accepts connections from any network"
	)
	tests := []struct {
		name  string
		props string
		want  []string
	}{
		{"nothing to report", `{"enablePurgeProtection":true,"publicNetworkAccess":"Enabled","networkAcls":{"defaultAction":"Deny"}}`, nil},
		{"purge protection off", `{"enablePurgeProtection":false,"networkAcls":{"defaultAction":"Deny"}}`, []string{noPurge}},
		{"purge protection missing", `{"networkAcls":{"defaultAction":"Deny"}}`, []string{noPurge}},
		{"public access with the default action Allow", `{"enablePurgeProtection":true,"publicNetworkAccess":"Enabled","networkAcls":{"defaultAction":"Allow"}}`, []string{open}},
		{"public access with the default action missing", `{"enablePurgeProtection":true,"publicNetworkAccess":"Enabled"}`, []string{open}},
		{"public access missing with the default action Allow", `{"enablePurgeProtection":true,"networkAcls":{"defaultAction":"Allow"}}`, []string{open}},
		{"public access disabled with the default action Allow", `{"enablePurgeProtection":true,"publicNetworkAccess":"Disabled","networkAcls":{"defaultAction":"Allow"}}`, nil},
		{"public access disabled in another case", `{"enablePurgeProtection":true,"publicNetworkAccess":"disabled"}`, nil},
		{"public access with the default action Deny", `{"enablePurgeProtection":true,"publicNetworkAccess":"Enabled","networkAcls":{"defaultAction":"Deny"}}`, nil},
		{"default action in another case", `{"enablePurgeProtection":true,"networkAcls":{"defaultAction":"allow"}}`, []string{open}},
		{"both", `{}`, []string{noPurge, open}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(kvAttention(typedRow(t, kvType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyVaultAttentionIsWarning(t *testing.T) {
	for _, a := range kvAttention(nil) {
		if a.severe {
			t.Errorf("%q is an error", a.text)
		}
	}
	if k := kindOf(kvType); k.attention == nil {
		t.Error("the kind has no attention rules")
	}
}

func TestKeyVaultAttentionBox(t *testing.T) {
	got := overviewBody(t, typedRow(t, kvType, `{"enablePurgeProtection":false,"networkAcls":{"defaultAction":"Deny"}}`), 80)
	i := slices.Index(got, "Attention")
	if i < 0 || got[i-2] != "Private endpoints      0" || got[i+1] != "▲ purge protection is off" || got[i+3] != "Tags" {
		t.Errorf("Attention is not between Network and Tags:\n%s", strings.Join(got, "\n"))
	}
	if safe := bodyOf(t, "kv", "", 100); contains(safe, "Attention") {
		t.Errorf("Attention shown without alerts:\n%s", strings.Join(safe, "\n"))
	}
	v := deliver(t, NewDetails("s1", azure.Resource{ID: "/r/x", Name: "x"}).Resize(80, 100), typedRow(t, kvType, `{}`), nil)
	wantFg(t, styledLine(t, v, "╭ Attention"), "Attention", fgWarn)
}

func TestKeyVaultTabs(t *testing.T) {
	if got := Tabs(kvType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
}

// resourceCases are details views of the resource fixtures on every tab.
func resourceCases(t *testing.T) []renderCase {
	var cases []renderCase
	for name, typ := range map[string]string{"kv": kvType, "mi": miType, "acr": acrType, "storage": storageType, "vnet": vnetType} {
		row := fixture(t, name)
		for _, tab := range Tabs(typ) {
			cases = append(cases, renderCase{name + " " + tab, func(w, h int) View {
				return deliver(t, newDetailsAt(tab).Resize(w, h), row, nil)
			}})
		}
	}
	return cases
}

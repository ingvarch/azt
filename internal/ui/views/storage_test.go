package views

import (
	"reflect"
	"slices"
	"testing"
)

const storageType = "microsoft.storage/storageaccounts"

func TestStorageOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status          ● Succeeded", "Created         2024-05-07", 50),
		twoCol("Kind            StorageV2", "Resource group  rg-data", 50),
		twoCol("SKU             Standard_ZRS", "Location        norwayeast", 50),
		twoCol("Access tier     Cool", "Subscription    Prod", 50),
		twoCol("Primary status  ● available", "", 50),
		"",
		"Endpoints",
		"blob   https://stdemo.blob.core.windows.net/",
		"dfs    https://stdemo.dfs.core.windows.net/",
		"file   https://stdemo.file.core.windows.net/",
		"queue  https://stdemo.queue.core.windows.net/",
		"table  https://stdemo.table.core.windows.net/",
		"web    https://stdemo.z1.web.core.windows.net/",
		"",
		"Security",
		"Minimum TLS                TLS1_1",
		"HTTPS only                 on",
		"Shared key access          off",
		"Blob public access         off",
		"OAuth by default           on",
		"Cross-tenant replication   on",
		"Encryption key             customer-managed key",
		"Infrastructure encryption  on",
		"",
		"Features",
		"Data Lake Gen2  on",
		"SFTP            off",
		"NFS v3          off",
		"Local users     on",
		"",
		"Network",
		"Public network access  Enabled",
		"Default action         Deny",
		"Bypass                 Logging, Metrics",
		"IP rules               2",
		"Virtual network rules  3",
		"Private endpoints      4",
		"",
		"Attention",
		"▲ the minimum TLS version is TLS1_1",
		"",
		"Tags",
		"owner = data",
		"",
	}
	wantLines(t, bodyOf(t, "storage", "", 110), want)
}

func TestStorageMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Status          —",
		"Kind            —",
		"SKU             —",
		"Access tier     —",
		"Primary status  —",
		"Created         —",
		"Resource group  —",
		"Location        —",
		"Subscription    —",
		"",
		"Endpoints",
		"none",
		"",
		"Security",
		"Minimum TLS                TLS1_0",
		"HTTPS only                 on",
		"Shared key access          on",
		"Blob public access         off",
		"OAuth by default           off",
		"Cross-tenant replication   off",
		"Encryption key             —",
		"Infrastructure encryption  off",
		"",
		"Features",
		"Data Lake Gen2  off",
		"SFTP            off",
		"NFS v3          off",
		"Local users     off",
		"",
		"Network",
		"Public network access  Enabled",
		"Default action         Allow",
		"Bypass                 —",
		"IP rules               0",
		"Virtual network rules  0",
		"Private endpoints      0",
		"",
		"Attention",
		"▲ shared key access is allowed",
		"▲ the minimum TLS version is TLS1_0",
		"▲ the account accepts connections from any network",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": storageType}, 80), want)
}

// accountWithoutOptional is like most real accounts: no allowSharedKeyAccess, no publicNetworkAccess.
const accountWithoutOptional = `{
	"accessTier":"Hot","allowBlobPublicAccess":false,"allowCrossTenantReplication":false,
	"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,
	"networkAcls":{"bypass":"AzureServices","defaultAction":"Deny","ipRules":[],"virtualNetworkRules":[]},
	"encryption":{"keySource":"Microsoft.Storage","requireInfrastructureEncryption":false},
	"provisioningState":"Succeeded","statusOfPrimary":"available"}`

func TestStorageAccountWithoutOptionalProperties(t *testing.T) {
	row := typedRow(t, storageType, accountWithoutOptional)
	lines := overviewBody(t, row, 80)
	for label, want := range map[string]string{
		"Shared key access":     "on",
		"Public network access": "Enabled",
		"OAuth by default":      "off",
		"Data Lake Gen2":        "off",
		"Local users":           "off",
		"Encryption key":        "Microsoft-managed key",
		"Default action":        "Deny",
	} {
		if got := valueOf(lines, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	if got, want := alertLines(storageAttention(row)), []string{"▲ shared key access is allowed"}; !reflect.DeepEqual(got, want) {
		t.Errorf("attention = %q, want %q", got, want)
	}
}

func TestStorageValues(t *testing.T) {
	acls := `{"networkAcls":{"ipRules":[{},{},{},{},{}],"ipv6Rules":[{}],"virtualNetworkRules":[{}]}}`
	for _, c := range []struct{ props, label, want string }{
		{`{"encryption":{"keySource":"Microsoft.Storage"}}`, "Encryption key", "Microsoft-managed key"},
		{`{"encryption":{"keySource":"Microsoft.Keyvault"}}`, "Encryption key", "customer-managed key"},
		{`{"encryption":{"keySource":"Microsoft.KeyVault"}}`, "Encryption key", "customer-managed key"},
		{`{"encryption":{"keySource":"Other.Source"}}`, "Encryption key", "Other.Source"},
		{`{"encryption":{}}`, "Encryption key", "—"},
		{`{}`, "Encryption key", "—"},
		{`{"minimumTlsVersion":"TLS1_3"}`, "Minimum TLS", "TLS1_3"},
		{`{"minimumTlsVersion":"TLS1_2"}`, "Minimum TLS", "TLS1_2"},
		{`{}`, "Minimum TLS", "TLS1_0"},
		{`{"provisioningState":"Failed"}`, "Status", "● Failed"},
		{`{"statusOfPrimary":"unavailable"}`, "Primary status", "● unavailable"},
		{`{"accessTier":"Premium"}`, "Access tier", "Premium"},
		{`{"creationTime":"2024-05-06T23:30:00-02:00"}`, "Created", "2024-05-07"},
		{`{"publicNetworkAccess":"Disabled"}`, "Public network access", "Disabled"},
		{`{"networkAcls":{"defaultAction":"Allow"}}`, "Default action", "Allow"},
		{`{"networkAcls":{"defaultAction":"Deny"}}`, "Default action", "Deny"},
		{`{"networkAcls":{"bypass":"None"}}`, "Bypass", "None"},
		{acls, "IP rules", "5"},
		{acls, "Virtual network rules", "1"},
		{`{"privateEndpointConnections":[{},{}]}`, "Private endpoints", "2"},
	} {
		got := valueOf(overviewBody(t, typedRow(t, storageType, c.props), 80), c.label)
		if got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.props, c.label, got, c.want)
		}
	}
}

func TestStorageTopLevelValues(t *testing.T) {
	row := typedRow(t, storageType, `{}`)
	row["kind"], row["sku"] = "BlobStorage", map[string]any{"name": "Standard_LRS", "tier": "Premium"}
	lines := overviewBody(t, row, 80)
	for label, want := range map[string]string{"Kind": "BlobStorage", "SKU": "Standard_LRS"} {
		if got := valueOf(lines, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
}

func TestStorageEndpoints(t *testing.T) {
	for _, c := range []struct {
		name, props string
		want        []string
	}{
		{"sorted by name", `{"primaryEndpoints":{"web":"w","queue":"q","blob":"b","table":"t","dfs":"d","file":"f"}}`,
			[]string{"blob   b", "dfs    d", "file   f", "queue  q", "table  t", "web    w"}},
		{"an entry that is no text", `{"primaryEndpoints":{"blob":"b","web":null}}`, []string{"blob  b", "web   —"}},
		{"empty", `{"primaryEndpoints":{}}`, []string{"none"}},
		{"missing", `{}`, []string{"none"}},
	} {
		got := overviewBody(t, typedRow(t, storageType, c.props), 80)
		i, j := slices.Index(got, "Endpoints"), slices.Index(got, "Security")
		if i < 0 || j < i {
			t.Fatalf("%s: no Endpoints box:\n%q", c.name, got)
		}
		if box := got[i+1 : j-1]; !reflect.DeepEqual(box, c.want) {
			t.Errorf("%s: Endpoints box = %q, want %q", c.name, box, c.want)
		}
	}
}

func TestStorageFlagsReadTheirOwnProperty(t *testing.T) {
	// Each flag alone is set to the opposite of its default; the others stay missing.
	flags := []struct{ label, props, def, changed string }{
		{"HTTPS only", `{"supportsHttpsTrafficOnly":false}`, "on", "off"},
		{"Shared key access", `{"allowSharedKeyAccess":false}`, "on", "off"},
		{"Blob public access", `{"allowBlobPublicAccess":true}`, "off", "on"},
		{"OAuth by default", `{"defaultToOAuthAuthentication":true}`, "off", "on"},
		{"Cross-tenant replication", `{"allowCrossTenantReplication":true}`, "off", "on"},
		{"Infrastructure encryption", `{"encryption":{"requireInfrastructureEncryption":true}}`, "off", "on"},
		{"Data Lake Gen2", `{"isHnsEnabled":true}`, "off", "on"},
		{"SFTP", `{"isSftpEnabled":true}`, "off", "on"},
		{"NFS v3", `{"isNfsV3Enabled":true}`, "off", "on"},
		{"Local users", `{"isLocalUserEnabled":true}`, "off", "on"},
	}
	for _, f := range flags {
		lines := overviewBody(t, typedRow(t, storageType, f.props), 80)
		for _, other := range flags {
			want := choose(other.label == f.label, f.changed, other.def)
			if got := valueOf(lines, other.label); got != want {
				t.Errorf("only %s set: %s = %q, want %q", f.label, other.label, got, want)
			}
		}
	}
}

func TestStorageColors(t *testing.T) {
	v := openedAt(t, "storage", "", 110, 80)
	wantFg(t, styledLine(t, v, "Status"), "Succeeded", fgGood)
	wantFg(t, styledLine(t, v, "Primary status"), "available", fgGood)
	if a := lastAttrsOf(t, styledLine(t, v, "HTTPS only"), "on"); a.fg != fgGood {
		t.Errorf("HTTPS only on = %+v, want good", a)
	}
	if a := lastAttrsOf(t, styledLine(t, v, "Shared key access"), "off"); a.fg != fgMuted {
		t.Errorf("Shared key access off = %+v, want muted", a)
	}

	down := deliver(t, openedAtRow(t, ""), typedRow(t, storageType, `{"statusOfPrimary":"unavailable"}`), nil)
	wantFg(t, styledLine(t, down, "Primary status"), "unavailable", fgBad)
	wantFg(t, styledLine(t, down, "╭ Attention"), "Attention", fgWarn)
}

func TestStorageAttention(t *testing.T) {
	const (
		shared = "▲ shared key access is allowed"
		blob   = "▲ blob public access is allowed"
		https  = "▲ HTTPS is not required"
		open   = "▲ the account accepts connections from any network"
	)
	tls := func(v string) string { return "▲ the minimum TLS version is " + v }
	tests := []struct {
		name  string
		props string
		want  []string
	}{
		{"nothing to report", `{"allowSharedKeyAccess":false,"allowBlobPublicAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, nil},
		{"shared key access allowed", `{"allowSharedKeyAccess":true,"allowBlobPublicAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, []string{shared}},
		{"shared key access missing", `{"allowBlobPublicAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, []string{shared}},
		{"blob public access", `{"allowSharedKeyAccess":false,"allowBlobPublicAccess":true,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, []string{blob}},
		{"blob public access missing", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, nil},
		{"TLS 1.0", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_0","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, []string{tls("TLS1_0")}},
		{"TLS 1.1", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_1","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, []string{tls("TLS1_1")}},
		{"TLS 1.1 in another case", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"tls1_1","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, []string{tls("tls1_1")}},
		{"TLS missing counts as 1.0", `{"allowSharedKeyAccess":false,"supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, []string{tls("TLS1_0")}},
		{"TLS 1.3", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_3","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled"}`, nil},
		{"HTTPS not required", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":false,"publicNetworkAccess":"Disabled"}`, []string{https}},
		{"HTTPS missing", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_2","publicNetworkAccess":"Disabled"}`, nil},
		{"public access with the default action Allow", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Enabled","networkAcls":{"defaultAction":"Allow"}}`, []string{open}},
		{"public access and the default action missing", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true}`, []string{open}},
		{"public access with the default action Deny", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"networkAcls":{"defaultAction":"Deny"}}`, nil},
		{"public access disabled with the default action Allow", `{"allowSharedKeyAccess":false,"minimumTlsVersion":"TLS1_2","supportsHttpsTrafficOnly":true,"publicNetworkAccess":"Disabled","networkAcls":{"defaultAction":"Allow"}}`, nil},
		{"all of them", `{"allowSharedKeyAccess":true,"allowBlobPublicAccess":true,"minimumTlsVersion":"TLS1_1","supportsHttpsTrafficOnly":false}`, []string{shared, blob, tls("TLS1_1"), https, open}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(storageAttention(typedRow(t, storageType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStorageAttentionIsWarning(t *testing.T) {
	for _, a := range storageAttention(nil) {
		if a.severe {
			t.Errorf("%q is an error", a.text)
		}
	}
	if k := kindOf(storageType); k.attention == nil {
		t.Error("the kind has no attention rules")
	}
}

func TestStorageTabs(t *testing.T) {
	if got := Tabs(storageType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if k := kindOf("Microsoft.Storage/StorageAccounts"); k.command != "storage" {
		t.Errorf("kindOf in another case = %q, want storage", k.command)
	}
}

func TestOpenToAnyNetwork(t *testing.T) {
	for _, c := range []struct {
		props string
		want  bool
	}{
		{`{}`, true},
		{`{"publicNetworkAccess":"Enabled","networkAcls":{"defaultAction":"allow"}}`, true},
		{`{"publicNetworkAccess":"Enabled","networkAcls":{"defaultAction":"Deny"}}`, false},
		{`{"publicNetworkAccess":"disabled"}`, false},
		{`{"publicNetworkAccess":"Disabled","networkAcls":{"defaultAction":"Allow"}}`, false},
	} {
		if got := openToAnyNetwork(typedRow(t, storageType, c.props)); got != c.want {
			t.Errorf("openToAnyNetwork(%s) = %v, want %v", c.props, got, c.want)
		}
	}
}

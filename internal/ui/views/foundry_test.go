package views

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

const (
	foundryType        = "microsoft.cognitiveservices/accounts"
	foundryProjectType = "microsoft.cognitiveservices/accounts/projects"
)

// foundryProbed is an account as the tenant returns it: local auth and network rules missing.
const foundryProbed = `{
	"id": "/subscriptions/s1/resourceGroups/rg-lab/providers/Microsoft.CognitiveServices/accounts/fa-lab",
	"resourceGroup": "rg-lab", "location": "northeurope", "subscription": "Dev",
	"kind": "OpenAI", "sku": {"name": "S1"}, "identity": {"type": "UserAssigned"}, "tags": {"env": "lab"},
	"properties": {
		"provisioningState": "Creating", "endpoint": "https://fa-lab.openai.azure.com/",
		"customSubDomainName": "fa-lab-dom", "dateCreated": "2026-08-01T00:00:00Z",
		"allowProjectManagement": false, "associatedProjects": ["proj-one"], "defaultProject": "proj-one",
		"publicNetworkAccess": "Enabled", "privateEndpointConnections": [],
		"endpoints": {"OpenAI API": "https://fa-lab.openai.azure.com/"}
	}
}`

// foundryProjectNoDesc is a project that is not the default and has no description.
const foundryProjectNoDesc = `{
	"id": "/subscriptions/s1/resourceGroups/rg-lab/providers/Microsoft.CognitiveServices/accounts/fa-lab/projects/proj-two",
	"resourceGroup": "rg-lab", "location": "northeurope", "subscription": "Dev",
	"identity": {"type": "SystemAssigned"}, "tags": {"env": "lab"},
	"properties": {
		"displayName": "Second", "isDefault": false, "provisioningState": "Failed",
		"endpoints": {"AI Foundry API": "https://fa-lab.services.ai.azure.com/api/projects/proj-two"}
	}
}`

var foundryEndpointRows = [][]string{
	{"Name", "URL"},
	{"AI Foundry API", "https://fa-orders.services.ai.azure.com/"},
	{"Azure AI Model Inference API", "https://fa-orders.services.ai.azure.com/models"},
	{"OpenAI Language Model Instance API", "https://fa-orders.openai.azure.com/"},
}

func TestFoundryOverview(t *testing.T) {
	// Inner width 134: two columns of 65 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status            ● Succeeded", "Created         2026-03-09", 65),
		twoCol("Kind              AIServices", "Identity        SystemAssigned", 65),
		twoCol("SKU               S0", "Resource group  rg-ai", 65),
		twoCol("Endpoint          https://fa-orders.cognitiveservices.azure.com/", "Location        swedencentral", 65),
		twoCol("Custom subdomain  fa-orders-sub", "Subscription    Prod", 65),
		"",
		"Access",
		"Local key auth         off",
		"Public network access  Enabled",
		"Default action         Deny",
		"Private endpoints      1",
		"Project management     on",
		"",
		"Projects",
	}
	want = append(want, wantTable([]int{10}, []string{"Project", "Default"}, []string{"proj-alpha", "no"}, []string{"proj-beta", "yes"})...)
	want = append(want, "", "Tags", "team = ml", "")
	wantLines(t, bodyOf(t, "foundry", "", 140), want)
}

func TestFoundryOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status            ● Creating",
		"Kind              OpenAI",
		"SKU               S1",
		"Endpoint          https://fa-lab.openai.azure.com/",
		"Custom subdomain  fa-lab-dom",
		"Created           2026-08-01",
		"Identity          UserAssigned",
		"Resource group    rg-lab",
		"Location          northeurope",
		"Subscription      Dev",
		"",
		"Access",
		"Local key auth         on",
		"Public network access  Enabled",
		"Default action         Allow",
		"Private endpoints      0",
		"Project management     off",
		"",
		"Projects",
	}
	want = append(want, wantTable([]int{8}, []string{"Project", "Default"}, []string{"proj-one", "yes"})...)
	want = append(want, "", "Attention", "▲ local key auth is enabled", "▲ the account accepts connections from any network", "", "Tags", "env = lab", "")
	wantLines(t, overviewBody(t, bodyRow(t, foundryType, foundryProbed), 80), want)
}

func TestFoundryEndpointsTab(t *testing.T) {
	want := append([]string{"Endpoints"}, wantTable([]int{34}, foundryEndpointRows...)...)
	wantLines(t, bodyOf(t, "foundry", "Endpoints", 130), append(want, ""))
}

func TestFoundryMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Status            —",
		"Kind              —",
		"SKU               —",
		"Endpoint          —",
		"Custom subdomain  —",
		"Created           —",
		"Identity          —",
		"Resource group    —",
		"Location          —",
		"Subscription      —",
		"",
		"Access",
		"Local key auth         on",
		"Public network access  Enabled",
		"Default action         Allow",
		"Private endpoints      0",
		"Project management     off",
		"",
		"Projects",
		"none",
		"",
		"Attention",
		"▲ local key auth is enabled",
		"▲ the account accepts connections from any network",
		"",
		"Tags",
		"none",
		"",
	}
	row := map[string]any{"type": foundryType}
	wantLines(t, overviewBody(t, row, 80), want)
	wantLines(t, bodyOfRow(t, row, "Endpoints", 80), []string{"Endpoints", "none", ""})
}

func TestFoundryProjectOverview(t *testing.T) {
	want := []string{
		"Essentials",
		twoCol("Display name     Beta Project", "Identity        SystemAssigned", 60),
		twoCol("Description      Beta project for the orders team", "Resource group  rg-ai", 60),
		twoCol("Default project  yes", "Location        swedencentral", 60),
		twoCol("Status           ● Succeeded", "Subscription    Prod", 60),
		twoCol("Account          fa-orders", "", 60),
		"",
		"Endpoints",
	}
	want = append(want, wantTable([]int{14}, []string{"Name", "URL"}, []string{"AI Foundry API", "https://fa-orders.services.ai.azure.com/api/projects/proj-beta"})...)
	want = append(want, "", "Tags", "owner = ml-team", "")
	wantLines(t, bodyOf(t, "foundry_project", "", 130), want)
}

func TestFoundryProjectOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Display name     Second",
		"Description      —",
		"Default project  no",
		"Status           ● Failed",
		"Account          fa-lab",
		"Identity         SystemAssigned",
		"Resource group   rg-lab",
		"Location         northeurope",
		"Subscription     Dev",
		"",
		"Endpoints",
	}
	want = append(want, wantTable([]int{14}, []string{"Name", "URL"}, []string{"AI Foundry API", "https://fa-lab.services.ai.azure.com/api/projects/proj-two"})...)
	want = append(want, "", "Tags", "env = lab", "")
	wantLines(t, overviewBody(t, bodyRow(t, foundryProjectType, foundryProjectNoDesc), 80), want)
}

func TestFoundryProjectMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Display name     —",
		"Description      —",
		"Default project  —",
		"Status           —",
		"Account          —",
		"Identity         —",
		"Resource group   —",
		"Location         —",
		"Subscription     —",
		"",
		"Endpoints",
		"none",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": foundryProjectType}, 80), want)
}

func TestFoundryProjectAccountFromID(t *testing.T) {
	for id, want := range map[string]string{
		"/subscriptions/s/resourceGroups/r/providers/Microsoft.CognitiveServices/accounts/acct-a/projects/p": "acct-a",
		"/subscriptions/s/resourceGroups/r/providers/Microsoft.CognitiveServices/Accounts/acct-b/projects/p": "acct-b",
		"/subscriptions/s/resourceGroups/r/providers/Microsoft.CognitiveServices/accounts":                   "—",
		"/subscriptions/s/resourceGroups/r/providers/Microsoft.CognitiveServices/projects/p":                 "—",
		"": "—",
	} {
		row := map[string]any{"type": foundryProjectType, "id": id}
		if got := valueOf(overviewBody(t, row, 80), "Account"); got != want {
			t.Errorf("%q: Account = %q, want %q", id, got, want)
		}
	}
}

func TestFoundryValues(t *testing.T) {
	for _, c := range []struct{ props, want string }{
		{`{"disableLocalAuth":true}`, "Local key auth         off"},
		{`{"disableLocalAuth":false}`, "Local key auth         on"},
		{`{}`, "Local key auth         on"},
		{`{"allowProjectManagement":true}`, "Project management     on"},
		{`{"allowProjectManagement":false}`, "Project management     off"},
		{`{}`, "Project management     off"},
		{`{"publicNetworkAccess":"Disabled"}`, "Public network access  Disabled"},
		{`{"networkAcls":{"defaultAction":"Deny"}}`, "Default action         Deny"},
		{`{"privateEndpointConnections":[{},{}]}`, "Private endpoints      2"},
	} {
		if got := overviewBody(t, typedRow(t, foundryType, c.props), 80); !contains(got, c.want) {
			t.Errorf("%s: lacks %q:\n%s", c.props, c.want, strings.Join(got, "\n"))
		}
	}
}

func TestFoundryProjectsTable(t *testing.T) {
	for _, c := range []struct {
		name, props string
		want        [][]string
	}{
		{"none", `{}`, nil},
		{"empty", `{"associatedProjects":[],"defaultProject":"x"}`, nil},
		{"no default", `{"associatedProjects":["b","a"]}`, [][]string{{"b", "no"}, {"a", "no"}}},
		{"default elsewhere", `{"associatedProjects":["b","a"],"defaultProject":"zz"}`, [][]string{{"b", "no"}, {"a", "no"}}},
		{"first is default", `{"associatedProjects":["b","a"],"defaultProject":"b"}`, [][]string{{"b", "yes"}, {"a", "no"}}},
	} {
		got := overviewBody(t, typedRow(t, foundryType, c.props), 80)
		i := slices.Index(got, "Projects")
		want := []string{"Projects", "none"}
		if c.want != nil {
			want = append([]string{"Projects"}, wantTable([]int{7}, append([][]string{{"Project", "Default"}}, c.want...)...)...)
		}
		if i < 0 || i+len(want) > len(got) || !reflect.DeepEqual(got[i:i+len(want)], want) {
			t.Errorf("%s:\n%s\nwant:\n%s", c.name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

func TestFoundryAttention(t *testing.T) {
	const (
		local   = "▲ local key auth is enabled"
		network = "▲ the account accepts connections from any network"
	)
	tests := []struct {
		name, props string
		want        []string
	}{
		{"nothing", `{"disableLocalAuth":true,"publicNetworkAccess":"Disabled"}`, nil},
		{"local auth off, selected networks", `{"disableLocalAuth":true,"networkAcls":{"defaultAction":"Deny"}}`, nil},
		{"local auth on", `{"disableLocalAuth":false,"networkAcls":{"defaultAction":"Deny"}}`, []string{local}},
		{"local auth missing", `{"networkAcls":{"defaultAction":"Deny"}}`, []string{local}},
		{"any network", `{"disableLocalAuth":true}`, []string{network}},
		{"any network, access in another case", `{"disableLocalAuth":true,"publicNetworkAccess":"enabled","networkAcls":{"defaultAction":"allow"}}`, []string{network}},
		{"public off, default allow", `{"disableLocalAuth":true,"publicNetworkAccess":"Disabled","networkAcls":{"defaultAction":"Allow"}}`, nil},
		{"both", `{}`, []string{local, network}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(foundryAttention(typedRow(t, foundryType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFoundryEndpointsSortedByName(t *testing.T) {
	row := typedRow(t, foundryType, `{"endpoints":{"b":"https://b/","a":"https://a/","c":true}}`)
	want := append([]string{"Endpoints"}, wantTable([]int{4}, []string{"Name", "URL"}, []string{"a", "https://a/"}, []string{"b", "https://b/"}, []string{"c", "—"})...)
	wantLines(t, bodyOfRow(t, row, "Endpoints", 80), append(want, ""))
}

func TestFoundryTabs(t *testing.T) {
	if got := Tabs(foundryType); !reflect.DeepEqual(got, []string{"Overview", "Endpoints", "JSON"}) {
		t.Errorf("account tabs = %v", got)
	}
	if got := Tabs(foundryProjectType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("project tabs = %v", got)
	}
	if k := kindOf("Microsoft.CognitiveServices/Accounts"); k.command != "foundry" || k.attention == nil {
		t.Errorf("account kind = %+v, want foundry with attention", k)
	}
	if k := kindOf("Microsoft.CognitiveServices/Accounts/Projects"); k.command != "foundry" || k.attention != nil {
		t.Errorf("project kind = %+v, want foundry without attention", k)
	}
}

func TestFoundryColors(t *testing.T) {
	v := openedAt(t, "foundry", "", 130, 80)
	wantFg(t, styledLine(t, v, "Status"), "Succeeded", fgGood)
	wantFg(t, styledLine(t, v, "Local key auth"), "off", fgMuted)
	wantFg(t, styledLine(t, v, "Project management"), "on", fgGood)
	on := deliver(t, openedAtRow(t, "").Resize(130, 80), typedRow(t, foundryType, `{}`), nil)
	wantFg(t, styledLine(t, on, "Local key auth"), "on", fgWarn)
}

package views

import (
	"reflect"
	"testing"
)

const afdType = "microsoft.cdn/profiles"

// afdProbed is a profile as the tenant returns it: Standard, Active, log scrubbing off.
const afdProbed = `{
	"resourceGroup": "rg-front", "location": "Global", "subscription": "Dev",
	"kind": "frontdoor", "sku": {"name": "Standard_AzureFrontDoor"}, "tags": {"env": "test"},
	"properties": {
		"frontDoorId": "0a1b2c3d-1111-2222-3333-444455556666", "originResponseTimeoutSeconds": 120,
		"provisioningState": "Succeeded", "resourceState": "Active",
		"logScrubbing": {"state": "Disabled", "scrubbingRules": []}
	}
}`

// afdCDN is a CDN profile: no Front Door ID, no timeout, no log scrubbing.
const afdCDN = `{
	"resourceGroup": "rg-cdn", "location": "Global", "subscription": "Dev",
	"kind": "cdn", "sku": {"name": "Standard_Microsoft"},
	"properties": {"provisioningState": "Succeeded", "resourceState": "Active"}
}`

func TestFrontDoorOverview(t *testing.T) {
	// Inner width 128: two columns of 62 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status                   ● Active · Succeeded", "Log scrubbing   on (2 rules)", 62),
		twoCol("Tier                     Premium_AzureFrontDoor", "Resource group  rg-edge", 62),
		twoCol("Front Door ID            6f1c2a9e-3b4d-4e58-9a07-c1d2e3f40b15", "Subscription    Prod", 62),
		twoCol("Origin response timeout  45 s", "", 62),
		"",
		"Tags",
		"team = web",
		"",
	}
	wantLines(t, bodyOf(t, "afd", "", 134), want)
}

func TestFrontDoorOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status                   ● Active · Succeeded",
		"Tier                     Standard_AzureFrontDoor",
		"Front Door ID            0a1b2c3d-1111-2222-3333-444455556666",
		"Origin response timeout  120 s",
		"Log scrubbing            off",
		"Resource group           rg-front",
		"Subscription             Dev",
		"",
		"Tags",
		"env = test",
		"",
	}
	wantLines(t, overviewBody(t, bodyRow(t, afdType, afdProbed), 80), want)
}

func TestFrontDoorCDNProfile(t *testing.T) {
	got := overviewBody(t, bodyRow(t, afdType, afdCDN), 80)
	for label, want := range map[string]string{
		"Status":                  "● Active · Succeeded",
		"Tier":                    "Standard_Microsoft",
		"Front Door ID":           "—",
		"Origin response timeout": "—",
		"Log scrubbing":           "off",
		"Resource group":          "rg-cdn",
	} {
		if v := valueOf(got, label); v != want {
			t.Errorf("%s = %q, want %q", label, v, want)
		}
	}
}

func TestFrontDoorMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Status                   —",
		"Tier                     —",
		"Front Door ID            —",
		"Origin response timeout  —",
		"Log scrubbing            off",
		"Resource group           —",
		"Subscription             —",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": afdType}, 80), want)
}

func TestFrontDoorLogScrubbing(t *testing.T) {
	for _, c := range []struct{ name, scrubbing, want string }{
		{"one rule", `{"state":"Enabled","scrubbingRules":[{}]}`, "on (1 rule)"},
		{"two rules", `{"state":"Enabled","scrubbingRules":[{},{}]}`, "on (2 rules)"},
		{"on without rules", `{"state":"Enabled","scrubbingRules":[]}`, "on"},
		{"on without a list", `{"state":"Enabled"}`, "on"},
		{"off with rules", `{"state":"Disabled","scrubbingRules":[{}]}`, "off"},
		{"no state", `{"scrubbingRules":[]}`, "off"},
	} {
		row := bodyRow(t, afdType, `{"properties":{"logScrubbing":`+c.scrubbing+`}}`)
		if got := valueOf(overviewBody(t, row, 80), "Log scrubbing"); got != c.want {
			t.Errorf("%s: Log scrubbing = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFrontDoorResourceState(t *testing.T) {
	for state, want := range map[string]string{
		"Disabled": "● Disabled · Succeeded",
		"Creating": "● Creating · Succeeded",
	} {
		row := bodyRow(t, afdType, `{"properties":{"resourceState":"`+state+`","provisioningState":"Succeeded"}}`)
		if got := valueOf(overviewBody(t, row, 80), "Status"); got != want {
			t.Errorf("%s: Status = %q, want %q", state, got, want)
		}
	}
}

func TestFrontDoorTabs(t *testing.T) {
	if got := Tabs(afdType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if k := kindOf("Microsoft.Cdn/Profiles"); k.command != "afd" || k.attention != nil {
		t.Errorf("kindOf in another case = %+v, want afd without attention", k)
	}
}

func TestFrontDoorColors(t *testing.T) {
	v := deliver(t, openedAtRow(t, "").Resize(134, 80), fixture(t, "afd"), nil)
	wantFg(t, styledLine(t, v, "Status"), "Active", fgGood)
	wantFg(t, styledLine(t, v, "Log scrubbing"), "on", fgGood)
	v = deliver(t, openedAtRow(t, "").Resize(134, 80), bodyRow(t, afdType, afdProbed), nil)
	wantFg(t, styledLine(t, v, "Log scrubbing"), "off", fgMuted)
}

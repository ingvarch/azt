package views

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const nsgType = "microsoft.network/networksecuritygroups"

var nsgRuleHead = []string{"Priority", "Name", "Ports", "Protocol", "Source", "Destination", "Action"}

func TestNSGOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status              ● Succeeded", "Resource group  rg-sec", 50),
		twoCol("Rules               7", "Location        norwayeast", 50),
		twoCol("Subnets             2", "Subscription    Prod", 50),
		twoCol("Network interfaces  1", "", 50),
		"",
		"Attached to",
	}
	want = append(want, wantTable([]int{9, 0},
		[]string{"Kind", "Name"},
		[]string{"subnet", "vnet-hub/snet-web"},
		[]string{"subnet", "vnet-spoke/snet-db"},
		[]string{"interface", "nic-web-01"},
	)...)
	want = append(want,
		"",
		"Attention",
		"▲ inbound rule open-rdp allows port 3389 from any address",
		"",
		"Tags",
		"owner = secops",
		"",
	)
	wantLines(t, bodyOf(t, "nsg", "", 110), want)
}

func TestNSGOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status              ● Succeeded",
		"Rules               7",
		"Subnets             2",
		"Network interfaces  1",
		"Resource group      rg-sec",
		"Location            norwayeast",
		"Subscription        Prod",
		"",
	}
	got := bodyOf(t, "nsg", "", 60)
	wantLines(t, got[:len(want)], want)
}

func TestNSGEssentialsDefaults(t *testing.T) {
	for _, c := range []struct {
		name, props, label, want string
	}{
		{"no state", `{}`, "Status", "—"},
		{"no counts", `{}`, "Rules", "0"},
		{"no subnets", `{}`, "Subnets", "0"},
		{"no interfaces", `{}`, "Network interfaces", "0"},
		{"rules count only security rules", `{"securityRules":[{}],"defaultSecurityRules":[{},{}],"subnets":[{},{},{}],"networkInterfaces":[{},{},{},{}]}`, "Rules", "1"},
		{"subnets count only subnets", `{"securityRules":[{}],"defaultSecurityRules":[{},{}],"subnets":[{},{},{}],"networkInterfaces":[{},{},{},{}]}`, "Subnets", "3"},
		{"interfaces count only interfaces", `{"securityRules":[{}],"defaultSecurityRules":[{},{}],"subnets":[{},{},{}],"networkInterfaces":[{},{},{},{}]}`, "Network interfaces", "4"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := valueOf(overviewBody(t, typedRow(t, nsgType, c.props), 80), c.label); got != c.want {
				t.Errorf("%s = %q, want %q", c.label, got, c.want)
			}
		})
	}
}

func TestNSGAttachedTo(t *testing.T) {
	const vnetPrefix = "/subscriptions/s/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/"
	tests := []struct {
		name  string
		props string
		want  [][]string
	}{
		{"nothing at all", `{}`, nil},
		{"empty lists", `{"subnets":[],"networkInterfaces":[]}`, nil},
		{"a subnet", `{"subnets":[{"id":"` + vnetPrefix + `vn1/subnets/sn1"}]}`, [][]string{{"subnet", "vn1/sn1"}}},
		{"a subnet in other case", `{"subnets":[{"id":"` + vnetPrefix + `vn1/SUBNETS/sn1"}]}`, [][]string{{"subnet", "vn1/sn1"}}},
		{"a subnet id of one segment", `{"subnets":[{"id":"lonely"}]}`, [][]string{{"subnet", "lonely"}}},
		{"a subnet id that is no subnet path", `{"subnets":[{"id":"/a/b/c/d"}]}`, [][]string{{"subnet", "d"}}},
		{"a subnet without id", `{"subnets":[{}]}`, [][]string{{"subnet", "—"}}},
		{"an interface", `{"networkInterfaces":[{"id":"/a/networkInterfaces/nic9"}]}`, [][]string{{"interface", "nic9"}}},
		{"an interface without id", `{"networkInterfaces":[{}]}`, [][]string{{"interface", "—"}}},
		{"subnets first, then interfaces, in list order", `{"networkInterfaces":[{"id":"/n/nic-b"},{"id":"/n/nic-a"}],"subnets":[{"id":"` + vnetPrefix + `v2/subnets/s2"},{"id":"` + vnetPrefix + `v1/subnets/s1"}]}`,
			[][]string{{"subnet", "v2/s2"}, {"subnet", "v1/s1"}, {"interface", "nic-b"}, {"interface", "nic-a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bodyOfRow(t, typedRow(t, nsgType, tt.props), "", 120)
			start := slices.Index(got, "Attached to")
			if start < 0 {
				t.Fatalf("no Attached to box:\n%s", strings.Join(got, "\n"))
			}
			var want []string
			if tt.want == nil {
				want = []string{"nothing"}
			} else {
				kind := 4
				for _, r := range tt.want {
					kind = max(kind, len(r[0]))
				}
				want = wantTable([]int{kind, 0}, append([][]string{{"Kind", "Name"}}, tt.want...)...)
			}
			wantLines(t, got[start+1:start+1+len(want)], want)
			if end := got[start+1+len(want)]; end != "" {
				t.Errorf("the box goes on with %q", end)
			}
		})
	}
}

// indexOf is the index of the first line equal to s, or -1.
func TestNSGRulesTab(t *testing.T) {
	widths := []int{8, 12, 8, 8, 26, 24}
	inbound := wantTable(widths, nsgRuleHead,
		[]string{"120", "ops-ssh", "22, 2222", "Tcp", "192.0.2.0/24, 198.51.100.7", "*", "Allow"},
		[]string{"200", "open-rdp", "3389", "Tcp", "Internet", "10.2.1.0/24, 10.2.2.0/24", "Allow"},
		[]string{"300", "web-https", "443", "Tcp", "10.1.0.0/16", "10.2.0.0/24", "Allow"},
		[]string{"1000", "web-http", "80", "Tcp", "10.1.0.0/16", "10.2.0.0/24", "Allow"},
		[]string{"4000", "block-all-in", "*", "*", "Internet", "*", "Deny"},
	)
	outbound := wantTable([]int{8, 6, 5, 8, 11, 13}, nsgRuleHead,
		[]string{"110", "to-dns", "53", "Udp", "*", "168.63.129.16", "Allow"},
		[]string{"150", "to-sql", "1433", "Tcp", "10.2.0.0/24", "Sql", "Allow"},
	)
	defaults := wantTable([]int{8, 29, 5, 8, 17, 14}, nsgRuleHead,
		[]string{"65000", "AllowVnetInBound", "*", "*", "VirtualNetwork", "VirtualNetwork", "Allow"},
		[]string{"65001", "AllowAzureLoadBalancerInBound", "8080", "Tcp", "AzureLoadBalancer", "*", "Allow"},
		[]string{"65500", "DenyAllInBound", "*", "*", "*", "*", "Deny"},
		[]string{"65000", "AllowVnetOutBound", "*", "*", "VirtualNetwork", "VirtualNetwork", "Allow"},
		[]string{"65001", "AllowInternetOutBound", "*", "Udp", "*", "Internet", "Allow"},
		[]string{"65500", "DenyAllOutBound", "*", "*", "*", "*", "Deny"},
	)
	var want []string
	for _, box := range []struct {
		title string
		lines []string
	}{{"Inbound", inbound}, {"Outbound", outbound}, {"Default rules", defaults}} {
		want = append(append(want, box.title), append(box.lines, "")...)
	}
	wantLines(t, bodyOf(t, "nsg", "Rules", 200), want)
}

func TestNSGRulesTabWithoutRules(t *testing.T) {
	for _, props := range []string{`{}`, `{"securityRules":[],"defaultSecurityRules":[]}`} {
		got := bodyOfRow(t, typedRow(t, nsgType, props), "Rules", 100)
		wantLines(t, got, []string{"Inbound", "none", "", "Outbound", "none", "", "Default rules", "none", ""})
	}
}

// nsgRules is a group with one rule of each list, with the properties from the JSON text.
func nsgRules(t *testing.T, ruleProps string) map[string]any {
	t.Helper()
	rule := `[{"name":"r","properties":` + ruleProps + `}]`
	return typedRow(t, nsgType, `{"securityRules":`+rule+`,"defaultSecurityRules":`+rule+`}`)
}

func TestNSGRuleCells(t *testing.T) {
	tests := []struct {
		name, props string
		want        []string
	}{
		{"single forms", `{"direction":"Inbound","priority":7,"protocol":"Tcp","access":"Allow","sourceAddressPrefix":"s1","destinationAddressPrefix":"d1","destinationPortRange":"p1"}`,
			[]string{"7", "r", "p1", "Tcp", "s1", "d1", "Allow"}},
		{"list forms", `{"direction":"Inbound","priority":7,"sourceAddressPrefixes":["s1","s2"],"destinationAddressPrefixes":["d1","d2"],"destinationPortRanges":["p1","p2"]}`,
			[]string{"7", "r", "p1, p2", "—", "s1, s2", "d1, d2", "—"}},
		{"the lists win when both are there", `{"direction":"Inbound","sourceAddressPrefix":"s0","sourceAddressPrefixes":["s1"],"destinationAddressPrefix":"d0","destinationAddressPrefixes":["d1"],"destinationPortRange":"p0","destinationPortRanges":["p1"]}`,
			[]string{"—", "r", "p1", "—", "s1", "d1", "—"}},
		{"empty lists give way to the single forms", `{"direction":"Inbound","sourceAddressPrefix":"s0","sourceAddressPrefixes":[],"destinationAddressPrefix":"d0","destinationAddressPrefixes":[],"destinationPortRange":"p0","destinationPortRanges":[]}`,
			[]string{"—", "r", "p0", "—", "s0", "d0", "—"}},
		{"source ports are not shown", `{"direction":"Inbound","sourcePortRange":"sp","sourcePortRanges":["sp1"],"destinationPortRange":"dp"}`,
			[]string{"—", "r", "dp", "—", "—", "—", "—"}},
		{"nothing", `{"direction":"Inbound"}`, []string{"—", "r", "—", "—", "—", "—", "—"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bodyOfRow(t, nsgRules(t, tt.props), "Rules", 200)
			// Inbound box: title, header, row.
			row := strings.Fields(strings.ReplaceAll(got[2], ", ", ","))
			if want := splitCommas(tt.want); !reflect.DeepEqual(row, want) {
				t.Errorf("row = %q, want cells %q", row, want)
			}
		})
	}
}

// splitCommas is the cells with the spaces after commas removed, as Fields would see them.
func splitCommas(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = strings.ReplaceAll(c, ", ", ",")
	}
	return out
}

func TestNSGRulesSplitByDirection(t *testing.T) {
	props := `{"securityRules":[` +
		`{"name":"in-a","properties":{"direction":"Inbound"}},` +
		`{"name":"out-a","properties":{"direction":"OUTBOUND"}},` +
		`{"name":"in-b","properties":{"direction":"inbound"}},` +
		`{"name":"nowhere","properties":{}},` +
		`{"name":"odd","properties":{"direction":"Sideways"}}],` +
		`"defaultSecurityRules":[{"name":"def-in","properties":{"direction":"Inbound"}}]}`
	got := bodyOfRow(t, typedRow(t, nsgType, props), "Rules", 200)
	box := func(title string) string {
		i := slices.Index(got, title)
		end := i
		for got[end] != "" {
			end++
		}
		return strings.Join(got[i:end], "|")
	}
	for title, want := range map[string][]string{"Inbound": {"in-a", "in-b"}, "Outbound": {"out-a"}} {
		b := box(title)
		for _, name := range []string{"in-a", "in-b", "out-a", "nowhere", "odd", "def-in"} {
			if has := strings.Contains(b, name); has != contains(want, name) {
				t.Errorf("%s box has %s = %v: %s", title, name, has, b)
			}
		}
	}
	if b := box("Default rules"); !strings.Contains(b, "def-in") || strings.Contains(b, "in-a") {
		t.Errorf("Default rules box = %s", b)
	}
}

func TestNSGRulesOrder(t *testing.T) {
	rules := func(parts ...string) string { return "[" + strings.Join(parts, ",") + "]" }
	rule := func(name, dir string, prio int) string {
		return fmt.Sprintf(`{"name":%q,"properties":{"direction":%q,"priority":%d}}`, name, dir, prio)
	}
	props := `{"securityRules":` + rules(rule("c", "Inbound", 1000), rule("a", "Inbound", 99), rule("b", "Inbound", 100), rule("z", "Outbound", 5), rule("y", "Outbound", 4)) +
		`,"defaultSecurityRules":` + rules(rule("o2", "Outbound", 20), rule("i2", "inbound", 20), rule("o1", "Outbound", 10), rule("i1", "Inbound", 30)) + `}`
	got := bodyOfRow(t, typedRow(t, nsgType, props), "Rules", 200)
	names := func(title string) []string {
		var out []string
		for _, l := range got[slices.Index(got, title)+2:] {
			if l == "" {
				break
			}
			out = append(out, strings.Fields(l)[1])
		}
		return out
	}
	for title, want := range map[string][]string{
		"Inbound":       {"a", "b", "c"},
		"Outbound":      {"y", "z"},
		"Default rules": {"i2", "i1", "o1", "o2"},
	} {
		if got := names(title); !reflect.DeepEqual(got, want) {
			t.Errorf("%s order = %v, want %v", title, got, want)
		}
	}
}

func TestNSGActionColors(t *testing.T) {
	v := openedAt(t, "nsg", "Rules", 200, 80)
	wantFg(t, styledLine(t, v, "open-rdp"), "Allow", fgGood)
	wantFg(t, styledLine(t, v, "block-all-in"), "Deny", fgBad)
	other := deliver(t, openedAtRow(t, "Rules").Resize(200, 80), nsgRules(t, `{"direction":"Inbound","access":"allow","priority":1}`), nil)
	wantFg(t, styledLine(t, other, "allow"), "allow", fgGood)
	odd := deliver(t, openedAtRow(t, "Rules").Resize(200, 80), nsgRules(t, `{"direction":"Inbound","access":"DENY","priority":1}`), nil)
	wantFg(t, styledLine(t, odd, "DENY"), "DENY", fgBad)
	weird := deliver(t, openedAtRow(t, "Rules").Resize(200, 80), nsgRules(t, `{"direction":"Inbound","access":"Maybe","priority":1}`), nil)
	wantFg(t, styledLine(t, weird, "Maybe"), "Maybe", "")
}

func TestNSGPortCovers(t *testing.T) {
	tests := []struct {
		rng  string
		port int
		want bool
	}{
		{"*", 22, true},
		{"*", 3389, true},
		{"22", 22, true},
		{"22", 3389, false},
		{"3389", 3389, true},
		{"3389", 22, false},
		{"21", 22, false},
		{"23", 22, false},
		{"1-21", 22, false},
		{"1-22", 22, true},
		{"20-24", 22, true},
		{"22-30", 22, true},
		{"23-30", 22, false},
		{"22-22", 22, true},
		{"0-65535", 22, true},
		{"0-65535", 3389, true},
		{"3000-4000", 3389, true},
		{"3000-3388", 3389, false},
		{"3390-4000", 3389, false},
		{"30-20", 25, false},
		{" 22", 22, false},
		{"22-", 22, false},
		{"-22", 22, false},
		{"a-b", 22, false},
		{"20-b", 22, false},
		{"b-30", 22, false},
		{"22-30-40", 25, false},
		{"ssh", 22, false},
		{"", 22, false},
		{"2 2", 22, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q covers %d", tt.rng, tt.port), func(t *testing.T) {
			if got := portCovers(tt.rng, tt.port); got != tt.want {
				t.Errorf("portCovers(%q, %d) = %v, want %v", tt.rng, tt.port, got, tt.want)
			}
		})
	}
}

// nsgCase is a group (properties as JSON) and the alert lines it must raise.
type nsgCase struct {
	name, props string
	want        []string
}

const attachedProp = `"subnets":[{"id":"/a/b/c"}]`

// inboundRule is an attached group with one inbound Allow rule r1 with the extra properties.
func inboundRule(extra string) string {
	return nsgWithRule(`"direction":"Inbound","access":"Allow",` + extra)
}

func nsgWithRule(props string) string {
	return `{` + attachedProp + `,"securityRules":[{"name":"r1","properties":{` + props + `}}]}`
}

func nsgWarn(what string) []string {
	return []string{"▲ inbound rule r1 allows " + what + " from any address"}
}

func nsgSourceCases() []nsgCase {
	var out []nsgCase
	for _, src := range []string{"*", "0.0.0.0/0", "Internet", "Any", "internet", "INTERNET", "any", "ANY"} {
		out = append(out,
			nsgCase{"source " + src, inboundRule(`"sourceAddressPrefix":"` + src + `","destinationPortRange":"22"`), nsgWarn("port 22")},
			nsgCase{"source in a list " + src, inboundRule(`"sourceAddressPrefixes":["10.0.0.0/8","` + src + `"],"destinationPortRange":"22"`), nsgWarn("port 22")})
	}
	for _, src := range []string{"10.0.0.0/8", "0.0.0.0/1", "VirtualNetwork", "AzureLoadBalancer", "anywhere", ""} {
		out = append(out, nsgCase{"source " + src + " is no one", inboundRule(`"sourceAddressPrefix":"` + src + `","destinationPortRange":"22"`), nil})
	}
	return append(out,
		nsgCase{"no source", inboundRule(`"destinationPortRange":"22"`), nil},
		nsgCase{"the source list wins over a single any", inboundRule(`"sourceAddressPrefix":"*","sourceAddressPrefixes":["10.0.0.0/8"],"destinationPortRange":"22"`), nil},
		nsgCase{"the source list wins over a single address", inboundRule(`"sourceAddressPrefix":"10.0.0.0/8","sourceAddressPrefixes":["*"],"destinationPortRange":"22"`), nsgWarn("port 22")},
		nsgCase{"an empty source list gives way", inboundRule(`"sourceAddressPrefix":"*","sourceAddressPrefixes":[],"destinationPortRange":"22"`), nsgWarn("port 22")},
	)
}

func nsgPortCases() []nsgCase {
	const anySource = `"sourceAddressPrefix":"*",`
	return []nsgCase{
		{"port 22", inboundRule(anySource + `"destinationPortRange":"22"`), nsgWarn("port 22")},
		{"port 3389", inboundRule(anySource + `"destinationPortRange":"3389"`), nsgWarn("port 3389")},
		{"both ports in a list", inboundRule(anySource + `"destinationPortRanges":["22","3389"]`), nsgWarn("port 22 and port 3389")},
		{"both ports in one range", inboundRule(anySource + `"destinationPortRange":"1-4000"`), nsgWarn("port 22 and port 3389")},
		{"both ports in two ranges", inboundRule(anySource + `"destinationPortRanges":["20-25","3000-4000"]`), nsgWarn("port 22 and port 3389")},
		{"port 22 in a range", inboundRule(anySource + `"destinationPortRanges":["80","20-25"]`), nsgWarn("port 22")},
		{"port 3389 in a range", inboundRule(anySource + `"destinationPortRange":"3000-4000"`), nsgWarn("port 3389")},
		{"every port by star", inboundRule(anySource + `"destinationPortRange":"*"`), nsgWarn("every port")},
		{"every port by the full range", inboundRule(anySource + `"destinationPortRange":"0-65535"`), nsgWarn("every port")},
		{"every port from 1", inboundRule(anySource + `"destinationPortRange":"1-65535"`), nsgWarn("every port")},
		{"every port wins over a listed port", inboundRule(anySource + `"destinationPortRanges":["22","*"]`), nsgWarn("every port")},
		{"every port wins over both ports", inboundRule(anySource + `"destinationPortRanges":["22","3389","0-65535"]`), nsgWarn("every port")},
		{"other ports", inboundRule(anySource + `"destinationPortRange":"80"`), nil},
		{"ports around 22", inboundRule(anySource + `"destinationPortRanges":["1-21","23-3388","3390-65535"]`), nil},
		{"malformed ports", inboundRule(anySource + `"destinationPortRanges":["ssh","22-","-22"]`), nil},
		{"no ports", inboundRule(`"sourceAddressPrefix":"*"`), nil},
		{"the port list wins over a single port", inboundRule(anySource + `"destinationPortRange":"22","destinationPortRanges":["80"]`), nil},
		{"the port list wins over another single port", inboundRule(anySource + `"destinationPortRange":"80","destinationPortRanges":["22"]`), nsgWarn("port 22")},
		{"an empty port list gives way", inboundRule(anySource + `"destinationPortRange":"22","destinationPortRanges":[]`), nsgWarn("port 22")},
	}
}

func nsgRuleCases() []nsgCase {
	const open = `"sourceAddressPrefix":"*","destinationPortRange":"22"`
	return []nsgCase{
		{"nothing to report", `{` + attachedProp + `}`, nil},
		{"direction and access in other case", nsgWithRule(`"direction":"inbound","access":"ALLOW",` + open), nsgWarn("port 22")},
		{"an outbound rule", nsgWithRule(`"direction":"Outbound","access":"Allow",` + open), nil},
		{"a deny rule", nsgWithRule(`"direction":"Inbound","access":"Deny",` + open), nil},
		{"a rule without direction", nsgWithRule(`"access":"Allow",` + open), nil},
		{"a rule without access", nsgWithRule(`"direction":"Inbound",` + open), nil},
		{"a default rule", `{` + attachedProp + `,"defaultSecurityRules":[{"name":"r1","properties":{"direction":"Inbound","access":"Allow",` + open + `}}]}`, nil},
		{"a rule without a name", `{` + attachedProp + `,"securityRules":[{"properties":{"direction":"Inbound","access":"Allow",` + open + `}}]}`,
			[]string{"▲ inbound rule — allows port 22 from any address"}},
		{"rules in list order", `{` + attachedProp + `,"securityRules":[` +
			`{"name":"b","properties":{"direction":"Inbound","access":"Allow","sourceAddressPrefix":"*","destinationPortRange":"3389"}},` +
			`{"name":"c","properties":{"direction":"Inbound","access":"Allow","sourceAddressPrefix":"10.0.0.0/8","destinationPortRange":"3389"}},` +
			`{"name":"a","properties":{"direction":"Inbound","access":"Allow","sourceAddressPrefix":"Internet","destinationPortRange":"*"}}]}`,
			[]string{"▲ inbound rule b allows port 3389 from any address", "▲ inbound rule a allows every port from any address"}},
	}
}

func nsgAttachmentCases() []nsgCase {
	return []nsgCase{
		{"not attached, nothing in the lists", `{"subnets":[],"networkInterfaces":[]}`, []string{unattached}},
		{"not attached, no lists", `{}`, []string{unattached}},
		{"attached to a subnet", `{"subnets":[{}]}`, nil},
		{"attached to an interface", `{"networkInterfaces":[{}]}`, nil},
		{"rule warnings come before the unattached one", `{"securityRules":[{"name":"r1","properties":{"direction":"Inbound","access":"Allow","sourceAddressPrefix":"*","destinationPortRange":"22"}}]}`,
			[]string{"▲ inbound rule r1 allows port 22 from any address", unattached}},
	}
}

func TestNSGAttention(t *testing.T) {
	var tests []nsgCase
	for _, cases := range [][]nsgCase{nsgRuleCases(), nsgSourceCases(), nsgPortCases(), nsgAttachmentCases()} {
		tests = append(tests, cases...)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(nsgAttention(typedRow(t, nsgType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

const unattached = "▲ the group is not attached to any subnet or network interface"

func TestNSGAttentionOfEmptyRow(t *testing.T) {
	if got := alertLines(nsgAttention(nil)); !reflect.DeepEqual(got, []string{unattached}) {
		t.Errorf("attention = %q", got)
	}
}

func TestNSGAttentionIsWarning(t *testing.T) {
	for _, a := range nsgAttention(fixture(t, "nsg")) {
		if a.severe {
			t.Errorf("%q is an error", a.text)
		}
	}
	for _, a := range nsgAttention(nil) {
		if a.severe {
			t.Errorf("%q is an error", a.text)
		}
	}
}

func TestNSGAttentionBoxColor(t *testing.T) {
	v := openedAt(t, "nsg", "", 110, 80)
	wantFg(t, styledLine(t, v, "╭ Attention"), "Attention", fgWarn)
}

func TestNSGStatusColor(t *testing.T) {
	wantFg(t, styledLine(t, openedAt(t, "nsg", "", 110, 80), "Status"), "Succeeded", fgGood)
}

func TestNSGTabs(t *testing.T) {
	if got := Tabs(nsgType); !reflect.DeepEqual(got, []string{"Overview", "Rules", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if got := Tabs("Microsoft.Network/NetworkSecurityGroups"); len(got) != 3 {
		t.Errorf("Tabs in another case = %v", got)
	}
	if k := kindOf("Microsoft.Network/NetworkSecurityGroups"); k.command != "nsg" || k.attention == nil {
		t.Errorf("kind = %+v", k)
	}
}

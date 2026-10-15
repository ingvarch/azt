package views

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

var nsgTabs = []tabDef{
	{overviewTab, boxesTab(nsgEssentials, nsgAttached)},
	{"Rules", boxesTab(nsgInbound, nsgOutbound, nsgDefaults)},
	{"JSON", jsonTab},
}

var nsgEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: []string{"properties", "provisioningState"}, format: fmtState},
	{label: "Rules", path: []string{"properties", "securityRules"}, format: fmtCount},
	{label: "Subnets", path: []string{"properties", "subnets"}, format: fmtCount},
	{label: "Network interfaces", path: []string{"properties", "networkInterfaces"}, format: fmtCount},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var nsgAttached = boxDef{title: "Attached to", table: &tableDef{
	list: nsgAttachments,
	cols: []colDef{
		{title: "Kind", path: []string{"kind"}},
		{title: "Name", path: []string{"name"}},
	},
	empty: "nothing",
}}

// nsgAttachments lists the subnets, then the network interfaces, as kind and name.
func nsgAttachments(row map[string]any) []any {
	var out []any
	for _, s := range elems(dig(row, "properties", "subnets")) {
		out = append(out, map[string]any{"kind": "subnet", "name": subnetName(str(s, "id"))})
	}
	for _, n := range elems(dig(row, "properties", "networkInterfaces")) {
		out = append(out, map[string]any{"kind": "interface", "name": lastSegment(str(n, "id"))})
	}
	return out
}

// subnetName is "<virtual network>/<subnet>" for a subnet ID, else its last segment.
func subnetName(id string) string {
	parts := strings.Split(id, "/")
	n := len(parts)
	if n >= 3 && strings.EqualFold(parts[n-2], "subnets") {
		return parts[n-3] + "/" + parts[n-1]
	}
	return lastSegment(id)
}

// nsgKey is a rule property that has a single and a list form.
type nsgKey struct{ one, many string }

var (
	nsgSources = nsgKey{"sourceAddressPrefix", "sourceAddressPrefixes"}
	nsgTargets = nsgKey{"destinationAddressPrefix", "destinationAddressPrefixes"}
	nsgPorts   = nsgKey{"destinationPortRange", "destinationPortRanges"}
)

// values are the list form when it is not empty, else the single form.
func (k nsgKey) values(rule any) []string {
	if l := items(rule, "properties", k.many); len(l) > 0 {
		return l
	}
	if s := str(rule, "properties", k.one); s != "" {
		return []string{s}
	}
	return nil
}

func (k nsgKey) cell(rule any) string { return orDash(strings.Join(k.values(rule), ", ")) }

var nsgRuleCols = []colDef{
	{title: "Priority", path: []string{"properties", "priority"}},
	{title: "Name", path: []string{"name"}},
	{title: "Ports", value: nsgPorts.cell},
	{title: "Protocol", path: []string{"properties", "protocol"}},
	{title: "Source", value: nsgSources.cell},
	{title: "Destination", value: nsgTargets.cell},
	{title: "Action", path: []string{"properties", "access"}, format: fmtAction},
}

var (
	nsgInbound  = nsgRuleBox("Inbound")
	nsgOutbound = nsgRuleBox("Outbound")
	nsgDefaults = boxDef{title: "Default rules", table: &tableDef{
		path: []string{"properties", "defaultSecurityRules"}, cols: nsgRuleCols, order: nsgByDirection,
	}}
	nsgIsInbound = nsgDirection("Inbound")
)

// nsgRuleBox is the box of the security rules of one direction, by priority.
func nsgRuleBox(direction string) boxDef {
	return boxDef{title: direction, table: &tableDef{
		path: []string{"properties", "securityRules"}, cols: nsgRuleCols, keep: nsgDirection(direction), order: nsgByPriority,
	}}
}

func nsgDirection(direction string) func(rule any) bool {
	return func(rule any) bool { return strings.EqualFold(str(rule, "properties", "direction"), direction) }
}

func nsgPriority(rule any) float64 {
	p, _ := number(rule, "properties", "priority")
	return p
}

func nsgByPriority(a, b any) int { return cmp.Compare(nsgPriority(a), nsgPriority(b)) }

// nsgByDirection puts the inbound rules first, then sorts by priority.
func nsgByDirection(a, b any) int {
	return cmp.Or(cmp.Compare(nsgRank(a), nsgRank(b)), nsgByPriority(a, b))
}

func nsgRank(rule any) int {
	if nsgIsInbound(rule) {
		return 0
	}
	return 1
}

// fmtAction draws Allow in good and Deny in bad; other text stays.
func fmtAction(v any) string {
	s := fmtText(v)
	switch strings.ToLower(s) {
	case "allow":
		return goodStyle.Render(s)
	case "deny":
		return badStyle.Render(s)
	}
	return s
}

// nsgAttention lists what needs a look in a network security group row.
func nsgAttention(row map[string]any) []alert {
	var out []alert
	for _, r := range elems(dig(row, "properties", "securityRules")) {
		if a, ok := openRuleAlert(r); ok {
			out = append(out, a)
		}
	}
	if len(elems(dig(row, "properties", "subnets")))+len(elems(dig(row, "properties", "networkInterfaces"))) == 0 {
		out = append(out, alert{text: "the group is not attached to any subnet or network interface"})
	}
	return out
}

// openRuleAlert warns about an inbound Allow rule that lets any address reach port 22, 3389 or every port.
func openRuleAlert(rule any) (alert, bool) {
	if !nsgIsInbound(rule) || !strings.EqualFold(str(rule, "properties", "access"), "Allow") ||
		!slices.ContainsFunc(nsgSources.values(rule), isAnyAddress) {
		return alert{}, false
	}
	what := exposedPorts(nsgPorts.values(rule))
	if what == "" {
		return alert{}, false
	}
	return alert{text: fmt.Sprintf("inbound rule %s allows %s from any address", orDash(str(rule, "name")), what)}, true
}

// nsgAnyAddress are the source prefixes that mean any address.
var nsgAnyAddress = []string{"*", "0.0.0.0/0", "Internet", "Any"}

func isAnyAddress(address string) bool { return containsFold(nsgAnyAddress, address) }

// exposedPorts names what the port ranges open: every port, or port 22 and port 3389 as far as they are covered.
func exposedPorts(ranges []string) string {
	if slices.ContainsFunc(ranges, func(r string) bool { return portCovers(r, 1) && portCovers(r, 65535) }) {
		return "every port"
	}
	var out []string
	for _, port := range []int{22, 3389} {
		if slices.ContainsFunc(ranges, func(r string) bool { return portCovers(r, port) }) {
			out = append(out, "port "+strconv.Itoa(port))
		}
	}
	return strings.Join(out, " and ")
}

// portCovers reports whether a port range is "*", the port, or "low-high" around it.
func portCovers(rng string, port int) bool {
	if rng == "*" {
		return true
	}
	low, high, isRange := strings.Cut(rng, "-")
	if !isRange {
		n, err := strconv.Atoi(rng)
		return err == nil && n == port
	}
	l, errLow := strconv.Atoi(low)
	h, errHigh := strconv.Atoi(high)
	return errLow == nil && errHigh == nil && l <= port && port <= h
}

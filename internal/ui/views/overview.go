package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const overviewTab = "Overview"

// tabDef is one tab of the details view and how it draws a row.
type tabDef struct {
	name string
	// render draws the tab for a body width cells wide; details cuts longer lines.
	render func(row map[string]any, width int) []string
}

var genericTabs = []tabDef{{overviewTab, boxesTab(genericEssentials)}, {"JSON", jsonTab}}

// Tabs lists the tabs the details of a resource of this type have.
func Tabs(resourceType string) []string {
	defs := tabDefs(resourceType)
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.name
	}
	return names
}

var boldStyle = lipgloss.NewStyle().Bold(true)

type pair struct{ key, value string }

// block lists pairs with the muted keys padded to the longest one; a value
// may carry its own styling.
func block(pairs []pair) []string {
	keyWidth := 0
	for _, p := range pairs {
		keyWidth = max(keyWidth, lipgloss.Width(p.key))
	}
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = mutedStyle.Render(p.key) + strings.Repeat(" ", keyWidth-lipgloss.Width(p.key)+2) + p.value
	}
	return out
}

func subscription(row map[string]any) string {
	return orDash(first(str(row, "subscription"), str(row, "subscriptionId")))
}

func tagLines(row map[string]any) []string {
	tags := dig(row, "tags")
	keys := mapKeys(tags)
	if len(keys) == 0 {
		return []string{"none"}
	}
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = fmt.Sprintf("%s = %s", k, text(dig(tags, k)))
	}
	return lines
}

var (
	resourceGroupField = fieldDef{label: "Resource group", path: []string{"resourceGroup"}}
	locationField      = fieldDef{label: "Location", path: []string{"location"}}
	identityField      = fieldDef{label: "Identity", path: []string{"identity", "type"}}
	// The network lines kinds share; the ACL ones are for kinds with properties.networkAcls.
	publicAccessField     = fieldDef{label: "Public network access", value: publicAccessText}
	defaultActionField    = fieldDef{label: "Default action", value: defaultAction}
	ipRulesField          = fieldDef{label: "IP rules", path: aclPath("ipRules"), format: fmtCount}
	vnetRulesField        = fieldDef{label: "Virtual network rules", path: aclPath("virtualNetworkRules"), format: fmtCount}
	privateEndpointsField = fieldDef{label: "Private endpoints", path: []string{"properties", "privateEndpointConnections"}, format: fmtCount}
	subscriptionField     = fieldDef{label: "Subscription", value: subscription}
)

var genericEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: []string{"properties", "provisioningState"}, format: fmtState},
	resourceGroupField,
	locationField,
	subscriptionField,
	{label: "Type", value: func(row map[string]any) string { return orDash(ShortType(str(row, "type"))) }},
	{label: "Kind", path: []string{"kind"}},
	{label: "SKU", value: func(row map[string]any) string {
		return orDash(join(" ", str(row, "sku", "name"), str(row, "sku", "tier")))
	}},
	{label: "Zones", path: []string{"zones"}, format: fmtJoin},
}}

// jsonTab shows the row as JSON; Update has checked that it encodes.
func jsonTab(row map[string]any, _ int) []string { return jsonLines(row) }

func aclPath(key string) []string { return []string{"properties", "networkAcls", key} }

// propPath is the path of a property of a resource.
func propPath(keys ...string) []string { return append([]string{"properties"}, keys...) }

// publicAccessText is publicNetworkAccess; a row without it is Enabled.
func publicAccessText(row map[string]any) string {
	return first(str(row, "properties", "publicNetworkAccess"), "Enabled")
}

// defaultAction is the default action of the network rules; a row without one allows.
func defaultAction(row map[string]any) string {
	return first(str(row, aclPath("defaultAction")...), "Allow")
}

// openToAnyNetwork reports public access with the default action Allow.
func openToAnyNetwork(row map[string]any) bool {
	return publicAccess(row) && strings.EqualFold(defaultAction(row), "Allow")
}

// publicAccess reports whether a resource takes connections from public
// networks: anything but publicNetworkAccess "Disabled".
func publicAccess(row map[string]any) bool {
	return !strings.EqualFold(str(row, "properties", "publicNetworkAccess"), "Disabled")
}

// The two kinds of encryption key a resource can use.
const (
	microsoftKey = "Microsoft-managed key"
	customerKey  = "customer-managed key"
)

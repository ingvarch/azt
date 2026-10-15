package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const overviewTab = "Overview"

// tabDef is one tab of the details view and how it draws a row.
type tabDef struct {
	name   string
	render func(row map[string]any, width int) []string
}

var genericTabs = []tabDef{{overviewTab, genericOverview}, {"JSON", jsonTab}}

const aksType = "microsoft.containerservice/managedclusters"

func tabDefs(resourceType string) []tabDef {
	if strings.EqualFold(resourceType, aksType) {
		return aksTabs
	}
	return genericTabs
}

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

// block lists pairs with the keys padded to the longest one.
func block(pairs []pair) []string {
	keyWidth := 0
	for _, p := range pairs {
		keyWidth = max(keyWidth, lipgloss.Width(p.key))
	}
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.key + strings.Repeat(" ", keyWidth-lipgloss.Width(p.key)) + "  " + p.value
	}
	return out
}

// section is a bold title, the lines and a blank line.
func section(title string, lines []string) []string {
	out := append([]string{boldStyle.Render(title)}, lines...)
	return append(out, "")
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

func genericOverview(row map[string]any, _ int) []string {
	essentials := block([]pair{
		{"Status", field(row, "properties", "provisioningState")},
		{"Resource group", field(row, "resourceGroup")},
		{"Location", field(row, "location")},
		{"Subscription", subscription(row)},
		{"Type", orDash(ShortType(str(row, "type")))},
		{"Kind", field(row, "kind")},
		{"SKU", orDash(join(" ", str(row, "sku", "name"), str(row, "sku", "tier")))},
		{"Zones", joined(row, "zones")},
	})
	return append(section("Essentials", essentials), section("Tags", tagLines(row))...)
}

// jsonTab shows the row as JSON; Update has checked that it encodes.
func jsonTab(row map[string]any, _ int) []string {
	out, _ := indentJSON(row)
	return strings.Split(out, "\n")
}

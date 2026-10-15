package views

import (
	"slices"
	"strings"
)

var docdbTabs = []tabDef{
	{overviewTab, boxesTab(docdbEssentials, docdbNodeGroups, docdbNetwork)},
	{"JSON", jsonTab},
}

var docdbEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", value: docdbStatus},
	{label: "Server version", path: propPath("serverVersion")},
	{label: "Admin", path: propPath("administratorLogin")},
	{label: "Host", value: docdbHost},
	{label: "Replica role", path: propPath("replica", "role")},
	{label: "Earliest restore", path: propPath("earliestRestoreTime"), format: fmtDateTime},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var docdbNodeGroups = boxDef{title: "Node groups", table: &tableDef{
	list: docdbGroups,
	cols: []colDef{
		{title: "Kind", path: []string{"kind"}},
		{title: "SKU", path: []string{"sku"}},
		{title: "Disk size", path: []string{"diskSizeGB"}, format: fmtGB},
		{title: "High availability", path: []string{"enableHa"}, format: fmtEnabled},
		{title: "Nodes", path: []string{"nodeCount"}},
	},
}}

var docdbNetwork = boxDef{title: "Network", fields: []fieldDef{
	{label: "Public network access", value: docdbPublicAccess},
	privateEndpointsField,
}}

func docdbStatus(row map[string]any) string {
	return powerStatus(str(row, propPath("clusterStatus")...), str(row, propPath("provisioningState")...))
}

// docdbHost is the host of the connection string, without the user and the password.
func docdbHost(row map[string]any) string {
	s := str(row, propPath("connectionString")...)
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	} else if _, rest, ok := strings.Cut(s, "://"); ok {
		s = rest
	}
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	return orDash(s)
}

func docdbPublicAccess(row map[string]any) string {
	return fmtEnabled(publicAccessText(row))
}

// docdbGroups are the node groups of a cluster: nodeGroupSpecs, else the one
// group the newer API shape describes, else none. A newer group's high
// availability is its target mode.
func docdbGroups(row map[string]any) []any {
	if specs := elems(dig(row, propPath("nodeGroupSpecs")...)); len(specs) > 0 {
		return specs
	}
	group := map[string]any{
		"kind":       "Shard",
		"sku":        dig(row, propPath("compute", "tier")...),
		"diskSizeGB": dig(row, propPath("storage", "sizeGb")...),
		"enableHa":   dig(row, propPath("highAvailability", "targetMode")...),
		"nodeCount":  dig(row, propPath("sharding", "shardCount")...),
	}
	if !slices.ContainsFunc([]string{"sku", "diskSizeGB", "enableHa", "nodeCount"}, func(k string) bool { return text(group[k]) != "" }) {
		return nil
	}
	return []any{group}
}

// docdbHAOff reports a node group without high availability, or a cluster
// where no group says.
func docdbHAOff(row map[string]any) bool {
	said := false
	for _, g := range docdbGroups(row) {
		switch enabledState(dig(g, "enableHa")) {
		case "off":
			return true
		case dash:
		default:
			said = true
		}
	}
	return !said
}

// docdbAttention lists what needs a look in a cluster row.
func docdbAttention(row map[string]any) []alert {
	var out []alert
	if publicAccess(row) {
		out = append(out, alert{text: "public network access is enabled"})
	}
	if docdbHAOff(row) {
		out = append(out, alert{text: "High availability is off"})
	}
	return out
}

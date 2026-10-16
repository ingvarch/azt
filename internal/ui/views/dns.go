package views

import "strconv"

var dnsTabs = []tabDef{
	{name: overviewTab, render: boxesTab(dnsEssentials, dnsNameServers)},
	{name: "JSON", render: jsonTab},
}

var pdnsTabs = []tabDef{
	{name: overviewTab, render: boxesTab(pdnsEssentials)},
	{name: "JSON", render: jsonTab},
}

var recordSetsField = fieldDef{label: "Record sets", value: countOfMax("numberOfRecordSets", "maxNumberOfRecordSets")}

var dnsEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Zone type", path: []string{"properties", "zoneType"}},
	recordSetsField,
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var dnsNameServers = boxDef{title: "Name servers", table: &tableDef{
	path: []string{"properties", "nameServers"},
	cols: []colDef{{title: "Name server"}},
}}

var pdnsEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: []string{"properties", "provisioningState"}, format: fmtState},
	recordSetsField,
	{label: "Virtual network links", value: countOfMax("numberOfVirtualNetworkLinks", "maxNumberOfVirtualNetworkLinks")},
	{label: "Links with registration", value: countOfMax("numberOfVirtualNetworkLinksWithRegistration", "maxNumberOfVirtualNetworkLinksWithRegistration")},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

func whole(n float64) string { return strconv.FormatFloat(n, 'f', 0, 64) }

// countOfMax shows the properties count and maximum as "<count> of <max>",
// the count alone without a maximum, and a dash without a count.
func countOfMax(count, maximum string) func(row map[string]any) string {
	return func(row map[string]any) string {
		c, ok := number(row, "properties", count)
		if !ok {
			return dash
		}
		if m, ok := number(row, "properties", maximum); ok {
			return whole(c) + " of " + whole(m)
		}
		return whole(c)
	}
}

// dnsAttention lists what needs a look in a public zone row.
func dnsAttention(row map[string]any) []alert { return recordSetsAlert(row) }

// pdnsAttention lists what needs a look in a private zone row.
func pdnsAttention(row map[string]any) []alert {
	var out []alert
	if links, _ := number(row, "properties", "numberOfVirtualNetworkLinks"); links == 0 {
		out = append(out, alert{text: "the zone is linked to no virtual network"})
	}
	return append(out, recordSetsAlert(row)...)
}

// recordSetsAlert warns when the record sets pass 90% of a present maximum.
func recordSetsAlert(row map[string]any) []alert {
	c, _ := number(row, "properties", "numberOfRecordSets")
	m, hasMax := number(row, "properties", "maxNumberOfRecordSets")
	if !hasMax || c*10 <= m*9 {
		return nil
	}
	return []alert{{text: "record sets use " + recordSetsField.text(row)}}
}

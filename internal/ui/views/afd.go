package views

import "strconv"

var afdTabs = []tabDef{
	{name: overviewTab, render: boxesTab(afdEssentials)},
	{name: "JSON", render: jsonTab},
}

var afdEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", value: afdStatus},
	{label: "Tier", path: []string{"sku", "name"}},
	{label: "Front Door ID", path: propPath("frontDoorId")},
	{label: "Origin response timeout", path: propPath("originResponseTimeoutSeconds"), format: fmtUnit("s")},
	{label: "Log scrubbing", value: afdLogScrubbing},
	resourceGroupField,
	subscriptionField,
}}

func afdStatus(row map[string]any) string {
	return powerStatus(str(row, propPath("resourceState")...), str(row, propPath("provisioningState")...))
}

// afdLogScrubbing is on or off, with the number of rules after "on".
func afdLogScrubbing(row map[string]any) string {
	state := enabledState(first(str(row, propPath("logScrubbing", "state")...), "Disabled"))
	n := len(elems(dig(row, propPath("logScrubbing", "scrubbingRules")...)))
	if state != "on" || n == 0 {
		return switchText(state)
	}
	return switchText(state) + " (" + strconv.Itoa(n) + choose(n == 1, " rule)", " rules)")
}

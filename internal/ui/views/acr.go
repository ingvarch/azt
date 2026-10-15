package views

import "strings"

var acrTabs = []tabDef{
	{overviewTab, boxesTab(acrEssentials, acrAccess, acrNetwork, acrPolicies)},
	{"JSON", jsonTab},
}

var acrEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: []string{"properties", "provisioningState"}, format: fmtState},
	{label: "Login server", path: []string{"properties", "loginServer"}},
	{label: "SKU", path: []string{"sku", "name"}},
	{label: "Created", path: []string{"properties", "creationDate"}, format: fmtDate},
	{label: "Zone redundancy", path: []string{"properties", "zoneRedundancy"}, format: fmtEnabled},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var acrAccess = boxDef{title: "Access", fields: []fieldDef{
	{label: "Admin user", path: []string{"properties", "adminUserEnabled"}, format: fmtOnOff},
	{label: "Anonymous pull", path: []string{"properties", "anonymousPullEnabled"}, format: fmtOnOff},
	{label: "Role assignment mode", path: []string{"properties", "roleAssignmentMode"}},
	{label: "Entra ID tokens for ARM", path: acrPolicy("azureADAuthenticationAsArmPolicy", "status"), format: fmtEnabled},
}}

var acrNetwork = boxDef{title: "Network", fields: []fieldDef{
	publicAccessField,
	{label: "Network rule bypass", path: []string{"properties", "networkRuleBypassOptions"}},
	{label: "Default action", path: acrDefaultAction},
	{label: "IP rules", path: []string{"properties", "networkRuleSet", "ipRules"}, format: fmtCount},
	{label: "Data endpoint", path: []string{"properties", "dataEndpointEnabled"}, format: fmtOnOff},
	privateEndpointsField,
}}

var acrPolicies = boxDef{title: "Policies", fields: []fieldDef{
	{label: "Retention", value: func(row map[string]any) string { return acrDaysPolicy(row, "retentionPolicy", "days") }},
	{label: "Soft delete", value: func(row map[string]any) string { return acrDaysPolicy(row, "softDeletePolicy", "retentionDays") }},
	{label: "Trust", path: acrPolicy("trustPolicy", "status"), format: fmtEnabled},
	{label: "Quarantine", path: acrPolicy("quarantinePolicy", "status"), format: fmtEnabled},
	{label: "Export", path: acrPolicy("exportPolicy", "status"), format: fmtEnabled},
	{label: "Encryption", value: acrEncryption},
}}

var acrDefaultAction = []string{"properties", "networkRuleSet", "defaultAction"}

// acrPolicy is the path of a setting of a registry policy.
func acrPolicy(policy, setting string) []string {
	return []string{"properties", "policies", policy, setting}
}

// acrDaysPolicy is the state of a policy with its retention in days.
func acrDaysPolicy(row map[string]any, policy, days string) string {
	status := enabledState(dig(row, acrPolicy(policy, "status")...))
	return withDays(status, str(row, acrPolicy(policy, days)...))
}

func acrEncryption(row map[string]any) string {
	switch enabledState(dig(row, "properties", "encryption", "status")) {
	case "on":
		return "customer-managed key"
	case "off":
		return "Microsoft-managed key"
	}
	return dash
}

// acrAttention lists what needs a look in a registry row.
func acrAttention(row map[string]any) []alert {
	var out []alert
	if on, _ := flag(row, "properties", "adminUserEnabled"); on {
		out = append(out, alert{text: "the admin user is enabled"})
	}
	if on, _ := flag(row, "properties", "anonymousPullEnabled"); on {
		out = append(out, alert{text: "anonymous pull is enabled"})
	}
	if publicAccess(row) && !strings.EqualFold(str(row, acrDefaultAction...), "Deny") {
		out = append(out, alert{text: "the registry accepts connections from any network"})
	}
	return out
}

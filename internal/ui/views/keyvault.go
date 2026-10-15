package views

import (
	"fmt"
	"strings"
)

var kvTabs = []tabDef{
	{overviewTab, boxesTab(kvEssentials, kvAccess, kvProtection, kvNetwork)},
	{"JSON", jsonTab},
}

var kvEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: []string{"properties", "provisioningState"}, format: fmtState},
	{label: "Vault URI", path: []string{"properties", "vaultUri"}},
	{label: "SKU", path: []string{"properties", "sku", "name"}},
	{label: "Tenant ID", path: []string{"properties", "tenantId"}},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var kvAccess = boxDef{title: "Access", fields: []fieldDef{
	{label: "Permission model", value: permissionModel},
	{label: "Deployment", path: []string{"properties", "enabledForDeployment"}, format: fmtOnOff},
	{label: "Disk encryption", path: []string{"properties", "enabledForDiskEncryption"}, format: fmtOnOff},
	{label: "Template deployment", path: []string{"properties", "enabledForTemplateDeployment"}, format: fmtOnOff},
}}

var kvProtection = boxDef{title: "Protection", fields: []fieldDef{
	{label: "Soft delete", value: softDelete},
	{label: "Purge protection", value: func(row map[string]any) string { return switchText(choose(purgeProtected(row), "on", "off")) }},
}}

var kvNetwork = boxDef{title: "Network", fields: []fieldDef{
	{label: "Public network access", path: []string{"properties", "publicNetworkAccess"}},
	{label: "Default action", value: defaultAction},
	{label: "IP rules", path: []string{"properties", "networkAcls", "ipRules"}, format: fmtCount},
	{label: "Virtual network rules", path: []string{"properties", "networkAcls", "virtualNetworkRules"}, format: fmtCount},
	{label: "Private endpoints", path: []string{"properties", "privateEndpointConnections"}, format: fmtCount},
}}

func permissionModel(row map[string]any) string {
	if rbac, _ := flag(row, "properties", "enableRbacAuthorization"); rbac {
		return "Azure RBAC"
	}
	return fmt.Sprintf("Access policies (%s)", fmtCount(dig(row, "properties", "accessPolicies")))
}

// softDelete is "on" with the retention in days, "off", or dash.
func softDelete(row map[string]any) string {
	p := dig(row, "properties")
	state := onOff(p, "enableSoftDelete")
	if state != "on" {
		return switchText(state)
	}
	days := ""
	if d := str(p, "softDeleteRetentionInDays"); d != "" {
		days = d + " days"
	}
	return join(stateSeparator, switchText(state), days)
}

// purgeProtected reports purge protection; a vault without the setting has it off.
func purgeProtected(row map[string]any) bool {
	on, _ := flag(row, "properties", "enablePurgeProtection")
	return on
}

// defaultAction is the default action of the network rules; a vault without one allows.
func defaultAction(row map[string]any) string {
	return first(str(row, "properties", "networkAcls", "defaultAction"), "Allow")
}

// kvAttention lists what needs a look in a vault row.
func kvAttention(row map[string]any) []alert {
	var out []alert
	if !purgeProtected(row) {
		out = append(out, alert{text: "purge protection is off"})
	}
	if !strings.EqualFold(str(row, "properties", "publicNetworkAccess"), "Disabled") && strings.EqualFold(defaultAction(row), "Allow") {
		out = append(out, alert{text: "the vault accepts connections from any network"})
	}
	return out
}

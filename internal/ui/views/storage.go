package views

import "strings"

var storageTabs = []tabDef{
	{name: overviewTab, render: boxesTab(storageEssentials, storageEndpoints, storageSecurity, storageFeatures, storageNetwork)},
	{name: "JSON", render: jsonTab},
}

var storageEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: []string{"properties", "provisioningState"}, format: fmtState},
	{label: "Kind", path: []string{"kind"}},
	{label: "SKU", path: []string{"sku", "name"}},
	{label: "Access tier", path: []string{"properties", "accessTier"}},
	{label: "Primary status", path: []string{"properties", "statusOfPrimary"}, format: fmtState},
	{label: "Created", path: []string{"properties", "creationTime"}, format: fmtDate},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var storageEndpoints = boxDef{title: "Endpoints", extra: func(row map[string]any) []pair {
	endpoints := dig(row, "properties", "primaryEndpoints")
	var out []pair
	for _, name := range mapKeys(endpoints) {
		out = append(out, pair{name, fmtText(dig(endpoints, name))})
	}
	return out
}}

var storageSecurity = boxDef{title: "Security", fields: []fieldDef{
	{label: "Minimum TLS", value: minimumTLS},
	{label: "HTTPS only", path: []string{"properties", "supportsHttpsTrafficOnly"}, format: fmtOnOffOr(true)},
	{label: "Shared key access", path: []string{"properties", "allowSharedKeyAccess"}, format: fmtOnOffOr(true)},
	{label: "Blob public access", path: []string{"properties", "allowBlobPublicAccess"}, format: fmtOnOffOr(false)},
	{label: "OAuth by default", path: []string{"properties", "defaultToOAuthAuthentication"}, format: fmtOnOffOr(false)},
	{label: "Cross-tenant replication", path: []string{"properties", "allowCrossTenantReplication"}, format: fmtOnOffOr(false)},
	{label: "Encryption key", value: storageEncryptionKey},
	{label: "Infrastructure encryption", path: []string{"properties", "encryption", "requireInfrastructureEncryption"}, format: fmtOnOffOr(false)},
}}

var storageFeatures = boxDef{title: "Features", fields: []fieldDef{
	{label: "Data Lake Gen2", path: []string{"properties", "isHnsEnabled"}, format: fmtOnOffOr(false)},
	{label: "SFTP", path: []string{"properties", "isSftpEnabled"}, format: fmtOnOffOr(false)},
	{label: "NFS v3", path: []string{"properties", "isNfsV3Enabled"}, format: fmtOnOffOr(false)},
	{label: "Local users", path: []string{"properties", "isLocalUserEnabled"}, format: fmtOnOffOr(false)},
}}

var storageNetwork = boxDef{title: "Network", fields: []fieldDef{
	publicAccessField,
	defaultActionField,
	{label: "Bypass", path: aclPath("bypass")},
	ipRulesField,
	vnetRulesField,
	privateEndpointsField,
}}

// minimumTLS is the minimum TLS version; an account without one accepts TLS1_0.
func minimumTLS(row map[string]any) string {
	return first(str(row, []string{"properties", "minimumTlsVersion"}...), "TLS1_0")
}

func storageEncryptionKey(row map[string]any) string {
	switch source := str(row, "properties", "encryption", "keySource"); strings.ToLower(source) {
	case "microsoft.storage":
		return microsoftKey
	case "microsoft.keyvault":
		return customerKey
	default:
		return orDash(source)
	}
}

// storageAttention lists what needs a look in a storage account row.
func storageAttention(row map[string]any) []alert {
	var out []alert
	if on, ok := flag(row, []string{"properties", "allowSharedKeyAccess"}...); on || !ok {
		out = append(out, alert{text: "shared key access is allowed"})
	}
	if on, _ := flag(row, []string{"properties", "allowBlobPublicAccess"}...); on {
		out = append(out, alert{text: "blob public access is allowed"})
	}
	switch v := minimumTLS(row); strings.ToUpper(v) {
	case "TLS1_0", "TLS1_1":
		out = append(out, alert{text: "the minimum TLS version is " + v})
	}
	if on, ok := flag(row, []string{"properties", "supportsHttpsTrafficOnly"}...); ok && !on {
		out = append(out, alert{text: "HTTPS is not required"})
	}
	if openToAnyNetwork(row) {
		out = append(out, alert{text: "the account accepts connections from any network"})
	}
	return out
}

package views

import (
	"fmt"
	"strings"
	"time"
)

var pgTabs = []tabDef{
	{overviewTab, boxesTab(pgEssentials, pgStorage, pgAvailability, pgNetwork)},
	{"JSON", jsonTab},
}

var pgEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: pgPath("state"), format: fmtState},
	{label: "Version", value: pgVersion},
	{label: "SKU", value: pgSKU},
	{label: "Server name", path: pgPath("fullyQualifiedDomainName")},
	{label: "Admin", path: pgPath("administratorLogin")},
	{label: "Zone", path: pgPath("availabilityZone")},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var pgStorage = boxDef{title: "Storage and backup", fields: []fieldDef{
	{label: "Size", path: pgPath("storage", "storageSizeGB"), format: fmtGB},
	{label: "Type", path: pgPath("storage", "type")},
	{label: "Tier", path: pgPath("storage", "tier")},
	{label: "IOPS", path: pgPath("storage", "iops")},
	{label: "Auto-grow", path: pgPath("storage", "autoGrow"), format: fmtEnabled},
	{label: "Backup retention", path: pgPath("backup", "backupRetentionDays"), format: fmtUnit("days")},
	{label: "Geo-redundant backup", path: pgPath("backup", "geoRedundantBackup"), format: fmtEnabled},
	{label: "Earliest restore", path: pgPath("backup", "earliestRestoreDate"), format: fmtDateTime},
}}

var pgAvailability = boxDef{title: "High availability", fields: []fieldDef{
	{label: "Mode", path: pgPath("highAvailability", "mode"), format: fmtEnabled},
	{label: "State", path: pgPath("highAvailability", "state")},
	{label: "Standby zone", path: pgPath("highAvailability", "standbyAvailabilityZone")},
	{label: "Replication role", value: pgReplica("role", "replicationRole")},
	{label: "Replica capacity", value: pgReplica("capacity", "replicaCapacity")},
}}

var pgNetwork = boxDef{title: "Network and auth", fields: []fieldDef{
	{label: "Public network access", path: pgPublicAccess, format: fmtEnabled},
	{label: "Delegated subnet", value: pgDelegatedSubnet},
	{label: "Private DNS zone", path: pgPath("network", "privateDnsZoneArmResourceId"), format: fmtLast},
	{label: "Entra ID auth", path: pgPath("authConfig", "activeDirectoryAuth"), format: fmtEnabled},
	{label: "Password auth", path: pgPath("authConfig", "passwordAuth"), format: fmtEnabled},
	{label: "Maintenance window", value: pgMaintenance},
	{label: "Data encryption", value: pgEncryption},
}}

// pgPath is the path of a property of a server.
func pgPath(keys ...string) []string { return append([]string{"properties"}, keys...) }

var pgPublicAccess = pgPath("network", "publicNetworkAccess")

// pgVersion is "<version>.<minor version>", the major version alone without a minor one.
func pgVersion(row map[string]any) string {
	major := str(row, pgPath("version")...)
	if major == "" {
		return dash
	}
	return join(".", major, str(row, pgPath("minorVersion")...))
}

// pgSKU is "<name> (<tier>)", the name alone without a tier.
func pgSKU(row map[string]any) string {
	name := str(row, "sku", "name")
	if name == "" {
		return dash
	}
	if tier := str(row, "sku", "tier"); tier != "" {
		return fmt.Sprintf("%s (%s)", name, tier)
	}
	return name
}

// pgReplica reads a replica setting, from the old key of the server when the replica object lacks it.
func pgReplica(key, old string) func(row map[string]any) string {
	return func(row map[string]any) string {
		return orDash(first(str(row, pgPath("replica", key)...), str(row, pgPath(old)...)))
	}
}

// pgDelegatedSubnet is the subnet of a private server as <network>/<subnet>.
func pgDelegatedSubnet(row map[string]any) string {
	return orDash(subnetName(str(row, pgPath("network", "delegatedSubnetResourceId")...)))
}

// pgMaintenance is the custom maintenance window; a server without one is patched when Azure chooses.
func pgMaintenance(row map[string]any) string {
	window := dig(row, pgPath("maintenanceWindow")...)
	if !strings.EqualFold(str(window, "customWindow"), "Enabled") {
		return "System managed"
	}
	day, _ := number(window, "dayOfWeek")
	hour, _ := number(window, "startHour")
	minute, _ := number(window, "startMinute")
	return fmt.Sprintf("%s %02d:%02d UTC", time.Weekday(int(day)), int(hour), int(minute))
}

func pgEncryption(row map[string]any) string {
	switch kind := str(row, pgPath("dataEncryption", "type")...); strings.ToLower(kind) {
	case "systemmanaged":
		return microsoftKey
	case "azurekeyvault":
		return customerKey
	default:
		return orDash(kind)
	}
}

// pgAttention lists what needs a look in a flexible server row.
func pgAttention(row map[string]any) []alert {
	var out []alert
	switch state := str(row, pgPath("state")...); strings.ToLower(state) {
	case "stopped":
		out = append(out, alert{text: "Stopped: Azure starts it again after seven days"})
	case "disabled", "dropping":
		out = append(out, alert{text: "the server is " + state, severe: true})
	}
	if strings.EqualFold(str(row, pgPublicAccess...), "Enabled") {
		out = append(out, alert{text: "public network access is enabled"})
	}
	return out
}

package views

import (
	"cmp"
	"strings"
)

var vmTabs = []tabDef{
	{name: overviewTab, render: boxesTab(vmEssentials, vmSecurity)},
	{name: "Disks", render: boxesTab(vmOSDisk, vmDataDisks)},
	{name: "Network", render: boxesTab(vmInterfaces)},
	{name: "JSON", render: jsonTab},
}

var vmEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", value: vmStatus},
	{label: "Size", path: []string{"properties", "hardwareProfile", "vmSize"}},
	{label: "OS", value: vmOS},
	{label: "Image", value: vmImage},
	{label: "Computer name", path: []string{"properties", "osProfile", "computerName"}},
	{label: "Admin user", path: []string{"properties", "osProfile", "adminUsername"}},
	{label: "Zones", path: []string{"zones"}, format: fmtJoin},
	{label: "Priority", value: vmPriority},
	{label: "Created", path: []string{"properties", "timeCreated"}, format: fmtDate},
	identityField,
	resourceGroupField,
	locationField,
	subscriptionField,
}}

// vmPower is the power state word of a row, such as "Running", or "".
func vmPower(row map[string]any) string {
	code := strings.TrimPrefix(str(row, "properties", "extended", "instanceView", "powerState", "code"), "PowerState/")
	if code == "" {
		return ""
	}
	return strings.ToUpper(code[:1]) + code[1:]
}

func vmStatus(row map[string]any) string {
	return powerStatus(vmPower(row), str(row, "properties", "provisioningState"))
}

// vmOS is the OS type, with the name and version the instance view reports in parentheses.
func vmOS(row map[string]any) string {
	view := dig(row, "properties", "extended", "instanceView")
	release := join(" ", str(view, "osName"), str(view, "osVersion"))
	if release != "" {
		release = "(" + release + ")"
	}
	return orDash(join(" ", str(row, "properties", "storageProfile", "osDisk", "osType"), release))
}

// vmImage is "<image> <version>" for a gallery image, "<offer> <sku> <version>" for a marketplace one.
func vmImage(row map[string]any) string {
	ref := dig(row, "properties", "storageProfile", "imageReference")
	id := str(ref, "id")
	if gallery := join(" ", segmentAfter(id, "images"), segmentAfter(id, "versions")); gallery != "" {
		return gallery
	}
	return orDash(join(" ", str(ref, "offer"), str(ref, "sku"), first(str(ref, "exactVersion"), str(ref, "version"))))
}

// segmentAfter is the segment of a resource ID after the one named key, or "".
func segmentAfter(id, key string) string {
	parts := strings.Split(id, "/")
	for i, p := range parts[:len(parts)-1] {
		if strings.EqualFold(p, key) {
			return parts[i+1]
		}
	}
	return ""
}

// vmPriority is the priority, Regular when missing, with the eviction policy of a Spot VM.
func vmPriority(row map[string]any) string {
	priority := first(str(row, "properties", "priority"), "Regular")
	if policy := str(row, "properties", "evictionPolicy"); strings.EqualFold(priority, "Spot") && policy != "" {
		return priority + ", evicts to " + policy
	}
	return priority
}

var vmSecurity = boxDef{title: "Security", fields: []fieldDef{
	{label: "Security type", path: []string{"properties", "securityProfile", "securityType"}, format: fmtStandard},
	{label: "Secure boot", path: []string{"properties", "securityProfile", "uefiSettings", "secureBootEnabled"}, format: fmtOnOffOr(false)},
	{label: "vTPM", path: []string{"properties", "securityProfile", "uefiSettings", "vTpmEnabled"}, format: fmtOnOffOr(false)},
	{label: "Encryption at host", path: []string{"properties", "securityProfile", "encryptionAtHost"}, format: fmtOnOffOr(false)},
	{label: "Boot diagnostics", path: []string{"properties", "diagnosticsProfile", "bootDiagnostics", "enabled"}, format: fmtOnOffOr(false)},
	{label: "Password login", value: vmPasswordLogin, show: isLinuxVM},
}}

func fmtStandard(v any) string { return first(text(v), "Standard") }

// passwordLogin reports whether a Linux VM allows password login; ok is false when the row does not say.
func passwordLogin(row map[string]any) (on, ok bool) {
	disabled, ok := flag(row, "properties", "osProfile", "linuxConfiguration", "disablePasswordAuthentication")
	return ok && !disabled, ok
}

// vmPasswordLogin is "on" in warn, "off", or a dash when the row does not say.
func vmPasswordLogin(row map[string]any) string {
	on, ok := passwordLogin(row)
	switch {
	case !ok:
		return dash
	case on:
		return warnStyle.Render("on")
	}
	return "off"
}

func isLinuxVM(row map[string]any) bool {
	return strings.EqualFold(str(row, "properties", "storageProfile", "osDisk", "osType"), "Linux")
}

var vmOSDisk = boxDef{title: "OS disk", fields: []fieldDef{
	{label: "Name", path: osDiskPath("name")},
	{label: "Size", path: osDiskPath("diskSizeGB"), format: fmtGB},
	{label: "Type", path: osDiskPath("managedDisk", "storageAccountType")},
	{label: "Caching", path: osDiskPath("caching")},
	{label: "Delete with VM", path: osDiskPath("deleteOption")},
}}

func osDiskPath(keys ...string) []string {
	return append([]string{"properties", "storageProfile", "osDisk"}, keys...)
}

// fmtGB is a disk size in gigabytes, a dash when missing.
func fmtGB(v any) string { return orDash(gb(text(v))) }

var vmDataDisks = boxDef{title: "Data disks", table: &tableDef{
	path: []string{"properties", "storageProfile", "dataDisks"},
	cols: []colDef{
		{title: "LUN", path: []string{"lun"}},
		{title: "Name", path: []string{"name"}},
		{title: "Size", path: []string{"diskSizeGB"}, format: fmtGB},
		{title: "Type", path: []string{"managedDisk", "storageAccountType"}},
		{title: "Caching", path: []string{"caching"}},
	},
	order: func(a, b any) int {
		x, _ := number(a, "lun")
		y, _ := number(b, "lun")
		return cmp.Compare(x, y)
	},
}}

var vmInterfaces = boxDef{title: "Network interfaces", table: &tableDef{
	list: vmInterfaceRows,
	cols: []colDef{
		{title: "Name", path: []string{"name"}},
		{title: "Primary", path: []string{"primary"}},
		{title: "Delete option", path: []string{"deleteOption"}},
	},
}}

// vmInterfaceRows lists the interfaces as name, primary and delete option; the only interface is primary.
func vmInterfaceRows(row map[string]any) []any {
	nics := elems(dig(row, "properties", "networkProfile", "networkInterfaces"))
	out := make([]any, len(nics))
	for i, n := range nics {
		primary, _ := flag(n, "properties", "primary")
		out[i] = map[string]any{
			"name":         lastSegment(str(n, "id")),
			"primary":      choose(primary || len(nics) == 1, "yes", "no"),
			"deleteOption": str(n, "properties", "deleteOption"),
		}
	}
	return out
}

// vmAttention lists what needs a look in a virtual machine row.
func vmAttention(row map[string]any) []alert {
	var out []alert
	if strings.EqualFold(str(row, "properties", "provisioningState"), "Failed") {
		out = append(out, alert{text: "provisioning failed", severe: true})
	}
	if strings.EqualFold(vmPower(row), "Stopped") {
		out = append(out, alert{text: "the VM is stopped but not deallocated and is still billed"})
	}
	if on, _ := passwordLogin(row); on && isLinuxVM(row) {
		out = append(out, alert{text: "password login is enabled"})
	}
	return out
}

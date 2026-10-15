package views

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const vmType = "microsoft.compute/virtualmachines"

func TestVMOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status         ● Running · Succeeded", "Priority        Regular", 50),
		twoCol("Size           Standard_D4s_v5", "Created         2025-03-14", 50),
		twoCol("OS             Linux (ubuntu 22.04)", "Identity        UserAssigned", 50),
		twoCol("Image          ubuntu-2204-hardened 2024.09.01", "Resource group  rg-web", 50),
		twoCol("Computer name  vm-web-01", "Location        norwayeast", 50),
		twoCol("Admin user     azureuser", "Subscription    Prod", 50),
		twoCol("Zones          1", "", 50),
		"",
		"Security",
		"Security type       Standard",
		"Secure boot         off",
		"vTPM                off",
		"Encryption at host  off",
		"Boot diagnostics    off",
		"Password login      off",
		"",
		"Tags",
		"env = prod",
		"owner = web-team",
		"",
	}
	wantLines(t, bodyOf(t, "vm", "", 110), want)
}

func TestVMOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status          ● Running · Succeeded",
		"Size            Standard_D4s_v5",
		"OS              Linux (ubuntu 22.04)",
		"Image           ubuntu-2204-hardened 2024.09.01",
		"Computer name   vm-web-01",
		"Admin user      azureuser",
		"Zones           1",
		"Priority        Regular",
		"Created         2025-03-14",
		"Identity        UserAssigned",
		"Resource group  rg-web",
		"Location        norwayeast",
		"Subscription    Prod",
		"",
	}
	got := bodyOf(t, "vm", "", 60)
	wantLines(t, got[:len(want)], want)
}

func TestVMDisksTab(t *testing.T) {
	want := []string{
		"OS disk",
		"Name            vm-web-01_OsDisk_1_f3a9",
		"Size            128 GB",
		"Type            Premium_LRS",
		"Caching         ReadWrite",
		"Delete with VM  Delete",
		"",
		"Data disks",
	}
	want = append(want, wantTable([]int{3, 16, 7, 15, 0},
		[]string{"LUN", "Name", "Size", "Type", "Caching"},
		[]string{"0", "vm-web-01-data-c", "256 GB", "StandardSSD_ZRS", "None"},
		[]string{"1", "vm-web-01-data-a", "1024 GB", "Premium_LRS", "ReadOnly"},
		[]string{"2", "vm-web-01-data-b", "64 GB", "Standard_LRS", "ReadWrite"},
	)...)
	want = append(want, "")
	wantLines(t, bodyOf(t, "vm", "Disks", 110), want)
}

func TestVMNetworkTab(t *testing.T) {
	want := []string{"Network interfaces"}
	want = append(want, wantTable([]int{13, 7, 0},
		[]string{"Name", "Primary", "Delete option"},
		[]string{"vm-web-01-nic", "yes", "Delete"},
	)...)
	want = append(want, "")
	wantLines(t, bodyOf(t, "vm", "Network", 110), want)
}

func TestVMJSONTab(t *testing.T) {
	got := plainLines(openedAt(t, "vm", "JSON", 100, 200).Render())[3:]
	for _, l := range []string{" {", `   "name": "vm-web-01",`, `   "properties": {`} {
		if !contains(got, l) {
			t.Errorf("json tab lacks %q", l)
		}
	}
}

func TestVMTabs(t *testing.T) {
	if got := Tabs(vmType); !reflect.DeepEqual(got, []string{"Overview", "Disks", "Network", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if k := kindOf("Microsoft.Compute/VirtualMachines"); k.command != "vm" || k.attention == nil {
		t.Errorf("kind = %+v", k)
	}
}

// vmRow is a virtual machine row from the JSON text of its fields.
func vmRow(t *testing.T, body string) map[string]any {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(body), &row); err != nil {
		t.Fatal(err)
	}
	row["type"] = vmType
	return row
}

// vmValue is the value of an Overview line of the row.
func vmValue(t *testing.T, row map[string]any, label string) string {
	t.Helper()
	return valueOf(overviewBody(t, row, 80), label)
}

// vmView is the details view of the row on the Overview.
func vmView(t *testing.T, row map[string]any) View {
	t.Helper()
	return deliver(t, openedAtRow(t, "").Resize(110, 80), row, nil)
}

func TestVMStatus(t *testing.T) {
	const state = `"properties":{"provisioningState":"Succeeded","extended":{"instanceView":{"powerState":{"code":"`
	for _, c := range []struct{ name, body, want string }{
		{"running", `{` + state + `PowerState/running"}}}}}`, "● Running · Succeeded"},
		{"deallocated", `{` + state + `PowerState/deallocated"}}}}}`, "● Deallocated · Succeeded"},
		{"stopped", `{` + state + `PowerState/stopped"}}}}}`, "● Stopped · Succeeded"},
		{"starting", `{` + state + `PowerState/starting"}}}}}`, "● Starting · Succeeded"},
		{"code without the prefix", `{` + state + `deallocating"}}}}}`, "● Deallocating · Succeeded"},
		{"no power state", `{"properties":{"provisioningState":"Succeeded"}}`, "● Succeeded"},
		{"no provisioning state", `{"properties":{"extended":{"instanceView":{"powerState":{"code":"PowerState/running"}}}}}`, "● Running"},
		{"failed", `{"properties":{"provisioningState":"Failed"}}`, "● Failed"},
		{"nothing", `{}`, "—"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := vmValue(t, vmRow(t, c.body), "Status"); got != c.want {
				t.Errorf("Status = %q, want %q", got, c.want)
			}
		})
	}
}

func TestVMStatusColors(t *testing.T) {
	for _, c := range []struct{ code, word, fg string }{
		{"running", "Running", fgGood},
		{"deallocated", "Deallocated", fgBad},
		{"stopped", "Stopped", fgBad},
		{"starting", "Starting", fgWarn},
	} {
		row := typedRow(t, vmType, `{"provisioningState":"Succeeded","extended":{"instanceView":{"powerState":{"code":"PowerState/`+c.code+`"}}}}`)
		wantFg(t, styledLine(t, vmView(t, row), "Status"), c.word, c.fg)
	}
}

func TestVMOS(t *testing.T) {
	disk := func(typ string) string { return `"storageProfile":{"osDisk":{"osType":"` + typ + `"}}` }
	view := func(name, version string) string {
		return `"extended":{"instanceView":{"osName":"` + name + `","osVersion":"` + version + `"}}`
	}
	for _, c := range []struct{ name, props, want string }{
		{"with the instance view", `{` + disk("Linux") + `,` + view("ubuntu", "22.04") + `}`, "Linux (ubuntu 22.04)"},
		{"windows", `{` + disk("Windows") + `,` + view("windows server 2022", "10.0.20348") + `}`, "Windows (windows server 2022 10.0.20348)"},
		{"off, no instance view", `{` + disk("Linux") + `}`, "Linux"},
		{"deallocated", `{` + disk("Linux") + `,"extended":{"instanceView":{"powerState":{"code":"PowerState/deallocated"}}}}`, "Linux"},
		{"nothing", `{}`, "—"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := vmValue(t, typedRow(t, vmType, c.props), "OS"); got != c.want {
				t.Errorf("OS = %q, want %q", got, c.want)
			}
		})
	}
}

func TestVMImage(t *testing.T) {
	const gallery = "/subscriptions/s/resourceGroups/g/providers/Microsoft.Compute/galleries/gal/images/img-a/versions/1.2.3"
	for _, c := range []struct{ name, ref, want string }{
		{"gallery", `{"id":"` + gallery + `","exactVersion":"9.9.9"}`, "img-a 1.2.3"},
		{"gallery in other case", `{"id":"/x/IMAGES/img-b/VERSIONS/4.5.6"}`, "img-b 4.5.6"},
		{"marketplace takes the exact version", `{"publisher":"pub","offer":"sql2022","sku":"std-gen2","version":"latest","exactVersion":"16.0.1"}`, "sql2022 std-gen2 16.0.1"},
		{"marketplace without an exact version", `{"publisher":"pub","offer":"sql2022","sku":"std-gen2","version":"latest"}`, "sql2022 std-gen2 latest"},
		{"an ID that names no image", `{"id":"/x/y/z"}`, "—"},
		{"an ID that ends at the key", `{"id":"/x/images"}`, "—"},
		{"an ID that ends at the version key", `{"id":"/x/images/img-c/versions"}`, "img-c"},
		{"an empty reference", `{}`, "—"},
	} {
		t.Run(c.name, func(t *testing.T) {
			row := typedRow(t, vmType, `{"storageProfile":{"imageReference":`+c.ref+`}}`)
			if got := vmValue(t, row, "Image"); got != c.want {
				t.Errorf("Image = %q, want %q", got, c.want)
			}
		})
	}
	if got := vmValue(t, typedRow(t, vmType, `{}`), "Image"); got != "—" {
		t.Errorf("Image without a reference = %q", got)
	}
}

func TestVMPriority(t *testing.T) {
	for _, c := range []struct{ props, want string }{
		{`{"priority":"Regular"}`, "Regular"},
		{`{}`, "Regular"},
		{`{"priority":"Spot","evictionPolicy":"Deallocate"}`, "Spot, evicts to Deallocate"},
		{`{"priority":"spot","evictionPolicy":"Delete"}`, "spot, evicts to Delete"},
		{`{"priority":"Spot"}`, "Spot"},
		{`{"priority":"Regular","evictionPolicy":"Delete"}`, "Regular"},
	} {
		if got := vmValue(t, typedRow(t, vmType, c.props), "Priority"); got != c.want {
			t.Errorf("%s: Priority = %q, want %q", c.props, got, c.want)
		}
	}
}

func TestVMEssentialsValues(t *testing.T) {
	row := vmRow(t, `{"zones":["1","3"],"identity":{"type":"SystemAssigned"},
		"properties":{"timeCreated":"2024-11-02T23:59:59Z","hardwareProfile":{"vmSize":"Standard_B2s"},
		"osProfile":{"computerName":"host9","adminUsername":"root9"}}}`)
	for label, want := range map[string]string{
		"Zones": "1, 3", "Identity": "SystemAssigned", "Created": "2024-11-02",
		"Size": "Standard_B2s", "Computer name": "host9", "Admin user": "root9",
	} {
		if got := vmValue(t, row, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
}

func TestVMEssentialsDefaults(t *testing.T) {
	// Identity null, as Azure returns it for a VM without one.
	row := vmRow(t, `{"identity":null,"zones":null,"properties":{}}`)
	for _, label := range []string{"Status", "Size", "OS", "Image", "Computer name", "Admin user", "Zones", "Created", "Identity", "Resource group", "Location", "Subscription"} {
		if got := vmValue(t, row, label); got != "—" {
			t.Errorf("%s = %q, want a dash", label, got)
		}
	}
	if got := vmValue(t, row, "Priority"); got != "Regular" {
		t.Errorf("Priority = %q", got)
	}
}

const (
	vmWindowsTrusted = `{"storageProfile":{"osDisk":{"osType":"Windows"},
		"imageReference":{"publisher":"pub","offer":"sql2022","sku":"enterprise-gen2","version":"latest","exactVersion":"16.0.250101"}},
		"securityProfile":{"securityType":"TrustedLaunch","uefiSettings":{"secureBootEnabled":true,"vTpmEnabled":true}},
		"osProfile":{"windowsConfiguration":{"provisionVMAgent":true}}}`
	linuxPassword = `"storageProfile":{"osDisk":{"osType":"Linux"}},"osProfile":{"linuxConfiguration":{"disablePasswordAuthentication":`
)

func TestVMSecurityOfTrustedLaunchWindowsVM(t *testing.T) {
	got := overviewBody(t, typedRow(t, vmType, vmWindowsTrusted), 80)
	i := slices.Index(got, "Security")
	want := []string{
		"Security type       TrustedLaunch",
		"Secure boot         on",
		"vTPM                on",
		"Encryption at host  off",
		"Boot diagnostics    off",
		"",
	}
	if i < 0 {
		t.Fatalf("no Security box:\n%s", strings.Join(got, "\n"))
	}
	wantLines(t, got[i+1:i+1+len(want)], want)
	if img := vmValue(t, typedRow(t, vmType, vmWindowsTrusted), "Image"); img != "sql2022 enterprise-gen2 16.0.250101" {
		t.Errorf("Image = %q", img)
	}
}

func TestVMSecurityFlags(t *testing.T) {
	for _, c := range []struct{ props, label, want string }{
		{`{"securityProfile":{"uefiSettings":{"secureBootEnabled":true}}}`, "Secure boot", "on"},
		{`{"securityProfile":{"uefiSettings":{"secureBootEnabled":true}}}`, "vTPM", "off"},
		{`{"securityProfile":{"uefiSettings":{"vTpmEnabled":true}}}`, "vTPM", "on"},
		{`{"securityProfile":{"uefiSettings":{"vTpmEnabled":true}}}`, "Secure boot", "off"},
		{`{"securityProfile":{"uefiSettings":{"secureBootEnabled":false,"vTpmEnabled":false}}}`, "Secure boot", "off"},
		{`{"securityProfile":{"encryptionAtHost":true}}`, "Encryption at host", "on"},
		{`{"securityProfile":{"encryptionAtHost":true}}`, "Boot diagnostics", "off"},
		{`{"securityProfile":{"encryptionAtHost":false}}`, "Encryption at host", "off"},
		{`{"diagnosticsProfile":{"bootDiagnostics":{"enabled":true}}}`, "Boot diagnostics", "on"},
		{`{"diagnosticsProfile":{"bootDiagnostics":{"enabled":true}}}`, "Encryption at host", "off"},
		{`{"diagnosticsProfile":{"bootDiagnostics":{"enabled":false}}}`, "Boot diagnostics", "off"},
		{`{"securityProfile":{"securityType":"ConfidentialVM"}}`, "Security type", "ConfidentialVM"},
		{`{"securityProfile":{}}`, "Security type", "Standard"},
		{`{}`, "Security type", "Standard"},
		{`{}`, "Secure boot", "off"},
		{`{}`, "vTPM", "off"},
		{`{}`, "Encryption at host", "off"},
		{`{}`, "Boot diagnostics", "off"},
	} {
		if got := vmValue(t, typedRow(t, vmType, c.props), c.label); got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.props, c.label, got, c.want)
		}
	}
}

func TestVMSecurityFlagColors(t *testing.T) {
	v := vmView(t, typedRow(t, vmType, `{"securityProfile":{"uefiSettings":{"secureBootEnabled":true}}}`))
	wantFg(t, styledLine(t, v, "Secure boot"), "on", fgGood)
	wantFg(t, styledLine(t, v, "vTPM"), "off", fgMuted)
}

func TestVMPasswordLoginColors(t *testing.T) {
	on := vmView(t, typedRow(t, vmType, `{`+linuxPassword+`false}}}`))
	wantFg(t, styledLine(t, on, "Password login"), "on", fgWarn)
	off := vmView(t, typedRow(t, vmType, `{`+linuxPassword+`true}}}`))
	wantFg(t, styledLine(t, off, "Password login"), "off", "")
}

func TestVMPasswordLogin(t *testing.T) {
	for _, c := range []struct{ name, props, want string }{
		{"disabled", `{` + linuxPassword + `true}}}`, "off"},
		{"enabled", `{` + linuxPassword + `false}}}`, "on"},
		{"not said", `{"storageProfile":{"osDisk":{"osType":"Linux"}}}`, "—"},
		{"linux in other case", `{"storageProfile":{"osDisk":{"osType":"LINUX"}},"osProfile":{"linuxConfiguration":{"disablePasswordAuthentication":false}}}`, "on"},
		{"windows", `{"storageProfile":{"osDisk":{"osType":"Windows"}},"osProfile":{"linuxConfiguration":{"disablePasswordAuthentication":false}}}`, ""},
		{"no OS", `{"osProfile":{"linuxConfiguration":{"disablePasswordAuthentication":false}}}`, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := vmValue(t, typedRow(t, vmType, c.props), "Password login"); got != c.want {
				t.Errorf("Password login = %q, want %q", got, c.want)
			}
		})
	}
}

// vmAttentionOf is the lines of the Attention box of the row, none without the box.
func vmAttentionOf(t *testing.T, row map[string]any) []string {
	t.Helper()
	got := overviewBody(t, row, 80)
	i := slices.Index(got, "Attention")
	if i < 0 {
		return nil
	}
	return got[i+1 : i+1+slices.Index(got[i+1:], "")]
}

func TestVMAttention(t *testing.T) {
	power := func(code string) string {
		return `"extended":{"instanceView":{"powerState":{"code":"PowerState/` + code + `"}}}`
	}
	const (
		failed  = "✗ Provisioning failed"
		stopped = "▲ Stopped but not deallocated: still billed"
		login   = "▲ Password login is enabled"
	)
	for _, c := range []struct {
		name, props string
		want        []string
	}{
		{"failed", `{"provisioningState":"Failed"}`, []string{failed}},
		{"failed in other case", `{"provisioningState":"FAILED"}`, []string{failed}},
		{"stopped", `{` + power("stopped") + `}`, []string{stopped}},
		{"password login", `{` + linuxPassword + `false}}}`, []string{login}},
		{"all three, errors first", `{"provisioningState":"Failed",` + power("stopped") + `,` + linuxPassword + `false}}}`, []string{failed, stopped, login}},
		{"deallocated", `{` + power("deallocated") + `}`, nil},
		{"running", `{` + power("running") + `,"provisioningState":"Succeeded"}`, nil},
		{"still creating", `{"provisioningState":"Creating"}`, nil},
		{"password login off", `{` + linuxPassword + `true}}}`, nil},
		{"password login not said", `{"storageProfile":{"osDisk":{"osType":"Linux"}}}`, nil},
		{"windows", `{"storageProfile":{"osDisk":{"osType":"Windows"}},"osProfile":{"windowsConfiguration":{}}}`, nil},
		{"windows with a Linux setting", `{"storageProfile":{"osDisk":{"osType":"Windows"}},"osProfile":{"linuxConfiguration":{"disablePasswordAuthentication":false}}}`, nil},
		{"nothing", `{}`, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := vmAttentionOf(t, typedRow(t, vmType, c.props))
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("attention = %q, want %q", got, c.want)
			}
		})
	}
}

func TestVMAttentionOfFixtureAndEmptyRow(t *testing.T) {
	if got := vmAttentionOf(t, fixture(t, "vm")); got != nil {
		t.Errorf("the fixture needs attention: %q", got)
	}
	if got := vmAttention(nil); len(got) != 0 {
		t.Errorf("attention of nil = %v", got)
	}
}

func TestVMAttentionColors(t *testing.T) {
	failed := vmView(t, typedRow(t, vmType, `{"provisioningState":"Failed"}`))
	wantFg(t, styledLine(t, failed, "╭ Attention"), "Attention", fgBad)
	wantFg(t, styledLine(t, failed, "Provisioning failed"), "Provisioning", fgBad)
	stopped := vmView(t, typedRow(t, vmType, `{"extended":{"instanceView":{"powerState":{"code":"PowerState/stopped"}}}}`))
	wantFg(t, styledLine(t, stopped, "╭ Attention"), "Attention", fgWarn)
	wantFg(t, styledLine(t, stopped, "Stopped but"), "Stopped", fgWarn)
}

func TestVMOSDiskDefaults(t *testing.T) {
	want := []string{
		"OS disk",
		"Name            —",
		"Size            —",
		"Type            —",
		"Caching         —",
		"Delete with VM  —",
		"",
		"Data disks",
		"none",
		"",
	}
	wantLines(t, bodyOfRow(t, typedRow(t, vmType, `{}`), "Disks", 80), want)
}

func TestVMOSDiskValues(t *testing.T) {
	row := typedRow(t, vmType, `{"storageProfile":{"osDisk":{"name":"os1","diskSizeGB":30,"caching":"None","deleteOption":"Detach",
		"managedDisk":{"storageAccountType":"StandardSSD_LRS"}}}}`)
	got := bodyOfRow(t, row, "Disks", 80)
	for label, want := range map[string]string{
		"Name": "os1", "Size": "30 GB", "Type": "StandardSSD_LRS", "Caching": "None", "Delete with VM": "Detach",
	} {
		if v := valueOf(got, label); v != want {
			t.Errorf("%s = %q, want %q", label, v, want)
		}
	}
}

func TestVMDataDisks(t *testing.T) {
	for _, c := range []struct {
		name, disks string
		widths      []int
		want        [][]string
	}{
		{"none", `[]`, nil, nil},
		{"missing", `null`, nil, nil},
		{"LUNs sort as numbers", `[{"lun":10,"name":"d10"},{"lun":2,"name":"d2"},{"lun":9,"name":"d9"}]`, []int{3, 4, 4, 4, 0},
			[][]string{{"2", "d2", "—", "—", "—"}, {"9", "d9", "—", "—", "—"}, {"10", "d10", "—", "—", "—"}}},
		{"a disk without size and type", `[{"lun":3,"name":"d3","caching":"ReadOnly"}]`, []int{3, 4, 4, 4, 0}, [][]string{{"3", "d3", "—", "—", "ReadOnly"}}},
		{"a disk without a size", `[{"lun":4,"name":"d4","caching":"None","managedDisk":{"storageAccountType":"Premium_LRS"}}]`, []int{3, 4, 4, 11, 0},
			[][]string{{"4", "d4", "—", "Premium_LRS", "None"}}},
		{"a disk without a type", `[{"lun":5,"name":"d5","diskSizeGB":32,"caching":"None"}]`, []int{3, 4, 5, 4, 0}, [][]string{{"5", "d5", "32 GB", "—", "None"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := bodyOfRow(t, typedRow(t, vmType, `{"storageProfile":{"dataDisks":`+c.disks+`}}`), "Disks", 100)
			start := slices.Index(got, "Data disks")
			var want []string
			if c.want == nil {
				want = []string{"none"}
			} else {
				want = wantTable(c.widths, append([][]string{{"LUN", "Name", "Size", "Type", "Caching"}}, c.want...)...)
			}
			wantLines(t, got[start+1:start+1+len(want)], want)
		})
	}
}

func TestVMInterfaces(t *testing.T) {
	const nic = `{"id":"/subscriptions/s/resourceGroups/g/providers/Microsoft.Network/networkInterfaces/`
	for _, c := range []struct {
		name, nics string
		want       [][]string
	}{
		{"none", `[]`, nil},
		{"missing", `null`, nil},
		{"one without primary", `[` + nic + `n1"}]`, [][]string{{"n1", "yes", "—"}}},
		{"one that is not primary", `[` + nic + `n1","properties":{"primary":false,"deleteOption":"Detach"}}]`, [][]string{{"n1", "yes", "Detach"}}},
		{"two, one primary", `[` + nic + `n1"},` + nic + `n2","properties":{"primary":true,"deleteOption":"Delete"}}]`,
			[][]string{{"n1", "no", "—"}, {"n2", "yes", "Delete"}}},
		{"two, none primary", `[` + nic + `n1","properties":{"primary":false}},` + nic + `n2"}]`, [][]string{{"n1", "no", "—"}, {"n2", "no", "—"}}},
		{"an interface without ID", `[{}]`, [][]string{{"—", "yes", "—"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := bodyOfRow(t, typedRow(t, vmType, `{"networkProfile":{"networkInterfaces":`+c.nics+`}}`), "Network", 100)
			var want []string
			if c.want == nil {
				want = []string{"none"}
			} else {
				want = wantTable([]int{4, 7, 0}, append([][]string{{"Name", "Primary", "Delete option"}}, c.want...)...)
			}
			wantLines(t, got[1:1+len(want)], want)
		})
	}
}

func TestVMOverviewOfEmptyRow(t *testing.T) {
	want := []string{
		"Essentials",
		"Status          —",
		"Size            —",
		"OS              —",
		"Image           —",
		"Computer name   —",
		"Admin user      —",
		"Zones           —",
		"Priority        Regular",
		"Created         —",
		"Identity        —",
		"Resource group  —",
		"Location        —",
		"Subscription    —",
		"",
		"Security",
		"Security type       Standard",
		"Secure boot         off",
		"vTPM                off",
		"Encryption at host  off",
		"Boot diagnostics    off",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": vmType}, 80), want)
}

func TestVMNetworkTabWithoutInterfaces(t *testing.T) {
	wantLines(t, bodyOfRow(t, typedRow(t, vmType, `{}`), "Network", 80), []string{"Network interfaces", "none", ""})
}

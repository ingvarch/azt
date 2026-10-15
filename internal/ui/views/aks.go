package views

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

var aksTabs = []tabDef{
	{overviewTab, aksOverview},
	{"Node pools", aksPools},
	{"Networking", aksNetworking},
	{"Identity", aksIdentity},
	{"Add-ons", aksAddons},
	{"JSON", jsonTab},
}

// twoColumnWidth is the narrowest body that shows the essentials in two columns.
const twoColumnWidth = 90

func aksOverview(row map[string]any, width int) []string {
	out := section("Essentials", aksEssentials(row, width))
	out = append(out, section("Node pools", poolTable(row, width))...)
	if att := attention(row); len(att) > 0 {
		out = append(out, section("Attention", att)...)
	}
	return append(out, section("Tags", tagLines(row))...)
}

func aksEssentials(row map[string]any, width int) []string {
	p := dig(row, "properties")
	left := []pair{
		{"Status", orDash(join(" · ", str(p, "powerState", "code"), str(p, "provisioningState")))},
		{"Resource group", field(row, "resourceGroup")},
		{"Location", field(row, "location")},
		{"Subscription", subscription(row)},
		{"Node resource group", field(p, "nodeResourceGroup")},
	}
	right := []pair{
		{"Kubernetes", orDash(clusterVersion(p))},
		{"Tier", tier(row)},
		{"API server", apiServer(p)},
		{"Private cluster", choose(private(p), "yes", "no")},
		{"Support plan", field(p, "supportPlan")},
		{"Auto-upgrade", orDash(join(" / ", str(p, "autoUpgradeProfile", "upgradeChannel"), str(p, "autoUpgradeProfile", "nodeOSUpgradeChannel")))},
	}
	if width < twoColumnWidth {
		return block(append(left, right...))
	}
	columns := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(block(left), "\n"), "    ", strings.Join(block(right), "\n"))
	return strings.Split(columns, "\n")
}

func clusterVersion(props any) string {
	return first(str(props, "currentKubernetesVersion"), str(props, "kubernetesVersion"))
}

// private reports a private cluster; a cluster without the setting is public.
func private(props any) bool {
	on, _ := flag(props, "apiServerAccessProfile", "enablePrivateCluster")
	return on
}

func apiServer(props any) string { return orDash(first(str(props, "fqdn"), str(props, "privateFQDN"))) }

// tier is the SKU tier, with the SKU name when it is not Base.
func tier(row map[string]any) string {
	tier, name := str(row, "sku", "tier"), str(row, "sku", "name")
	switch {
	case name == "" || strings.EqualFold(name, "Base"):
		return orDash(tier)
	case tier == "":
		return name
	}
	return tier + " (" + name + ")"
}

// pools returns the node pools of a cluster's properties.
func pools(props any) []map[string]any {
	var out []map[string]any
	for _, p := range elems(dig(props, "agentPoolProfiles")) {
		if m, ok := p.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func poolVersion(pool map[string]any) string {
	return first(str(pool, "currentOrchestratorVersion"), str(pool, "orchestratorVersion"))
}

// poolRange is "min–max" for a pool with autoscaling, else "".
func poolRange(pool map[string]any) string {
	lo, hi := str(pool, "minCount"), str(pool, "maxCount")
	if on, _ := flag(pool, "enableAutoScaling"); !on || lo == "" || hi == "" {
		return ""
	}
	return lo + "–" + hi
}

func poolNodes(pool map[string]any) string {
	count := str(pool, "count")
	if r := poolRange(pool); count != "" && r != "" {
		return count + " (" + r + ")"
	}
	return orDash(count)
}

func poolAutoscaling(pool map[string]any) string {
	on, ok := flag(pool, "enableAutoScaling")
	switch {
	case !ok:
		return dash
	case !on:
		return "off"
	}
	return first(poolRange(pool), "on")
}

func poolState(pool map[string]any) string {
	return orDash(join(" · ", str(pool, "powerState", "code"), notSucceeded(str(pool, "provisioningState"))))
}

func poolTable(row map[string]any, width int) []string {
	ps := pools(dig(row, "properties"))
	if len(ps) == 0 {
		return []string{"none"}
	}
	cols := []column{{title: "Name"}, {title: "Mode"}, {title: "VM size"}, {title: "Nodes"}, {title: "Version"}, {title: "State"}}
	rows := make([][]string, len(ps))
	for i, p := range ps {
		rows[i] = []string{field(p, "name"), field(p, "mode"), field(p, "vmSize"), poolNodes(p), orDash(poolVersion(p)), poolState(p)}
	}
	for i := range cols {
		cols[i].width = lipgloss.Width(cols[i].title)
		for _, r := range rows {
			cols[i].width = max(cols[i].width, lipgloss.Width(r[i]))
		}
	}
	t := newTable(cols)
	t.rows = rows
	t.resize(width, len(rows)+headerLines)
	return strings.Split(t.view(), "\n")
}

// attention lists what needs a look in a cluster row, one "! " line each.
func attention(row map[string]any) []string {
	p := dig(row, "properties")
	var out []string
	if strings.EqualFold(str(p, "powerState", "code"), "Stopped") {
		out = append(out, "! the cluster is stopped")
	}
	if s := notSucceeded(str(p, "provisioningState")); s != "" {
		out = append(out, "! provisioning state is "+s)
	}
	for _, pool := range pools(p) {
		out = append(out, poolAttention(pool, clusterVersion(p))...)
	}
	if disabled, _ := flag(p, "disableLocalAccounts"); !disabled {
		out = append(out, "! local accounts are enabled")
	}
	if !private(p) && len(items(p, "apiServerAccessProfile", "authorizedIPRanges")) == 0 {
		out = append(out, "! the API server accepts connections from any IP address")
	}
	return out
}

func poolAttention(pool map[string]any, controlPlane string) []string {
	name := field(pool, "name")
	var out []string
	if strings.EqualFold(str(pool, "powerState", "code"), "Stopped") {
		out = append(out, fmt.Sprintf("! node pool %s is stopped", name))
	}
	if s := notSucceeded(str(pool, "provisioningState")); s != "" {
		out = append(out, fmt.Sprintf("! node pool %s is %s", name, s))
	}
	if v := poolVersion(pool); v != "" && controlPlane != "" && v != controlPlane {
		out = append(out, fmt.Sprintf("! node pool %s runs %s, the control plane %s", name, v, controlPlane))
	}
	return out
}

func aksPools(row map[string]any, _ int) []string {
	ps := pools(dig(row, "properties"))
	if len(ps) == 0 {
		return []string{"none"}
	}
	var out []string
	for _, p := range ps {
		title := field(p, "name")
		if mode := str(p, "mode"); mode != "" {
			title += " (" + mode + ")"
		}
		out = append(out, section(title, block(poolPairs(p)))...)
	}
	return out
}

func poolPairs(pool map[string]any) []pair {
	disk := join(" ", str(pool, "osDiskType"), diskSize(pool))
	return []pair{
		{"VM size", field(pool, "vmSize")},
		{"Nodes", poolNodes(pool)},
		{"Autoscaling", poolAutoscaling(pool)},
		{"Kubernetes", orDash(poolVersion(pool))},
		{"OS", orDash(join(" ", str(pool, "osType"), str(pool, "osSKU")))},
		{"Node image", field(pool, "nodeImageVersion")},
		{"Zones", joined(pool, "availabilityZones")},
		{"Max pods", field(pool, "maxPods")},
		{"Priority", first(str(pool, "scaleSetPriority"), "Regular")},
		{"OS disk", orDash(disk)},
		{"Subnet", orDash(lastSegment(str(pool, "vnetSubnetID")))},
		{"Max surge", field(pool, "upgradeSettings", "maxSurge")},
		{"State", poolState(pool)},
	}
}

func diskSize(pool map[string]any) string {
	if size := str(pool, "osDiskSizeGB"); size != "" {
		return size + " GB"
	}
	return ""
}

func aksNetworking(row map[string]any, _ int) []string {
	p := dig(row, "properties")
	n := dig(p, "networkProfile")
	plugin := str(n, "networkPlugin")
	if mode := str(n, "networkPluginMode"); mode != "" {
		plugin += " (" + mode + ")"
	}
	return block([]pair{
		{"Network plugin", orDash(plugin)},
		{"Network policy", field(n, "networkPolicy")},
		{"Dataplane", field(n, "networkDataplane")},
		{"Pod CIDR", orDash(first(str(n, "podCidr"), strings.Join(items(n, "podCidrs"), ", ")))},
		{"Service CIDR", orDash(first(str(n, "serviceCidr"), strings.Join(items(n, "serviceCidrs"), ", ")))},
		{"DNS service IP", field(n, "dnsServiceIP")},
		{"Outbound type", field(n, "outboundType")},
		{"Load balancer SKU", field(n, "loadBalancerSku")},
		{"IP families", joined(n, "ipFamilies")},
		{"Private cluster", choose(private(p), "yes", "no")},
		{"Authorized IP ranges", first(strings.Join(items(p, "apiServerAccessProfile", "authorizedIPRanges"), ", "), "any")},
		{"API server", apiServer(p)},
		{"Private FQDN", field(p, "privateFQDN")},
	})
}

func aksIdentity(row map[string]any, _ int) []string {
	p := dig(row, "properties")
	return block([]pair{
		{"Identity", field(row, "identity", "type")},
		{"User-assigned identities", userAssigned(row)},
		{"Kubelet identity", orDash(lastSegment(str(p, "identityProfile", "kubeletidentity", "resourceId")))},
		{"Kubernetes RBAC", onOff(p, "enableRBAC")},
		{"Microsoft Entra ID", entra(p)},
		{"Azure RBAC", onOff(p, "aadProfile", "enableAzureRBAC")},
		{"Admin groups", adminGroups(p)},
		{"Local accounts", localAccounts(p)},
		{"OIDC issuer", onOff(p, "oidcIssuerProfile", "enabled")},
		{"Workload identity", onOff(p, "securityProfile", "workloadIdentity", "enabled")},
		{"Defender", onOff(p, "securityProfile", "defender", "securityMonitoring", "enabled")},
		{"Image cleaner", onOff(p, "securityProfile", "imageCleaner", "enabled")},
	})
}

func userAssigned(row map[string]any) string {
	ids := mapKeys(dig(row, "identity", "userAssignedIdentities"))
	for i, id := range ids {
		ids[i] = lastSegment(id)
	}
	slices.Sort(ids)
	return orDash(strings.Join(ids, ", "))
}

func entra(props any) string {
	managed, _ := flag(props, "aadProfile", "managed")
	return choose(managed, "managed", "off")
}

func adminGroups(props any) string {
	groups, ok := dig(props, "aadProfile", "adminGroupObjectIDs").([]any)
	if !ok {
		return dash
	}
	return strconv.Itoa(len(groups))
}

func localAccounts(props any) string {
	disabled, _ := flag(props, "disableLocalAccounts")
	return choose(disabled, "disabled", "enabled")
}

func aksAddons(row map[string]any, _ int) []string {
	p := dig(row, "properties")
	var pairs []pair
	addons := dig(p, "addonProfiles")
	for _, name := range mapKeys(addons) {
		a := dig(addons, name)
		pairs = append(pairs, pair{name, join(" · ", onOff(a, "enabled"), lastSegment(str(a, "config", "logAnalyticsWorkspaceResourceID")))})
	}
	return block(append(pairs,
		pair{"Web app routing", onOff(p, "ingressProfile", "webAppRouting", "enabled")},
		pair{"KEDA", onOff(p, "workloadAutoScalerProfile", "keda", "enabled")},
		pair{"Vertical pod autoscaler", onOff(p, "workloadAutoScalerProfile", "verticalPodAutoscaler", "enabled")},
		pair{"Service mesh", field(p, "serviceMeshProfile", "mode")},
		pair{"Managed Prometheus", onOff(p, "azureMonitorProfile", "metrics", "enabled")},
		pair{"Cost analysis", onOff(p, "metricsProfile", "costAnalysis", "enabled")},
	))
}

package views

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

var aksTabs = []tabDef{
	{name: overviewTab, render: aksOverview},
	{name: "Node pools", render: aksPools},
	{name: "Networking", render: boxesTab(aksNetworking)},
	{name: "Identity", render: boxesTab(aksIdentity)},
	{name: "Add-ons", render: boxesTab(aksAddons)},
	{name: "Monitoring", key: aksMonitoringKey, draw: aksMonitoring},
	{name: "JSON", render: jsonTab},
}

func aksOverview(row map[string]any, width int) []string {
	return sections(width,
		section{title: "Essentials", lines: aksEssentials(row, innerWidth(width))},
		aksPoolSummary.asSection(row, width),
	)
}

// overviewPools is the title of the box with the node pools.
const overviewPools = "Node pools"

// aksEssentials lists the essentials in a box of the given inner width.
func aksEssentials(row map[string]any, width int) []string {
	p := dig(row, "properties")
	left := []pair{
		{"Status", powerStatus(str(p, "powerState", "code"), str(p, "provisioningState"))},
		{"Resource group", field(row, "resourceGroup")},
		{"Location", field(row, "location")},
		{"Subscription", subscription(row)},
		{"Node resource group", field(p, "nodeResourceGroup")},
	}
	right := []pair{
		{"Kubernetes", orDash(clusterVersion(p))},
		{"Tier", tier(row)},
		{"API server", apiServer(p)},
		{"Private cluster", privateText(p)},
		{"Support plan", field(p, "supportPlan")},
		{"Auto-upgrade", orDash(join(" / ", str(p, "autoUpgradeProfile", "upgradeChannel"), str(p, "autoUpgradeProfile", "nodeOSUpgradeChannel")))},
	}
	return columns(left, right, width)
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

func poolVersion(pool any) string {
	return first(str(pool, "currentOrchestratorVersion"), str(pool, "orchestratorVersion"))
}

// poolRange is "min–max" for a pool with autoscaling, else "".
func poolRange(pool any) string {
	lo, hi := str(pool, "minCount"), str(pool, "maxCount")
	if on, _ := flag(pool, "enableAutoScaling"); !on || lo == "" || hi == "" {
		return ""
	}
	return lo + "–" + hi
}

func poolNodes(pool any) string {
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

func poolState(pool any) string {
	return orDash(join(stateSeparator, str(pool, "powerState", "code"), notSucceeded(str(pool, "provisioningState"))))
}

var aksPoolSummary = boxDef{title: overviewPools, table: &tableDef{
	path: []string{"properties", "agentPoolProfiles"},
	keep: isMap,
	cols: []colDef{
		{title: "Name", path: []string{"name"}},
		{title: "Mode", path: []string{"mode"}},
		{title: "VM size", path: []string{"vmSize"}},
		{title: "Nodes", value: poolNodes},
		{title: "Version", value: func(pool any) string { return orDash(poolVersion(pool)) }},
		{title: "State", value: func(pool any) string { return stateText(poolState(pool)) }},
	},
}}

func isMap(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func isStopped(v any) bool { return strings.EqualFold(str(v, "powerState", "code"), "Stopped") }

// provisioning is the item for a provisioning state that is not Succeeded.
func provisioning(what, state string) []alert {
	if state = notSucceeded(state); state == "" {
		return nil
	}
	return []alert{{text: what + state, severe: strings.EqualFold(state, "Failed")}}
}

// attention lists what needs a look in a cluster row.
func aksAttention(row map[string]any) []alert {
	p := dig(row, "properties")
	var out []alert
	if isStopped(p) {
		out = append(out, alert{text: "the cluster is stopped", severe: true})
	}
	out = append(out, provisioning("provisioning state is ", str(p, "provisioningState"))...)
	for _, pool := range pools(p) {
		out = append(out, poolAttention(pool, clusterVersion(p))...)
	}
	if disabled, _ := flag(p, "disableLocalAccounts"); !disabled {
		out = append(out, alert{text: "local accounts are enabled"})
	}
	if !private(p) && len(items(p, "apiServerAccessProfile", "authorizedIPRanges")) == 0 {
		out = append(out, alert{text: "the API server accepts connections from any IP address"})
	}
	return out
}

func poolAttention(pool map[string]any, controlPlane string) []alert {
	name := field(pool, "name")
	var out []alert
	if isStopped(pool) {
		out = append(out, alert{text: fmt.Sprintf("node pool %s is stopped", name), severe: true})
	}
	out = append(out, provisioning(fmt.Sprintf("node pool %s is ", name), str(pool, "provisioningState"))...)
	if v := poolVersion(pool); v != "" && controlPlane != "" && v != controlPlane {
		out = append(out, alert{text: fmt.Sprintf("node pool %s runs %s, the control plane %s", name, v, controlPlane)})
	}
	return out
}

func aksPools(row map[string]any, width int) []string {
	ps := pools(dig(row, "properties"))
	if len(ps) == 0 {
		return section{title: overviewPools, lines: []string{"none"}}.render(width)
	}
	secs := make([]section, len(ps))
	for i, p := range ps {
		title := field(p, "name")
		if mode := str(p, "mode"); mode != "" {
			title += " · " + mode
		}
		secs[i] = section{title: title, lines: block(poolPairs(p))}
	}
	return sections(width, secs...)
}

func poolPairs(pool map[string]any) []pair {
	disk := join(" ", str(pool, "osDiskType"), gb(str(pool, "osDiskSizeGB")))
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
		{"State", stateText(poolState(pool))},
	}
}

var aksNetworking = boxDef{title: "Networking", fields: []fieldDef{
	{label: "Network plugin", value: networkPlugin},
	{label: "Network policy", path: []string{"properties", "networkProfile", "networkPolicy"}},
	{label: "Dataplane", path: []string{"properties", "networkProfile", "networkDataplane"}},
	{label: "Pod CIDR", value: func(row map[string]any) string { return cidrs(row, "podCidr", "podCidrs") }},
	{label: "Service CIDR", value: func(row map[string]any) string { return cidrs(row, "serviceCidr", "serviceCidrs") }},
	{label: "DNS service IP", path: []string{"properties", "networkProfile", "dnsServiceIP"}},
	{label: "Outbound type", path: []string{"properties", "networkProfile", "outboundType"}},
	{label: "Load balancer SKU", path: []string{"properties", "networkProfile", "loadBalancerSku"}},
	{label: "IP families", path: []string{"properties", "networkProfile", "ipFamilies"}, format: fmtJoin},
	{label: "Private cluster", value: fromProps(privateText)},
	{label: "Authorized IP ranges", value: authorizedRanges},
	{label: "API server", value: fromProps(apiServer)},
	{label: "Private FQDN", path: []string{"properties", "privateFQDN"}},
}}

func networkPlugin(row map[string]any) string {
	n := dig(row, "properties", "networkProfile")
	plugin := str(n, "networkPlugin")
	if mode := str(n, "networkPluginMode"); mode != "" {
		plugin += " (" + mode + ")"
	}
	return orDash(plugin)
}

// cidrs is the single CIDR of the network profile, else the list of them.
func cidrs(row map[string]any, one, many string) string {
	n := dig(row, "properties", "networkProfile")
	return orDash(first(str(n, one), strings.Join(items(n, many), ", ")))
}

func authorizedRanges(row map[string]any) string {
	return first(strings.Join(items(row, "properties", "apiServerAccessProfile", "authorizedIPRanges"), ", "), "any")
}

// fromProps applies f to the properties of a row.
func fromProps(f func(any) string) func(row map[string]any) string {
	return func(row map[string]any) string { return f(dig(row, "properties")) }
}

// privateText is "yes" for a private cluster, else "no".
func privateText(props any) string { return choose(private(props), "yes", "no") }

// rawOnOff reads a flag as on, off or dash, without color.
func rawOnOff(path ...string) func(row map[string]any) string {
	return func(row map[string]any) string { return onOff(row, path...) }
}

var aksIdentity = boxDef{title: "Identity", fields: []fieldDef{
	identityField,
	{label: "User-assigned identities", value: userAssigned},
	{label: "Kubelet identity", path: []string{"properties", "identityProfile", "kubeletidentity", "resourceId"}, format: fmtLast},
	{label: "Kubernetes RBAC", value: rawOnOff("properties", "enableRBAC")},
	{label: "Microsoft Entra ID", value: fromProps(entra)},
	{label: "Azure RBAC", value: rawOnOff("properties", "aadProfile", "enableAzureRBAC")},
	{label: "Admin groups", value: fromProps(adminGroups)},
	{label: "Local accounts", value: fromProps(localAccounts)},
	{label: "OIDC issuer", value: rawOnOff("properties", "oidcIssuerProfile", "enabled")},
	{label: "Workload identity", value: rawOnOff("properties", "securityProfile", "workloadIdentity", "enabled")},
	{label: "Defender", value: rawOnOff("properties", "securityProfile", "defender", "securityMonitoring", "enabled")},
	{label: "Image cleaner", value: rawOnOff("properties", "securityProfile", "imageCleaner", "enabled")},
}}

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

// localAccounts is "disabled", or "enabled" in warn.
func localAccounts(props any) string {
	if disabled, _ := flag(props, "disableLocalAccounts"); disabled {
		return "disabled"
	}
	return warnStyle.Render("enabled")
}

var aksAddons = boxDef{title: "Add-ons", extra: addonProfiles, fields: []fieldDef{
	{label: "Web app routing", path: []string{"properties", "ingressProfile", "webAppRouting", "enabled"}, format: fmtOnOff},
	{label: "KEDA", path: []string{"properties", "workloadAutoScalerProfile", "keda", "enabled"}, format: fmtOnOff},
	{label: "Vertical pod autoscaler", path: []string{"properties", "workloadAutoScalerProfile", "verticalPodAutoscaler", "enabled"}, format: fmtOnOff},
	{label: "Service mesh", path: []string{"properties", "serviceMeshProfile", "mode"}},
	{label: "Managed Prometheus", path: []string{"properties", "azureMonitorProfile", "metrics", "enabled"}, format: fmtOnOff},
	{label: "Cost analysis", path: []string{"properties", "metricsProfile", "costAnalysis", "enabled"}, format: fmtOnOff},
}}

// addonProfiles lists the add-ons the cluster has, whatever their names.
func addonProfiles(row map[string]any) []pair {
	var pairs []pair
	addons := dig(row, "properties", "addonProfiles")
	for _, name := range mapKeys(addons) {
		pairs = append(pairs, pair{name, addonState(dig(addons, name))})
	}
	return pairs
}

// addonState is the add-on as on or off, with its Log Analytics workspace when it has one.
func addonState(addon any) string {
	return join(stateSeparator, switchText(onOff(addon, "enabled")), lastSegment(str(addon, "config", "logAnalyticsWorkspaceResourceID")))
}

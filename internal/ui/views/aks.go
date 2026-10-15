package views

import (
	"fmt"
	"image/color"
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

const (
	// twoColumnWidth is the narrowest box inside that shows the essentials in two columns.
	twoColumnWidth = 90
	// columnGap is the number of cells between the two columns.
	columnGap = 4
)

func aksOverview(row map[string]any, width int) []string {
	secs := []section{
		{title: "Essentials", lines: aksEssentials(row, innerWidth(width))},
		{title: overviewPools, lines: poolTable(row)},
	}
	if alerts := attention(row); len(alerts) > 0 {
		secs = append(secs, alertSection(alerts))
	}
	return sections(width, append(secs, section{title: "Tags", lines: tagLines(row)})...)
}

// overviewPools is the title of the box with the node pools.
const overviewPools = "Node pools"

// aksEssentials lists the essentials in a box of the given inner width.
func aksEssentials(row map[string]any, width int) []string {
	p := dig(row, "properties")
	left := []pair{
		{"Status", stateText(orDash(join(stateSeparator, str(p, "powerState", "code"), str(p, "provisioningState"))))},
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
	return twoColumns(block(left), block(right), width)
}

// twoColumns puts the lines side by side in a width split between them; a
// line longer than its column ends with "…".
func twoColumns(left, right []string, width int) []string {
	leftWidth := (width - columnGap) / 2
	rightWidth := width - columnGap - leftWidth
	out := make([]string, max(len(left), len(right)))
	for i := range out {
		out[i] = fitCell(cellAt(left, i), leftWidth) + strings.Repeat(" ", columnGap) + ellipsize(cellAt(right, i), rightWidth)
	}
	return out
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
	return orDash(join(stateSeparator, str(pool, "powerState", "code"), notSucceeded(str(pool, "provisioningState"))))
}

func poolTable(row map[string]any) []string {
	ps := pools(dig(row, "properties"))
	if len(ps) == 0 {
		return []string{"none"}
	}
	cols := []column{{title: "Name"}, {title: "Mode"}, {title: "VM size"}, {title: "Nodes"}, {title: "Version"}, {title: "State", state: true}}
	rows := make([][]string, len(ps))
	for i, p := range ps {
		rows[i] = []string{field(p, "name"), field(p, "mode"), field(p, "vmSize"), poolNodes(p), orDash(poolVersion(p)), poolState(p)}
	}
	return summaryTable(cols, rows)
}

// alert is an item of the Attention box: an error, or else a warning.
type alert struct {
	text   string
	severe bool
}

func (a alert) tone() color.Color {
	if a.severe {
		return bad
	}
	return warn
}

// line is the item with its symbol: "✗" for an error, "▲" for a warning.
func (a alert) line() string {
	if a.severe {
		return "✗ " + a.text
	}
	return "▲ " + a.text
}

func (a alert) render() string { return lipgloss.NewStyle().Foreground(a.tone()).Render(a.line()) }

// alertSection is the Attention box; it is bad when an item is an error, else warn.
func alertSection(alerts []alert) section {
	lines := make([]string, len(alerts))
	tone := warn
	for i, a := range alerts {
		lines[i] = a.render()
		if a.severe {
			tone = bad
		}
	}
	return section{title: "Attention", lines: lines, tone: tone}
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
func attention(row map[string]any) []alert {
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
		{"State", stateText(poolState(pool))},
	}
}

func diskSize(pool map[string]any) string {
	if size := str(pool, "osDiskSizeGB"); size != "" {
		return size + " GB"
	}
	return ""
}

func aksNetworking(row map[string]any, width int) []string {
	p := dig(row, "properties")
	n := dig(p, "networkProfile")
	plugin := str(n, "networkPlugin")
	if mode := str(n, "networkPluginMode"); mode != "" {
		plugin += " (" + mode + ")"
	}
	return sections(width, section{title: "Networking", lines: block([]pair{
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
	})})
}

func aksIdentity(row map[string]any, width int) []string {
	p := dig(row, "properties")
	return sections(width, section{title: "Identity", lines: block([]pair{
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
	})})
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

// localAccounts is "disabled", or "enabled" in warn.
func localAccounts(props any) string {
	if disabled, _ := flag(props, "disableLocalAccounts"); disabled {
		return "disabled"
	}
	return warnStyle.Render("enabled")
}

func aksAddons(row map[string]any, width int) []string {
	p := dig(row, "properties")
	var pairs []pair
	addons := dig(p, "addonProfiles")
	for _, name := range mapKeys(addons) {
		a := dig(addons, name)
		pairs = append(pairs, pair{name, join(" · ", switchText(onOff(a, "enabled")), lastSegment(str(a, "config", "logAnalyticsWorkspaceResourceID")))})
	}
	fixed := []pair{
		{"Web app routing", onOff(p, "ingressProfile", "webAppRouting", "enabled")},
		{"KEDA", onOff(p, "workloadAutoScalerProfile", "keda", "enabled")},
		{"Vertical pod autoscaler", onOff(p, "workloadAutoScalerProfile", "verticalPodAutoscaler", "enabled")},
		{"Service mesh", field(p, "serviceMeshProfile", "mode")},
		{"Managed Prometheus", onOff(p, "azureMonitorProfile", "metrics", "enabled")},
		{"Cost analysis", onOff(p, "metricsProfile", "costAnalysis", "enabled")},
	}
	for i := range fixed {
		fixed[i].value = switchText(fixed[i].value)
	}
	return sections(width, section{title: "Add-ons", lines: block(append(pairs, fixed...))})
}

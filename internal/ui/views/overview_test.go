package views

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
)

// fixture reads a Resource Graph row from testdata.
func fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatal(err)
	}
	return row
}

// plainLines returns the lines of s without styling or trailing spaces.
func plainLines(s string) []string {
	lines := strings.Split(plain(s), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// openedAt returns a details view of the fixture, loaded and sized, on the tab.
func openedAt(t *testing.T, name, tab string, width, height int) View {
	t.Helper()
	row := fixture(t, name)
	v := NewDetailsAt("s1", azure.Resource{ID: "/r/x", Name: "x"}, tab).Resize(width, height)
	return deliver(t, v, row, nil)
}

// unbox turns boxed sections into the title and the lines of each, with a
// blank line after each; the blank lines between boxes and the margin go.
func unbox(lines []string) []string {
	var out []string
	for _, l := range lines {
		l = strings.TrimPrefix(l, " ")
		switch {
		case strings.HasPrefix(l, "╭ "):
			title := strings.TrimRight(strings.TrimPrefix(l, "╭ "), "╮─")
			out = append(out, strings.TrimSpace(title))
		case strings.HasPrefix(l, "│ "):
			out = append(out, strings.TrimRight(strings.TrimSuffix(strings.TrimPrefix(l, "│ "), "│"), " "))
		case strings.HasPrefix(l, "╰"):
			out = append(out, "")
		case l != "":
			out = append(out, l)
		}
	}
	return out
}

// renderBody is the body of a rendered details view, unboxed.
func renderBody(v View) []string { return unbox(plainLines(v.Render())[3:]) }

// bodyOf returns the unboxed body of a fixture on a pane of this width.
func bodyOf(t *testing.T, name, tab string, width int) []string {
	t.Helper()
	return renderBody(openedAt(t, name, tab, width, 200))
}

// styledLine returns the first rendered line whose text contains sub.
func styledLine(t *testing.T, v View, sub string) string {
	t.Helper()
	for _, l := range strings.Split(v.Render(), "\n") {
		if strings.Contains(plain(l), sub) {
			return l
		}
	}
	t.Fatalf("no line with %q:\n%s", sub, plain(v.Render()))
	return ""
}

// valueAttrs are the attributes of the last word of the line that contains sub.
func valueAttrs(t *testing.T, v View, sub string) attrs {
	t.Helper()
	line := styledLine(t, v, sub)
	words := strings.Fields(strings.Trim(plain(line), "│ "))
	return attrsOf(t, line, words[len(words)-1])
}

func wantLines(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTabs(t *testing.T) {
	aks := []string{"Overview", "Node pools", "Networking", "Identity", "Add-ons", "Monitoring", "JSON"}
	other := []string{"Overview", "JSON"}
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"microsoft.containerservice/managedclusters", aks},
		{"Microsoft.ContainerService/managedClusters", aks},
		{"MICROSOFT.CONTAINERSERVICE/MANAGEDCLUSTERS", aks},
		{"microsoft.web/sites", other},
		{"microsoft.containerservice/fleets", other},
		{"", other},
	} {
		if got := Tabs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Tabs(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestGenericOverview(t *testing.T) {
	// Inner width 94: two columns of 45 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status          ● Succeeded", "Type   web/sites", 45),
		twoCol("Resource group  rg-web", "Kind   app,linux", 45),
		twoCol("Location        northeurope", "SKU    P1v3 PremiumV3", 45),
		twoCol("Subscription    Prod", "Zones  1, 2", 45),
		"",
		"Tags",
		"cost = 42",
		"team = web",
		"",
	}
	wantLines(t, bodyOf(t, "webapp", "", 100), want)
}

func TestGenericOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status          ● Succeeded",
		"Resource group  rg-web",
		"Location        northeurope",
		"Subscription    Prod",
		"Type            web/sites",
		"Kind            app,linux",
		"SKU             P1v3 PremiumV3",
		"Zones           1, 2",
		"",
		"Tags",
		"cost = 42",
		"team = web",
		"",
	}
	wantLines(t, bodyOf(t, "webapp", "", 80), want)
	// Inner width 89 is one column, 90 is two.
	if got := bodyOf(t, "webapp", "", 95); got[1] != want[1] {
		t.Errorf("inner width 89: line 1 = %q", got[1])
	}
	if got := bodyOf(t, "webapp", "", 96); !strings.Contains(got[1], "Type") {
		t.Errorf("inner width 90: line 1 = %q", got[1])
	}
}

func TestGenericOverviewMissingFields(t *testing.T) {
	v := deliver(t, NewDetails("s1", azure.Resource{ID: "/r/x", Name: "x"}).Resize(80, 40), map[string]any{"subscriptionId": "s9"}, nil)
	want := []string{
		"Essentials",
		"Status          —",
		"Resource group  —",
		"Location        —",
		"Subscription    s9",
		"Type            —",
		"Kind            —",
		"SKU             —",
		"Zones           —",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, renderBody(v)[:len(want)], want)
}

func TestGenericOverviewSkuParts(t *testing.T) {
	for _, c := range []struct {
		sku  map[string]any
		want string
	}{
		{map[string]any{"name": "B1"}, "SKU             B1"},
		{map[string]any{"tier": "Basic"}, "SKU             Basic"},
		{map[string]any{}, "SKU             —"},
	} {
		row := map[string]any{"sku": c.sku}
		v := deliver(t, NewDetails("s1", azure.Resource{ID: "/r/x"}).Resize(80, 40), row, nil)
		if got := renderBody(v); !contains(got, c.want) {
			t.Errorf("sku %v: lacks %q:\n%s", c.sku, c.want, strings.Join(got, "\n"))
		}
	}
}

func contains(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

var (
	aksPoolTable = []string{
		"Name    Mode    VM size          Nodes    Version  State",
		"system  System  Standard_D4s_v5  3 (2–5)  1.30.4   ● Running",
		"work    User    Standard_E8s_v5  4        1.29.8   ● Stopped · Updating",
	}
	aksAttentionLines = []string{
		"Attention",
		"✗ node pool work is stopped",
		"▲ node pool work is Updating",
		"▲ node pool work runs 1.29.8, the control plane 1.30.4",
		"▲ local accounts are enabled",
		"▲ the API server accepts connections from any IP address",
		"",
	}
	aksTags = []string{"Tags", "env = dev", "owner = team-a", ""}
)

func overviewLines(essentials []string) []string { return overviewWithPools(essentials, aksPoolTable) }

// overviewWithPools is overviewLines with the given node pool table.
func overviewWithPools(essentials, pools []string) []string {
	out := append([]string{"Essentials"}, essentials...)
	out = append(out, "", "Node pools")
	out = append(out, pools...)
	out = append(out, "")
	out = append(out, aksAttentionLines...)
	return append(out, aksTags...)
}

// twoCol joins two column lines the way the essentials draw them.
func twoCol(left, right string, leftWidth int) string {
	return strings.TrimRight(fmt.Sprintf("%-*s    %s", leftWidth, left, right), " ")
}

func TestAKSOverviewTwoColumns(t *testing.T) {
	left := []string{
		"Status               ● Running · Succeeded",
		"Resource group       rg-app",
		"Location             westeurope",
		"Subscription         Dev",
		"Node resource group  MC_rg-app_aks-demo_westeurope",
		"",
	}
	right := []string{
		"Kubernetes       1.30.4",
		"Tier             Standard",
		"API server       aks-demo-abc.hcp.westeurope.azmk8s.io",
		"Private cluster  no",
		"Support plan     KubernetesOfficial",
		"Auto-upgrade     patch / NodeImage",
	}
	// The box is 118 cells wide, 114 inside: two columns of 55 and a gap of 4.
	essentials := make([]string, len(left))
	for i := range left {
		essentials[i] = twoCol(left[i], right[i], 55)
	}
	wantLines(t, bodyOf(t, "aks", "", 120), overviewLines(essentials))
}

func TestAKSOverviewTwoColumnsOnceEverythingFits(t *testing.T) {
	// A pane 6 cells wider than the box inside it. The API server line needs
	// 54 cells and the node resource group line 50: inner width 111 gives
	// columns of 53 and 54, inner width 110 two of 53.
	one := bodyOf(t, "aks", "", 116)
	if one[6] != "Kubernetes           1.30.4" {
		t.Errorf("inner width 110: not one column:\n%s", strings.Join(one, "\n"))
	}
	two := bodyOf(t, "aks", "", 117)
	if !strings.Contains(two[3], "API server       aks-demo-abc.hcp.westeurope.azmk8s.io") {
		t.Errorf("inner width 111: not two columns:\n%s", strings.Join(two, "\n"))
	}
}

func TestAKSOverviewTakesOneColumnRatherThanCut(t *testing.T) {
	// Inner width 90 would give columns of 43: the API server and the node
	// resource group do not fit, so the essentials stay in one column.
	got := bodyOf(t, "aks", "", 96)
	want := []string{
		"Status               ● Running · Succeeded",
		"Resource group       rg-app",
		"Location             westeurope",
		"Subscription         Dev",
		"Node resource group  MC_rg-app_aks-demo_westeurope",
		"Kubernetes           1.30.4",
		"Tier                 Standard",
		"API server           aks-demo-abc.hcp.westeurope.azmk8s.io",
		"Private cluster      no",
		"Support plan         KubernetesOfficial",
		"Auto-upgrade         patch / NodeImage",
	}
	wantLines(t, got[1:12], want)
}

func TestAKSOverviewOneColumn(t *testing.T) {
	essentials := []string{
		"Status               ● Running · Succeeded",
		"Resource group       rg-app",
		"Location             westeurope",
		"Subscription         Dev",
		"Node resource group  MC_rg-app_aks-demo_westeurope",
		"Kubernetes           1.30.4",
		"Tier                 Standard",
		"API server           aks-demo-abc.hcp.westeurope.azmk8s.io",
		"Private cluster      no",
		"Support plan         KubernetesOfficial",
		"Auto-upgrade         patch / NodeImage",
	}
	head := []string{"Name", "Mode", "VM size", "Nodes", "Version", "State"}
	// Inside 64 the longest pool columns are cut to 14 cells.
	pools70 := wantTable([]int{6, 6, 14, 7, 7}, head,
		[]string{"system", "System", "Standard_D4s_…", "3 (2–5)", "1.30.4", "● Running"},
		[]string{"work", "User", "Standard_E8s_…", "4", "1.29.8", "● Stopped · U…"})
	// Inside 34 the columns stop at 8 cells and the box cuts the lines.
	pools40 := wantTable([]int{6, 6, 8, 7, 7}, head,
		[]string{"system", "System", "Standar…", "3 (2–5)", "1.30.4", "● Runnin…"},
		[]string{"work", "User", "Standar…", "4", "1.29.8", "● Stoppe…"})
	wantLines(t, bodyOf(t, "aks", "", 70), cutAll(overviewWithPools(essentials, pools70), innerWidth(68)))
	wantLines(t, bodyOf(t, "aks", "", 40), cutAll(overviewWithPools(essentials, pools40), innerWidth(38)))
}

// cutAll cuts the lines to width cells, ending a cut line with "…".
func cutAll(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if r := []rune(l); len(r) > width {
			l = string(r[:width-1]) + "…"
		}
		out[i] = l
	}
	return out
}

func TestAKSNodePools(t *testing.T) {
	want := []string{
		"system · System",
		"VM size      Standard_D4s_v5",
		"Nodes        3 (2–5)",
		"Autoscaling  2–5",
		"Kubernetes   1.30.4",
		"OS           Linux AzureLinux",
		"Node image   AKSAzureLinux-2024.09.01",
		"Zones        1, 2, 3",
		"Max pods     110",
		"Priority     Regular",
		"OS disk      Managed 128 GB",
		"Subnet       snet-aks",
		"Max surge    33%",
		"State        ● Running",
		"",
		"work · User",
		"VM size      Standard_E8s_v5",
		"Nodes        4",
		"Autoscaling  off",
		"Kubernetes   1.29.8",
		"OS           Linux Ubuntu",
		"Node image   —",
		"Zones        —",
		"Max pods     —",
		"Priority     Spot",
		"OS disk      Ephemeral 64 GB",
		"Subnet       —",
		"Max surge    —",
		"State        ● Stopped · Updating",
		"",
	}
	wantLines(t, bodyOf(t, "aks", "Node pools", 100), want)
	wantLines(t, bodyOf(t, "aks", "Node pools", 40), cutAll(want, innerWidth(38)))
}

func TestAKSNetworking(t *testing.T) {
	want := []string{
		"Networking",
		"Network plugin        azure (overlay)",
		"Network policy        cilium",
		"Dataplane             cilium",
		"Pod CIDR              10.244.0.0/16",
		"Service CIDR          10.0.0.0/16",
		"DNS service IP        10.0.0.10",
		"Outbound type         loadBalancer",
		"Load balancer SKU     standard",
		"IP families           IPv4",
		"Private cluster       no",
		"Authorized IP ranges  any",
		"API server            aks-demo-abc.hcp.westeurope.azmk8s.io",
		"Private FQDN          —",
		"",
	}
	wantLines(t, bodyOf(t, "aks", "Networking", 100), want)
	wantLines(t, bodyOf(t, "aks", "Networking", 40), cutAll(want, innerWidth(38)))
}

func TestAKSNetworkingListFallbacks(t *testing.T) {
	got := bodyOf(t, "aks_stopped", "Networking", 100)
	for _, want := range []string{
		"Pod CIDR              10.2.0.0/16",
		"Service CIDR          10.1.0.0/16, fd00::/108",
		"Private cluster       yes",
		"Authorized IP ranges  any",
		"API server            aks-off.privatelink.westeurope.azmk8s.io",
		"Network plugin        —",
	} {
		if !contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, strings.Join(got, "\n"))
		}
	}
}

func TestAKSNetworkingAuthorizedRanges(t *testing.T) {
	row := fixture(t, "aks")
	props := row["properties"].(map[string]any)
	props["apiServerAccessProfile"] = map[string]any{"authorizedIPRanges": []any{"1.2.3.4/32", "5.6.7.0/24"}}
	v := NewDetailsAt("s1", azure.Resource{ID: "/r/x"}, "Networking").Resize(100, 40)
	got := renderBody(deliver(t, v, row, nil))
	if want := "Authorized IP ranges  1.2.3.4/32, 5.6.7.0/24"; !contains(got, want) {
		t.Errorf("lacks %q:\n%s", want, strings.Join(got, "\n"))
	}
}

func TestAKSIdentity(t *testing.T) {
	want := []string{
		"Identity",
		"Identity                  UserAssigned",
		"User-assigned identities  id-alpha, id-zeta",
		"Kubelet identity          aks-demo-agentpool",
		"Kubernetes RBAC           on",
		"Microsoft Entra ID        managed",
		"Azure RBAC                on",
		"Admin groups              2",
		"Local accounts            enabled",
		"OIDC issuer               on",
		"Workload identity         on",
		"Defender                  off",
		"Image cleaner             on",
		"",
	}
	wantLines(t, bodyOf(t, "aks", "Identity", 100), want)
	wantLines(t, bodyOf(t, "aks", "Identity", 40), cutAll(want, innerWidth(38)))
}

func TestAKSIdentityMissingFields(t *testing.T) {
	want := []string{
		"Identity",
		"Identity                  —",
		"User-assigned identities  —",
		"Kubelet identity          —",
		"Kubernetes RBAC           —",
		"Microsoft Entra ID        off",
		"Azure RBAC                —",
		"Admin groups              —",
		"Local accounts            enabled",
		"OIDC issuer               —",
		"Workload identity         —",
		"Defender                  —",
		"Image cleaner             —",
		"",
	}
	wantLines(t, bodyOf(t, "aks_min", "Identity", 100), want)
	stopped := bodyOf(t, "aks_stopped", "Identity", 100)
	if !contains(stopped, "Local accounts            disabled") {
		t.Errorf("disabled local accounts:\n%s", strings.Join(stopped, "\n"))
	}
}

func TestAKSAddons(t *testing.T) {
	want := []string{
		"Add-ons",
		"azurepolicy              off",
		"omsagent                 on · law-main",
		"Web app routing          on",
		"KEDA                     off",
		"Vertical pod autoscaler  on",
		"Service mesh             Istio",
		"Managed Prometheus       on",
		"Cost analysis            off",
		"",
	}
	wantLines(t, bodyOf(t, "aks", "Add-ons", 100), want)
	wantLines(t, bodyOf(t, "aks", "Add-ons", 40), cutAll(want, innerWidth(38)))
}

func TestAKSAddonsMissingFields(t *testing.T) {
	want := []string{
		"Add-ons",
		"Web app routing          —",
		"KEDA                     —",
		"Vertical pod autoscaler  —",
		"Service mesh             —",
		"Managed Prometheus       —",
		"Cost analysis            —",
		"",
	}
	wantLines(t, bodyOf(t, "aks_min", "Add-ons", 100), want)
}

func TestAKSMissingFieldsOverview(t *testing.T) {
	// Inner width 94: two columns of 45 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status               —", "Kubernetes       —", 45),
		twoCol("Resource group       —", "Tier             —", 45),
		twoCol("Location             —", "API server       —", 45),
		twoCol("Subscription         —", "Private cluster  no", 45),
		twoCol("Node resource group  —", "Support plan     —", 45),
		twoCol("", "Auto-upgrade     —", 45),
		"",
		"Node pools",
		"none",
		"",
		"Attention",
		"▲ local accounts are enabled",
		"▲ the API server accepts connections from any IP address",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, bodyOf(t, "aks_min", "", 100), want)
}

func TestAKSOverviewVariants(t *testing.T) {
	got := bodyOf(t, "aks_stopped", "", 70)
	for _, want := range []string{
		"Status               ● Stopped · Failed",
		"Kubernetes           1.31",
		"Tier                 Standard (Automatic)",
		"API server           aks-off.privatelink.westeurope.azmk8s.io",
		"Private cluster      yes",
		"Auto-upgrade         —",
		"p1    System  —        —      1.31     ● Running",
		"✗ the cluster is stopped",
		"✗ provisioning state is Failed",
	} {
		if !contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, strings.Join(got, "\n"))
		}
	}
}

func TestTierShowsSKUNameUnlessBase(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"Base", "Tier                 Standard"},
		{"base", "Tier                 Standard"},
		{"Automatic", "Tier                 Standard (Automatic)"},
		{"", "Tier                 Standard"},
	} {
		row := fixture(t, "aks")
		row["sku"] = map[string]any{"name": c.name, "tier": "Standard"}
		v := deliver(t, NewDetails("s1", azure.Resource{ID: "/r/x"}).Resize(70, 60), row, nil)
		if got := renderBody(v); !contains(got, c.want) {
			t.Errorf("sku name %q: lacks %q:\n%s", c.name, c.want, strings.Join(got, "\n"))
		}
	}
}

func TestPoolNodesAndState(t *testing.T) {
	for _, c := range []struct {
		pool         map[string]any
		nodes, state string
	}{
		{map[string]any{"count": 3.0}, "3", "—"},
		{map[string]any{"count": 3.0, "enableAutoScaling": true}, "3", "—"},
		{map[string]any{"count": 3.0, "enableAutoScaling": true, "minCount": 1.0, "maxCount": 9.0}, "3 (1–9)", "—"},
		{map[string]any{"count": 3.0, "enableAutoScaling": false, "minCount": 1.0, "maxCount": 9.0}, "3", "—"},
		{map[string]any{}, "—", "—"},
		{map[string]any{"powerState": map[string]any{"code": "Running"}, "provisioningState": "Succeeded"}, "—", "Running"},
		{map[string]any{"powerState": map[string]any{"code": "Running"}, "provisioningState": "Failed"}, "—", "Running · Failed"},
		{map[string]any{"provisioningState": "Failed"}, "—", "Failed"},
	} {
		if got := poolNodes(c.pool); got != c.nodes {
			t.Errorf("poolNodes(%v) = %q, want %q", c.pool, got, c.nodes)
		}
		if got := poolState(c.pool); got != c.state {
			t.Errorf("poolState(%v) = %q, want %q", c.pool, got, c.state)
		}
	}
}

func TestPoolVersionPrefersCurrent(t *testing.T) {
	both := map[string]any{"currentOrchestratorVersion": "1.30.4", "orchestratorVersion": "1.30"}
	if got := poolVersion(both); got != "1.30.4" {
		t.Errorf("both = %q", got)
	}
	if got := poolVersion(map[string]any{"orchestratorVersion": "1.30"}); got != "1.30" {
		t.Errorf("orchestratorVersion only = %q", got)
	}
	if got := poolVersion(map[string]any{}); got != "" {
		t.Errorf("none = %q", got)
	}
}

func TestLastSegment(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"/a/b/c", "c"}, {"c", "c"}, {"", ""}, {"/a/b/", ""},
	} {
		if got := lastSegment(c.in); got != c.want {
			t.Errorf("lastSegment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJSONTab(t *testing.T) {
	got := plainLines(openedAt(t, "webapp", "JSON", 100, 200).Render())[3:]
	if got[0] != " {" || !contains(got, `   "name": "web-demo",`) || !contains(got, `     "cost": "42",`) {
		t.Errorf("json tab:\n%s", strings.Join(got, "\n"))
	}
}

func TestSectionTitlesAreBoldAccent(t *testing.T) {
	for _, c := range []struct{ fixture, title string }{{"webapp", "Essentials"}, {"aks", "Node pools"}} {
		line := styledLine(t, openedAt(t, c.fixture, "", 100, 80), "╭ "+c.title)
		if a := attrsOf(t, line, c.title); !a.bold || a.fg != fgAccent {
			t.Errorf("%s: title %q is %+v, want bold accent", c.fixture, c.title, a)
		}
	}
}

func TestAKSJSONTab(t *testing.T) {
	wide := plainLines(openedAt(t, "aks", "JSON", 100, 200).Render())[3:]
	for _, l := range []string{" {", `   "name": "aks-demo",`, `   "properties": {`} {
		if !contains(wide, l) {
			t.Errorf("json tab lacks %q:\n%s", l, strings.Join(wide[:10], "\n"))
		}
	}
	narrow := plainLines(openedAt(t, "aks", "JSON", 20, 200).Render())[3:]
	for i, l := range narrow {
		want := wide[i]
		if r := []rune(want); len(r) > 19 {
			want = strings.TrimRight(string(r[:19]), " ")
		}
		if l != want {
			t.Errorf("line %d = %q, want %q", i, l, want)
		}
	}
}

func TestPoolAutoscaling(t *testing.T) {
	for _, c := range []struct {
		pool map[string]any
		want string
	}{
		{map[string]any{}, "—"},
		{map[string]any{"enableAutoScaling": false}, "off"},
		{map[string]any{"enableAutoScaling": true}, "on"},
		{map[string]any{"enableAutoScaling": true, "minCount": 2.0}, "on"},
		{map[string]any{"enableAutoScaling": true, "minCount": 2.0, "maxCount": 5.0}, "2–5"},
	} {
		if got := poolAutoscaling(c.pool); got != c.want {
			t.Errorf("poolAutoscaling(%v) = %q, want %q", c.pool, got, c.want)
		}
	}
}

func TestTierWithoutTier(t *testing.T) {
	row := map[string]any{"sku": map[string]any{"name": "Automatic"}}
	if got := tier(row); got != "Automatic" {
		t.Errorf("tier = %q, want Automatic", got)
	}
}

func TestAKSNodePoolsNone(t *testing.T) {
	wantLines(t, bodyOf(t, "aks_min", "Node pools", 100), []string{"Node pools", "none", ""})
}

// alertLines are the items as their plain lines.
func alertLines(items []alert) []string {
	out := make([]string, len(items))
	for i, a := range items {
		out[i] = a.line()
	}
	return out
}

func TestAttentionIgnoresEmptyRanges(t *testing.T) {
	got := alertLines(aksAttention(safeRow(t, `{"apiServerAccessProfile":{"authorizedIPRanges":[""]}}`)))
	want := []string{"▲ the API server accepts connections from any IP address"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("attention = %q, want %q", got, want)
	}
	net := safeRow(t, `{"apiServerAccessProfile":{"authorizedIPRanges":[""]}}`)
	net["type"] = aksType
	v := deliver(t, openedAtRow(t, "Networking"), net, nil)
	if got := renderBody(v); !contains(got, "Authorized IP ranges  any") {
		t.Errorf("networking:\n%s", strings.Join(got, "\n"))
	}
}

func openedAtRow(t *testing.T, tab string) View {
	t.Helper()
	return NewDetailsAt("s1", azure.Resource{ID: "/r/x"}, tab).Resize(100, 60)
}

// safeRow is a cluster with nothing to report; props override its properties.
func safeRow(t *testing.T, props string) map[string]any {
	t.Helper()
	row := map[string]any{"properties": map[string]any{
		"powerState":               map[string]any{"code": "Running"},
		"provisioningState":        "Succeeded",
		"currentKubernetesVersion": "1.30.4",
		"disableLocalAccounts":     true,
		"apiServerAccessProfile":   map[string]any{"enablePrivateCluster": true},
	}}
	var over map[string]any
	if err := json.Unmarshal([]byte(props), &over); err != nil {
		t.Fatal(err)
	}
	for k, v := range over {
		row["properties"].(map[string]any)[k] = v
	}
	return row
}

func TestAttention(t *testing.T) {
	tests := []struct {
		name  string
		props string
		want  []string
	}{
		{"nothing to report", `{}`, nil},
		{"stopped cluster", `{"powerState":{"code":"Stopped"}}`, []string{"✗ the cluster is stopped"}},
		{"running cluster", `{"powerState":{"code":"Running"}}`, nil},
		{"failed provisioning", `{"provisioningState":"Failed"}`, []string{"✗ provisioning state is Failed"}},
		{"updating provisioning", `{"provisioningState":"Updating"}`, []string{"▲ provisioning state is Updating"}},
		{"canceled provisioning", `{"provisioningState":"Canceled"}`, []string{"▲ provisioning state is Canceled"}},
		{"failed in another case", `{"provisioningState":"failed"}`, []string{"✗ provisioning state is failed"}},
		{"missing provisioning state", `{"provisioningState":null}`, nil},
		{"stopped pool", `{"agentPoolProfiles":[{"name":"a","powerState":{"code":"Stopped"}}]}`, []string{"✗ node pool a is stopped"}},
		{"running pool", `{"agentPoolProfiles":[{"name":"a","powerState":{"code":"Running"},"provisioningState":"Succeeded"}]}`, nil},
		{"updating pool", `{"agentPoolProfiles":[{"name":"a","provisioningState":"Updating"}]}`, []string{"▲ node pool a is Updating"}},
		{"stopped and failed pool", `{"agentPoolProfiles":[{"name":"a","powerState":{"code":"Stopped"},"provisioningState":"Failed"}]}`,
			[]string{"✗ node pool a is stopped", "✗ node pool a is Failed"}},
		{"pool on another version", `{"agentPoolProfiles":[{"name":"a","currentOrchestratorVersion":"1.29.1"}]}`,
			[]string{"▲ node pool a runs 1.29.1, the control plane 1.30.4"}},
		{"pool on the same version", `{"agentPoolProfiles":[{"name":"a","currentOrchestratorVersion":"1.30.4"}]}`, nil},
		{"versions without current", `{"currentKubernetesVersion":null,"kubernetesVersion":"1.30","agentPoolProfiles":[{"name":"a","orchestratorVersion":"1.29"}]}`,
			[]string{"▲ node pool a runs 1.29, the control plane 1.30"}},
		{"pool version missing", `{"agentPoolProfiles":[{"name":"a"}]}`, nil},
		{"cluster version missing", `{"currentKubernetesVersion":null,"agentPoolProfiles":[{"name":"a","orchestratorVersion":"1.29"}]}`, nil},
		{"two pools keep their order", `{"agentPoolProfiles":[{"name":"a","provisioningState":"Failed"},{"name":"b","powerState":{"code":"Stopped"}}]}`,
			[]string{"✗ node pool a is Failed", "✗ node pool b is stopped"}},
		{"local accounts enabled", `{"disableLocalAccounts":false}`, []string{"▲ local accounts are enabled"}},
		{"local accounts missing", `{"disableLocalAccounts":null}`, []string{"▲ local accounts are enabled"}},
		{"open API server", `{"apiServerAccessProfile":{"enablePrivateCluster":false}}`, []string{"▲ the API server accepts connections from any IP address"}},
		{"open API server, empty ranges", `{"apiServerAccessProfile":{"authorizedIPRanges":[]}}`, []string{"▲ the API server accepts connections from any IP address"}},
		{"no API server profile", `{"apiServerAccessProfile":null}`, []string{"▲ the API server accepts connections from any IP address"}},
		{"authorized ranges", `{"apiServerAccessProfile":{"authorizedIPRanges":["1.2.3.4/32"]}}`, nil},
		{"private cluster", `{"apiServerAccessProfile":{"enablePrivateCluster":true,"authorizedIPRanges":[]}}`, nil},
		{"all rules in order", `{"powerState":{"code":"Stopped"},"provisioningState":"Failed","disableLocalAccounts":false,"apiServerAccessProfile":null,"agentPoolProfiles":[{"name":"a","powerState":{"code":"Stopped"},"provisioningState":"Failed","orchestratorVersion":"1.1"}]}`,
			[]string{
				"✗ the cluster is stopped",
				"✗ provisioning state is Failed",
				"✗ node pool a is stopped",
				"✗ node pool a is Failed",
				"▲ node pool a runs 1.1, the control plane 1.30.4",
				"▲ local accounts are enabled",
				"▲ the API server accepts connections from any IP address",
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(aksAttention(safeRow(t, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAttentionOfEmptyRow(t *testing.T) {
	want := []string{"▲ local accounts are enabled", "▲ the API server accepts connections from any IP address"}
	if got := alertLines(aksAttention(nil)); !reflect.DeepEqual(got, want) {
		t.Errorf("aksAttention(nil) = %q, want %q", got, want)
	}
}

func TestOverviewHidesAttentionWithoutItems(t *testing.T) {
	v := NewDetails("s1", azure.Resource{ID: "/r/x"}).Resize(100, 60)
	row := safeRow(t, `{}`)
	row["type"] = "microsoft.containerservice/managedclusters"
	got := renderBody(deliver(t, v, row, nil))
	if contains(got, "Attention") {
		t.Errorf("Attention shown without items:\n%s", strings.Join(got, "\n"))
	}
}

func TestPoolWithoutModeIsTitledByName(t *testing.T) {
	row := map[string]any{"properties": map[string]any{
		"agentPoolProfiles": []any{map[string]any{"name": "np1"}},
	}}
	if got := unbox(plainLines(strings.Join(aksPools(row, 80), "\n")))[0]; got != "np1" {
		t.Errorf("title = %q, want np1", got)
	}
}

func TestUserAssignedIdentitiesSortByName(t *testing.T) {
	row := map[string]any{"identity": map[string]any{"userAssignedIdentities": map[string]any{
		"/subscriptions/s/resourceGroups/rg-a/providers/Microsoft.ManagedIdentity/userAssignedIdentities/zeta":  map[string]any{},
		"/subscriptions/s/resourceGroups/rg-b/providers/Microsoft.ManagedIdentity/userAssignedIdentities/alpha": map[string]any{},
	}}}
	if got := userAssigned(row); got != "alpha, zeta" {
		t.Errorf("userAssigned = %q, want alpha, zeta", got)
	}
}

func TestOverviewSectionsAreBoxes(t *testing.T) {
	for _, c := range []struct {
		fixture, tab string
		titles       []string
	}{
		{"aks", "", []string{"Essentials", "Node pools", "Attention", "Tags"}},
		{"aks", "Node pools", []string{"system · System", "work · User"}},
		{"aks", "Networking", []string{"Networking"}},
		{"aks", "Identity", []string{"Identity"}},
		{"aks", "Add-ons", []string{"Add-ons"}},
		{"webapp", "", []string{"Essentials", "Tags"}},
	} {
		t.Run(c.fixture+" "+c.tab, func(t *testing.T) {
			lines := plainLines(openedAt(t, c.fixture, c.tab, 100, 200).Render())[3:]
			var titles []string
			for i, l := range lines {
				if strings.HasPrefix(l, " ╭ ") {
					titles = append(titles, unbox([]string{l})[0])
					if i > 0 && lines[i-1] != "" {
						t.Errorf("no blank line before the box on line %d", i)
					}
				}
				if l != "" && lipgloss.Width(l) != 99 {
					t.Errorf("line %d is %d wide, want 99: %q", i, lipgloss.Width(l), l)
				}
			}
			if !reflect.DeepEqual(titles, c.titles) {
				t.Errorf("titles = %q, want %q", titles, c.titles)
			}
		})
	}
}

func TestOverviewHasNoBoxesWhenNarrow(t *testing.T) {
	for _, tab := range []string{"", "Node pools", "Networking", "Identity", "Add-ons"} {
		out := openedAt(t, "aks", tab, 21, 200).Render()
		if strings.ContainsAny(plain(out), "╭│╰") {
			t.Errorf("%q at 21 cells has a box:\n%s", tab, plain(out))
		}
		title := strings.TrimSpace(plainLines(out)[3])
		if tab == "" && title != "Essentials" {
			t.Errorf("first line = %q, want the title", title)
		}
		if a := attrsOf(t, strings.Split(out, "\n")[3], strings.TrimSpace(plain(strings.Split(out, "\n")[3]))); tab == "" && !a.bold {
			t.Errorf("title is not bold: %+v", a)
		}
	}
	if out := plain(openedAt(t, "aks", "", 22, 200).Render()); !strings.Contains(out, "╭ Essentials") {
		t.Errorf("22 cells have no box:\n%s", out)
	}
}

func TestKeysAreMutedAndValuesPlain(t *testing.T) {
	v := openedAt(t, "aks", "Networking", 100, 40)
	line := styledLine(t, v, "Network plugin")
	if a := attrsOf(t, line, "Network plugin"); a.fg != fgMuted || a.bold {
		t.Errorf("key = %+v, want muted", a)
	}
	if a := attrsOf(t, line, "azure (overlay)"); a.fg != "" || a.bold {
		t.Errorf("value = %+v, want the default", a)
	}
	wide := openedAt(t, "aks", "", 120, 40)
	for _, c := range []struct{ line, key, value string }{
		{"Status", "Status", "rg-app"},
		{"Kubernetes", "Kubernetes", "1.30.4"},
	} {
		l := styledLine(t, wide, c.line)
		if a := attrsOf(t, l, c.key); a.fg != fgMuted {
			t.Errorf("%s key = %+v, want muted", c.key, a)
		}
	}
	if a := attrsOf(t, styledLine(t, wide, "Kubernetes"), "1.30.4"); a.fg != "" {
		t.Errorf("value = %+v, want the default", a)
	}
}

func TestStatusValuesUseStateColors(t *testing.T) {
	wide := openedAt(t, "aks", "", 120, 40)
	line := styledLine(t, wide, "Status")
	wantFg(t, line, "●", fgGood)
	wantFg(t, line, "Running", fgGood)
	wantFg(t, line, "Succeeded", fgGood)
	stopped := styledLine(t, openedAt(t, "aks_stopped", "", 70, 40), "Status")
	wantFg(t, stopped, "●", fgBad)
	wantFg(t, stopped, "Stopped", fgBad)
	wantFg(t, stopped, "Failed", fgBad)
	generic := styledLine(t, openedAt(t, "webapp", "", 100, 40), "Status")
	wantFg(t, generic, "●", fgGood)
	wantFg(t, generic, "Succeeded", fgGood)
	pool := styledLine(t, openedAt(t, "aks", "Node pools", 100, 80), "State        ● Stopped")
	wantFg(t, pool, "●", fgBad)
	wantFg(t, pool, "Stopped", fgBad)
	wantFg(t, pool, "Updating", fgWarn)
}

func TestPoolSummaryTableStyles(t *testing.T) {
	v := openedAt(t, "aks", "", 120, 40)
	header := styledLine(t, v, "VM size")
	if a := attrsOf(t, header, "VM size"); !a.bold || a.fg != fgMuted {
		t.Errorf("header = %+v, want bold muted", a)
	}
	wantFg(t, styledLine(t, v, "system  System"), "Running", fgGood)
	row := styledLine(t, v, "work    User")
	wantFg(t, row, "●", fgBad)
	wantFg(t, row, "Stopped", fgBad)
	wantFg(t, row, "Updating", fgWarn)
	wantFg(t, row, "work", "")
	if attrsOf(t, row, "work").reverse {
		t.Error("a summary row is highlighted")
	}
}

func TestAttentionBoxColors(t *testing.T) {
	v := openedAt(t, "aks", "", 120, 80)
	top := styledLine(t, v, "╭ Attention")
	if a := attrsOf(t, top, "Attention"); !a.bold || a.fg != fgBad {
		t.Errorf("title = %+v, want bold bad", a)
	}
	wantFg(t, top, "╭", fgBad)
	wantFg(t, styledLine(t, v, "✗ node pool work is stopped"), "│", fgBad)
	wantFg(t, styledLine(t, v, "✗ node pool work is stopped"), "✗ node pool work is stopped", fgBad)
	wantFg(t, styledLine(t, v, "▲ node pool work is Updating"), "▲ node pool work is Updating", fgWarn)
	wantFg(t, styledLine(t, v, "▲ local accounts"), "▲ local accounts are enabled", fgWarn)
}

func TestAttentionBoxIsWarnWithoutErrors(t *testing.T) {
	row := safeRow(t, `{"disableLocalAccounts":false,"provisioningState":"Updating"}`)
	row["type"] = aksType
	v := deliver(t, NewDetails("s1", azure.Resource{ID: "/r/x"}).Resize(100, 60), row, nil)
	top := styledLine(t, v, "╭ Attention")
	if a := attrsOf(t, top, "Attention"); !a.bold || a.fg != fgWarn {
		t.Errorf("title = %+v, want bold warn", a)
	}
	wantFg(t, top, "╭", fgWarn)
	wantFg(t, styledLine(t, v, "▲ provisioning state is Updating"), "▲ provisioning", fgWarn)
}

func TestAddonsColors(t *testing.T) {
	v := openedAt(t, "aks", "Add-ons", 100, 40)
	for _, c := range []struct{ line, sub, fg string }{
		{"azurepolicy", "off", fgMuted},
		{"Web app routing", "on", fgGood},
		{"KEDA", "off", fgMuted},
		{"omsagent", "on · law-main", fgGood},
		{"omsagent", "law-main", ""},
		{"Service mesh", "Istio", ""},
	} {
		wantFg(t, styledLine(t, v, c.line), c.sub, c.fg)
	}
	wantFg(t, styledLine(t, openedAt(t, "aks_min", "Add-ons", 100, 40), "KEDA"), "—", "")
}

func TestAddonsColorEveryValue(t *testing.T) {
	v := openedAt(t, "aks", "Add-ons", 100, 40)
	for _, line := range strings.Split(v.Render(), "\n") {
		words := strings.Fields(strings.Trim(plain(line), "│ "))
		if len(words) < 2 || (words[len(words)-1] != "on" && words[len(words)-1] != "off") {
			continue
		}
		want := fgMuted
		if words[len(words)-1] == "on" {
			want = fgGood
		}
		if got := lastAttrsOf(t, line, words[len(words)-1]).fg; got != want {
			t.Errorf("%q: value foreground %q, want %q", plain(line), got, want)
		}
	}
}

func TestIdentityColors(t *testing.T) {
	v := openedAt(t, "aks", "Identity", 100, 40)
	if a := valueAttrs(t, v, "Local accounts"); a.fg != fgWarn {
		t.Errorf("enabled local accounts = %+v, want warn", a)
	}
	for _, line := range []string{"Kubernetes RBAC", "Microsoft Entra ID", "│ Identity  ", "Azure RBAC", "Defender", "OIDC issuer", "Workload identity", "Image cleaner"} {
		if a := valueAttrs(t, v, line); a.fg != "" {
			t.Errorf("%s value = %+v, want the default", line, a)
		}
	}
	stopped := openedAt(t, "aks_stopped", "Identity", 100, 40)
	if a := valueAttrs(t, stopped, "Local accounts"); a.fg != "" {
		t.Errorf("disabled local accounts = %+v, want the default", a)
	}
}

func TestColumnWidthsSplitAnOddWidth(t *testing.T) {
	// The gap takes 4 cells; an odd rest gives the extra cell to the right.
	for _, c := range []struct{ width, left, right int }{{21, 8, 9}, {90, 43, 43}, {91, 43, 44}} {
		if l, r := columnWidths(c.width); l != c.left || r != c.right {
			t.Errorf("columnWidths(%d) = %d, %d, want %d, %d", c.width, l, r, c.left, c.right)
		}
	}
}

func TestAKSPoolSummarySkipsElementsThatAreNoMaps(t *testing.T) {
	row := map[string]any{"properties": map[string]any{"agentPoolProfiles": []any{
		"stray", map[string]any{"name": "np1", "mode": "System"}, 7.0,
	}}}
	got := plainAll(aksPoolSummary.lines(row, 100))
	wantLines(t, got, []string{"Name  Mode    VM size  Nodes  Version  State", "np1   System  —        —      —        —"})
	none := map[string]any{"properties": map[string]any{"agentPoolProfiles": []any{"stray"}}}
	wantLines(t, plainAll(aksPoolSummary.lines(none, 100)), []string{"none"})
}

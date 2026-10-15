package views

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

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

// bodyOf returns the lines under the tab bar.
func bodyOf(t *testing.T, name, tab string, width int) []string {
	t.Helper()
	return plainLines(openedAt(t, name, tab, width, 200).Render())[1:]
}

func wantLines(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTabs(t *testing.T) {
	aks := []string{"Overview", "Node pools", "Networking", "Identity", "Add-ons", "JSON"}
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
	want := []string{
		"Essentials",
		"Status          Succeeded",
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
	wantLines(t, bodyOf(t, "webapp", "", 100), want)
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
	wantLines(t, plainLines(v.Render())[1:len(want)+1], want)
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
		if got := plainLines(v.Render()); !contains(got, c.want) {
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
		"Name   Mode   VM size         Nodes   Version State",
		"────────────────────────────────────────────────────────────────",
		"system System Standard_D4s_v5 3 (2–5) 1.30.4  Running",
		"work   User   Standard_E8s_v5 4       1.29.8  Stopped · Updating",
	}
	aksAttention = []string{
		"Attention",
		"! node pool work is stopped",
		"! node pool work is Updating",
		"! node pool work runs 1.29.8, the control plane 1.30.4",
		"! local accounts are enabled",
		"! the API server accepts connections from any IP address",
	}
	aksTags = []string{"Tags", "env = dev", "owner = team-a", ""}
)

func overviewLines(essentials []string) []string {
	out := append([]string{"Essentials"}, essentials...)
	out = append(out, "", "Node pools")
	out = append(out, aksPoolTable...)
	out = append(out, "")
	out = append(out, aksAttention...)
	out = append(out, "")
	return append(out, aksTags...)
}

func TestAKSOverviewTwoColumns(t *testing.T) {
	want := overviewLines([]string{
		"Status               Running · Succeeded              Kubernetes       1.30.4",
		"Resource group       rg-app                           Tier             Standard",
		"Location             westeurope                       API server       aks-demo-abc.hcp.westeurope.azmk8s.io",
		"Subscription         Dev                              Private cluster  no",
		"Node resource group  MC_rg-app_aks-demo_westeurope    Support plan     KubernetesOfficial",
		"                                                      Auto-upgrade     patch / NodeImage",
	})
	wantLines(t, bodyOf(t, "aks", "", 120), want)
}

func TestAKSOverviewColumnSwitchAt90(t *testing.T) {
	one := bodyOf(t, "aks", "", 89)
	if !strings.HasPrefix(one[1], "Status               Running · Succeeded") || strings.Contains(one[1], "Kubernetes") {
		t.Errorf("89 cells: not one column:\n%s", strings.Join(one, "\n"))
	}
	if one[6] != "Kubernetes           1.30.4" {
		t.Errorf("89 cells: line 6 = %q", one[6])
	}
	two := bodyOf(t, "aks", "", 90)
	if !strings.Contains(two[1], "Kubernetes       1.30.4") {
		t.Errorf("90 cells: not two columns:\n%s", strings.Join(two, "\n"))
	}
}

func TestAKSOverviewOneColumn(t *testing.T) {
	want := overviewLines([]string{
		"Status               Running · Succeeded",
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
	})
	wantLines(t, bodyOf(t, "aks", "", 70), want)
	wantLines(t, bodyOf(t, "aks", "", 40), cutAll(want, 40))
}

func cutAll(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if r := []rune(l); len(r) > width {
			l = strings.TrimRight(string(r[:width]), " ")
		}
		out[i] = l
	}
	return out
}

func TestAKSNodePools(t *testing.T) {
	want := []string{
		"system (System)",
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
		"State        Running",
		"",
		"work (User)",
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
		"State        Stopped · Updating",
		"",
	}
	wantLines(t, bodyOf(t, "aks", "Node pools", 100), want)
	wantLines(t, bodyOf(t, "aks", "Node pools", 40), cutAll(want, 40))
}

func TestAKSNetworking(t *testing.T) {
	want := []string{
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
	}
	wantLines(t, bodyOf(t, "aks", "Networking", 100), want)
	wantLines(t, bodyOf(t, "aks", "Networking", 40), cutAll(want, 40))
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
	got := plainLines(deliver(t, v, row, nil).Render())
	if want := "Authorized IP ranges  1.2.3.4/32, 5.6.7.0/24"; !contains(got, want) {
		t.Errorf("lacks %q:\n%s", want, strings.Join(got, "\n"))
	}
}

func TestAKSIdentity(t *testing.T) {
	want := []string{
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
	}
	wantLines(t, bodyOf(t, "aks", "Identity", 100), want)
	wantLines(t, bodyOf(t, "aks", "Identity", 40), cutAll(want, 40))
}

func TestAKSIdentityMissingFields(t *testing.T) {
	want := []string{
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
	}
	wantLines(t, bodyOf(t, "aks_min", "Identity", 100), want)
	stopped := bodyOf(t, "aks_stopped", "Identity", 100)
	if !contains(stopped, "Local accounts            disabled") {
		t.Errorf("disabled local accounts:\n%s", strings.Join(stopped, "\n"))
	}
}

func TestAKSAddons(t *testing.T) {
	want := []string{
		"azurepolicy              off",
		"omsagent                 on · law-main",
		"Web app routing          on",
		"KEDA                     off",
		"Vertical pod autoscaler  on",
		"Service mesh             Istio",
		"Managed Prometheus       on",
		"Cost analysis            off",
	}
	wantLines(t, bodyOf(t, "aks", "Add-ons", 100), want)
	wantLines(t, bodyOf(t, "aks", "Add-ons", 40), cutAll(want, 40))
}

func TestAKSAddonsMissingFields(t *testing.T) {
	want := []string{
		"Web app routing          —",
		"KEDA                     —",
		"Vertical pod autoscaler  —",
		"Service mesh             —",
		"Managed Prometheus       —",
		"Cost analysis            —",
	}
	wantLines(t, bodyOf(t, "aks_min", "Add-ons", 100), want)
}

func TestAKSMissingFieldsOverview(t *testing.T) {
	want := []string{
		"Essentials",
		"Status               —    Kubernetes       —",
		"Resource group       —    Tier             —",
		"Location             —    API server       —",
		"Subscription         —    Private cluster  no",
		"Node resource group  —    Support plan     —",
		"                          Auto-upgrade     —",
		"",
		"Node pools",
		"none",
		"",
		"Attention",
		"! local accounts are enabled",
		"! the API server accepts connections from any IP address",
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
		"Status               Stopped · Failed",
		"Kubernetes           1.31",
		"Tier                 Standard (Automatic)",
		"API server           aks-off.privatelink.westeurope.azmk8s.io",
		"Private cluster      yes",
		"Auto-upgrade         —",
		"p1   System —       —     1.31    Running",
		"! the cluster is stopped",
		"! provisioning state is Failed",
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
		if got := plainLines(v.Render()); !contains(got, c.want) {
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
	got := bodyOf(t, "webapp", "JSON", 100)
	if got[0] != "{" || !contains(got, `  "name": "web-demo",`) || !contains(got, `    "cost": "42",`) {
		t.Errorf("json tab:\n%s", strings.Join(got, "\n"))
	}
}

func TestTabBar(t *testing.T) {
	v := openedAt(t, "aks", "", 100, 10)
	if got := plainLines(v.Render())[0]; got != "Overview  Node pools  Networking  Identity  Add-ons  JSON" {
		t.Errorf("tab bar = %q", got)
	}
	if got := plainLines(openedAt(t, "aks", "", 15, 10).Render())[0]; got != "Overview  Node" {
		t.Errorf("narrow tab bar = %q", got)
	}
	if got := plainLines(openedAt(t, "webapp", "", 100, 10).Render())[0]; got != "Overview  JSON" {
		t.Errorf("generic tab bar = %q", got)
	}
	if got := v.Render(); !strings.Contains(strings.Split(got, "\n")[0], "\x1b[7mOverview") {
		t.Errorf("current tab is not reversed: %q", strings.Split(got, "\n")[0])
	}
	bar := strings.Split(openedAt(t, "aks", "Networking", 100, 10).Render(), "\n")[0]
	if !strings.Contains(bar, "\x1b[7mNetworking") || strings.Contains(bar, "\x1b[7mOverview") {
		t.Errorf("Networking is not the only reversed tab: %q", bar)
	}
}

func TestSectionTitlesAreBold(t *testing.T) {
	for _, c := range []struct{ fixture, title string }{{"webapp", "Essentials"}, {"aks", "Node pools"}} {
		var found bool
		for _, l := range strings.Split(openedAt(t, c.fixture, "", 100, 80).Render(), "\n") {
			if plain(l) != "" && strings.TrimRight(plain(l), " ") == c.title {
				found = strings.Contains(l, "\x1b[1m")
			}
		}
		if !found {
			t.Errorf("%s: title %q is not bold", c.fixture, c.title)
		}
	}
}

func TestAKSJSONTab(t *testing.T) {
	wide := bodyOf(t, "aks", "JSON", 100)
	for _, l := range []string{"{", `  "name": "aks-demo",`, `  "properties": {`} {
		if !contains(wide, l) {
			t.Errorf("json tab lacks %q:\n%s", l, strings.Join(wide[:10], "\n"))
		}
	}
	wantLines(t, bodyOf(t, "aks", "JSON", 20), cutAll(wide, 20))
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
	wantLines(t, bodyOf(t, "aks_min", "Node pools", 100), []string{"none"})
}

func TestAttentionIgnoresEmptyRanges(t *testing.T) {
	got := attention(safeRow(t, `{"apiServerAccessProfile":{"authorizedIPRanges":[""]}}`))
	want := []string{"! the API server accepts connections from any IP address"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("attention = %q, want %q", got, want)
	}
	net := safeRow(t, `{"apiServerAccessProfile":{"authorizedIPRanges":[""]}}`)
	net["type"] = aksType
	v := deliver(t, openedAtRow(t, "Networking"), net, nil)
	if got := plainLines(v.Render()); !contains(got, "Authorized IP ranges  any") {
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
		{"stopped cluster", `{"powerState":{"code":"Stopped"}}`, []string{"! the cluster is stopped"}},
		{"running cluster", `{"powerState":{"code":"Running"}}`, nil},
		{"failed provisioning", `{"provisioningState":"Failed"}`, []string{"! provisioning state is Failed"}},
		{"missing provisioning state", `{"provisioningState":null}`, nil},
		{"stopped pool", `{"agentPoolProfiles":[{"name":"a","powerState":{"code":"Stopped"}}]}`, []string{"! node pool a is stopped"}},
		{"running pool", `{"agentPoolProfiles":[{"name":"a","powerState":{"code":"Running"},"provisioningState":"Succeeded"}]}`, nil},
		{"updating pool", `{"agentPoolProfiles":[{"name":"a","provisioningState":"Updating"}]}`, []string{"! node pool a is Updating"}},
		{"stopped and failed pool", `{"agentPoolProfiles":[{"name":"a","powerState":{"code":"Stopped"},"provisioningState":"Failed"}]}`,
			[]string{"! node pool a is stopped", "! node pool a is Failed"}},
		{"pool on another version", `{"agentPoolProfiles":[{"name":"a","currentOrchestratorVersion":"1.29.1"}]}`,
			[]string{"! node pool a runs 1.29.1, the control plane 1.30.4"}},
		{"pool on the same version", `{"agentPoolProfiles":[{"name":"a","currentOrchestratorVersion":"1.30.4"}]}`, nil},
		{"versions without current", `{"currentKubernetesVersion":null,"kubernetesVersion":"1.30","agentPoolProfiles":[{"name":"a","orchestratorVersion":"1.29"}]}`,
			[]string{"! node pool a runs 1.29, the control plane 1.30"}},
		{"pool version missing", `{"agentPoolProfiles":[{"name":"a"}]}`, nil},
		{"cluster version missing", `{"currentKubernetesVersion":null,"agentPoolProfiles":[{"name":"a","orchestratorVersion":"1.29"}]}`, nil},
		{"two pools keep their order", `{"agentPoolProfiles":[{"name":"a","provisioningState":"Failed"},{"name":"b","powerState":{"code":"Stopped"}}]}`,
			[]string{"! node pool a is Failed", "! node pool b is stopped"}},
		{"local accounts enabled", `{"disableLocalAccounts":false}`, []string{"! local accounts are enabled"}},
		{"local accounts missing", `{"disableLocalAccounts":null}`, []string{"! local accounts are enabled"}},
		{"open API server", `{"apiServerAccessProfile":{"enablePrivateCluster":false}}`, []string{"! the API server accepts connections from any IP address"}},
		{"open API server, empty ranges", `{"apiServerAccessProfile":{"authorizedIPRanges":[]}}`, []string{"! the API server accepts connections from any IP address"}},
		{"no API server profile", `{"apiServerAccessProfile":null}`, []string{"! the API server accepts connections from any IP address"}},
		{"authorized ranges", `{"apiServerAccessProfile":{"authorizedIPRanges":["1.2.3.4/32"]}}`, nil},
		{"private cluster", `{"apiServerAccessProfile":{"enablePrivateCluster":true,"authorizedIPRanges":[]}}`, nil},
		{"all rules in order", `{"powerState":{"code":"Stopped"},"provisioningState":"Failed","disableLocalAccounts":false,"apiServerAccessProfile":null,"agentPoolProfiles":[{"name":"a","powerState":{"code":"Stopped"},"provisioningState":"Failed","orchestratorVersion":"1.1"}]}`,
			[]string{
				"! the cluster is stopped",
				"! provisioning state is Failed",
				"! node pool a is stopped",
				"! node pool a is Failed",
				"! node pool a runs 1.1, the control plane 1.30.4",
				"! local accounts are enabled",
				"! the API server accepts connections from any IP address",
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attention(safeRow(t, tt.props))
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
	want := []string{"! local accounts are enabled", "! the API server accepts connections from any IP address"}
	if got := attention(nil); !reflect.DeepEqual(got, want) {
		t.Errorf("attention(nil) = %q, want %q", got, want)
	}
}

func TestOverviewHidesAttentionWithoutItems(t *testing.T) {
	v := NewDetails("s1", azure.Resource{ID: "/r/x"}).Resize(100, 60)
	row := safeRow(t, `{}`)
	row["type"] = "microsoft.containerservice/managedclusters"
	got := plainLines(deliver(t, v, row, nil).Render())
	if contains(got, "Attention") {
		t.Errorf("Attention shown without items:\n%s", strings.Join(got, "\n"))
	}
}

func TestPoolWithoutModeIsTitledByName(t *testing.T) {
	row := map[string]any{"properties": map[string]any{
		"agentPoolProfiles": []any{map[string]any{"name": "np1"}},
	}}
	if got := plainLines(strings.Join(aksPools(row, 80), "\n"))[0]; got != "np1" {
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

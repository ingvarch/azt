package views

import (
	"reflect"
	"strings"
	"testing"
)

const vnetType = "microsoft.network/virtualnetworks"

// wantTable pads the cells of each row to the widths, two spaces between and
// none after the last cell.
func wantTable(widths []int, rows ...[]string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		cells := make([]string, len(r))
		for j, c := range r {
			cells[j] = c
			if j < len(r)-1 {
				cells[j] += strings.Repeat(" ", widths[j]-len([]rune(c)))
			}
		}
		out[i] = strings.Join(cells, "  ")
	}
	return out
}

func TestVNetOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status           ● Succeeded", "Peerings        3", 50),
		twoCol("Address space    10.10.0.0/16, 10.11.0.0/16", "Resource group  rg-net", 50),
		twoCol("DNS servers      10.10.0.4, 10.10.0.5", "Location        swedencentral", 50),
		twoCol("DDoS protection  on", "Subscription    Prod", 50),
		twoCol("Subnets          4", "", 50),
		"",
		"Attention",
		"▲ subnet snet-data has no network security group",
		"▲ peering peer-spoke is Initiated",
		"✗ peering peer-old is disconnected",
		"",
		"Tags",
		"owner = netops",
		"",
	}
	wantLines(t, bodyOf(t, "vnet", "", 110), want)
}

func TestVNetOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status           ● Succeeded",
		"Address space    10.10.0.0/16, 10.11.0.0/16",
		"DNS servers      10.10.0.4, 10.10.0.5",
		"DDoS protection  on",
		"Subnets          4",
		"Peerings         3",
		"Resource group   rg-net",
		"Location         swedencentral",
		"Subscription     Prod",
		"",
	}
	got := bodyOf(t, "vnet", "", 60)
	wantLines(t, got[:len(want)], want)
}

func TestVNetSubnets(t *testing.T) {
	widths := []int{13, 28, 10, 8, 11, 11, 41, 25}
	want := append([]string{"Subnets"}, wantTable(widths,
		[]string{"Name", "Prefix", "IPs in use", "NSG", "Route table", "NAT gateway", "Delegation", "Private endpoint policies"},
		[]string{"snet-app", "10.10.1.0/24", "3", "nsg-app", "rt-egress", "nat-prod", "Web/serverFarms", "Enabled"},
		[]string{"snet-data", "10.10.2.0/25, 10.10.2.128/25", "1", "—", "—", "—", "Sql/managedInstances, Batch/batchAccounts", "Disabled"},
		[]string{"GatewaySubnet", "10.10.255.0/27", "2", "—", "—", "—", "—", "—"},
		[]string{"snet-jobs", "10.10.3.0/24", "0", "nsg-jobs", "—", "—", "—", "Enabled"},
	)...)
	wantLines(t, bodyOf(t, "vnet", "Subnets", 200), append(want, ""))
}

func TestVNetSubnetsAreCutByTheBox(t *testing.T) {
	got := bodyOf(t, "vnet", "Subnets", 60)
	for _, l := range got {
		if w := len([]rune(l)); w > innerWidth(58) {
			t.Errorf("line %q is %d cells wide", l, w)
		}
	}
	if !strings.HasSuffix(got[2], "…") {
		t.Errorf("row = %q, want it cut with an ellipsis", got[2])
	}
}

func TestVNetPeerings(t *testing.T) {
	widths := []int{10, 14, 26, 14, 17, 17, 15, 15}
	want := append([]string{"Peerings"}, wantTable(widths,
		[]string{"Name", "Remote network", "Remote address space", "State", "Sync", "Forwarded traffic", "Gateway transit", "Remote gateways"},
		[]string{"peer-hub", "vnet-hub", "10.0.0.0/16", "● Connected", "● FullyInSync", "on", "off", "off"},
		[]string{"peer-spoke", "vnet-spoke", "10.20.0.0/16, 10.21.0.0/16", "● Initiated", "● LocalNotInSync", "off", "on", "off"},
		[]string{"peer-old", "vnet-legacy", "10.30.0.0/16", "● Disconnected", "● RemoteNotInSync", "off", "off", "on"},
	)...)
	wantLines(t, bodyOf(t, "vnet", "Peerings", 160), append(want, ""))
}

func TestVNetPeeringColors(t *testing.T) {
	v := openedAt(t, "vnet", "Peerings", 160, 40)
	hub, spoke, old := styledLine(t, v, "peer-hub"), styledLine(t, v, "peer-spoke"), styledLine(t, v, "peer-old")
	wantFg(t, hub, "Connected", fgGood)
	wantFg(t, hub, "FullyInSync", fgGood)
	wantFg(t, spoke, "Initiated", fgWarn)
	wantFg(t, spoke, "LocalNotInSync", fgWarn)
	wantFg(t, old, "Disconnected", fgBad)
	wantFg(t, old, "RemoteNotInSync", fgWarn)
	wantFg(t, hub, "on", fgGood)
	wantFg(t, hub, "off", fgMuted)
}

func TestVNetWithoutPeeringsOrSubnets(t *testing.T) {
	row := typedRow(t, vnetType, `{}`)
	for _, tab := range []string{"Subnets", "Peerings"} {
		v := deliver(t, openedAtRow(t, tab), row, nil)
		wantLines(t, renderBody(v), []string{tab, "none", ""})
	}
}

func TestVNetEssentialsDefaults(t *testing.T) {
	for _, c := range []struct {
		name, props, label, want string
	}{
		{"no dhcpOptions", `{}`, "DNS servers", "Azure-provided"},
		{"empty dhcpOptions", `{"dhcpOptions":{}}`, "DNS servers", "Azure-provided"},
		{"empty dns list", `{"dhcpOptions":{"dnsServers":[]}}`, "DNS servers", "Azure-provided"},
		{"custom dns", `{"dhcpOptions":{"dnsServers":["1.1.1.1","8.8.8.8"]}}`, "DNS servers", "1.1.1.1, 8.8.8.8"},
		{"ddos missing", `{}`, "DDoS protection", "off"},
		{"ddos off", `{"enableDdosProtection":false}`, "DDoS protection", "off"},
		{"ddos on", `{"enableDdosProtection":true}`, "DDoS protection", "on"},
		{"no address space", `{}`, "Address space", "—"},
		{"no counts", `{}`, "Subnets", "0"},
		{"no counts of peerings", `{}`, "Peerings", "0"},
		{"counts", `{"subnets":[{},{}],"virtualNetworkPeerings":[{},{},{}]}`, "Peerings", "3"},
		{"subnet count", `{"subnets":[{},{}],"virtualNetworkPeerings":[{},{},{}]}`, "Subnets", "2"},
		{"no state", `{}`, "Status", "—"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := valueOf(overviewBody(t, typedRow(t, vnetType, c.props), 80), c.label); got != c.want {
				t.Errorf("%s = %q, want %q", c.label, got, c.want)
			}
		})
	}
}

func TestVNetSubnetCells(t *testing.T) {
	props := `{"subnets":[
		{"name":"both","properties":{"addressPrefix":"10.1.0.0/24","addressPrefixes":["10.9.0.0/24"]}},
		{"name":"many","properties":{"addressPrefixes":["10.2.0.0/25","10.2.0.128/25"],"delegations":[{"properties":{}},{"properties":{"serviceName":"Svc.One/x"}}]}},
		{"name":"bare"}
	]}`
	got := bodyOfRow(t, typedRow(t, vnetType, props), "Subnets", 200)
	want := append([]string{"Subnets"}, wantTable([]int{4, 26, 10, 3, 11, 11, 10, 25},
		[]string{"Name", "Prefix", "IPs in use", "NSG", "Route table", "NAT gateway", "Delegation", "Private endpoint policies"},
		[]string{"both", "10.1.0.0/24", "0", "—", "—", "—", "—", "—"},
		[]string{"many", "10.2.0.0/25, 10.2.0.128/25", "0", "—", "—", "—", "Svc.One/x", "—"},
		[]string{"bare", "—", "0", "—", "—", "—", "—", "—"},
	)...)
	wantLines(t, got, append(want, ""))
}

// bodyOfRow is the unboxed body of a row on the tab, drawn on a pane of this width.
func TestVNetPeeringCells(t *testing.T) {
	props := `{"virtualNetworkPeerings":[{"name":"p1","properties":{"peeringState":"Connected"}},{"name":"p2"}]}`
	got := bodyOfRow(t, typedRow(t, vnetType, props), "Peerings", 200)
	want := append([]string{"Peerings"}, wantTable([]int{4, 14, 20, 11, 4, 17, 15, 15},
		[]string{"Name", "Remote network", "Remote address space", "State", "Sync", "Forwarded traffic", "Gateway transit", "Remote gateways"},
		[]string{"p1", "—", "—", "● Connected", "—", "—", "—", "—"},
		[]string{"p2", "—", "—", "—", "—", "—", "—", "—"},
	)...)
	wantLines(t, got, append(want, ""))
}

func TestVNetAttention(t *testing.T) {
	const nsg = `"networkSecurityGroup":{"id":"/x/nsg"}`
	subnet := func(name, extra string) string {
		return `{"subnets":[{"name":"` + name + `","properties":{` + extra + `}}]}`
	}
	peering := func(state string) string {
		return `{"virtualNetworkPeerings":[{"name":"pr","properties":{"peeringState":"` + state + `"}}]}`
	}
	noNSG := func(name string) []string { return []string{"▲ subnet " + name + " has no network security group"} }
	tests := []struct {
		name  string
		props string
		want  []string
	}{
		{"nothing to report", `{}`, nil},
		{"subnet without NSG", subnet("snet-a", ""), noNSG("snet-a")},
		{"subnet with NSG", subnet("snet-a", nsg), nil},
		{"subnet with an empty NSG id", subnet("snet-a", `"networkSecurityGroup":{}`), noNSG("snet-a")},
		{"subnet without properties", `{"subnets":[{"name":"snet-a"}]}`, noNSG("snet-a")},
		{"name that only contains an exempt one", subnet("my-GatewaySubnet", ""), noNSG("my-GatewaySubnet")},
		{"gateway subnet", subnet("GatewaySubnet", ""), nil},
		{"gateway subnet in lower case", subnet("gatewaysubnet", ""), nil},
		{"firewall subnet", subnet("AzureFirewallSubnet", ""), nil},
		{"firewall subnet in upper case", subnet("AZUREFIREWALLSUBNET", ""), nil},
		{"firewall management subnet", subnet("AzureFirewallManagementSubnet", ""), nil},
		{"firewall management subnet in lower case", subnet("azurefirewallmanagementsubnet", ""), nil},
		{"bastion subnet", subnet("AzureBastionSubnet", ""), nil},
		{"bastion subnet in mixed case", subnet("azureBASTIONsubnet", ""), nil},
		{"route server subnet", subnet("RouteServerSubnet", ""), nil},
		{"route server subnet in lower case", subnet("routeserversubnet", ""), nil},
		{"exempt subnet with NSG", subnet("GatewaySubnet", nsg), nil},
		{"connected peering", peering("Connected"), nil},
		{"connected peering in lower case", peering("connected"), nil},
		{"disconnected peering", peering("Disconnected"), []string{"✗ peering pr is disconnected"}},
		{"disconnected peering in upper case", peering("DISCONNECTED"), []string{"✗ peering pr is disconnected"}},
		{"initiated peering", peering("Initiated"), []string{"▲ peering pr is Initiated"}},
		{"peering in another state", peering("Failed"), []string{"▲ peering pr is Failed"}},
		{"peering without a state", `{"virtualNetworkPeerings":[{"name":"pr"}]}`, nil},
		{"subnets first, then peerings in order", `{"subnets":[{"name":"s1"},{"name":"s2","properties":{}}],"virtualNetworkPeerings":[` +
			`{"name":"a","properties":{"peeringState":"Initiated"}},{"name":"b","properties":{"peeringState":"Disconnected"}},{"name":"c","properties":{"peeringState":"Connected"}}]}`,
			[]string{"▲ subnet s1 has no network security group", "▲ subnet s2 has no network security group", "▲ peering a is Initiated", "✗ peering b is disconnected"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(vnetAttention(typedRow(t, vnetType, tt.props)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVNetAttentionOfEmptyRow(t *testing.T) {
	if got := vnetAttention(nil); len(got) != 0 {
		t.Errorf("attention = %v, want none", got)
	}
}

func TestVNetAttentionBoxColor(t *testing.T) {
	v := openedAt(t, "vnet", "", 110, 80)
	wantFg(t, styledLine(t, v, "╭ Attention"), "Attention", fgBad)
	warnOnly := deliver(t, openedAtRow(t, ""), typedRow(t, vnetType, `{"subnets":[{"name":"s"}]}`), nil)
	wantFg(t, styledLine(t, warnOnly, "╭ Attention"), "Attention", fgWarn)
}

func TestVNetTabs(t *testing.T) {
	if got := Tabs(vnetType); !reflect.DeepEqual(got, []string{"Overview", "Subnets", "Peerings", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if got := Tabs("Microsoft.Network/VirtualNetworks"); len(got) != 4 {
		t.Errorf("Tabs in another case = %v", got)
	}
	if k := kindOf("Microsoft.Network/VirtualNetworks"); k.command != "vnet" || k.attention == nil {
		t.Errorf("kind = %+v", k)
	}
}

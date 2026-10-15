package views

import (
	"fmt"
	"strings"
)

var vnetTabs = []tabDef{
	{overviewTab, boxesTab(vnetEssentials)},
	{"Subnets", boxesTab(vnetSubnets)},
	{"Peerings", boxesTab(vnetPeerings)},
	{"JSON", jsonTab},
}

var vnetEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: []string{"properties", "provisioningState"}, format: fmtState},
	{label: "Address space", path: []string{"properties", "addressSpace", "addressPrefixes"}, format: fmtJoin},
	{label: "DNS servers", value: dnsServers},
	{label: "DDoS protection", path: []string{"properties", "enableDdosProtection"}, format: fmtOnOffOr(false)},
	{label: "Subnets", path: []string{"properties", "subnets"}, format: fmtCount},
	{label: "Peerings", path: []string{"properties", "virtualNetworkPeerings"}, format: fmtCount},
	resourceGroupField,
	locationField,
	subscriptionField,
}}

// dnsServers lists the custom DNS servers; a network without any uses Azure's.
func dnsServers(row map[string]any) string {
	return first(strings.Join(items(row, "properties", "dhcpOptions", "dnsServers"), ", "), "Azure-provided")
}

var vnetSubnets = boxDef{title: "Subnets", table: &tableDef{
	path: []string{"properties", "subnets"},
	cols: []colDef{
		{title: "Name", path: []string{"name"}},
		{title: "Prefix", value: subnetPrefix},
		{title: "IPs in use", path: []string{"properties", "ipConfigurations"}, format: fmtCount},
		{title: "NSG", path: []string{"properties", "networkSecurityGroup", "id"}, format: fmtLast},
		{title: "Route table", path: []string{"properties", "routeTable", "id"}, format: fmtLast},
		{title: "NAT gateway", path: []string{"properties", "natGateway", "id"}, format: fmtLast},
		{title: "Delegation", value: delegations},
		{title: "Private endpoint policies", path: []string{"properties", "privateEndpointNetworkPolicies"}},
	},
}}

// subnetPrefix is the address prefix of a subnet, else its address prefixes.
func subnetPrefix(subnet any) string {
	return orDash(first(str(subnet, "properties", "addressPrefix"), strings.Join(items(subnet, "properties", "addressPrefixes"), ", ")))
}

// delegations lists the services a subnet is delegated to, without the
// "Microsoft." prefix, as resource types are shown.
func delegations(subnet any) string {
	var names []string
	for _, d := range elems(dig(subnet, "properties", "delegations")) {
		if name := str(d, "properties", "serviceName"); name != "" {
			names = append(names, ShortType(name))
		}
	}
	return orDash(strings.Join(names, ", "))
}

var vnetPeerings = boxDef{title: "Peerings", table: &tableDef{
	path: []string{"properties", "virtualNetworkPeerings"},
	cols: []colDef{
		{title: "Name", path: []string{"name"}},
		{title: "Remote network", path: []string{"properties", "remoteVirtualNetwork", "id"}, format: fmtLast},
		{title: "Remote address space", path: []string{"properties", "remoteAddressSpace", "addressPrefixes"}, format: fmtJoin},
		{title: "State", path: []string{"properties", "peeringState"}, format: fmtState},
		{title: "Sync", path: []string{"properties", "peeringSyncLevel"}, format: fmtState},
		{title: "Forwarded traffic", path: []string{"properties", "allowForwardedTraffic"}, format: fmtOnOff},
		{title: "Gateway transit", path: []string{"properties", "allowGatewayTransit"}, format: fmtOnOff},
		{title: "Remote gateways", path: []string{"properties", "useRemoteGateways"}, format: fmtOnOff},
	},
}}

// nsgExempt are the subnets the missing-NSG rule skips: Azure services that
// manage their own traffic rules.
var nsgExempt = []string{"GatewaySubnet", "AzureFirewallSubnet", "AzureFirewallManagementSubnet", "AzureBastionSubnet", "RouteServerSubnet"}

// vnetAttention lists what needs a look in a virtual network row.
func vnetAttention(row map[string]any) []alert {
	var out []alert
	for _, s := range elems(dig(row, "properties", "subnets")) {
		name := str(s, "name")
		if str(s, "properties", "networkSecurityGroup", "id") == "" && !containsFold(nsgExempt, name) {
			out = append(out, alert{text: fmt.Sprintf("subnet %s has no network security group", orDash(name))})
		}
	}
	for _, p := range elems(dig(row, "properties", "virtualNetworkPeerings")) {
		out = append(out, peeringAttention(p)...)
	}
	return out
}

func peeringAttention(peering any) []alert {
	name, state := orDash(str(peering, "name")), str(peering, "properties", "peeringState")
	switch {
	case state == "" || strings.EqualFold(state, "Connected"):
		return nil
	case strings.EqualFold(state, "Disconnected"):
		return []alert{{text: fmt.Sprintf("peering %s is disconnected", name), severe: true}}
	}
	return []alert{{text: fmt.Sprintf("peering %s is %s", name, state)}}
}

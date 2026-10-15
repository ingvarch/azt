package views

import (
	"slices"
)

// resourceKind describes how azt shows one kind of resource.
type resourceKind struct {
	// types are the resource types the kind covers, in lower case.
	types []string
	// command is the palette command that lists the kind; desc is its description.
	command, desc string
	// tabs are the tabs of the details; nil means the generic tabs.
	tabs []tabDef
	// attention lists what needs a look; the Overview tab draws it.
	attention func(row map[string]any) []alert
}

const aksType = "microsoft.containerservice/managedclusters"

// kinds lists the kinds in the order of the palette; a type belongs to the first kind that lists it.
var kinds = []resourceKind{
	{types: []string{"microsoft.compute/virtualmachines"}, command: "vm", desc: "virtual machines in every subscription"},
	{types: []string{aksType}, command: "aks", desc: "AKS clusters in every subscription", tabs: aksTabs, attention: aksAttention},
	{types: []string{"microsoft.keyvault/vaults"}, command: "kv", desc: "key vaults in every subscription", tabs: kvTabs, attention: kvAttention},
	{types: []string{"microsoft.sql/servers"}, command: "sql", desc: "SQL servers in every subscription"},
	{types: []string{"microsoft.storage/storageaccounts"}, command: "storage", desc: "storage accounts in every subscription", tabs: storageTabs, attention: storageAttention},
	{types: []string{"microsoft.web/sites"}, command: "app", desc: "web apps in every subscription"},
	{types: []string{"microsoft.network/virtualnetworks"}, command: "vnet", desc: "virtual networks in every subscription", tabs: vnetTabs, attention: vnetAttention},
	{types: []string{"microsoft.managedidentity/userassignedidentities"}, command: "mi", desc: "managed identities in every subscription", tabs: miTabs},
	{types: []string{"microsoft.containerregistry/registries"}, command: "acr", desc: "container registries in every subscription", tabs: acrTabs, attention: acrAttention},
	{types: []string{"microsoft.network/networksecuritygroups"}, command: "nsg", desc: "network security groups in every subscription", tabs: nsgTabs, attention: nsgAttention},
}

// kindOf is the kind of a resource type; the zero kind for a type no kind lists.
func kindOf(resourceType string) resourceKind { return findKind(kinds, resourceType) }

func findKind(list []resourceKind, resourceType string) resourceKind {
	i := slices.IndexFunc(list, func(k resourceKind) bool {
		return containsFold(k.types, resourceType)
	})
	if i < 0 {
		return resourceKind{}
	}
	return list[i]
}

// tabDefs returns the tabs of a resource type, the Overview ending with the
// kind's Attention box and Tags.
func tabDefs(resourceType string) []tabDef {
	k := kindOf(resourceType)
	defs := slices.Clone(k.tabs)
	if k.tabs == nil {
		defs = slices.Clone(genericTabs)
	}
	for i, d := range defs {
		if d.name == overviewTab {
			defs[i].render = withTail(d.render, k.attention)
		}
	}
	return defs
}

// TypeCommand is a palette command that lists the resources of one type.
type TypeCommand struct{ Name, Desc, Type string }

// TypeCommands lists the type commands in the order of the palette.
func TypeCommands() []TypeCommand { return typeCommands(kinds) }

func typeCommands(list []resourceKind) []TypeCommand {
	var out []TypeCommand
	for _, k := range list {
		if k.command != "" {
			out = append(out, TypeCommand{k.command, k.desc, k.types[0]})
		}
	}
	return out
}

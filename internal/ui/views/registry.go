package views

import (
	"slices"
)

// resourceKind describes how azt shows one kind of resource.
type resourceKind struct {
	// name is the plural a person says; it titles the lists of the kind.
	name string
	// types are the resource types the kind covers, in lower case.
	types []string
	// command is the palette command that lists the kind; kinds that share a
	// command merge, and the first one's desc describes it.
	command, desc string
	// tabs are the tabs of the details; nil means the generic tabs.
	tabs []tabDef
	// attention lists what needs a look; the Overview tab draws it.
	attention func(row map[string]any) []alert
}

const aksType = "microsoft.containerservice/managedclusters"

// kinds lists the kinds in the order of the palette; a type belongs to the first kind that lists it.
var kinds = []resourceKind{
	{name: "Virtual machines", types: []string{"microsoft.compute/virtualmachines"}, command: "vm", desc: "virtual machines in every subscription", tabs: vmTabs, attention: vmAttention},
	{name: "AKS clusters", types: []string{aksType}, command: "aks", desc: "AKS clusters in every subscription", tabs: aksTabs, attention: aksAttention},
	{name: "Key vaults", types: []string{"microsoft.keyvault/vaults"}, command: "kv", desc: "key vaults in every subscription", tabs: kvTabs, attention: kvAttention},
	{name: "SQL servers", types: []string{"microsoft.sql/servers"}, command: "sql", desc: "SQL servers in every subscription"},
	{name: "PostgreSQL servers", types: []string{"microsoft.dbforpostgresql/flexibleservers"}, command: "pg", desc: "PostgreSQL servers in every subscription", tabs: pgTabs, attention: pgAttention},
	{name: "DocumentDB clusters", types: []string{"microsoft.documentdb/mongoclusters"}, command: "docdb", desc: "DocumentDB clusters in every subscription", tabs: docdbTabs, attention: docdbAttention},
	{name: "AI Foundry", types: []string{"microsoft.cognitiveservices/accounts"}, command: "foundry", desc: "AI Foundry resources and projects in every subscription", tabs: foundryTabs, attention: foundryAttention},
	{name: "AI Foundry projects", types: []string{"microsoft.cognitiveservices/accounts/projects"}, command: "foundry", tabs: foundryProjectTabs},
	{name: "Front Door and CDN profiles", types: []string{"microsoft.cdn/profiles"}, command: "afd", desc: "Front Door and CDN profiles in every subscription", tabs: afdTabs},
	{name: "Storage accounts", types: []string{"microsoft.storage/storageaccounts"}, command: "storage", desc: "storage accounts in every subscription", tabs: storageTabs, attention: storageAttention},
	{name: "Web apps", types: []string{"microsoft.web/sites"}, command: "app", desc: "web apps in every subscription"},
	{name: "Virtual networks", types: []string{"microsoft.network/virtualnetworks"}, command: "vnet", desc: "virtual networks in every subscription", tabs: vnetTabs, attention: vnetAttention},
	{name: "Managed identities", types: []string{"microsoft.managedidentity/userassignedidentities"}, command: "mi", desc: "managed identities in every subscription", tabs: miTabs},
	{name: "Container registries", types: []string{"microsoft.containerregistry/registries"}, command: "acr", desc: "container registries in every subscription", tabs: acrTabs, attention: acrAttention},
	{name: "Network security groups", types: []string{"microsoft.network/networksecuritygroups"}, command: "nsg", desc: "network security groups in every subscription", tabs: nsgTabs, attention: nsgAttention},
	{name: "DNS zones", types: []string{"microsoft.network/dnszones"}, command: "dns", desc: "public and private DNS zones in every subscription", tabs: dnsTabs, attention: dnsAttention},
	{name: "Private DNS zones", types: []string{"microsoft.network/privatednszones"}, command: "dns", tabs: pdnsTabs, attention: pdnsAttention},
}

// kindOf is the kind of a resource type; the zero kind for a type no kind lists.
func kindOf(resourceType string) resourceKind { return findKind(kinds, resourceType) }

// KindName is the name of the kind of a resource type, or ShortType for a type no kind lists.
func KindName(resourceType string) string {
	if k := kindOf(resourceType); k.name != "" {
		return k.name
	}
	return ShortType(resourceType)
}

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

// TypeCommand is a palette command that lists the resources of one or more types.
type TypeCommand struct {
	Name, Desc string
	Types      []string
}

// TypeCommands lists the type commands in the order of the palette.
func TypeCommands() []TypeCommand { return typeCommands(kinds) }

// typeCommands merges kinds that share a command into the place of the first.
func typeCommands(list []resourceKind) []TypeCommand {
	var out []TypeCommand
	for _, k := range list {
		if k.command == "" {
			continue
		}
		i := slices.IndexFunc(out, func(c TypeCommand) bool { return c.Name == k.command })
		if i < 0 {
			out = append(out, TypeCommand{k.command, k.desc, slices.Clone(k.types)})
			continue
		}
		out[i].Types = append(out[i].Types, k.types...)
	}
	return out
}

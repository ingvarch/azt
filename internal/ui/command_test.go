package ui

import (
	"slices"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		in    string
		kind  commandKind
		title string // of the view, for cmdView
		err   string
	}{
		{in: "subs", kind: cmdView, title: "Subscriptions"},
		{in: "rg", kind: cmdView, title: "Resource groups"},
		{in: "vm", kind: cmdView, title: "compute/virtualmachines"},
		{in: "aks", kind: cmdView, title: "containerservice/managedclusters"},
		{in: "kv", kind: cmdView, title: "keyvault/vaults"},
		{in: "sql", kind: cmdView, title: "sql/servers"},
		{in: "storage", kind: cmdView, title: "storage/storageaccounts"},
		{in: "app", kind: cmdView, title: "web/sites"},
		{in: "vnet", kind: cmdView, title: "network/virtualnetworks"},
		{in: "mi", kind: cmdView, title: "managedidentity/userassignedidentities"},
		{in: "MI", kind: cmdView, title: "managedidentity/userassignedidentities"},
		{in: "acr", kind: cmdView, title: "containerregistry/registries"},
		{in: "nsg", kind: cmdView, title: "network/networksecuritygroups"},
		{in: "NSG", kind: cmdView, title: "network/networksecuritygroups"},
		{in: "dns", kind: cmdView, title: "network/dnszones"},
		{in: "pdns", kind: cmdView, title: "network/privatednszones"},
		{in: "PDNS", kind: cmdView, title: "network/privatednszones"},
		{in: "type microsoft.cache/redis", kind: cmdView, title: "cache/redis"},
		{in: "  type   microsoft.cache/redis  ", kind: cmdView, title: "cache/redis"},
		{in: "VM", kind: cmdView, title: "compute/virtualmachines"},
		{in: "type", err: "usage: type <resource type>"},
		{in: "type  ", err: "usage: type <resource type>"},
		{in: "help", kind: cmdHelp},
		{in: "q", kind: cmdQuit},
		{in: "quit", kind: cmdQuit},
		{in: "nope", err: "unknown command: nope"},
		{in: "nope arg", err: "unknown command: nope"},
		{in: "", kind: cmdNone},
		{in: "   ", kind: cmdNone},
	}
	for _, tt := range tests {
		c, err := parseCommand(tt.in)
		if tt.err != "" {
			if err == nil || err.Error() != tt.err {
				t.Errorf("parseCommand(%q) error = %v, want %q", tt.in, err, tt.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCommand(%q) error = %v", tt.in, err)
			continue
		}
		if c.kind != tt.kind {
			t.Errorf("parseCommand(%q) kind = %v, want %v", tt.in, c.kind, tt.kind)
		}
		if tt.kind == cmdView && c.view.Title() != tt.title {
			t.Errorf("parseCommand(%q) title = %q, want %q", tt.in, c.view.Title(), tt.title)
		}
		if tt.kind != cmdView && c.view != nil {
			t.Errorf("parseCommand(%q) has a view", tt.in)
		}
	}
}

func TestCommandTable(t *testing.T) {
	want := []commandInfo{
		{"subs", "subscriptions"},
		{"rg", "resource groups in every subscription"},
		{"vm", "virtual machines in every subscription"},
		{"aks", "AKS clusters in every subscription"},
		{"kv", "key vaults in every subscription"},
		{"sql", "SQL servers in every subscription"},
		{"storage", "storage accounts in every subscription"},
		{"app", "web apps in every subscription"},
		{"vnet", "virtual networks in every subscription"},
		{"mi", "managed identities in every subscription"},
		{"acr", "container registries in every subscription"},
		{"nsg", "network security groups in every subscription"},
		{"dns", "DNS zones in every subscription"},
		{"pdns", "private DNS zones in every subscription"},
		{"type", "resources of any type: type <resource type>"},
		{"help", "keys and commands"},
		{"q", "quit"},
	}
	if !slices.Equal(commandTable, want) {
		t.Errorf("commandTable = %v, want %v", commandTable, want)
	}
}

func TestMatchingCommands(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", allNames()},
		{"s", []string{"subs", "sql", "storage"}},
		{"st", []string{"storage"}},
		{"S", []string{"subs", "sql", "storage"}},
		{"type vm", []string{"type"}},
		{"  v", []string{"vm", "vnet"}},
		{"m", []string{"mi"}},
		{"ac", []string{"acr"}},
		{"ns", []string{"nsg"}},
		{"d", []string{"dns"}},
		{"p", []string{"pdns"}},
		{"pd", []string{"pdns"}},
		{"zzz", nil},
	}
	for _, tt := range tests {
		if got := matchingCommands(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("matchingCommands(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func allNames() []string {
	var names []string
	for _, c := range commandTable {
		names = append(names, c.name)
	}
	return names
}

package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
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
		{in: "vm", kind: cmdView, title: "Virtual machines"},
		{in: "aks", kind: cmdView, title: "AKS clusters"},
		{in: "kv", kind: cmdView, title: "Key vaults"},
		{in: "sql", kind: cmdView, title: "SQL servers"},
		{in: "pg", kind: cmdView, title: "PostgreSQL servers"},
		{in: "PG", kind: cmdView, title: "PostgreSQL servers"},
		{in: "docdb", kind: cmdView, title: "DocumentDB clusters"},
		{in: "storage", kind: cmdView, title: "Storage accounts"},
		{in: "app", kind: cmdView, title: "Web apps"},
		{in: "vnet", kind: cmdView, title: "Virtual networks"},
		{in: "mi", kind: cmdView, title: "Managed identities"},
		{in: "MI", kind: cmdView, title: "Managed identities"},
		{in: "acr", kind: cmdView, title: "Container registries"},
		{in: "nsg", kind: cmdView, title: "Network security groups"},
		{in: "NSG", kind: cmdView, title: "Network security groups"},
		{in: "dns", kind: cmdView, title: "DNS zones"},
		{in: "pdns", err: "unknown command: pdns"},
		{in: "PDNS", err: "unknown command: PDNS"},
		{in: "type microsoft.web/serverfarms", kind: cmdView, title: "web/serverfarms"},
		{in: "  type   microsoft.web/serverfarms  ", kind: cmdView, title: "web/serverfarms"},
		{in: "VM", kind: cmdView, title: "Virtual machines"},
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

func TestParseCommandFoundry(t *testing.T) {
	c, err := parseCommand("Foundry")
	if err != nil || c.kind != cmdView {
		t.Fatalf("parseCommand(Foundry) = %+v, %v", c, err)
	}
	if got, want := c.view.Title(), "AI Foundry"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if _, err := parseCommand("ai"); err == nil || err.Error() != "unknown command: ai" {
		t.Errorf("parseCommand(ai) error = %v", err)
	}
}

func TestParseCommandTypes(t *testing.T) {
	tests := []struct{ in, want string }{
		{"dns", "microsoft.network/dnszones,microsoft.network/privatednszones"},
		{"foundry", "microsoft.cognitiveservices/accounts,microsoft.cognitiveservices/accounts/projects"},
		{"afd", "microsoft.cdn/profiles"},
		{"app", "microsoft.web/sites"},
		{"type microsoft.web/sites", "microsoft.web/sites"},
	}
	for _, tt := range tests {
		c, err := parseCommand(tt.in)
		if err != nil {
			t.Fatalf("parseCommand(%q) error = %v", tt.in, err)
		}
		if got := c.view.Spec().Type; got != tt.want {
			t.Errorf("parseCommand(%q) types = %q, want %q", tt.in, got, tt.want)
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
		{"pg", "PostgreSQL servers in every subscription"},
		{"docdb", "DocumentDB clusters in every subscription"},
		{"foundry", "AI Foundry resources and projects in every subscription"},
		{"afd", "Front Door and CDN profiles in every subscription"},
		{"storage", "storage accounts in every subscription"},
		{"app", "web apps in every subscription"},
		{"vnet", "virtual networks in every subscription"},
		{"mi", "managed identities in every subscription"},
		{"acr", "container registries in every subscription"},
		{"nsg", "network security groups in every subscription"},
		{"dns", "public and private DNS zones in every subscription"},
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
		{"d", []string{"docdb", "dns"}},
		{"p", []string{"pg"}},
		{"PG", []string{"pg"}},
		{"doc", []string{"docdb"}},
		{"f", []string{"foundry"}},
		{"af", []string{"afd"}},
		{"a", []string{"aks", "afd", "app", "acr"}},
		{"pd", nil},
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

// header is the column titles a command's view draws once the data for key has arrived.
func header(t *testing.T, input string, key data.Key, rows any) string {
	t.Helper()
	c, err := parseCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := c.view.Resize(200, 10).Update(data.ResultMsg{Key: key, Data: rows})
	first, _, _ := strings.Cut(v.Render(), "\n")
	return first
}

func TestParseCommandOpensTheListsOfAKind(t *testing.T) {
	if got := header(t, "vm", data.Key{Kind: data.VirtualMachines}, []azure.VM{{}}); !strings.Contains(got, "Public IP") {
		t.Errorf("vm header = %q, want the VM columns", got)
	}
	if got := header(t, "aks", data.Key{Kind: data.AKSClusters}, []azure.Cluster{{}}); !strings.Contains(got, "Auto-upgrade") {
		t.Errorf("aks header = %q, want the cluster columns", got)
	}
	if got := header(t, "type microsoft.compute/virtualmachines", data.Key{Kind: data.VirtualMachines}, []azure.VM{{}}); !strings.Contains(got, "Public IP") {
		t.Errorf("type header = %q, want the VM columns", got)
	}
}

func TestParseCommandOpensTheGenericListForOtherTypes(t *testing.T) {
	key := data.Key{Kind: data.ResourcesOfType, Type: "microsoft.web/sites"}
	got := header(t, "app", key, []azure.Resource{{}})
	if !strings.Contains(got, "Resource group") || strings.Contains(got, "Size") {
		t.Errorf("app header = %q, want the generic columns", got)
	}
}

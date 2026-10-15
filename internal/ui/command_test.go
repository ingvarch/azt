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

func TestCommandNamesOrder(t *testing.T) {
	want := []string{"subs", "rg", "vm", "aks", "kv", "sql", "storage", "app", "vnet", "type", "help", "q"}
	if !slices.Equal(commandNames, want) {
		t.Errorf("commandNames = %v, want %v", commandNames, want)
	}
}

func TestMatchingCommands(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", commandNames},
		{"s", []string{"subs", "sql", "storage"}},
		{"st", []string{"storage"}},
		{"S", []string{"subs", "sql", "storage"}},
		{"type vm", []string{"type"}},
		{"  v", []string{"vm", "vnet"}},
		{"zzz", nil},
	}
	for _, tt := range tests {
		if got := matchingCommands(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("matchingCommands(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestComplete(t *testing.T) {
	tests := []struct{ in, want string }{
		{"st", "storage"},
		{"s", "subs"},
		{"ty vm", "type vm"},
		{"k", "kv"},
		{"zzz", "zzz"},
		{"zzz arg", "zzz arg"},
		{"", "subs"},
	}
	for _, tt := range tests {
		if got := complete(tt.in); got != tt.want {
			t.Errorf("complete(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

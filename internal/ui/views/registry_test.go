package views

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestKindOfIgnoresCase(t *testing.T) {
	for _, in := range []string{"microsoft.keyvault/vaults", "Microsoft.KeyVault/Vaults", "MICROSOFT.KEYVAULT/VAULTS"} {
		if k := kindOf(in); k.command != "kv" {
			t.Errorf("kindOf(%q) = %q, want kv", in, k.command)
		}
	}
}

func TestKindOfFirstMatchWins(t *testing.T) {
	list := []resourceKind{
		{types: []string{"a/x", "a/y"}, command: "one"},
		{types: []string{"A/Y"}, command: "two"},
	}
	for _, in := range []string{"a/x", "a/y", "A/Y"} {
		if k := findKind(list, in); k.command != "one" {
			t.Errorf("findKind(%q) = %q, want one", in, k.command)
		}
	}
	if k := findKind(list, "b/z"); k.command != "" || k.types != nil {
		t.Errorf("findKind(unknown) = %+v, want the zero kind", k)
	}
}

func TestUnknownTypeGetsGenericTabs(t *testing.T) {
	for _, in := range []string{"microsoft.cache/redis", "", "microsoft.keyvault/vaults/secrets"} {
		if got := Tabs(in); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
			t.Errorf("Tabs(%q) = %v, want the generic tabs", in, got)
		}
	}
}

func TestTabsOfEveryKind(t *testing.T) {
	for _, k := range kinds {
		want := []string{"Overview", "JSON"}
		if k.command == "aks" {
			want = aksTabNames
		}
		for _, typ := range k.types {
			if got := Tabs(typ); !reflect.DeepEqual(got, want) {
				t.Errorf("Tabs(%q) = %v, want %v", typ, got, want)
			}
		}
	}
}

func TestKindsListEachTypeOnce(t *testing.T) {
	var seen []string
	for _, k := range kinds {
		for _, typ := range k.types {
			if slices.Contains(seen, typ) {
				t.Errorf("type %q is listed twice", typ)
			}
			seen = append(seen, typ)
			if typ != strings.ToLower(typ) {
				t.Errorf("type %q is not lower case", typ)
			}
		}
	}
}

func TestTypeCommands(t *testing.T) {
	want := []TypeCommand{
		{"vm", "virtual machines in every subscription", "microsoft.compute/virtualmachines"},
		{"aks", "AKS clusters in every subscription", "microsoft.containerservice/managedclusters"},
		{"kv", "key vaults in every subscription", "microsoft.keyvault/vaults"},
		{"sql", "SQL servers in every subscription", "microsoft.sql/servers"},
		{"storage", "storage accounts in every subscription", "microsoft.storage/storageaccounts"},
		{"app", "web apps in every subscription", "microsoft.web/sites"},
		{"vnet", "virtual networks in every subscription", "microsoft.network/virtualnetworks"},
		{"mi", "managed identities in every subscription", "microsoft.managedidentity/userassignedidentities"},
		{"acr", "container registries in every subscription", "microsoft.containerregistry/registries"},
	}
	if got := TypeCommands(); !reflect.DeepEqual(got, want) {
		t.Errorf("TypeCommands() = %v, want %v", got, want)
	}
}

func TestTypeCommandsSkipKindsWithoutCommand(t *testing.T) {
	list := []resourceKind{{types: []string{"a/x"}}, {types: []string{"a/y"}, command: "y", desc: "d"}}
	want := []TypeCommand{{"y", "d", "a/y"}}
	if got := typeCommands(list); !reflect.DeepEqual(got, want) {
		t.Errorf("typeCommands = %v, want %v", got, want)
	}
}

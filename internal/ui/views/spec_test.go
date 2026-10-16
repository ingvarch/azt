package views

import (
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

func specCases() map[string]View {
	sub := azure.Subscription{ID: "id-1", Name: "Dev"}
	group := azure.ResourceGroup{Name: "rg-a"}
	return map[string]View{
		"subscriptions":     NewSubscriptions(),
		"resourceGroups":    NewResourceGroups(sub),
		"resources":         NewResources(sub.ID, group),
		"resourcesOfType":   NewResourcesOfType(sub.ID, group, "microsoft.web/sites"),
		"details":           NewDetails(sub.ID, azure.Resource{ID: "/r/web", Name: "web-app"}),
		"allResourceGroups": NewAllResourceGroups(),
		"resourcesByType":   NewResourcesByType("microsoft.keyvault/vaults"),
	}
}

func TestSpecKinds(t *testing.T) {
	for kind, v := range specCases() {
		if got := v.Spec().Kind; got != kind {
			t.Errorf("%s: Kind = %q", kind, got)
		}
	}
}

func TestBuildRoundTrip(t *testing.T) {
	for kind, v := range specCases() {
		t.Run(kind, func(t *testing.T) {
			got, err := Build(v.Spec())
			if err != nil {
				t.Fatal(err)
			}
			if got.Title() != v.Title() {
				t.Errorf("Title = %q, want %q", got.Title(), v.Title())
			}
			if fetchKey(t, got) != fetchKey(t, v) {
				t.Errorf("key = %+v, want %+v", fetchKey(t, got), fetchKey(t, v))
			}
			if got.Spec() != v.Spec() {
				t.Errorf("Spec = %+v, want %+v", got.Spec(), v.Spec())
			}
		})
	}
}

func TestSpecCarriesFilter(t *testing.T) {
	for kind, v := range specCases() {
		f, ok := v.(Filterer)
		if !ok {
			if v.Spec().Filter != "" {
				t.Errorf("%s: filter on a view without one", kind)
			}
			continue
		}
		t.Run(kind, func(t *testing.T) {
			spec := f.SetFilter("web").Spec()
			if spec.Filter != "web" {
				t.Fatalf("Spec.Filter = %q", spec.Filter)
			}
			got, err := Build(spec)
			if err != nil {
				t.Fatal(err)
			}
			if got.(Filterer).Filter() != "web" {
				t.Errorf("built Filter = %q", got.(Filterer).Filter())
			}
		})
	}
}

func TestBuildErrors(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{"empty kind", Spec{}, "unknown view kind"},
		{"unknown kind", Spec{Kind: "nope"}, "unknown view kind"},
		{"explorer is not buildable", Spec{Kind: "explorer"}, "unknown view kind"},
		{"groups no subscription", Spec{Kind: "resourceGroups", SubscriptionName: "Dev"}, "subscription"},
		{"groups no name", Spec{Kind: "resourceGroups", Subscription: "id-1"}, "subscriptionName"},
		{"resources no subscription", Spec{Kind: "resources", Group: "rg"}, "subscription"},
		{"resources no group", Spec{Kind: "resources", Subscription: "id-1"}, "group"},
		{"of type no type", Spec{Kind: "resourcesOfType", Subscription: "id-1", Group: "rg"}, "type"},
		{"of type no group", Spec{Kind: "resourcesOfType", Subscription: "id-1", Type: "t"}, "group"},
		{"of type no subscription", Spec{Kind: "resourcesOfType", Group: "rg", Type: "t"}, "subscription"},
		{"details no id", Spec{Kind: "details", Subscription: "id-1", ResourceName: "n"}, "resourceId"},
		{"details no name", Spec{Kind: "details", Subscription: "id-1", ResourceID: "/r"}, "resourceName"},
		{"details no subscription", Spec{Kind: "details", ResourceID: "/r", ResourceName: "n"}, "subscription"},
		{"by type no type", Spec{Kind: "resourcesByType"}, "type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Build(tt.spec)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestBuildRestoresTheListsOfAKind(t *testing.T) {
	tests := []struct {
		spec Spec
		want data.Key
	}{
		{Spec{Kind: "resourcesByType", Type: vmType, Filter: "run"}, data.Key{Kind: data.VirtualMachines}},
		{Spec{Kind: "resourcesByType", Type: aksType}, data.Key{Kind: data.AKSClusters}},
		{Spec{Kind: "resourcesByType", Type: "microsoft.web/sites"}, data.Key{Kind: data.ResourcesOfType, Type: "microsoft.web/sites"}},
	}
	for _, tt := range tests {
		v, err := Build(tt.spec)
		if err != nil {
			t.Fatal(err)
		}
		if got := fetchKey(t, v); got != tt.want {
			t.Errorf("Build(%+v) key = %+v, want %+v", tt.spec, got, tt.want)
		}
		if got := v.Spec(); got != tt.spec {
			t.Errorf("Spec = %+v, want %+v", got, tt.spec)
		}
	}
}

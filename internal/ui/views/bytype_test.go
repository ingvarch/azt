package views

import (
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

const dnsTypes = dnsType + "," + pdnsType

var zones = []azure.Resource{
	{ID: "/r/pub", Name: "contoso.com", Type: dnsType, ResourceGroup: "rg-a", SubscriptionID: "id-1", Subscription: "Dev"},
	{ID: "/r/priv", Name: "corp.local", Type: pdnsType, ResourceGroup: "rg-b", SubscriptionID: "id-2", Subscription: "Prod"},
}

func newZones() View {
	return NewResourcesByType(dnsType, pdnsType)
}

func TestResourcesByTypeSeveralTypes(t *testing.T) {
	v := newZones()
	if got, want := fetchKey(t, v), (data.Key{Kind: data.ResourcesOfType, Type: dnsTypes}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
	if got, want := v.Spec(), (Spec{Kind: "resourcesByType", Type: dnsTypes}); got != want {
		t.Errorf("Spec = %+v, want %+v", got, want)
	}
	if got, want := v.Title(), "DNS zones"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
}

func TestResourcesByTypeSeveralTypesShowsTypeColumn(t *testing.T) {
	got := deliver(t, newZones().Resize(140, 20), zones, nil).Render()
	header, rows, _ := strings.Cut(got, "\n")
	if n, ty, g := strings.Index(header, "Name"), strings.Index(header, "Type"), strings.Index(header, "Resource group"); n < 0 || ty < n || g < ty {
		t.Errorf("header %q lacks Name, Type, Resource group in order", header)
	}
	for _, want := range []string{"network/dnszones", "network/privatednszones"} {
		if !strings.Contains(rows, want) {
			t.Errorf("render lacks %q:\n%s", want, got)
		}
	}
}

func TestResourcesByTypeFilterFindsTypeColumn(t *testing.T) {
	got := filtered(deliver(t, newZones().Resize(140, 20), zones, nil), "private").Render()
	if !strings.Contains(got, "corp.local") || strings.Contains(got, "contoso.com") {
		t.Errorf("render:\n%s", got)
	}
}

func TestResourcesByTypeOneTypeHasNoTypeColumn(t *testing.T) {
	v := NewResourcesByType(dnsType)
	if got, want := fetchKey(t, v), (data.Key{Kind: data.ResourcesOfType, Type: dnsType}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
	if got, want := v.Spec(), (Spec{Kind: "resourcesByType", Type: dnsType}); got != want {
		t.Errorf("Spec = %+v, want %+v", got, want)
	}
	if got, want := v.Title(), "DNS zones"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
	header, _, _ := strings.Cut(deliver(t, v.Resize(140, 20), zones, nil).Render(), "\n")
	if strings.Contains(header, "Type") {
		t.Errorf("header %q has a Type column", header)
	}
}

func TestBuildResourcesByTypeSplitsTypes(t *testing.T) {
	for _, tt := range []struct {
		spec  Spec
		title string
	}{
		{Spec{Kind: "resourcesByType", Type: dnsTypes}, "DNS zones"},
		{Spec{Kind: "resourcesByType", Type: dnsType}, "DNS zones"},
	} {
		v, err := Build(tt.spec)
		if err != nil {
			t.Fatal(err)
		}
		if got := v.Spec(); got != tt.spec {
			t.Errorf("Spec = %+v, want %+v", got, tt.spec)
		}
		if got, want := fetchKey(t, v).Type, tt.spec.Type; got != want {
			t.Errorf("key type = %q, want %q", got, want)
		}
		if got := v.Title(); got != tt.title {
			t.Errorf("Title = %q, want %q", got, tt.title)
		}
	}
}

func TestResourcesByTypeEmptyText(t *testing.T) {
	for _, tt := range []struct {
		view View
		want string
	}{
		{newZones(), "No resources of these types."},
		{NewResourcesByType(dnsType), "No resources of this type."},
	} {
		if got := deliver(t, tt.view.Resize(140, 20), []azure.Resource{}, nil).Render(); !strings.Contains(got, tt.want) {
			t.Errorf("render lacks %q:\n%s", tt.want, got)
		}
	}
}

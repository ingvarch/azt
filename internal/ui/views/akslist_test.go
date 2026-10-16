package views

import (
	"slices"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

var clusterRows = []azure.Cluster{
	{
		Resource: azure.Resource{ID: "/r/prod", Name: "aks-prod", Type: aksType, Location: "westeurope", ResourceGroup: "rg-a", SubscriptionID: "id-1", Subscription: "Dev"},
		Power:    "Running", Provisioning: "Succeeded", CurrentVersion: "1.30.4", Version: "1.30",
		Pools: []azure.Pool{{Count: 6}}, Karpenter: 3, Tier: "Standard", Channel: "stable",
	},
	{
		Resource: azure.Resource{ID: "/r/idle", Name: "aks-idle", Type: aksType, Location: "northeurope", ResourceGroup: "rg-b", SubscriptionID: "id-2", Subscription: "Prod"},
		Power:    "Stopped", Provisioning: "Succeeded", Version: "1.29",
		Tier: "Free",
	},
	{
		Resource: azure.Resource{ID: "/r/up", Name: "aks-up", Type: aksType, Location: "northeurope", ResourceGroup: "rg-b", SubscriptionID: "id-2", Subscription: "Prod"},
		Power:    "Running", Provisioning: "Upgrading", CurrentVersion: "1.29.8", Version: "1.30",
		Pools: []azure.Pool{{Count: 1}, {Count: 3}}, Tier: "Premium", Channel: "rapid",
	},
}

func TestAKSClustersKeyTitleAndSpec(t *testing.T) {
	v := NewAKSClusters()
	if got, want := fetchKey(t, v), (data.Key{Kind: data.AKSClusters}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
	if got, want := v.Title(), "AKS clusters"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
	if got, want := v.Spec(), (Spec{Kind: "resourcesByType", Type: aksType}); got != want {
		t.Errorf("Spec = %+v, want %+v", got, want)
	}
}

func TestAKSClustersColumns(t *testing.T) {
	header := plain(firstLine(deliver(t, NewAKSClusters().Resize(200, 10), clusterRows, nil)))
	var got []string
	for _, c := range []string{"Name", "Status", "Kubernetes", "Nodes", "Tier", "Auto-upgrade", "Resource group", "Subscription", "Location"} {
		if !strings.Contains(header, c) {
			t.Fatalf("header %q lacks %q", header, c)
		}
		got = append(got, c)
	}
	if !slices.IsSortedFunc(got, func(a, b string) int { return strings.Index(header, a) - strings.Index(header, b) }) {
		t.Errorf("header %q has the columns out of order", header)
	}
}

func TestClusterRow(t *testing.T) {
	want := [][]string{
		{"aks-prod", "Running", "1.30.4", "9", "Standard", "stable", "rg-a", "Dev", "westeurope"},
		{"aks-idle", "Stopped", "1.29", "0", "Free", "none", "rg-b", "Prod", "northeurope"},
		{"aks-up", "Running · Upgrading", "1.29.8", "4", "Premium", "rapid", "rg-b", "Prod", "northeurope"},
	}
	for i, c := range clusterRows {
		if got := clusterRow(c); !slices.Equal(got, want[i]) {
			t.Errorf("clusterRow(%s) = %q, want %q", c.Name, got, want[i])
		}
	}
}

func TestClusterRowDashesMissingVersionAndTier(t *testing.T) {
	got := clusterRow(azure.Cluster{Resource: azure.Resource{Name: "aks-new"}})
	if got[2] != dash || got[4] != dash {
		t.Errorf("version, tier = %q, %q, want dashes", got[2], got[4])
	}
}

func TestAKSClustersDrawStatusAsState(t *testing.T) {
	v := deliver(t, NewAKSClusters().Resize(200, 10), clusterRows, nil)
	wantFg(t, styledLine(t, v, "aks-idle"), "● Stopped", fgBad)
}

func TestAKSClustersFilterByStatus(t *testing.T) {
	v := filtered(deliver(t, NewAKSClusters().Resize(200, 10), clusterRows, nil), "stopped")
	got := v.Render()
	if !strings.Contains(got, "aks-idle") || strings.Contains(got, "aks-prod") || strings.Contains(got, "aks-up") {
		t.Errorf("render:\n%s", got)
	}
}

func TestAKSClustersCount(t *testing.T) {
	v := NewAKSClusters()
	if _, ok := v.(Counter).Count(); ok {
		t.Error("Count() ok while loading")
	}
	if n, ok := deliver(t, v, clusterRows, nil).(Counter).Count(); !ok || n != len(clusterRows) {
		t.Errorf("Count() = %d, %v, want %d, true", n, ok, len(clusterRows))
	}
}

func TestAKSClustersOpen(t *testing.T) {
	v := deliver(t, NewAKSClusters().Resize(200, 10), clusterRows, nil)
	v, _ = press(v, keyDown)
	next, ok := v.Open()
	if !ok {
		t.Fatal("Open is false")
	}
	if got := next.Title(); got != "aks-idle" {
		t.Errorf("title = %q, want aks-idle", got)
	}
	if got, want := fetchKey(t, next), (data.Key{Kind: data.Details, Subscription: "id-2", ID: "/r/idle"}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
}

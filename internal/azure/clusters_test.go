package azure

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

func TestAKSClusters(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(nil, map[string]any{
			"id": "/subscriptions/s1/aks1", "name": "aks1", "type": "microsoft.containerservice/managedclusters", "location": "northeurope",
			"resourceGroup": "rg-a", "subscriptionId": "s1", "subscription": "Dev",
			"power": "Running", "provisioning": "Succeeded", "currentVersion": "1.30.4", "version": "1.30",
			"pools": []any{map[string]any{"count": 3}, map[string]any{"count": 2}}, "karpenter": 2,
			"tier": "Standard", "channel": "stable",
		}),
	}}

	got, err := AKSClusters(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}

	want := []Cluster{{
		Resource: Resource{
			ID: "/subscriptions/s1/aks1", Name: "aks1", Type: "microsoft.containerservice/managedclusters", Location: "northeurope",
			ResourceGroup: "rg-a", SubscriptionID: "s1", Subscription: "Dev",
		},
		Power: "Running", Provisioning: "Succeeded", CurrentVersion: "1.30.4", Version: "1.30",
		Pools: []Pool{{Count: 3}, {Count: 2}}, Karpenter: 2, Tier: "Standard", Channel: "stable",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AKSClusters() = %+v, want %+v", got, want)
	}
	if n := got[0].Nodes(); n != 7 {
		t.Errorf("Nodes() = %d, want 7: the pool counts plus the Karpenter nodes", n)
	}
	if got := scope(graph.requests[0]); len(got) != 0 {
		t.Errorf("scope = %v, want empty", got)
	}
}

func TestAKSClustersQuery(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}
	if _, err := AKSClusters(context.Background(), graph); err != nil {
		t.Fatal(err)
	}

	q := *graph.requests[0].Query
	for _, want := range []string{
		"where type =~ 'microsoft.containerservice/managedclusters'",
		"nodeResourceGroup",
		"where type =~ 'microsoft.compute/virtualmachines'",
		"karpenter.sh_nodepool",
		"pools = properties.agentPoolProfiles",
		"karpenter = coalesce(karpenter, 0)",
		wantJoin,
		"power = tostring(properties.powerState.code)",
		"provisioning = tostring(properties.provisioningState)",
		"currentVersion = tostring(properties.currentKubernetesVersion)",
		"version = tostring(properties.kubernetesVersion)",
		"tier = tostring(sku.tier)",
		"channel = tostring(properties.autoUpgradeProfile.upgradeChannel)",
		"summarize karpenter = count() by subscriptionId, nodeRg = tolower(resourceGroup)",
		") on subscriptionId, nodeRg",
		"| project id, name, type, location, resourceGroup, subscriptionId, subscription,\n    power = tostring(properties.powerState.code),",
		"| order by name asc",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %q:\n%s", want, q)
		}
	}
	if n := len(joins.FindAllString(q, -1)); n > 3 {
		t.Errorf("query has %d joins and unions, at most 3 are allowed", n)
	}
	// Resource Graph returns only the first page of a top-level summarize.
	if strings.Count(q, "summarize") != 1 || strings.Contains(q, "mv-expand") {
		t.Errorf("query summarizes outside the Karpenter join:\n%s", q)
	}
	if strings.Contains(q, " kind =") || strings.Contains(q, "project kind") {
		t.Errorf("query names a column kind:\n%s", q)
	}
}

func TestAKSClustersGraphError(t *testing.T) {
	boom := errors.New("boom")
	graph := &fakeGraph{err: boom}

	if _, err := AKSClusters(context.Background(), graph); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

package azure

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

func TestKQLString(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", `'plain'`},
		{"it's", `'it\'s'`},
		{`a\b`, `'a\\b'`},
		{`\'`, `'\\\''`},
		{"", `''`},
	}
	for _, tt := range tests {
		if got := kqlString(tt.in); got != tt.want {
			t.Errorf("kqlString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func scope(q armresourcegraph.QueryRequest) []string {
	var ids []string
	for _, s := range q.Subscriptions {
		ids = append(ids, *s)
	}
	return ids
}

func TestQueryFollowsPagesWithScope(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(to.Ptr("next"), map[string]any{"name": "a"}),
		page(nil, map[string]any{"name": "b"}),
	}}

	got, err := query[map[string]string](context.Background(), graph, "kql", "sub-1", "sub-2")
	if err != nil {
		t.Fatal(err)
	}

	want := []map[string]string{{"name": "a"}, {"name": "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("query() = %v, want %v", got, want)
	}
	for i, req := range graph.requests {
		if !reflect.DeepEqual(scope(req), []string{"sub-1", "sub-2"}) {
			t.Errorf("request %d scope = %v", i, scope(req))
		}
		if *req.Query != "kql" {
			t.Errorf("request %d query = %q", i, *req.Query)
		}
	}
	if tok := graph.requests[1].Options.SkipToken; tok == nil || *tok != "next" {
		t.Errorf("second request skip token = %v, want next", tok)
	}
}

func TestSubscriptionsUseTenantScope(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}

	if _, err := Subscriptions(context.Background(), graph); err != nil {
		t.Fatal(err)
	}
	if got := scope(graph.requests[0]); len(got) != 0 {
		t.Errorf("scope = %v, want empty", got)
	}
}

func TestResourceGroups(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(nil, map[string]any{"name": "rg-a", "location": "westeurope", "state": "Succeeded"}),
	}}

	got, err := ResourceGroups(context.Background(), graph, "sub-1")
	if err != nil {
		t.Fatal(err)
	}

	want := []ResourceGroup{{Name: "rg-a", Location: "westeurope", State: "Succeeded"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResourceGroups() = %+v, want %+v", got, want)
	}
	req := graph.requests[0]
	if !reflect.DeepEqual(scope(req), []string{"sub-1"}) {
		t.Errorf("scope = %v, want [sub-1]", scope(req))
	}
	wantQuery := `resourcecontainers
| where type =~ 'microsoft.resources/subscriptions/resourcegroups'
| project name, location, state = tostring(properties.provisioningState), subscriptionId
| order by name asc`
	if *req.Query != wantQuery {
		t.Errorf("query = %q, want %q", *req.Query, wantQuery)
	}
}

func TestResources(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(nil, map[string]any{"id": "/subscriptions/s/vm1", "name": "vm1", "type": "microsoft.compute/virtualmachines", "location": "northeurope"}),
	}}

	got, err := Resources(context.Background(), graph, "sub-1", "rg's")
	if err != nil {
		t.Fatal(err)
	}

	want := []Resource{{ID: "/subscriptions/s/vm1", Name: "vm1", Type: "microsoft.compute/virtualmachines", Location: "northeurope"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resources() = %+v, want %+v", got, want)
	}
	req := graph.requests[0]
	if !reflect.DeepEqual(scope(req), []string{"sub-1"}) {
		t.Errorf("scope = %v, want [sub-1]", scope(req))
	}
	wantQuery := `resources
| where resourceGroup =~ 'rg\'s'
| project id, name, type, location, resourceGroup, subscriptionId
| order by name asc`
	if *req.Query != wantQuery {
		t.Errorf("query = %q, want %q", *req.Query, wantQuery)
	}
}

func TestResourceDetails(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(nil, map[string]any{"id": "/x", "name": "vm1"}, map[string]any{"id": "/y"}),
	}}

	got, err := ResourceDetails(context.Background(), graph, "sub-1", `/x'\`)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{"id": "/x", "name": "vm1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResourceDetails() = %v, want %v", got, want)
	}
	req := graph.requests[0]
	if !reflect.DeepEqual(scope(req), []string{"sub-1"}) {
		t.Errorf("scope = %v, want [sub-1]", scope(req))
	}
	wantQuery := `resources
| where id =~ '/x\'\\'
` + wantJoin + `
| project-away subscriptionId1`
	if *req.Query != wantQuery {
		t.Errorf("query = %q, want %q", *req.Query, wantQuery)
	}
}

func TestResourceDetailsNotFound(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}

	_, err := ResourceDetails(context.Background(), graph, "sub-1", "/missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if got := err.Error(); !strings.Contains(got, "/missing") {
		t.Errorf("err = %q, want it to name the ID", got)
	}
}

const wantJoin = "| join kind=leftouter (resourcecontainers | where type =~ 'microsoft.resources/subscriptions' | project subscriptionId, subscription = name) on subscriptionId"

func TestAllResourceGroups(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(nil, map[string]any{"name": "rg-a", "location": "westeurope", "state": "Succeeded", "subscriptionId": "s1", "subscription": "Dev"}),
	}}

	got, err := AllResourceGroups(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}

	want := []ResourceGroup{{Name: "rg-a", Location: "westeurope", State: "Succeeded", SubscriptionID: "s1", Subscription: "Dev"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllResourceGroups() = %+v, want %+v", got, want)
	}
	req := graph.requests[0]
	if got := scope(req); len(got) != 0 {
		t.Errorf("scope = %v, want empty", got)
	}
	wantQuery := `resourcecontainers
| where type =~ 'microsoft.resources/subscriptions/resourcegroups'
` + wantJoin + `
| project name, location, state = tostring(properties.provisioningState), subscriptionId, subscription
| order by subscription asc, name asc`
	if *req.Query != wantQuery {
		t.Errorf("query = %q, want %q", *req.Query, wantQuery)
	}
}

func TestResourcesOfTypeTakesSeveralTypes(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}

	if _, err := ResourcesOfType(context.Background(), graph, []string{"Microsoft.A/x", "microsoft.b/y'z"}); err != nil {
		t.Fatal(err)
	}

	wantQuery := `resources
| where type in~ ('Microsoft.A/x', 'microsoft.b/y\'z')
` + wantJoin + `
| project id, name, type, location, resourceGroup, subscriptionId, subscription
| order by name asc`
	if got := *graph.requests[0].Query; got != wantQuery {
		t.Errorf("query = %q, want %q", got, wantQuery)
	}
}

func TestResourcesOfType(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(nil, map[string]any{
			"id": "/subscriptions/s1/vm1", "name": "vm1", "type": "microsoft.compute/virtualmachines", "location": "northeurope",
			"resourceGroup": "rg-a", "subscriptionId": "s1", "subscription": "Dev",
		}),
	}}

	got, err := ResourcesOfType(context.Background(), graph, []string{"microsoft.x/it's"})
	if err != nil {
		t.Fatal(err)
	}

	want := []Resource{{
		ID: "/subscriptions/s1/vm1", Name: "vm1", Type: "microsoft.compute/virtualmachines", Location: "northeurope",
		ResourceGroup: "rg-a", SubscriptionID: "s1", Subscription: "Dev",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResourcesOfType() = %+v, want %+v", got, want)
	}
	req := graph.requests[0]
	if got := scope(req); len(got) != 0 {
		t.Errorf("scope = %v, want empty", got)
	}
	wantQuery := `resources
| where type in~ ('microsoft.x/it\'s')
` + wantJoin + `
| project id, name, type, location, resourceGroup, subscriptionId, subscription
| order by name asc`
	if *req.Query != wantQuery {
		t.Errorf("query = %q, want %q", *req.Query, wantQuery)
	}
}

func TestResourceGroupID(t *testing.T) {
	g := ResourceGroup{Name: "rg-app", SubscriptionID: "0000-1111"}
	if got, want := g.ID(), "/subscriptions/0000-1111/resourceGroups/rg-app"; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
}

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
| project name, location, state = tostring(properties.provisioningState)
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
| project id, name, type, location
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
| where id =~ '/x\'\\'`
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

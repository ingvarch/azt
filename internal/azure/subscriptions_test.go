package azure

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

type fakeGraph struct {
	pages    []armresourcegraph.ClientResourcesResponse
	err      error
	requests []armresourcegraph.QueryRequest
}

func (f *fakeGraph) Resources(_ context.Context, q armresourcegraph.QueryRequest, _ *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	f.requests = append(f.requests, q)
	if f.err != nil {
		return armresourcegraph.ClientResourcesResponse{}, f.err
	}
	return f.pages[len(f.requests)-1], nil
}

// page builds a response the way the SDK decodes an objectArray result.
func page(skipToken *string, rows ...map[string]any) armresourcegraph.ClientResourcesResponse {
	data := make([]any, len(rows))
	for i, r := range rows {
		data[i] = r
	}
	return armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{Data: data, SkipToken: skipToken},
	}
}

func row(id, name, state string) map[string]any {
	return map[string]any{"subscriptionId": id, "name": name, "state": state}
}

func TestSubscriptionsFollowsPages(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(to.Ptr("next"), row("id-1", "Dev", "Enabled"), row("id-2", "Prod", "Enabled")),
		page(nil, row("id-3", "Sandbox", "Disabled")),
	}}

	got, err := Subscriptions(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}

	want := []Subscription{
		{ID: "id-1", Name: "Dev", State: "Enabled"},
		{ID: "id-2", Name: "Prod", State: "Enabled"},
		{ID: "id-3", Name: "Sandbox", State: "Disabled"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Subscriptions() = %+v, want %+v", got, want)
	}
	if len(graph.requests) != 2 {
		t.Fatalf("made %d requests, want 2", len(graph.requests))
	}
	if tok := graph.requests[1].Options.SkipToken; tok == nil || *tok != "next" {
		t.Errorf("second request skip token = %v, want next", tok)
	}
}

func TestSubscriptionsAsksForObjectArray(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}

	if _, err := Subscriptions(context.Background(), graph); err != nil {
		t.Fatal(err)
	}

	opts := graph.requests[0].Options
	if opts == nil || opts.ResultFormat == nil || *opts.ResultFormat != armresourcegraph.ResultFormatObjectArray {
		t.Errorf("result format = %+v, want objectArray", opts)
	}
}

func TestSubscriptionsReturnsQueryError(t *testing.T) {
	queryErr := errors.New("az login required")
	graph := &fakeGraph{err: queryErr}

	if _, err := Subscriptions(context.Background(), graph); !errors.Is(err, queryErr) {
		t.Errorf("err = %v, want %v", err, queryErr)
	}
}

func TestSubscriptionsRejectsUnexpectedData(t *testing.T) {
	resp := armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{Data: "not an array"},
	}
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{resp}}

	if _, err := Subscriptions(context.Background(), graph); err == nil {
		t.Error("err = nil, want decode error")
	}
}

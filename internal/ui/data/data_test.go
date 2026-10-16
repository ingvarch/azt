package data

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"

	"github.com/ingvarch/azt/internal/azure"
)

type countingFetcher struct {
	calls int
	data  any
	err   error
}

func (f *countingFetcher) fetch(Key) (any, error) {
	f.calls++
	return f.data, f.err
}

func run(t *testing.T, cmd tea.Cmd) ResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd = nil, want a command")
	}
	msg := cmd()
	res, ok := msg.(ResultMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want ResultMsg", msg)
	}
	return res
}

var key = Key{Kind: ResourceGroups, Subscription: "sub-1"}

func TestFetchAndRefresh(t *testing.T) {
	if got := Fetch(key)(); got != (FetchMsg{Key: key}) {
		t.Errorf("Fetch() = %+v", got)
	}
	if got := Refresh(key)(); got != (FetchMsg{Key: key, Force: true}) {
		t.Errorf("Refresh() = %+v", got)
	}
}

func TestRequestMissFetchesOnce(t *testing.T) {
	f := &countingFetcher{data: "rows"}
	c := NewCache(f.fetch)

	cmd := c.Request(FetchMsg{Key: key})
	msg := run(t, cmd)

	if msg.Key != key || msg.Data != "rows" || msg.Err != nil {
		t.Errorf("msg = %+v", msg)
	}
	if f.calls != 1 {
		t.Errorf("fetcher called %d times, want 1", f.calls)
	}
}

func TestRequestHitSkipsFetcher(t *testing.T) {
	f := &countingFetcher{data: "rows"}
	c := NewCache(f.fetch)
	c.Request(FetchMsg{Key: key})()
	c.Store(ResultMsg{Key: key, Data: "rows"})

	cmd := c.Request(FetchMsg{Key: key})
	msg := run(t, cmd)

	if msg.Data != "rows" || msg.Key != key {
		t.Errorf("msg = %+v", msg)
	}
	if f.calls != 1 {
		t.Errorf("fetcher called %d times, want 1", f.calls)
	}
}

func TestRequestInFlightReturnsNil(t *testing.T) {
	for _, force := range []bool{false, true} {
		f := &countingFetcher{}
		c := NewCache(f.fetch)
		c.Request(FetchMsg{Key: key})

		if cmd := c.Request(FetchMsg{Key: key, Force: force}); cmd != nil {
			t.Errorf("force=%v: cmd != nil while in flight", force)
		}
	}
}

func TestRequestInFlightTakesPrecedenceOverCache(t *testing.T) {
	f := &countingFetcher{data: "new"}
	c := NewCache(f.fetch)
	c.Store(ResultMsg{Key: key, Data: "old"})
	c.Request(FetchMsg{Key: key, Force: true})

	if cmd := c.Request(FetchMsg{Key: key}); cmd != nil {
		t.Error("cmd != nil while a refresh is in flight")
	}
	if f.calls != 0 {
		t.Errorf("fetcher called %d times, want 0", f.calls)
	}
}

func TestRequestForceFetchesAgainWhileCached(t *testing.T) {
	f := &countingFetcher{data: "new"}
	c := NewCache(f.fetch)
	c.Store(ResultMsg{Key: key, Data: "old"})

	cmd := c.Request(FetchMsg{Key: key, Force: true})
	msg := run(t, cmd)

	if msg.Data != "new" || f.calls != 1 {
		t.Errorf("msg = %+v, calls = %d", msg, f.calls)
	}
}

func TestStoreClearsInFlight(t *testing.T) {
	f := &countingFetcher{data: "rows"}
	c := NewCache(f.fetch)
	c.Request(FetchMsg{Key: key})
	c.Store(ResultMsg{Key: key, Data: "rows"})

	if cmd := c.Request(FetchMsg{Key: key, Force: true}); cmd == nil {
		t.Error("cmd = nil after Store, want a fetch")
	}
}

func TestErrorIsDeliveredButNotCached(t *testing.T) {
	fetchErr := errors.New("boom")
	f := &countingFetcher{err: fetchErr}
	c := NewCache(f.fetch)

	cmd := c.Request(FetchMsg{Key: key})
	msg := run(t, cmd)
	if !errors.Is(msg.Err, fetchErr) {
		t.Fatalf("msg.Err = %v, want %v", msg.Err, fetchErr)
	}
	c.Store(msg)

	f.err, f.data = nil, "rows"
	cmd = c.Request(FetchMsg{Key: key})
	msg = run(t, cmd)
	if msg.Data != "rows" || f.calls != 2 {
		t.Errorf("msg = %+v, calls = %d, want a second fetch", msg, f.calls)
	}
}

func TestErrorDropsCachedEntry(t *testing.T) {
	f := &countingFetcher{data: "rows"}
	c := NewCache(f.fetch)
	c.Store(ResultMsg{Key: key, Data: "old"})
	c.Store(ResultMsg{Key: key, Err: errors.New("boom")})

	c.Request(FetchMsg{Key: key})()
	if f.calls != 1 {
		t.Errorf("fetcher called %d times, want 1", f.calls)
	}
}

type fakeGraph struct {
	mu       sync.Mutex // the monitoring reads run at the same time
	rows     []map[string]any
	requests []armresourcegraph.QueryRequest
}

func (g *fakeGraph) Resources(_ context.Context, q armresourcegraph.QueryRequest, _ *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests = append(g.requests, q)
	data := make([]any, len(g.rows))
	for i, r := range g.rows {
		data[i] = r
	}
	return armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{Data: data},
	}, nil
}

type fakeARM struct {
	mu    sync.Mutex
	paths []string
}

func (a *fakeARM) Get(_ context.Context, path string, _ url.Values) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.paths = append(a.paths, path)
	return []byte(`{"value":[]}`), nil
}

type fetcherCase struct {
	name      string
	key       Key
	row       map[string]any
	want      any
	wantScope []string
	wantQuery string
}

var fetcherCases = []fetcherCase{
	{
		name: "subscriptions",
		key:  Key{Kind: Subscriptions},
		row:  map[string]any{"subscriptionId": "s1", "name": "Dev", "state": "Enabled"},
		want: []azure.Subscription{{ID: "s1", Name: "Dev", State: "Enabled"}},

		wantQuery: "microsoft.resources/subscriptions'",
	},
	{
		name:      "groups",
		key:       Key{Kind: ResourceGroups, Subscription: "s1"},
		row:       map[string]any{"name": "rg"},
		want:      []azure.ResourceGroup{{Name: "rg"}},
		wantScope: []string{"s1"},
		wantQuery: "resourcegroups'",
	},
	{
		name:      "resources",
		key:       Key{Kind: Resources, Subscription: "s1", Group: "rg"},
		row:       map[string]any{"id": "/a", "name": "a"},
		want:      []azure.Resource{{ID: "/a", Name: "a"}},
		wantScope: []string{"s1"},
		wantQuery: "resourceGroup =~ 'rg'",
	},
	{
		name:      "all groups",
		key:       Key{Kind: AllResourceGroups},
		row:       map[string]any{"name": "rg", "subscriptionId": "s1", "subscription": "Dev"},
		want:      []azure.ResourceGroup{{Name: "rg", SubscriptionID: "s1", Subscription: "Dev"}},
		wantQuery: "order by subscription asc, name asc",
	},
	{
		name:      "resources of type",
		key:       Key{Kind: ResourcesOfType, Type: "microsoft.web/sites"},
		row:       map[string]any{"id": "/a", "name": "a", "resourceGroup": "rg", "subscription": "Dev"},
		want:      []azure.Resource{{ID: "/a", Name: "a", ResourceGroup: "rg", Subscription: "Dev"}},
		wantQuery: "type in~ ('microsoft.web/sites')",
	},
	{
		name:      "resources of two types",
		key:       Key{Kind: ResourcesOfType, Type: "microsoft.network/dnszones,microsoft.network/privatednszones"},
		row:       map[string]any{"id": "/z", "name": "z"},
		want:      []azure.Resource{{ID: "/z", Name: "z"}},
		wantQuery: "type in~ ('microsoft.network/dnszones', 'microsoft.network/privatednszones')",
	},
	{
		name:      "details",
		key:       Key{Kind: Details, Subscription: "s1", ID: "/a"},
		row:       map[string]any{"id": "/a"},
		want:      map[string]any{"id": "/a"},
		wantScope: []string{"s1"},
		wantQuery: "id =~ '/a'",
	},
}

func TestNewFetcherRoutesKinds(t *testing.T) {
	for _, tt := range fetcherCases {
		t.Run(tt.name, func(t *testing.T) {
			graph := &fakeGraph{rows: []map[string]any{tt.row}}

			got, err := NewFetcher(context.Background(), graph, &fakeARM{})(tt.key)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("data = %#v, want %#v", got, tt.want)
			}
			req := graph.requests[0]
			var scope []string
			for _, s := range req.Subscriptions {
				scope = append(scope, *s)
			}
			if !reflect.DeepEqual(scope, tt.wantScope) {
				t.Errorf("scope = %v, want %v", scope, tt.wantScope)
			}
			if q := req.Query; !strings.Contains(*q, tt.wantQuery) {
				t.Errorf("query = %q, want it to contain %q", *q, tt.wantQuery)
			}
		})
	}
}

func TestNewFetcherRoutesAKSMonitoring(t *testing.T) {
	const id = "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.ContainerService/managedClusters/aks1"
	graph := &fakeGraph{}
	arm := &fakeARM{}

	got, err := NewFetcher(context.Background(), graph, arm)(Key{Kind: AKSMonitoring, Subscription: "s1", ID: id, Period: "7d"})
	if err != nil {
		t.Fatal(err)
	}

	m, ok := got.(azure.Monitoring)
	if !ok {
		t.Fatalf("data is %T, want azure.Monitoring", got)
	}
	if m.Interval != 30*time.Minute {
		t.Errorf("Interval = %v, want the 7d grain of 30m", m.Interval)
	}
	if len(arm.paths) != 6 {
		t.Errorf("%d ARM requests, want 6", len(arm.paths))
	}
	for _, p := range arm.paths {
		if !strings.HasPrefix(p, id+"/providers/Microsoft.Insights/") {
			t.Errorf("ARM path = %s", p)
		}
	}
	if len(graph.requests) != 2 {
		t.Fatalf("%d graph queries, want 2", len(graph.requests))
	}
	for i, req := range graph.requests {
		var subs []string
		for _, s := range req.Subscriptions {
			subs = append(subs, *s)
		}
		if !reflect.DeepEqual(subs, []string{"s1"}) {
			t.Errorf("graph query %d scope = %v, want [s1]", i, subs)
		}
	}
}

func TestNewFetcherRejectsUnknownKind(t *testing.T) {
	if _, err := NewFetcher(context.Background(), &fakeGraph{}, &fakeARM{})(Key{Kind: Kind(99)}); err == nil {
		t.Error("err = nil, want unknown kind error")
	}
}

func TestKeysOverDifferentTypeListsDiffer(t *testing.T) {
	one := Key{Kind: ResourcesOfType, Type: "microsoft.network/dnszones"}
	two := Key{Kind: ResourcesOfType, Type: "microsoft.network/dnszones,microsoft.network/privatednszones"}
	if one == two {
		t.Error("keys over different type lists are equal")
	}
}

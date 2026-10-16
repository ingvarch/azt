package azure

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

// safeARM answers metrics requests with one empty metric named after the first
// requested name, with the request's $filter as its unit, and fails the ones
// listed in fail by "names|filter".
type safeARM struct {
	mu    sync.Mutex
	calls []armCall
	fail  map[string]error
}

type armCall struct {
	path  string
	query url.Values
}

func (a *safeARM) Get(_ context.Context, path string, query url.Values) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, armCall{path, query})
	if strings.HasSuffix(path, "/diagnosticSettings") {
		return []byte(`{"value":[{"name":"d1","properties":{"workspaceId":"w"}}]}`), a.fail["diagnostics"]
	}
	names := query.Get("metricnames")
	if err := a.fail[names+"|"+query.Get("$filter")]; err != nil {
		return nil, err
	}
	first, _, _ := strings.Cut(names, ",")
	return []byte(`{"value":[{"name":{"value":"` + first + `"},"unit":` + strconv.Quote(query.Get("$filter")) + `,"timeseries":[]}]}`), nil
}

func (a *safeARM) metricsCall(t *testing.T, names, filter string) armCall {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.calls {
		if c.query.Get("metricnames") == names && c.query.Get("$filter") == filter {
			return c
		}
	}
	t.Fatalf("no request for %s with filter %q", names, filter)
	return armCall{}
}

// safeGraph answers rules and alerts queries with one row each.
type safeGraph struct {
	mu      sync.Mutex
	queries []string
	scopes  [][]string
	err     error
}

func (g *safeGraph) Resources(_ context.Context, q armresourcegraph.QueryRequest, _ *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.queries = append(g.queries, *q.Query)
	g.scopes = append(g.scopes, scope(q))
	if g.err != nil {
		return armresourcegraph.ClientResourcesResponse{}, g.err
	}
	if strings.HasPrefix(*q.Query, "alertsmanagementresources") {
		return page(nil, map[string]any{"alertRule": "/x/r1", "started": "2026-10-15T10:00:00Z"}), nil
	}
	return page(nil, map[string]any{"name": "rule1", "kind": "metric", "enabled": true}), nil
}

var monitoringNow = time.Date(2026, 10, 16, 10, 17, 42, 0, time.FixedZone("CEST", 2*3600))

func TestAKSMonitoringFillsEverySection(t *testing.T) {
	arm := &safeARM{}
	graph := &safeGraph{}

	m := AKSMonitoring(context.Background(), graph, arm, "s1", clusterID, "24h", monitoringNow)

	sections := []struct {
		name   string
		part   Part[[]Metric]
		first  string
		filter string
	}{
		{"pools", m.Pools, "node_cpu_usage_percentage", "nodepool eq '*'"},
		{"ready", m.Ready, "kube_node_status_condition", "condition eq 'Ready' and status2 eq '*'"},
		{"conditions", m.Conditions, "kube_node_status_condition", "condition eq '*' and status eq 'true'"},
		{"pods", m.Pods, "kube_pod_status_phase", "phase eq '*'"},
		{"cluster", m.Cluster, "apiserver_cpu_usage_percentage", ""},
	}
	for _, s := range sections {
		if s.part.Err != nil || len(s.part.Data) != 1 || s.part.Data[0].Name != s.first || s.part.Data[0].Unit != s.filter {
			t.Errorf("%s = %+v, want one metric %s with filter %q", s.name, s.part, s.first, s.filter)
		}
	}
	if m.Diagnostics.Err != nil || len(m.Diagnostics.Data) != 1 || m.Diagnostics.Data[0].Name != "d1" {
		t.Errorf("diagnostics = %+v", m.Diagnostics)
	}
	if m.Rules.Err != nil || len(m.Rules.Data) != 1 || m.Rules.Data[0].Name != "rule1" {
		t.Errorf("rules = %+v", m.Rules)
	}
	if m.Alerts.Err != nil || len(m.Alerts.Data) != 1 || m.Alerts.Data[0].Rule != "r1" {
		t.Errorf("alerts = %+v", m.Alerts)
	}
}

func TestAKSMonitoringRequests(t *testing.T) {
	arm := &safeARM{}
	graph := &safeGraph{}

	AKSMonitoring(context.Background(), graph, arm, "s1", clusterID, "24h", monitoringNow)

	if len(arm.calls) != 6 {
		t.Errorf("%d ARM requests, want 6", len(arm.calls))
	}
	for _, c := range arm.calls {
		if !strings.HasPrefix(c.path, clusterID+"/providers/Microsoft.Insights/") {
			t.Errorf("path = %s", c.path)
		}
	}

	requests := []struct{ names, filter string }{
		{"node_cpu_usage_percentage,node_memory_working_set_percentage", "nodepool eq '*'"},
		{"kube_node_status_condition", "condition eq 'Ready' and status2 eq '*'"},
		{"kube_node_status_condition", "condition eq '*' and status eq 'true'"},
		{"kube_pod_status_phase", "phase eq '*'"},
		{"apiserver_cpu_usage_percentage,apiserver_memory_usage_percentage,etcd_database_usage_percentage," +
			"kube_node_status_allocatable_cpu_cores,kube_node_status_allocatable_memory_bytes," +
			"cluster_autoscaler_unneeded_nodes_count,cluster_autoscaler_unschedulable_pods_count," +
			"cluster_autoscaler_cluster_safe_to_autoscale,cluster_autoscaler_scale_down_in_cooldown", ""},
	}
	for _, r := range requests {
		c := arm.metricsCall(t, r.names, r.filter)
		if r.filter == "" {
			if _, ok := c.query["$filter"]; ok {
				t.Errorf("%s: has a $filter", r.names)
			}
		}
	}

	if len(graph.queries) != 2 {
		t.Fatalf("%d graph queries, want 2", len(graph.queries))
	}
	for i, s := range graph.scopes {
		if len(s) != 1 || s[0] != "s1" {
			t.Errorf("graph query %d scope = %v, want [s1]", i, s)
		}
	}
}

func TestAKSMonitoringPeriods(t *testing.T) {
	end := time.Date(2026, 10, 16, 8, 17, 42, 0, time.UTC)
	tests := []struct {
		period   string
		start    time.Time
		interval time.Duration
		grain    string
	}{
		{"24h", time.Date(2026, 10, 15, 8, 15, 0, 0, time.UTC), 5 * time.Minute, "PT5M"},
		{"7d", time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC), 30 * time.Minute, "PT30M"},
		{"30d", time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC), time.Hour, "PT1H"},
		{"", time.Date(2026, 10, 15, 8, 15, 0, 0, time.UTC), 5 * time.Minute, "PT5M"},
		{"1y", time.Date(2026, 10, 15, 8, 15, 0, 0, time.UTC), 5 * time.Minute, "PT5M"},
	}
	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			arm := &safeARM{}

			m := AKSMonitoring(context.Background(), &safeGraph{}, arm, "s1", clusterID, tt.period, monitoringNow)

			if !m.End.Equal(end) || m.End.Location() != time.UTC {
				t.Errorf("End = %v, want %v in UTC", m.End, end)
			}
			if !m.Start.Equal(tt.start) {
				t.Errorf("Start = %v, want %v", m.Start, tt.start)
			}
			if m.Interval != tt.interval {
				t.Errorf("Interval = %v, want %v", m.Interval, tt.interval)
			}
			wantSpan := tt.start.UTC().Format(time.RFC3339) + "/" + end.Format(time.RFC3339)
			for _, c := range arm.calls {
				if strings.HasSuffix(c.path, "/diagnosticSettings") {
					continue
				}
				if got := c.query.Get("timespan"); got != wantSpan {
					t.Errorf("timespan = %s, want %s", got, wantSpan)
				}
				if got := c.query.Get("interval"); got != tt.grain {
					t.Errorf("interval = %s, want %s", got, tt.grain)
				}
			}
		})
	}
}

func TestAKSMonitoringKeepsEachError(t *testing.T) {
	boom := errors.New("boom")
	arm := &safeARM{fail: map[string]error{"kube_pod_status_phase|phase eq '*'": boom, "diagnostics": boom}}
	graph := &safeGraph{}

	m := AKSMonitoring(context.Background(), graph, arm, "s1", clusterID, "24h", monitoringNow)

	if !errors.Is(m.Pods.Err, boom) || !errors.Is(m.Diagnostics.Err, boom) {
		t.Errorf("pods err = %v, diagnostics err = %v, want boom", m.Pods.Err, m.Diagnostics.Err)
	}
	if m.Pools.Err != nil || len(m.Pools.Data) != 1 || m.Ready.Err != nil || m.Conditions.Err != nil || m.Cluster.Err != nil {
		t.Errorf("other sections failed too: %+v", m)
	}
	if m.Rules.Err != nil || m.Alerts.Err != nil || len(m.Rules.Data) != 1 || len(m.Alerts.Data) != 1 {
		t.Errorf("graph sections failed too: rules %+v, alerts %+v", m.Rules, m.Alerts)
	}
}

func TestAKSMonitoringGraphErrorLeavesMetrics(t *testing.T) {
	boom := errors.New("boom")

	m := AKSMonitoring(context.Background(), &safeGraph{err: boom}, &safeARM{}, "s1", clusterID, "24h", monitoringNow)

	if !errors.Is(m.Rules.Err, boom) || !errors.Is(m.Alerts.Err, boom) {
		t.Errorf("rules err = %v, alerts err = %v, want boom", m.Rules.Err, m.Alerts.Err)
	}
	if m.Pools.Err != nil || len(m.Pools.Data) != 1 {
		t.Errorf("pools = %+v", m.Pools)
	}
}

type barrier struct {
	mu      sync.Mutex
	n       int
	allHere chan struct{}
}

func (b *barrier) wait(ctx context.Context) error {
	b.mu.Lock()
	b.n++
	if b.n == 8 { // AKSMonitoring makes eight reads
		close(b.allHere)
	}
	b.mu.Unlock()
	select {
	case <-b.allHere:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type barrierARM struct {
	*barrier
	safeARM
}

func (a *barrierARM) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	if err := a.wait(ctx); err != nil {
		return nil, err
	}
	return a.safeARM.Get(ctx, path, query)
}

type barrierGraph struct {
	*barrier
	safeGraph
}

func (g *barrierGraph) Resources(ctx context.Context, q armresourcegraph.QueryRequest, o *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	if err := g.wait(ctx); err != nil {
		return armresourcegraph.ClientResourcesResponse{}, err
	}
	return g.safeGraph.Resources(ctx, q, o)
}

func TestAKSMonitoringReadsAtOnce(t *testing.T) {
	b := &barrier{allHere: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	m := AKSMonitoring(ctx, &barrierGraph{barrier: b}, &barrierARM{barrier: b}, "s1", clusterID, "24h", monitoringNow)

	for name, err := range map[string]error{
		"pools": m.Pools.Err, "ready": m.Ready.Err, "conditions": m.Conditions.Err, "pods": m.Pods.Err,
		"cluster": m.Cluster.Err, "diagnostics": m.Diagnostics.Err, "rules": m.Rules.Err, "alerts": m.Alerts.Err,
	} {
		if err != nil {
			t.Errorf("%s: %v, want the eight reads to run at the same time", name, err)
		}
	}
}

package azure

import (
	"context"
	"sync"
	"time"
)

// Periods are the spans the monitoring tab can show, in the order p steps through them.
var Periods = []string{"24h", "7d", "30d"}

type periodSpec struct {
	span, interval time.Duration
	grain          string // the interval as ISO 8601
}

var periodSpecs = map[string]periodSpec{
	"24h": {24 * time.Hour, 5 * time.Minute, "PT5M"},
	"7d":  {7 * 24 * time.Hour, 30 * time.Minute, "PT30M"},
	"30d": {30 * 24 * time.Hour, time.Hour, "PT1H"},
}

// Part is one section's data or the error that stopped it.
type Part[T any] struct {
	Data T
	Err  error
}

// Monitoring is what the AKS monitoring tab shows for one period.
type Monitoring struct {
	Start, End  time.Time
	Interval    time.Duration
	Pools       Part[[]Metric] // node CPU and memory working set percentage by nodepool
	Ready       Part[[]Metric] // nodes by Ready status
	Conditions  Part[[]Metric] // node conditions that are true
	Pods        Part[[]Metric] // pods by phase
	Cluster     Part[[]Metric] // control plane, allocatable and autoscaler metrics
	Diagnostics Part[[]DiagnosticSetting]
	Rules       Part[[]AlertRule]
	Alerts      Part[[]Alert]
}

// AKSMonitoring reads the monitoring data of a cluster for a period ending at now.
func AKSMonitoring(ctx context.Context, graph ResourceGraph, arm ARM, subscriptionID, clusterID, period string, now time.Time) Monitoring {
	spec, ok := periodSpecs[period]
	if !ok {
		spec = periodSpecs["24h"]
	}
	end := now.UTC()
	m := Monitoring{
		// The API starts its buckets at the start of the timespan.
		Start:    end.Add(-spec.span).Truncate(spec.interval),
		End:      end,
		Interval: spec.interval,
	}

	metrics := func(filter string, names ...string) func() ([]Metric, error) {
		return func() ([]Metric, error) {
			return Metrics(ctx, arm, clusterID, MetricsQuery{
				Names: names, Filter: filter, Start: m.Start, End: m.End, Interval: spec.grain,
			})
		}
	}

	var wg sync.WaitGroup
	fill(&wg, &m.Pools, metrics("nodepool eq '*'", "node_cpu_usage_percentage", "node_memory_working_set_percentage"))
	fill(&wg, &m.Ready, metrics("condition eq 'Ready' and status2 eq '*'", "kube_node_status_condition"))
	fill(&wg, &m.Conditions, metrics("condition eq '*' and status eq 'true'", "kube_node_status_condition"))
	fill(&wg, &m.Pods, metrics("phase eq '*'", "kube_pod_status_phase"))
	fill(&wg, &m.Cluster, metrics("",
		"apiserver_cpu_usage_percentage",
		"apiserver_memory_usage_percentage",
		"etcd_database_usage_percentage",
		"kube_node_status_allocatable_cpu_cores",
		"kube_node_status_allocatable_memory_bytes",
		"cluster_autoscaler_unneeded_nodes_count",
		"cluster_autoscaler_unschedulable_pods_count",
		"cluster_autoscaler_cluster_safe_to_autoscale",
		"cluster_autoscaler_scale_down_in_cooldown",
	))
	fill(&wg, &m.Diagnostics, func() ([]DiagnosticSetting, error) {
		return DiagnosticSettings(ctx, arm, clusterID)
	})
	fill(&wg, &m.Rules, func() ([]AlertRule, error) {
		return AlertRules(ctx, graph, subscriptionID, clusterID)
	})
	fill(&wg, &m.Alerts, func() ([]Alert, error) {
		return Alerts(ctx, graph, subscriptionID, clusterID)
	})
	wg.Wait()
	return m
}

// fill runs read in its own goroutine and stores the result in p.
func fill[T any](wg *sync.WaitGroup, p *Part[T], read func() (T, error)) {
	wg.Go(func() { p.Data, p.Err = read() })
}

package views

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

const fgRed = "31"

var nan = math.NaN()

func monDims(kv ...string) map[string]string {
	dims := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		dims[kv[i]] = kv[i+1]
	}
	return dims
}

// monSeries is a series of points given as average and maximum pairs.
func monSeries(dims map[string]string, avgMax ...float64) azure.Series {
	s := azure.Series{Dims: dims}
	for i := 0; i+1 < len(avgMax); i += 2 {
		s.Points = append(s.Points, azure.Point{Average: avgMax[i], Maximum: avgMax[i+1]})
	}
	return s
}

func monMetric(name string, series ...azure.Series) azure.Metric {
	return azure.Metric{Name: name, Series: series}
}

func monOK[T any](v T) azure.Part[T] { return azure.Part[T]{Data: v} }

func monFailed[T any](err error) azure.Part[T] { return azure.Part[T]{Err: err} }

var monStarted = time.Date(2026, 10, 3, 14, 5, 0, 0, time.UTC)

func monitoringFixture() azure.Monitoring {
	cond := func(name string, avgMax ...float64) azure.Series {
		return monSeries(monDims("condition", name, "status", "true"), avgMax...)
	}
	ready := func(status string, avgMax ...float64) azure.Series {
		return monSeries(monDims("condition", "Ready", "status2", status), avgMax...)
	}
	phase := func(name string, avgMax ...float64) azure.Series {
		return monSeries(monDims("phase", name), avgMax...)
	}
	return azure.Monitoring{
		Pools: monOK([]azure.Metric{
			monMetric("node_cpu_usage_percentage",
				monSeries(monDims("nodepool", "system"), 20, 30, nan, nan, 26, 55),
				monSeries(monDims("nodepool", "burst"), 10.4, 12)),
			monMetric("node_memory_working_set_percentage",
				monSeries(monDims("nodepool", "system"), 40, 50, 60, 70)),
		}),
		Ready: monOK([]azure.Metric{monMetric("kube_node_status_condition",
			ready("Ready", 2, 3, 3.4, 4, nan, nan),
			ready("NotReady", 0, 0, 1, 1),
			ready("Unknown", 0, 0))}),
		Conditions: monOK([]azure.Metric{monMetric("kube_node_status_condition",
			cond("Ready", 3, 3),
			cond("MemoryPressure", 0, 0, 1.2, 2),
			cond("DiskPressure", 2, 2),
			cond("PIDPressure", 0.2, 1))}),
		Pods: monOK([]azure.Metric{monMetric("kube_pod_status_phase",
			phase("Running", 11, 12, 12, 14, nan, nan),
			phase("Pending", 1, 4, 2.6, 3),
			phase("Failed", 0, 0),
			phase("Succeeded", 5, 5))}),
		Cluster: monOK([]azure.Metric{
			monMetric("apiserver_cpu_usage_percentage", monSeries(nil, 2, 5, 4, 9)),
			monMetric("apiserver_memory_usage_percentage", monSeries(nil, 10, 20, nan, nan)),
			monMetric("etcd_database_usage_percentage", monSeries(nil, 1.4, 2, 2.6, 3)),
			monMetric("kube_node_status_allocatable_cpu_cores", monSeries(nil, 8, 8)),
			monMetric("kube_node_status_allocatable_memory_bytes", monSeries(nil, 28.4*gib, 28.4*gib)),
			monMetric("cluster_autoscaler_unneeded_nodes_count", monSeries(nil, 0, 1, 2, 3, nan, nan)),
			monMetric("cluster_autoscaler_unschedulable_pods_count", monSeries(nil, 0, 0)),
			monMetric("cluster_autoscaler_cluster_safe_to_autoscale", monSeries(nil, 1, 1)),
			monMetric("cluster_autoscaler_scale_down_in_cooldown", monSeries(nil, 1, 1, 0, 0, nan, nan)),
		}),
		Diagnostics: monOK([]azure.DiagnosticSetting{
			{Name: "to-law", Logs: []string{"kube-audit", "kube-apiserver"}, Metrics: true, Destinations: []string{"Log Analytics"}},
			{Name: "archive", Destinations: []string{"Storage"}},
		}),
		Rules: monOK([]azure.AlertRule{
			{Name: "a", Kind: "metric", Enabled: true},
			{Name: "b", Kind: "log", Enabled: false},
			{Name: "c", Kind: "metric", Enabled: true},
		}),
		Alerts: monOK([]azure.Alert{
			{Rule: "cpu-high", Severity: "Sev1", Condition: "Fired", Started: monStarted},
			{Rule: "disk-full", Severity: "Sev3", Condition: "Fired", Started: monStarted.Add(-time.Hour)},
			{Rule: "old", Severity: "Sev2", Condition: "Resolved", Started: monStarted.Add(-48 * time.Hour)},
			{Rule: "older", Severity: "Sev2", Condition: "Resolved", Started: monStarted.Add(-72 * time.Hour)},
		}),
	}
}

// monRow is a cluster row from JSON properties.
func monRow(t *testing.T, props string) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal([]byte(props), &p); err != nil {
		t.Fatal(err)
	}
	return map[string]any{"id": "/r/x", "subscriptionId": "s1", "properties": p}
}

// monLines draws the Monitoring tab at width 100 and returns its plain, unboxed lines.
func monLines(row map[string]any, m azure.Monitoring) []string {
	return unbox(plainLines(strings.Join(aksMonitoring(row, tabData{data: m, period: "24h"}, 100), "\n")))
}

// monSection returns the lines of the box with the title, without the title.
func monSection(t *testing.T, lines []string, title string) []string {
	t.Helper()
	for i, l := range lines {
		if l != title {
			continue
		}
		end := i + 1
		for lines[end] != "" {
			end++
		}
		return lines[i+1 : end]
	}
	t.Fatalf("no %q box in:\n%s", title, strings.Join(lines, "\n"))
	return nil
}

func monRaw(row map[string]any, m azure.Monitoring) []string {
	return aksMonitoring(row, tabData{data: m, period: "24h"}, 100)
}

func rawLine(t *testing.T, lines []string, sub string) string {
	t.Helper()
	for _, l := range lines {
		if strings.Contains(plain(l), sub) {
			return l
		}
	}
	t.Fatalf("no line with %q:\n%s", sub, plain(strings.Join(lines, "\n")))
	return ""
}

// rawLineIn is rawLine in the box with the title.
func rawLineIn(t *testing.T, lines []string, title, sub string) string {
	t.Helper()
	for i, l := range lines {
		if strings.HasPrefix(plain(l), "╭ "+title) {
			return rawLine(t, lines[i:], sub)
		}
	}
	t.Fatalf("no %q box", title)
	return ""
}

var aksRow = func() map[string]any {
	var row map[string]any
	raw := `{"id":"/r/x","subscriptionId":"s1","properties":{
		"agentPoolProfiles":[{"name":"system","enableAutoScaling":true},{"name":"work"}],
		"azureMonitorProfile":{"metrics":{"enabled":true}},
		"addonProfiles":{"omsagent":{"enabled":true,"config":{"logAnalyticsWorkspaceResourceID":"/subscriptions/s1/resourceGroups/rg/providers/Microsoft.OperationalInsights/workspaces/law-main"}}}}}`
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		panic(err)
	}
	return row
}()

func TestMonitoringTabKey(t *testing.T) {
	v := openedAt(t, "aks", "", 80, 10)
	_, cmd := press(v, key("]"))
	if cmd != nil {
		t.Fatalf("Node pools asked for %+v", fetches(cmd))
	}
	row := fixture(t, "aks")
	got := aksMonitoringKey(row, "7d")
	want := data.Key{Kind: data.AKSMonitoring, Subscription: "s1", ID: str(row, "id"), Period: "7d"}
	if got != want || want.ID == "" {
		t.Errorf("key = %+v, want %+v", got, want)
	}
}

func TestMonitoringFetchesOnOpening(t *testing.T) {
	v := NewDetailsAt("s1", azure.Resource{ID: "/r/x", Name: "x"}, "Monitoring").Resize(100, 60)
	row := fixture(t, "aks")
	_, cmd := v.Update(data.ResultMsg{Key: fetchKey(t, v), Data: row})
	wantFetches(t, cmd, data.FetchMsg{Key: data.Key{Kind: data.AKSMonitoring, Subscription: "s1", ID: str(row, "id"), Period: "24h"}})
}

func TestMonitoringLoading(t *testing.T) {
	got := aksMonitoring(aksRow, tabData{loading: true, period: "24h"}, 100)
	if len(got) != 1 || plain(got[0]) != "Loading monitoring…" {
		t.Errorf("lines = %q", got)
	}
}

func TestMonitoringFailedFetch(t *testing.T) {
	got := aksMonitoring(aksRow, tabData{err: errors.New("boom"), period: "24h"}, 100)
	if len(got) != 1 || plain(got[0]) != "failed: boom" {
		t.Fatalf("lines = %q", got)
	}
	wantFg(t, got[0], "failed", fgRed)
}

func TestMonitoringPeriodLine(t *testing.T) {
	for _, c := range []struct{ period, left, right string }{
		{"24h", "Last 24 hours", "p: 7 days"},
		{"7d", "Last 7 days", "p: 30 days"},
		{"30d", "Last 30 days", "p: 24 hours"},
	} {
		t.Run(c.period, func(t *testing.T) {
			lines := aksMonitoring(aksRow, tabData{data: monitoringFixture(), period: c.period}, 60)
			want := c.left + strings.Repeat(" ", 60-len(c.left)-len(c.right)) + c.right
			if got := plain(lines[0]); got != want {
				t.Errorf("line = %q, want %q", got, want)
			}
			if !strings.Contains(lines[0], faintStyle.Render(c.right)) {
				t.Errorf("the hint is not faint: %q", lines[0])
			}
			if lines[1] != "" || !strings.HasPrefix(plain(lines[2]), "╭ Setup") {
				t.Errorf("after the period line: %q, %q", lines[1], lines[2])
			}
		})
	}
}

func TestMonitoringSectionOrder(t *testing.T) {
	var titles []string
	for _, l := range monLines(aksRow, monitoringFixture()) {
		switch l {
		case "Setup", "Node pools", "Nodes", "Pods", "Control plane", "Autoscaler", "Alerts":
			titles = append(titles, l)
		}
	}
	want := "Setup,Node pools,Nodes,Pods,Control plane,Autoscaler,Alerts"
	if got := strings.Join(titles, ","); got != want {
		t.Errorf("boxes = %s, want %s", got, want)
	}
}

func TestMonitoringSetup(t *testing.T) {
	wantLines(t, monSection(t, monLines(aksRow, monitoringFixture()), "Setup"), []string{
		"Container insights   on · law-main",
		"Managed Prometheus   on",
		"Diagnostic settings  to-law: kube-audit, kube-apiserver, metrics → Log Analytics",
		"                     archive: no logs → Storage",
	})
}

func TestMonitoringSetupOffAndNone(t *testing.T) {
	m := monitoringFixture()
	m.Diagnostics = monOK([]azure.DiagnosticSetting{})
	for _, row := range []map[string]any{
		monRow(t, `{}`),
		monRow(t, `{"addonProfiles":{"omsagent":{"enabled":false}},"azureMonitorProfile":{"metrics":{"enabled":false}}}`),
	} {
		wantLines(t, monSection(t, monLines(row, m), "Setup"), []string{
			"Container insights   off",
			"Managed Prometheus   off",
			"Diagnostic settings  none",
		})
	}
}

func TestMonitoringSetupAddonKeyInAnyCase(t *testing.T) {
	row := monRow(t, `{"addonProfiles":{"OmsAgent":{"enabled":true,"config":{"logAnalyticsWorkspaceResourceID":"/w/Law-X"}}}}`)
	got := monSection(t, monLines(row, monitoringFixture()), "Setup")[0]
	if got != "Container insights   on · Law-X" {
		t.Errorf("line = %q", got)
	}
}

func TestMonitoringSetupOnWithoutWorkspace(t *testing.T) {
	row := monRow(t, `{"addonProfiles":{"omsagent":{"enabled":true}}}`)
	if got := monSection(t, monLines(row, monitoringFixture()), "Setup")[0]; got != "Container insights   on" {
		t.Errorf("line = %q", got)
	}
}

func TestMonitoringSetupColors(t *testing.T) {
	lines := monRaw(aksRow, monitoringFixture())
	wantFg(t, rawLine(t, lines, "Container insights"), "on · law-main", fgGood)
	wantFg(t, rawLine(t, lines, "Managed Prometheus"), "on", fgGood)
	off := monRaw(monRow(t, `{}`), monitoringFixture())
	wantFg(t, rawLine(t, off, "Container insights"), "off", fgMuted)
	wantFg(t, rawLine(t, off, "Managed Prometheus"), "off", fgMuted)
}

func TestMonitoringDiagnosticLines(t *testing.T) {
	for _, c := range []struct {
		name string
		s    azure.DiagnosticSetting
		want string
	}{
		{"logs and metrics", azure.DiagnosticSetting{Name: "a", Logs: []string{"x", "y"}, Metrics: true, Destinations: []string{"Storage", "Event Hubs"}}, "a: x, y, metrics → Storage, Event Hubs"},
		{"logs only", azure.DiagnosticSetting{Name: "a", Logs: []string{"x"}, Destinations: []string{"Log Analytics"}}, "a: x → Log Analytics"},
		{"metrics only", azure.DiagnosticSetting{Name: "a", Metrics: true, Destinations: []string{"Log Analytics"}}, "a: metrics → Log Analytics"},
		{"nothing", azure.DiagnosticSetting{Name: "a", Destinations: []string{"Log Analytics"}}, "a: no logs → Log Analytics"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := monitoringFixture()
			m.Diagnostics = monOK([]azure.DiagnosticSetting{c.s})
			got := monSection(t, monLines(aksRow, m), "Setup")[2]
			if want := "Diagnostic settings  " + c.want; got != want {
				t.Errorf("line = %q, want %q", got, want)
			}
		})
	}
}

func TestMonitoringNodePools(t *testing.T) {
	row := func(a, b, c, d, e string) string { return fmt.Sprintf("%-12s  %-7s  %-7s  %-10s  %s", a, b, c, d, e) }
	wantLines(t, monSection(t, monLines(aksRow, monitoringFixture()), "Node pools"), []string{
		row("Pool", "CPU avg", "CPU max", "Memory avg", "Memory max"),
		row("burst (auto)", "10%", "12%", "—", "—"),
		row("system", "23%", "55%", "50%", "70%"),
	})
}

func TestMonitoringNodePoolNameMatchesInAnyCase(t *testing.T) {
	m := monitoringFixture()
	m.Pools = monOK([]azure.Metric{monMetric("node_cpu_usage_percentage", monSeries(monDims("nodepool", "SYSTEM"), 1, 2))})
	got := strings.Join(monSection(t, monLines(aksRow, m), "Node pools"), "\n")
	if strings.Contains(got, "auto") || !strings.Contains(got, "SYSTEM") {
		t.Errorf("lines:\n%s", got)
	}
}

func TestMonitoringNodePoolsNoSeries(t *testing.T) {
	m := monitoringFixture()
	m.Pools = monOK([]azure.Metric{})
	wantLines(t, monSection(t, monLines(aksRow, m), "Node pools"), []string{"none"})
}

func TestMonitoringNodePoolsNoData(t *testing.T) {
	m := monitoringFixture()
	m.Pools = monOK([]azure.Metric{monMetric("node_cpu_usage_percentage", monSeries(monDims("nodepool", "system"), nan, nan))})
	wantLines(t, monSection(t, monLines(aksRow, m), "Node pools"), []string{"no data"})
}

func TestMonitoringNodes(t *testing.T) {
	wantLines(t, monSection(t, monLines(aksRow, monitoringFixture()), "Nodes"), []string{
		"Ready        3",
		"Not ready    1",
		"Unknown      0",
		"Problems     DiskPressure on 2 nodes",
		"             MemoryPressure on 1 node",
		"Allocatable  8 cores, 28.4 GiB",
	})
}

func TestMonitoringProblemsOneLineEach(t *testing.T) {
	condition := func(name string, n float64) azure.Series {
		return monSeries(monDims("condition", name, "status", "true"), n, n)
	}
	m := monitoringFixture()
	m.Conditions = monOK([]azure.Metric{monMetric("kube_node_status_condition",
		condition("Ready", 4), condition("MemoryPressure", 1), condition("DiskPressure", 2),
		condition("PIDPressure", 1), condition("KernelDeadlock", 1))})
	got := monSection(t, monLinesAt(aksRow, m, 80), "Nodes")
	wantLines(t, got[3:7], []string{
		"Problems     DiskPressure on 2 nodes",
		"             KernelDeadlock on 1 node",
		"             MemoryPressure on 1 node",
		"             PIDPressure on 1 node",
	})
}

func TestMonitoringNodesColors(t *testing.T) {
	lines := monRaw(aksRow, monitoringFixture())
	wantFg(t, rawLine(t, lines, "Ready  "), "3", "")
	wantFg(t, rawLine(t, lines, "Not ready"), "1", fgWarn)
	wantFg(t, rawLine(t, lines, "Unknown      0"), "0", "")
	wantFg(t, rawLine(t, lines, "Problems"), "DiskPressure on 2 nodes", fgWarn)
	wantFg(t, rawLine(t, lines, "MemoryPressure"), "MemoryPressure on 1 node", fgWarn)

	m := monitoringFixture()
	m.Ready = monOK([]azure.Metric{monMetric("kube_node_status_condition",
		monSeries(monDims("condition", "Ready", "status2", "NotReady"), 0, 0),
		monSeries(monDims("condition", "Ready", "status2", "Unknown"), 2, 2))})
	m.Conditions = monOK([]azure.Metric{monMetric("kube_node_status_condition", monSeries(monDims("condition", "MemoryPressure", "status", "true"), 0.4, 1))})
	lines = monRaw(aksRow, m)
	wantFg(t, rawLine(t, lines, "Not ready"), "0", "")
	wantFg(t, rawLine(t, lines, "Unknown      2"), "2", fgWarn)
	wantFg(t, rawLine(t, lines, "Problems"), "none", "")
}

func TestMonitoringNodesWithoutData(t *testing.T) {
	m := monitoringFixture()
	m.Ready = monOK([]azure.Metric{monMetric("kube_node_status_condition", monSeries(monDims("condition", "Ready", "status2", "Ready"), 4, 4))})
	m.Conditions = monOK([]azure.Metric{})
	m.Cluster = monOK([]azure.Metric{monMetric("kube_node_status_allocatable_cpu_cores", monSeries(nil, nan, nan))})
	wantLines(t, monSection(t, monLines(aksRow, m), "Nodes"), []string{
		"Ready        4",
		"Not ready    —",
		"Unknown      —",
		"Problems     none",
		"Allocatable  —, —",
	})
}

func TestMonitoringNodesAllocatableOneMissing(t *testing.T) {
	m := monitoringFixture()
	m.Cluster = monOK([]azure.Metric{monMetric("kube_node_status_allocatable_memory_bytes", monSeries(nil, 2*gib, 2*gib))})
	if got := lastLine(monSection(t, monLines(aksRow, m), "Nodes")); got != "Allocatable  —, 2.0 GiB" {
		t.Errorf("line = %q", got)
	}
	m.Cluster = monOK([]azure.Metric{monMetric("kube_node_status_allocatable_cpu_cores", monSeries(nil, 1.5, 1.5, nan, nan))})
	if got := lastLine(monSection(t, monLines(aksRow, m), "Nodes")); got != "Allocatable  1.5 cores, —" {
		t.Errorf("line = %q", got)
	}
}

func lastLine(lines []string) string { return lines[len(lines)-1] }

func TestMonitoringNodesNoDataAtAll(t *testing.T) {
	m := monitoringFixture()
	m.Ready, m.Conditions = monOK([]azure.Metric{}), monOK([]azure.Metric{})
	m.Cluster = monOK([]azure.Metric{})
	wantLines(t, monSection(t, monLines(aksRow, m), "Nodes"), []string{"no data"})
}

func TestMonitoringPods(t *testing.T) {
	wantLines(t, monSection(t, monLines(aksRow, monitoringFixture()), "Pods"), []string{
		"Running    12  max 14",
		"Pending     3  max 4",
		"Failed      0  max 0",
		"Unknown     —",
		"Succeeded   5  max 5",
	})
}

func TestMonitoringPodsColors(t *testing.T) {
	m := monitoringFixture()
	m.Pods = monOK([]azure.Metric{monMetric("kube_pod_status_phase",
		monSeries(monDims("phase", "Running"), 2, 2),
		monSeries(monDims("phase", "Pending"), 1, 1),
		monSeries(monDims("phase", "Failed"), 0, 0),
		monSeries(monDims("phase", "Unknown"), 2, 2),
		monSeries(monDims("phase", "Succeeded"), 9, 9))})
	lines := monRaw(aksRow, m)
	wantFg(t, rawLineIn(t, lines, "Pods", "Running"), "2", "")
	wantFg(t, rawLineIn(t, lines, "Pods", "Pending"), "1", fgWarn)
	wantFg(t, rawLineIn(t, lines, "Pods", "Pending"), "max 1", "")
	wantFg(t, rawLineIn(t, lines, "Pods", "Failed"), "0", "")
	wantFg(t, rawLineIn(t, lines, "Pods", "Unknown"), "2", fgWarn)
	wantFg(t, rawLineIn(t, lines, "Pods", "Succeeded"), "9", "")
}

func TestMonitoringPodsNoData(t *testing.T) {
	m := monitoringFixture()
	m.Pods = monOK([]azure.Metric{monMetric("kube_pod_status_phase", monSeries(monDims("phase", "Running"), nan, nan))})
	wantLines(t, monSection(t, monLines(aksRow, m), "Pods"), []string{"no data"})
}

func TestMonitoringControlPlane(t *testing.T) {
	wantLines(t, monSection(t, monLines(aksRow, monitoringFixture()), "Control plane"), []string{
		"API server CPU     avg 3%   max 9%",
		"API server memory  avg 10%  max 20%",
		"etcd database      3%",
	})
}

func TestMonitoringControlPlaneMissingAndNoData(t *testing.T) {
	m := monitoringFixture()
	m.Cluster = monOK([]azure.Metric{monMetric("etcd_database_usage_percentage", monSeries(nil, 7, 7))})
	wantLines(t, monSection(t, monLines(aksRow, m), "Control plane"), []string{
		"API server CPU     —",
		"API server memory  —",
		"etcd database      7%",
	})
	m.Cluster = monOK([]azure.Metric{monMetric("etcd_database_usage_percentage", monSeries(nil, nan, nan))})
	wantLines(t, monSection(t, monLines(aksRow, m), "Control plane"), []string{"no data"})
}

func TestMonitoringAutoscaler(t *testing.T) {
	wantLines(t, monSection(t, monLines(aksRow, monitoringFixture()), "Autoscaler"), []string{
		"Unneeded nodes       2  max 3",
		"Unschedulable pods   0  max 0",
		"Safe to scale        yes",
		"Scale-down cooldown  no",
	})
}

func TestMonitoringAutoscalerNoYes(t *testing.T) {
	m := monitoringFixture()
	m.Cluster = monOK([]azure.Metric{
		monMetric("cluster_autoscaler_cluster_safe_to_autoscale", monSeries(nil, 0, 0)),
		monMetric("cluster_autoscaler_scale_down_in_cooldown", monSeries(nil, 1, 1)),
	})
	got := monSection(t, monLines(aksRow, m), "Autoscaler")
	if got[2] != "Safe to scale        no" || got[3] != "Scale-down cooldown  yes" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
}

func TestMonitoringAutoscalerWithoutData(t *testing.T) {
	m := monitoringFixture()
	m.Cluster = monOK([]azure.Metric{
		monMetric("cluster_autoscaler_unneeded_nodes_count", monSeries(nil, nan, nan)),
		monMetric("cluster_autoscaler_cluster_safe_to_autoscale", monSeries(nil, nan, nan)),
	})
	wantLines(t, monSection(t, monLines(aksRow, m), "Autoscaler"), []string{
		"Unneeded nodes       —",
		"Unschedulable pods   —",
		"Safe to scale        —",
		"Scale-down cooldown  —",
	})
}

func TestMonitoringNoAutoscalerBoxWithoutAutoscaling(t *testing.T) {
	for _, props := range []string{
		`{}`,
		`{"agentPoolProfiles":[{"name":"a"},{"name":"b","enableAutoScaling":false}]}`,
	} {
		for _, l := range monLines(monRow(t, props), monitoringFixture()) {
			if l == "Autoscaler" {
				t.Errorf("%s: has an Autoscaler box", props)
			}
		}
	}
	row := monRow(t, `{"agentPoolProfiles":[{"name":"a"},{"name":"b","enableAutoScaling":true}]}`)
	monSection(t, monLines(row, monitoringFixture()), "Autoscaler")
}

func TestMonitoringAlerts(t *testing.T) {
	saved := time.Local
	time.Local = time.FixedZone("T", 3*3600)
	t.Cleanup(func() { time.Local = saved })
	wantLines(t, monSection(t, monLines(aksRow, monitoringFixture()), "Alerts"), []string{
		"Rules     3: 2 metric, 1 log; 1 disabled",
		"Firing    Sev1  since Oct 3 17:05  cpu-high",
		"          Sev3  since Oct 3 16:05  disk-full",
		"Resolved  2 in the last 30 days",
	})
}

func TestMonitoringAlertsNone(t *testing.T) {
	m := monitoringFixture()
	m.Rules, m.Alerts = monOK([]azure.AlertRule{}), monOK([]azure.Alert{})
	wantLines(t, monSection(t, monLines(aksRow, m), "Alerts"), []string{
		"Rules   none",
		"Firing  none",
	})
}

func TestMonitoringAlertRuleCounts(t *testing.T) {
	for _, c := range []struct {
		name  string
		rules []azure.AlertRule
		want  string
	}{
		{"one", []azure.AlertRule{{Kind: "Prometheus", Enabled: true}}, "1: 1 Prometheus"},
		{"all kinds in order", []azure.AlertRule{
			{Kind: "Prometheus", Enabled: true}, {Kind: "activity log", Enabled: true},
			{Kind: "log", Enabled: true}, {Kind: "metric", Enabled: true}, {Kind: "metric", Enabled: true},
		}, "5: 2 metric, 1 log, 1 activity log, 1 Prometheus"},
		{"all disabled", []azure.AlertRule{{Kind: "log"}, {Kind: "log"}}, "2: 2 log; 2 disabled"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := monitoringFixture()
			m.Rules = monOK(c.rules)
			if got := monSection(t, monLines(aksRow, m), "Alerts")[0]; got != "Rules     "+c.want {
				t.Errorf("line = %q", got)
			}
		})
	}
}

func TestMonitoringAlertsOnlyResolved(t *testing.T) {
	m := monitoringFixture()
	m.Alerts = monOK([]azure.Alert{{Rule: "old", Severity: "Sev2", Condition: "Resolved", Started: monStarted}})
	got := monSection(t, monLines(aksRow, m), "Alerts")
	if len(got) != 3 || got[1] != "Firing    none" || got[2] != "Resolved  1 in the last 30 days" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
}

func TestMonitoringAlertColors(t *testing.T) {
	m := monitoringFixture()
	m.Alerts = monOK([]azure.Alert{
		{Rule: "r0", Severity: "Sev0", Condition: "Fired", Started: monStarted},
		{Rule: "r1", Severity: "Sev1", Condition: "Fired", Started: monStarted},
		{Rule: "r2", Severity: "Sev2", Condition: "Fired", Started: monStarted},
		{Rule: "r4", Severity: "Sev4", Condition: "Fired", Started: monStarted},
	})
	lines := monRaw(aksRow, m)
	for sev, want := range map[string]string{"Sev0": fgRed, "Sev1": fgRed, "Sev2": fgWarn, "Sev4": fgWarn} {
		wantFg(t, rawLine(t, lines, sev), sev, want)
	}
}

func TestMonitoringPartErrors(t *testing.T) {
	boom := errors.New("boom")
	for _, c := range []struct {
		name  string
		title string
		set   func(*azure.Monitoring)
	}{
		{"pools", "Node pools", func(m *azure.Monitoring) { m.Pools = monFailed[[]azure.Metric](boom) }},
		{"pods", "Pods", func(m *azure.Monitoring) { m.Pods = monFailed[[]azure.Metric](boom) }},
		{"control plane", "Control plane", func(m *azure.Monitoring) { m.Cluster = monFailed[[]azure.Metric](boom) }},
		{"autoscaler", "Autoscaler", func(m *azure.Monitoring) { m.Cluster = monFailed[[]azure.Metric](boom) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := monitoringFixture()
			c.set(&m)
			lines := monRaw(aksRow, m)
			wantLines(t, monSection(t, monLines(aksRow, m), c.title), []string{"failed: boom"})
			wantFg(t, rawLine(t, lines, "failed: boom"), "failed: boom", fgRed)
		})
	}
}

func TestMonitoringPartErrorsInSectionsOfSeveralParts(t *testing.T) {
	boom := errors.New("boom")
	m := monitoringFixture()
	m.Diagnostics = monFailed[[]azure.DiagnosticSetting](boom)
	m.Ready = monFailed[[]azure.Metric](boom)
	m.Conditions = monFailed[[]azure.Metric](boom)
	m.Rules = monFailed[[]azure.AlertRule](boom)
	m.Alerts = monFailed[[]azure.Alert](boom)
	lines := monLines(aksRow, m)
	wantLines(t, monSection(t, lines, "Setup"), []string{
		"Container insights   on · law-main",
		"Managed Prometheus   on",
		"Diagnostic settings  failed: boom",
	})
	wantLines(t, monSection(t, lines, "Nodes"), []string{
		"Ready        failed: boom",
		"Problems     failed: boom",
		"Allocatable  8 cores, 28.4 GiB",
	})
	wantLines(t, monSection(t, lines, "Alerts"), []string{
		"Rules   failed: boom",
		"Firing  failed: boom",
	})
}

func TestMonitoringNodesAllocatableError(t *testing.T) {
	m := monitoringFixture()
	m.Cluster = monFailed[[]azure.Metric](errors.New("boom"))
	got := monSection(t, monLines(aksRow, m), "Nodes")
	if got[len(got)-1] != "Allocatable  failed: boom" || got[0] != "Ready        3" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
}

func TestMonitoringForbidden(t *testing.T) {
	m := monitoringFixture()
	denied := &azcore.ResponseError{StatusCode: 403, ErrorCode: "AuthorizationFailed"}
	m.Pools = monFailed[[]azure.Metric](fmt.Errorf("read: %w", denied))
	wantLines(t, monSection(t, monLines(aksRow, m), "Node pools"),
		[]string{"no access: needs Reader or Monitoring Reader on the cluster"})
	wantFg(t, rawLine(t, monRaw(aksRow, m), "no access"), "no access", fgRed)
}

func TestMonitoringErrorTextHasNoRequestURL(t *testing.T) {
	m := monitoringFixture()
	m.Pods = monFailed[[]azure.Metric](&azcore.ResponseError{StatusCode: 400, ErrorCode: "BadRequest"})
	wantLines(t, monSection(t, monLines(aksRow, m), "Pods"), []string{"failed: 400 BadRequest"})
}

func TestMonitoringTabInDetailsView(t *testing.T) {
	v := openedAt(t, "aks", "Monitoring", 100, 90)
	row := fixture(t, "aks")
	mk := aksMonitoringKey(row, "24h")
	if got := bodyText(v); !strings.Contains(got, "Loading monitoring…") {
		t.Errorf("render:\n%s", got)
	}
	v, _ = v.Update(data.ResultMsg{Key: mk, Data: monitoringFixture()})
	got := bodyText(v)
	for _, want := range []string{"Last 24 hours", "p: 7 days", "╭ Setup", "law-main", "╭ Alerts"} {
		if !strings.Contains(got, want) {
			t.Errorf("render lacks %q:\n%s", want, got)
		}
	}
	v, cmd := press(v, key("p"))
	wantFetches(t, cmd, data.FetchMsg{Key: aksMonitoringKey(row, "7d")})
	if got := bodyText(v); !strings.Contains(got, "Loading monitoring…") {
		t.Errorf("render after p:\n%s", got)
	}
	v, _ = v.Update(data.ResultMsg{Key: aksMonitoringKey(row, "7d"), Data: monitoringFixture()})
	got = bodyText(v)
	for _, want := range []string{"Last 7 days", "p: 30 days"} {
		if !strings.Contains(got, want) {
			t.Errorf("render after the 7d result lacks %q:\n%s", want, got)
		}
	}
}

// monLinesAt is monLines at the width.
func monLinesAt(row map[string]any, m azure.Monitoring, width int) []string {
	return unbox(plainLines(strings.Join(aksMonitoring(row, tabData{data: m, period: "24h"}, width), "\n")))
}

// wantAligned checks that sub starts at the same cell on every line.
func wantAligned(t *testing.T, lines []string, sub string) {
	t.Helper()
	at := -1
	for _, l := range lines {
		i := strings.Index(l, sub)
		if i < 0 {
			continue
		}
		if at >= 0 && i != at {
			t.Errorf("%q starts at cell %d and %d:\n%s", sub, at, i, strings.Join(lines, "\n"))
			return
		}
		at = i
	}
	if at < 0 {
		t.Errorf("no %q in:\n%s", sub, strings.Join(lines, "\n"))
	}
}

func TestMonitoringPodsLineUp(t *testing.T) {
	m := monitoringFixture()
	m.Pods = monOK([]azure.Metric{monMetric("kube_pod_status_phase",
		monSeries(monDims("phase", "Running"), 1038, 1041),
		monSeries(monDims("phase", "Pending"), 0, 3))})
	got := monSection(t, monLines(aksRow, m), "Pods")
	if got[0] != "Running    1038  max 1041" || got[1] != "Pending       0  max 3" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
	wantAligned(t, got, "max")
}

func TestMonitoringPodsWarnColorsOnlyTheValue(t *testing.T) {
	m := monitoringFixture()
	m.Pods = monOK([]azure.Metric{monMetric("kube_pod_status_phase",
		monSeries(monDims("phase", "Running"), 1038, 1041),
		monSeries(monDims("phase", "Pending"), 3, 3))})
	line := rawLineIn(t, monRaw(aksRow, m), "Pods", "Pending")
	if !strings.Contains(line, " "+warnStyle.Render("3")+"  max 3") {
		t.Errorf("line = %q", line)
	}
}

func TestMonitoringControlPlaneLinesUp(t *testing.T) {
	m := monitoringFixture()
	m.Cluster = monOK([]azure.Metric{
		monMetric("apiserver_cpu_usage_percentage", monSeries(nil, 3, 9)),
		monMetric("apiserver_memory_usage_percentage", monSeries(nil, 10, 20)),
	})
	got := monSection(t, monLines(aksRow, m), "Control plane")
	if got[0] != "API server CPU     avg 3%   max 9%" || got[1] != "API server memory  avg 10%  max 20%" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
	wantAligned(t, got[:2], "max")
}

func TestMonitoringAutoscalerLinesUp(t *testing.T) {
	m := monitoringFixture()
	m.Cluster = monOK([]azure.Metric{
		monMetric("cluster_autoscaler_unneeded_nodes_count", monSeries(nil, 12, 14)),
		monMetric("cluster_autoscaler_unschedulable_pods_count", monSeries(nil, 0, 5)),
	})
	got := monSection(t, monLines(aksRow, m), "Autoscaler")
	if got[0] != "Unneeded nodes       12  max 14" || got[1] != "Unschedulable pods    0  max 5" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
	wantAligned(t, got[:2], "max")
}

func TestMonitoringFiringLinesUp(t *testing.T) {
	m := monitoringFixture()
	m.Alerts = monOK([]azure.Alert{
		{Rule: "cpu-high", Severity: "Sev1", Condition: "Fired", Started: monStarted},
		{Rule: "disk-is-nearly-full", Severity: "Sev3", Condition: "Fired", Started: monStarted},
	})
	got := monSection(t, monLines(aksRow, m), "Alerts")
	wantAligned(t, got[1:3], "since")
	wantAligned(t, got[1:3], "Sev")
	if !strings.HasPrefix(got[1], "Firing  Sev1  since ") || !strings.HasSuffix(got[1], "  cpu-high") {
		t.Errorf("line = %q", got[1])
	}
}

func TestMonitoringFiringKeepsTheTimeWithALongRule(t *testing.T) {
	m := monitoringFixture()
	m.Alerts = monOK([]azure.Alert{
		{Rule: "node-cpu-high", Severity: "Sev1", Condition: "Fired", Started: monStarted},
		{Rule: "a-very-long-alert-rule-name-for-the-disk-on-the-cluster-nodes", Severity: "Sev3", Condition: "Fired", Started: monStarted},
	})
	got := monSection(t, monLinesAt(aksRow, m, 80), "Alerts")
	for _, l := range got[1:3] {
		if !strings.Contains(l, "since Oct 3 ") {
			t.Errorf("the start time is cut:\n%s", strings.Join(got, "\n"))
		}
	}
	if !strings.HasSuffix(got[1], "node-cpu-high") {
		t.Errorf("line = %q", got[1])
	}
}

func TestMonitoringDiagnosticLinesWrapBeforeTheDestination(t *testing.T) {
	logs := []string{"kube-apiserver", "kube-audit", "kube-audit-admin", "kube-controller-manager", "kube-scheduler", "cluster-autoscaler", "guard"}
	m := monitoringFixture()
	m.Diagnostics = monOK([]azure.DiagnosticSetting{{Name: "to-law", Logs: logs, Metrics: true, Destinations: []string{"Log Analytics"}}})
	for _, width := range []int{80, 120} {
		got := strings.Join(monSection(t, monLinesAt(aksRow, m, width), "Setup")[2:], "\n")
		if !strings.Contains(got, "→ Log Analytics") {
			t.Errorf("width %d: the destination is cut:\n%s", width, got)
		}
		for _, name := range append(logs, "to-law", "metrics") {
			if !strings.Contains(got, name) {
				t.Errorf("width %d: lacks %q:\n%s", width, name, got)
			}
		}
		if strings.Contains(got, "…") {
			t.Errorf("width %d: a line is cut:\n%s", width, got)
		}
	}
}

func TestMonitoringDiagnosticDestinationOnItsOwnLine(t *testing.T) {
	m := monitoringFixture()
	m.Diagnostics = monOK([]azure.DiagnosticSetting{{Name: "to-law",
		Logs: []string{"kube-apiserver", "kube-audit", "kube-audit-admin", "cloud-controller"}, Destinations: []string{"Log Analytics"}}})
	wantLines(t, monSection(t, monLines(aksRow, m), "Setup")[2:], []string{
		"Diagnostic settings  to-law: kube-apiserver, kube-audit, kube-audit-admin, cloud-controller",
		"                       → Log Analytics",
	})
}

func TestMonitoringRounding(t *testing.T) {
	ready := func(status string, v float64) azure.Series {
		return monSeries(monDims("condition", "Ready", "status2", status), v, v)
	}
	phase := func(name string, v float64) azure.Series { return monSeries(monDims("phase", name), v, v) }
	m := monitoringFixture()
	m.Ready = monOK([]azure.Metric{monMetric("kube_node_status_condition",
		ready("Ready", 3), ready("NotReady", 0.4), ready("Unknown", 0.6))})
	m.Pods = monOK([]azure.Metric{monMetric("kube_pod_status_phase", phase("Running", 3), phase("Pending", 0.4), phase("Failed", 0.6))})
	lines := monRaw(aksRow, m)
	for _, c := range []struct {
		box, label, value string
		warned            bool
	}{
		{"Nodes", "Not ready", "0", false},
		{"Nodes", "Unknown", "1", true},
		{"Pods", "Pending", "0", false},
		{"Pods", "Failed", "1", true},
	} {
		line := rawLineIn(t, lines, c.box, c.label)
		if got := strings.Contains(line, warnStyle.Render(c.value)); got != c.warned {
			t.Errorf("%s %s: warned = %v in %q", c.box, c.label, got, line)
		}
		if !strings.Contains(plain(line), c.label) || !strings.Contains(plain(line), "  "+c.value) {
			t.Errorf("%s %s: line = %q", c.box, c.label, plain(line))
		}
	}
}

func TestMonitoringYesNoRounds(t *testing.T) {
	for _, c := range []struct {
		v    float64
		want string
	}{{0.6, "yes"}, {0.4, "no"}} {
		m := monitoringFixture()
		m.Cluster = monOK([]azure.Metric{monMetric("cluster_autoscaler_cluster_safe_to_autoscale", monSeries(nil, c.v, c.v))})
		if got := monSection(t, monLines(aksRow, m), "Autoscaler")[2]; got != "Safe to scale        "+c.want {
			t.Errorf("%v: line = %q", c.v, got)
		}
	}
}

func TestMonitoringAllocatableCores(t *testing.T) {
	for _, c := range []struct {
		cores float64
		want  string
	}{{3.86, "3.9 cores"}, {1, "1 core"}, {8, "8 cores"}} {
		m := monitoringFixture()
		m.Cluster = monOK([]azure.Metric{
			monMetric("kube_node_status_allocatable_cpu_cores", monSeries(nil, c.cores, c.cores)),
			monMetric("kube_node_status_allocatable_memory_bytes", monSeries(nil, 4*gib, 4*gib)),
		})
		got := monSection(t, monLines(aksRow, m), "Nodes")
		if want := "Allocatable  " + c.want + ", 4.0 GiB"; got[len(got)-1] != want {
			t.Errorf("%v: line = %q, want %q", c.cores, got[len(got)-1], want)
		}
	}
}

func TestMonitoringNodesFailedPartIsNotNoData(t *testing.T) {
	m := monitoringFixture()
	m.Ready = monFailed[[]azure.Metric](errors.New("boom"))
	m.Conditions = monOK([]azure.Metric(nil))
	m.Cluster = monOK([]azure.Metric(nil))
	got := monSection(t, monLines(aksRow, m), "Nodes")
	if got[0] != "Ready        failed: boom" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
}

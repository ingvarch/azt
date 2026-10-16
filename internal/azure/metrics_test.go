package azure

import (
	"context"
	"errors"
	"math"
	"net/url"
	"reflect"
	"testing"
	"time"
)

// fakeARM answers every Get with body, or err, and records the calls.
type fakeARM struct {
	body  string
	err   error
	paths []string
	query []url.Values
}

func (f *fakeARM) Get(_ context.Context, path string, query url.Values) ([]byte, error) {
	f.paths = append(f.paths, path)
	f.query = append(f.query, query)
	return []byte(f.body), f.err
}

const clusterID = "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.ContainerService/managedClusters/aks1"

var metricsPeriod = MetricsQuery{
	Names:    []string{"node_cpu_usage_percentage", "node_memory_working_set_percentage"},
	Filter:   "nodepool eq '*'",
	Start:    time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC),
	End:      time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC),
	Interval: "PT1H",
}

func TestMetricsRequest(t *testing.T) {
	arm := &fakeARM{body: `{"value":[]}`}

	if _, err := Metrics(context.Background(), arm, clusterID, metricsPeriod); err != nil {
		t.Fatal(err)
	}

	if want := []string{clusterID + "/providers/Microsoft.Insights/metrics"}; !reflect.DeepEqual(arm.paths, want) {
		t.Errorf("paths = %v, want %v", arm.paths, want)
	}
	want := url.Values{
		"api-version": {"2024-02-01"},
		"metricnames": {"node_cpu_usage_percentage,node_memory_working_set_percentage"},
		"timespan":    {"2026-10-15T10:00:00Z/2026-10-16T10:00:00Z"},
		"interval":    {"PT1H"},
		"aggregation": {"Average,Maximum"},
		"top":         {"100"},
		"$filter":     {"nodepool eq '*'"},
	}
	if !reflect.DeepEqual(arm.query[0], want) {
		t.Errorf("query = %v, want %v", arm.query[0], want)
	}
}

func TestMetricsTimespanIsUTC(t *testing.T) {
	arm := &fakeARM{body: `{"value":[]}`}
	q := metricsPeriod
	zone := time.FixedZone("CEST", 2*3600)
	q.Start, q.End = q.Start.In(zone), q.End.In(zone)

	if _, err := Metrics(context.Background(), arm, clusterID, q); err != nil {
		t.Fatal(err)
	}
	if got, want := arm.query[0].Get("timespan"), "2026-10-15T10:00:00Z/2026-10-16T10:00:00Z"; got != want {
		t.Errorf("timespan = %s, want %s", got, want)
	}
}

func TestMetricsWithoutFilterSendsNone(t *testing.T) {
	arm := &fakeARM{body: `{"value":[]}`}
	q := metricsPeriod
	q.Filter = ""

	if _, err := Metrics(context.Background(), arm, clusterID, q); err != nil {
		t.Fatal(err)
	}
	if _, ok := arm.query[0]["$filter"]; ok {
		t.Errorf("query has $filter: %v", arm.query[0])
	}
}

const metricsBody = `{"timespan":"2026-10-15T10:00:00Z/2026-10-16T10:00:00Z","interval":"PT1H","value":[
  {"name":{"value":"node_cpu_usage_percentage","localizedValue":"CPU Usage Percentage"},"unit":"Percent",
   "timeseries":[
     {"metadatavalues":[{"name":{"value":"nodepool","localizedValue":"nodepool"},"value":"system"}],
      "data":[{"timeStamp":"2026-10-15T10:00:00Z","average":12.5,"maximum":40.1},
              {"timeStamp":"2026-10-15T11:00:00Z"},
              {"timeStamp":"2026-10-15T12:00:00Z","average":3}]},
     {"metadatavalues":[{"name":{"value":"nodepool","localizedValue":"nodepool"},"value":"user"}],
      "data":[]}]},
  {"name":{"value":"apiserver_cpu_usage_percentage"},"unit":"Count",
   "timeseries":[{"data":[{"timeStamp":"2026-10-15T10:00:00Z","maximum":7}]}]}]}`

func TestMetricsParsesResponse(t *testing.T) {
	got, err := Metrics(context.Background(), &fakeARM{body: metricsBody}, clusterID, metricsPeriod)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Fatalf("%d metrics, want 2", len(got))
	}
	cpu := got[0]
	if cpu.Name != "node_cpu_usage_percentage" || cpu.Unit != "Percent" {
		t.Errorf("metric = %s %s", cpu.Name, cpu.Unit)
	}
	if len(cpu.Series) != 2 {
		t.Fatalf("%d series, want 2", len(cpu.Series))
	}
	if want := map[string]string{"nodepool": "system"}; !reflect.DeepEqual(cpu.Series[0].Dims, want) {
		t.Errorf("dims = %v, want %v", cpu.Series[0].Dims, want)
	}
	if want := map[string]string{"nodepool": "user"}; !reflect.DeepEqual(cpu.Series[1].Dims, want) {
		t.Errorf("dims = %v, want %v", cpu.Series[1].Dims, want)
	}

	points := cpu.Series[0].Points
	if len(points) != 3 {
		t.Fatalf("%d points, want 3", len(points))
	}
	if want := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC); !points[0].Time.Equal(want) {
		t.Errorf("time = %v, want %v", points[0].Time, want)
	}
	if points[0].Average != 12.5 || points[0].Maximum != 40.1 {
		t.Errorf("point 0 = %+v", points[0])
	}
	if !math.IsNaN(points[1].Average) || !math.IsNaN(points[1].Maximum) {
		t.Errorf("point 1 = %+v, want NaN values", points[1])
	}
	if points[2].Average != 3 || !math.IsNaN(points[2].Maximum) {
		t.Errorf("point 2 = %+v, want average 3 and NaN maximum", points[2])
	}

	plain := got[1]
	if len(plain.Series) != 1 || len(plain.Series[0].Dims) != 0 {
		t.Errorf("series = %+v, want one without dims", plain.Series)
	}
}

func TestMetricsReturnsARMError(t *testing.T) {
	boom := errors.New("boom")

	if _, err := Metrics(context.Background(), &fakeARM{err: boom}, clusterID, metricsPeriod); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

func TestMetricsRejectsBadBody(t *testing.T) {
	if _, err := Metrics(context.Background(), &fakeARM{body: `nope`}, clusterID, metricsPeriod); err == nil {
		t.Error("err = nil, want a decode error")
	}
}

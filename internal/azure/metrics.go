package azure

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"time"
)

// MetricsQuery asks for metrics of one resource.
type MetricsQuery struct {
	Names      []string // at most 20; one Filter applies to all of them
	Filter     string   // the $filter, such as "nodepool eq '*'"; empty for none
	Start, End time.Time
	Interval   string // ISO 8601 grain: PT5M, PT30M, PT1H
}

// Metric is one metric with a series per dimension combination.
type Metric struct {
	Name, Unit string
	Series     []Series
}

// Series is the points of one dimension combination.
type Series struct {
	Dims   map[string]string // dimension name to value; empty without a filter
	Points []Point
}

// Point is one interval; Average and Maximum are NaN when the API has no value.
type Point struct {
	Time             time.Time
	Average, Maximum float64
}

// Metrics reads metrics of the resource with the given ID.
func Metrics(ctx context.Context, arm ARM, resourceID string, q MetricsQuery) ([]Metric, error) {
	body, err := arm.Get(ctx, resourceID+"/providers/Microsoft.Insights/metrics", q.values())
	if err != nil {
		return nil, err
	}
	var resp metricsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return resp.metrics(), nil
}

func (q MetricsQuery) values() url.Values {
	v := url.Values{
		"api-version": {"2024-02-01"},
		"metricnames": {strings.Join(q.Names, ",")},
		"timespan":    {q.Start.UTC().Format(time.RFC3339) + "/" + q.End.UTC().Format(time.RFC3339)},
		"interval":    {q.Interval},
		"aggregation": {"Average,Maximum"},
		// The default of 10 series cuts the rest without a warning.
		"top": {"100"},
	}
	if q.Filter != "" {
		v.Set("$filter", q.Filter)
	}
	return v
}

type metricsResponse struct {
	Value []struct {
		Name       struct{ Value string }
		Unit       string
		Timeseries []struct {
			Metadatavalues []struct {
				Name  struct{ Value string }
				Value string
			}
			Data []struct {
				TimeStamp time.Time
				Average   *float64
				Maximum   *float64
			}
		}
	}
}

func (r metricsResponse) metrics() []Metric {
	metrics := make([]Metric, len(r.Value))
	for i, m := range r.Value {
		metrics[i] = Metric{Name: m.Name.Value, Unit: m.Unit, Series: make([]Series, len(m.Timeseries))}
		for j, ts := range m.Timeseries {
			dims := make(map[string]string, len(ts.Metadatavalues))
			for _, md := range ts.Metadatavalues {
				dims[md.Name.Value] = md.Value
			}
			points := make([]Point, len(ts.Data))
			for k, d := range ts.Data {
				points[k] = Point{Time: d.TimeStamp, Average: orNaN(d.Average), Maximum: orNaN(d.Maximum)}
			}
			metrics[i].Series[j] = Series{Dims: dims, Points: points}
		}
	}
	return metrics
}

func orNaN(v *float64) float64 {
	if v == nil {
		return math.NaN()
	}
	return *v
}

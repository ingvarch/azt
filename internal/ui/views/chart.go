package views

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/ingvarch/pulse"

	"github.com/ingvarch/azt/internal/azure"
)

const (
	chartHeight     = 7
	chartLabelWidth = 4
	// chartAxis is the cell between the labels and the plot.
	chartAxis = 1
	// chartMinPlot is the narrowest plot worth drawing.
	chartMinPlot = 20
)

// chartColors are the series colors, used in turn.
var chartColors = []color.Color{lipgloss.Color("4"), lipgloss.Color("5"), lipgloss.Color("6"), lipgloss.Color("2"), lipgloss.Color("3")}

// plotWidth is the cells of the plot in a box with the inner width.
func plotWidth(inner int) int { return inner - chartLabelWidth - chartAxis }

// resample is the averages of the points in w columns over [start, end). A
// column is the mean of the points in it, or the point holding its middle
// when it has none; NaN when there is no finite value.
func resample(points []azure.Point, start, end time.Time, interval time.Duration, w int) []float64 {
	out := make([]float64, max(w, 0))
	for i := range out {
		out[i] = math.NaN()
	}
	span := end.Sub(start)
	if span <= 0 || w <= 0 {
		return out
	}
	var in []azure.Point
	for _, p := range points {
		if !p.Time.Before(start) && p.Time.Before(end) {
			in = append(in, p)
		}
	}
	slices.SortStableFunc(in, func(a, b azure.Point) int { return a.Time.Compare(b.Time) })
	sums, counts, holds := make([]float64, w), make([]int, w), make([]bool, w)
	for _, p := range in {
		i := int(p.Time.Sub(start) * time.Duration(w) / span)
		holds[i] = true
		if finite(p.Average) {
			sums[i] += p.Average
			counts[i]++
		}
	}
	for i := range out {
		switch {
		case counts[i] > 0:
			out[i] = sums[i] / float64(counts[i])
		case !holds[i]:
			mid := start.Add(span * time.Duration(2*i+1) / time.Duration(2*w))
			at := sort.Search(len(in), func(j int) bool { return in[j].Time.After(mid) }) - 1
			if at >= 0 && mid.Before(in[at].Time.Add(interval)) && finite(in[at].Average) {
				out[i] = in[at].Average
			}
		}
	}
	return out
}

// chartCeiling is the top of the scale: the first of 10, 20, 25, 50 and 100
// that holds the largest finite average, else that average rounded up to the
// next 10; 100 without data.
func chartCeiling(series [][]azure.Point) float64 {
	top, found := 0.0, false
	for _, pts := range series {
		for _, p := range pts {
			if finite(p.Average) {
				top, found = max(top, p.Average), true
			}
		}
	}
	if !found {
		return 100
	}
	for _, c := range []float64{10, 20, 25, 50, 100} {
		if top <= c {
			return c
		}
	}
	return math.Ceil(top/10) * 10
}

// axisLabel labels the column age steps from the newest with its time, or "now".
func axisLabel(m azure.Monitoring, period string, w int) func(age int) string {
	layout := choose(period == "24h", "15:04", "Jan 2")
	span := m.End.Sub(m.Start)
	return func(age int) string {
		if age == 0 {
			return "now"
		}
		return m.Start.Add(span * time.Duration(w-1-age) / time.Duration(w)).Local().Format(layout)
	}
}

// newChart is an empty chart of w columns with a series for each name.
func newChart(w int, names []string, ceiling float64, label func(age int) string) *pulse.Model {
	opts := []pulse.Option{
		pulse.WithRange(0, ceiling),
		pulse.WithGrid(true),
		pulse.WithAxisStyle(mutedStyle),
		pulse.WithLineWidth(1),
		pulse.WithFill(false),
		pulse.WithLabelWidth(chartLabelWidth),
		pulse.WithLabelFormatter(func(v float64) string { return fmt.Sprintf("%.0f%%", v) }),
		pulse.WithXAxis(label),
	}
	for i, name := range names {
		opts = append(opts, pulse.WithSeriesStyle(name, lipgloss.NewStyle().Foreground(chartColors[i%len(chartColors)])))
	}
	return pulse.New(w, chartHeight, opts...)
}

// chartLines draws the series, one per name, as a chart with its legend line.
func chartLines(m azure.Monitoring, period string, names []string, series [][]azure.Point, w int) []string {
	chart := newChart(w, names, chartCeiling(series), axisLabel(m, period, w))
	for i, name := range names {
		for _, v := range resample(series[i], m.Start, m.End, m.Interval, w) {
			chart.PushSeries(name, v)
		}
	}
	entries := make([]string, len(names))
	for i, name := range names {
		entries[i] = chart.SeriesStyle(name).Render("■") + " " + name
	}
	return append(strings.Split(chart.View(), "\n"), wrapEntries(entries, w+chartLabelWidth+chartAxis)...)
}

// wrapEntries joins the entries with two spaces, breaking lines to fit width.
func wrapEntries(entries []string, width int) []string {
	var lines []string
	for _, e := range entries {
		if n := len(lines) - 1; n >= 0 && lipgloss.Width(lines[n])+2+lipgloss.Width(e) <= width {
			lines[n] += "  " + e
		} else {
			lines = append(lines, e)
		}
	}
	return lines
}

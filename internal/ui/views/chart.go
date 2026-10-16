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

// tickStep is the spacing of the time labels: ticks at the local hours
// divisible by hours, or, when hours is 0, at the local midnights keep accepts.
type tickStep struct {
	span  time.Duration
	hours int
	keep  func(midnight time.Time) bool
}

const (
	chartDay = 24 * time.Hour
	// chartHourGap and chartDayGap are the fewest columns between two time
	// labels of the hour and the day steps.
	chartHourGap, chartDayGap = 12, 10
)

var (
	hourSteps = []tickStep{
		{span: time.Hour, hours: 1},
		{span: 2 * time.Hour, hours: 2},
		{span: 3 * time.Hour, hours: 3},
		{span: 6 * time.Hour, hours: 6},
		{span: 12 * time.Hour, hours: 12},
	}
	daySteps = []tickStep{
		{span: chartDay, keep: func(time.Time) bool { return true }},
		{span: 2 * chartDay, keep: func(d time.Time) bool { return epochDay(d)%2 == 0 }},
		{span: 7 * chartDay, keep: isMonday},
		{span: 14 * chartDay, keep: isEvenWeekMonday},
	}
)

func isMonday(d time.Time) bool { return d.Weekday() == time.Monday }

// isEvenWeekMonday is every other Monday, counted from Monday 1970-01-05, so
// the ticks stay 14 days apart across years with 53 ISO weeks.
func isEvenWeekMonday(d time.Time) bool { return isMonday(d) && (epochDay(d)-4)/7%2 == 0 }

// epochDay is the local calendar day of d counted from 1970-01-01.
func epochDay(d time.Time) int {
	return int(time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).Unix() / int64(chartDay/time.Second))
}

// chartStep is the smallest step of the period whose ticks are at least
// chartHourGap or chartDayGap columns apart in w columns over span, else the largest.
func chartStep(period string, span time.Duration, w int) tickStep {
	steps, gap := daySteps, time.Duration(chartDayGap)
	if period == "24h" {
		steps, gap = hourSteps, chartHourGap
	}
	for _, s := range steps {
		if s.span*time.Duration(w) >= gap*span {
			return s
		}
	}
	return steps[len(steps)-1]
}

// wallHour is hour h on the local day, false when the clocks skip it.
func wallHour(y int, mo time.Month, d, h int) (time.Time, bool) {
	t := time.Date(y, mo, d, h, 0, 0, 0, time.Local)
	return t, t.Hour() == h && t.Minute() == 0
}

// tickTimes is the local tick times of the step in [start, end), built from
// wall-clock fields so a daylight saving change does not shift them; a time
// the clocks skip is left out.
func tickTimes(step tickStep, start, end time.Time) []time.Time {
	var ticks []time.Time
	add := func(t time.Time) {
		if !t.Before(start) && t.Before(end) {
			ticks = append(ticks, t)
		}
	}
	y, mo, d := start.In(time.Local).Date()
	for i := 0; ; i++ {
		midnight, ok := wallHour(y, mo, d+i, 0)
		if !midnight.Before(end) {
			break
		}
		if step.hours == 0 {
			if ok && step.keep(midnight) {
				add(midnight)
			}
			continue
		}
		for h := 0; h < 24; h += step.hours {
			if t, ok := wallHour(y, mo, d+i, h); ok {
				add(t)
			}
		}
	}
	return ticks
}

// tickLabel labels the column holding a round time with it, and the newest
// column "now"; the columns map to times as in resample.
func tickLabel(m azure.Monitoring, period string, w int) func(age int) string {
	labels := map[int]string{}
	if span := m.End.Sub(m.Start); span > 0 && w > 0 {
		step := chartStep(period, span, w)
		layout := choose(step.hours > 0, "15:04", "Jan 2")
		for _, t := range tickTimes(step, m.Start, m.End) {
			labels[w-1-int(t.Sub(m.Start)*time.Duration(w)/span)] = t.Format(layout)
		}
	}
	labels[0] = "now"
	return func(age int) string { return labels[age] }
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
		pulse.WithXAxisAt(label),
	}
	for i, name := range names {
		opts = append(opts, pulse.WithSeriesStyle(name, lipgloss.NewStyle().Foreground(chartColors[i%len(chartColors)])))
	}
	return pulse.New(w, chartHeight, opts...)
}

// chartLines draws the series, one per name, as a chart with its legend line.
func chartLines(m azure.Monitoring, period string, names []string, series [][]azure.Point, w int) []string {
	chart := newChart(w, names, chartCeiling(series), tickLabel(m, period, w))
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

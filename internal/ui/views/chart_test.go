package views

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
)

var chartStart = time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)

// hourly is points of one hour each from chartStart, with the averages.
func hourly(avgs ...float64) []azure.Point {
	pts := make([]azure.Point, len(avgs))
	for i, a := range avgs {
		pts[i] = azure.Point{Time: chartStart.Add(time.Duration(i) * time.Hour), Average: a, Maximum: a}
	}
	return pts
}

func sameValues(got, want []float64) bool {
	return slices.EqualFunc(got, want, func(a, b float64) bool {
		return a == b || (math.IsNaN(a) && math.IsNaN(b))
	})
}

func TestResample(t *testing.T) {
	end := chartStart.Add(8 * time.Hour)
	for _, c := range []struct {
		name string
		pts  []azure.Point
		step time.Duration
		w    int
		want []float64
	}{
		{"more points than columns: the mean per column", hourly(1, 3, 5, 7, 9, 11, 13, 15), time.Hour, 4, []float64{2, 6, 10, 14}},
		{"fewer points than columns: the point under the middle", []azure.Point{
			{Time: chartStart, Average: 10},
			{Time: chartStart.Add(4 * time.Hour), Average: 20},
		}, 4 * time.Hour, 8, []float64{10, 10, 10, 10, 20, 20, 20, 20}},
		{"NaN points leave NaN and do not count in the mean", hourly(2, nan, 6, nan, nan, nan, 8, 10), time.Hour, 4, []float64{2, 6, nan, 9}},
		{"a missing point gives NaN", hourly(5, 5), time.Hour, 8, []float64{5, 5, nan, nan, nan, nan, nan, nan}},
		{"points outside the period are ignored", append(
			[]azure.Point{{Time: chartStart.Add(-time.Hour), Average: 99}, {Time: end, Average: 99}}, hourly(4, 4, 4, 4, 4, 4, 4, 4)...,
		), time.Hour, 4, []float64{4, 4, 4, 4}},
		{"a point outside the period does not fill a column under its middle", []azure.Point{
			{Time: chartStart.Add(-time.Hour), Average: 99}, {Time: chartStart.Add(time.Hour), Average: 5},
		}, 2 * time.Hour, 8, []float64{nan, 5, 5, nan, nan, nan, nan, nan}},
		{"gaps with fewer points than columns", []azure.Point{
			{Time: chartStart, Average: 10}, {Time: chartStart.Add(4 * time.Hour), Average: nan},
		}, 4 * time.Hour, 8, []float64{10, 10, 10, 10, nan, nan, nan, nan}},
		{"a missing timestamp gives NaN", []azure.Point{{Average: 7}}, time.Hour, 2, []float64{nan, nan}},
		{"a column with only NaN points stays NaN", hourly(1, 2, 3, nan, 5, 6, 7, 8), time.Hour, 10, []float64{1, 2, 3, nan, nan, 5, 6, 7, 8, 8}},
		{"the column middle picks the point", hourly(5, 5), time.Hour, 10, []float64{5, 5, nan, nan, nan, nan, nan, nan, nan, nan}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := resample(c.pts, chartStart, end, c.step, c.w); !sameValues(got, c.want) {
				t.Errorf("resample = %v, want %v", got, c.want)
			}
		})
	}
}

func TestResampleEmptyPeriod(t *testing.T) {
	if got := resample(hourly(1, 2), chartStart, chartStart, time.Hour, 3); !sameValues(got, []float64{nan, nan, nan}) {
		t.Errorf("zero span = %v, want 3 NaN", got)
	}
	if got := resample(hourly(1, 2), chartStart, chartStart.Add(2*time.Hour), time.Hour, 0); len(got) != 0 {
		t.Errorf("zero width = %v, want none", got)
	}
}

func TestAxisLabel(t *testing.T) {
	prev := time.Local
	time.Local = time.FixedZone("test", 2*3600)
	t.Cleanup(func() { time.Local = prev })
	m := azure.Monitoring{Start: chartStart, End: chartStart.Add(24 * time.Hour)}
	for _, c := range []struct {
		period string
		span   time.Duration
		w, age int
		want   string
	}{
		{"24h", 24 * time.Hour, 24, 0, "now"},
		{"24h", 24 * time.Hour, 24, 23, "02:00"},
		{"24h", 24 * time.Hour, 24, 12, "13:00"},
		{"7d", 7 * 24 * time.Hour, 7, 0, "now"},
		{"7d", 7 * 24 * time.Hour, 7, 6, "Oct 3"},
		{"30d", 30 * 24 * time.Hour, 30, 15, "Oct 17"},
	} {
		m.End = m.Start.Add(c.span)
		if got := axisLabel(m, c.period, c.w)(c.age); got != c.want {
			t.Errorf("%s age %d = %q, want %q", c.period, c.age, got, c.want)
		}
	}
}

func TestChartCeiling(t *testing.T) {
	for _, c := range []struct {
		name string
		pts  []azure.Point
		want float64
	}{
		{"low", hourly(3, 6.5), 10},
		{"zero", hourly(0, 0), 10},
		{"exactly 10", hourly(10), 10},
		{"to 20", hourly(5, 12), 20},
		{"to 25", hourly(21), 25},
		{"to 50", hourly(30), 50},
		{"to 100", hourly(20, 99.5), 100},
		{"above 100", hourly(20, 102), 110},
		{"NaN ignored", hourly(nan, 30, math.Inf(1)), 50},
		{"nothing", nil, 100},
	} {
		if got := chartCeiling([][]azure.Point{c.pts}); got != c.want {
			t.Errorf("%s: ceiling = %v, want %v", c.name, got, c.want)
		}
	}
}

// chartMonitoring is the fixture with a period of 91 hours, one point per plot column at width 100.
func chartMonitoring(pools ...azure.Series) azure.Monitoring {
	m := monitoringFixture()
	m.Start, m.End, m.Interval = chartStart, chartStart.Add(91*time.Hour), time.Hour
	m.Pools = monOK([]azure.Metric{monMetric("node_cpu_usage_percentage", pools...)})
	return m
}

func poolSeries(name string, pts []azure.Point) azure.Series {
	return azure.Series{Dims: monDims("nodepool", name), Points: pts}
}

func flat(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func TestChartBoxes(t *testing.T) {
	m := chartMonitoring(poolSeries("system", hourly(flat(91, 30)...)), poolSeries("burst", hourly(flat(91, 40)...)))
	lines := monLines(aksRow, m)
	var titles []string
	for _, l := range lines {
		if slices.Contains([]string{"Setup", "Node CPU", "Node memory", "Node pools"}, l) {
			titles = append(titles, l)
		}
	}
	if want := []string{"Setup", "Node CPU", "Node memory", "Node pools"}; !slices.Equal(titles, want) {
		t.Fatalf("boxes = %v, want %v", titles, want)
	}
	for _, title := range []string{"Node CPU", "Node memory"} {
		body := monSection(t, lines, title)
		if len(body) != 10 {
			t.Fatalf("%s has %d lines, want 10 (7 plot rows, axis, labels, legend):\n%s", title, len(body), strings.Join(body, "\n"))
		}
		if legend := body[len(body)-1]; legend != "■ burst (auto)  ■ system" {
			t.Errorf("%s legend = %q", title, legend)
		}
		// The fixture has CPU data only: its top is 50 for 40%; memory has none and keeps 100.
		top := map[string]string{"Node CPU": " 50%", "Node memory": "100%"}[title]
		if !strings.HasPrefix(body[0], top+"│") {
			t.Errorf("%s first line = %q, want it to start with %q", title, body[0], top)
		}
		if last := body[chartHeight+1]; !strings.HasSuffix(last, "now") {
			t.Errorf("%s label row = %q, want it to end with now", title, last)
		}
		for _, row := range body[:chartHeight] {
			if strings.ContainsAny(row, "░━┃┏┓┗┛╋┳┻") {
				t.Errorf("%s draws a fill or a bold line: %q", title, row)
			}
		}
		if label := rawLineIn(t, monRaw(aksRow, m), title, strings.TrimSpace(top)); !strings.Contains(label, mutedStyle.Render(strings.TrimSpace(top))) {
			t.Errorf("%s axis label is not muted: %q", title, label)
		}
	}
	for _, l := range monRaw(aksRow, m) {
		if w := lipgloss.Width(l); w > 100 {
			t.Errorf("line is %d cells wide: %q", w, plain(l))
		}
	}
}

func TestChartCeilingInBoxes(t *testing.T) {
	mem := hourly(flat(91, 50)...)
	mem[10].Average = 102
	m := chartMonitoring()
	m.Pools = monOK([]azure.Metric{
		monMetric("node_cpu_usage_percentage", poolSeries("system", hourly(flat(91, 30)...))),
		monMetric("node_memory_working_set_percentage", poolSeries("system", mem)),
	})
	lines := monLines(aksRow, m)
	cpu, memory := monSection(t, lines, "Node CPU"), monSection(t, lines, "Node memory")
	if !strings.HasPrefix(cpu[0], " 50%│") {
		t.Errorf("CPU first row = %q, want the 50%% label (top is 50 for 30%%)", cpu[0])
	}
	if !strings.HasPrefix(memory[0], "    │") || !strings.HasPrefix(memory[1], "100%│") {
		t.Errorf("memory rows = %q, %q, want a blank label then 100%% (top is 110)", memory[0], memory[1])
	}
}

func TestChartLegendWraps(t *testing.T) {
	m := chartMonitoring(
		poolSeries("alpha-long-pool", hourly(flat(91, 30)...)),
		poolSeries("beta-long-pool", hourly(flat(91, 40)...)),
	)
	body := monSection(t, monLinesAt(aksRow, m, 29), "Node CPU")
	legend := body[chartHeight+2:]
	if len(legend) < 2 {
		t.Fatalf("legend is %d line, want it wrapped:\n%s", len(legend), strings.Join(body, "\n"))
	}
	got := strings.Join(legend, " ")
	for _, want := range []string{"■ alpha-long-pool", "■ beta-long-pool"} {
		if !strings.Contains(got, want) {
			t.Errorf("legend lost %q:\n%s", want, strings.Join(legend, "\n"))
		}
	}
}

func TestChartBoxesLeftOut(t *testing.T) {
	good := chartMonitoring(poolSeries("system", hourly(flat(91, 30)...)))
	failed := good
	failed.Pools = monFailed[[]azure.Metric](errors.New("boom"))
	noSeries := good
	noSeries.Pools = monOK([]azure.Metric{})
	noData := chartMonitoring(poolSeries("system", hourly(nan, nan)))
	for _, c := range []struct {
		name  string
		m     azure.Monitoring
		width int
	}{
		{"narrow", good, 28},
		{"pools failed", failed, 100},
		{"no series", noSeries, 100},
		{"no data", noData, 100},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(monLinesAt(aksRow, c.m, c.width), "\n")
			for _, title := range []string{"Node CPU", "Node memory"} {
				if strings.Contains(got, title) {
					t.Errorf("%q is drawn:\n%s", title, got)
				}
			}
			if !strings.Contains(got, "Node pools") {
				t.Errorf("Node pools is gone:\n%s", got)
			}
		})
	}
	if got := strings.Join(monLinesAt(aksRow, good, 29), "\n"); !strings.Contains(got, "Node CPU") {
		t.Errorf("no chart box at the narrowest width with 20 plot cells:\n%s", got)
	}
}

func TestChartGap(t *testing.T) {
	avgs := flat(91, 15) // row 2 of a 0-20 scale, off the grid rows
	for i := 30; i < 40; i++ {
		avgs[i] = nan
	}
	m := chartMonitoring(poolSeries("system", hourly(avgs...)))
	rows := monSection(t, monLines(aksRow, m), "Node CPU")
	const labels = 5
	drawn := false
	for _, r := range []int{1, 2, 4, 5} {
		cells := []rune(rows[r])
		for col := 30; col < 40; col++ {
			if labels+col < len(cells) && cells[labels+col] != ' ' {
				t.Errorf("row %d column %d = %q, want a gap", r, col, cells[labels+col])
			}
		}
		drawn = drawn || (labels+10 < len(cells) && cells[labels+10] != ' ')
	}
	if !drawn {
		t.Errorf("the line is not drawn outside the gap:\n%s", strings.Join(rows, "\n"))
	}
}

func TestChartColors(t *testing.T) {
	m := chartMonitoring(poolSeries("b", hourly(flat(91, 30)...)), poolSeries("a", hourly(flat(91, 40)...)))
	legend := rawLine(t, monRaw(aksRow, m), "■ a")
	first, second := strings.Index(legend, "\x1b[34m■"), strings.Index(legend, "\x1b[35m■")
	if first < 0 || second < first {
		t.Errorf("legend colors are not blue then magenta in name order: %q", legend)
	}
}

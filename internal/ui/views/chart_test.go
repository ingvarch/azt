package views

import (
	"errors"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

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

// localZone sets time.Local to loc for the test.
func localZone(t *testing.T, loc *time.Location) {
	t.Helper()
	prev := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = prev })
}

func TestChartStep(t *testing.T) {
	for _, c := range []struct {
		period string
		span   time.Duration
		w      int
		want   time.Duration
	}{
		{"24h", chartDay, 40, 12 * time.Hour},
		{"24h", chartDay, 95, 6 * time.Hour},
		{"24h", chartDay, 140, 3 * time.Hour},
		{"24h", chartDay, 96, 3 * time.Hour},
		{"24h", chartDay, 5, 12 * time.Hour},
		{"7d", 7 * chartDay, 40, 2 * chartDay},
		{"7d", 7 * chartDay, 60, 2 * chartDay},
		{"7d", 7 * chartDay, 80, chartDay},
		{"7d", 7 * chartDay, 95, chartDay},
		{"7d", 7 * chartDay, 140, chartDay},
		{"30d", 30 * chartDay, 40, 14 * chartDay},
		{"30d", 30 * chartDay, 95, 7 * chartDay},
		{"30d", 30 * chartDay, 140, 7 * chartDay},
		{"30d", 30 * chartDay, 5, 14 * chartDay},
	} {
		if got := chartStep(c.period, c.span, c.w).span; got != c.want {
			t.Errorf("%s at width %d: step %v, want %v", c.period, c.w, got, c.want)
		}
	}
}

func TestTickTimesDaySteps(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)
	days := func(step time.Duration, from, to time.Time) []int {
		var out []int
		for _, tk := range tickTimes(stepOf(t, step), from, to) {
			if tk.Hour() != 0 || tk.Minute() != 0 {
				t.Errorf("tick %v is not a local midnight", tk)
			}
			out = append(out, tk.Day())
		}
		return out
	}
	// 2026-10-01 is day 20727 since 1970: odd, so the even days are the 2nd, 4th, ...
	if got, want := days(2*chartDay, start, start.AddDate(0, 0, 7)), []int{2, 4, 6}; !slices.Equal(got, want) {
		t.Errorf("2 days = %v, want %v", got, want)
	}
	if got, want := days(7*chartDay, start, end), []int{5, 12, 19, 26}; !slices.Equal(got, want) {
		t.Errorf("7 days = %v, want %v", got, want)
	}
	// ISO weeks 41 to 44 start on the 5th, 12th, 19th and 26th.
	if got, want := days(14*chartDay, start, end), []int{12, 26}; !slices.Equal(got, want) {
		t.Errorf("14 days = %v, want %v", got, want)
	}
}

// stepOf is the step with the span.
func stepOf(t *testing.T, span time.Duration) tickStep {
	t.Helper()
	for _, s := range slices.Concat(hourSteps, daySteps) {
		if s.span == span {
			return s
		}
	}
	t.Fatalf("no step of %v", span)
	return tickStep{}
}

// labelsAt is the non-empty labels of the chart by age.
func labelsAt(label func(int) string, w int) map[int]string {
	got := map[int]string{}
	for age := range w {
		if s := label(age); s != "" {
			got[age] = s
		}
	}
	return got
}

func wantLabels(t *testing.T, got, want map[int]string) {
	t.Helper()
	if !maps.Equal(got, want) {
		t.Errorf("labels = %v, want %v", got, want)
	}
}

func TestTickLabelHours(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	start := time.Date(2026, 10, 3, 13, 0, 0, 0, time.Local)
	m := azure.Monitoring{Start: start, End: start.Add(24 * time.Hour)}
	// Width 96 gives 3-hour ticks, four columns an hour: 15:00 is 2 hours in, column 8.
	wantLabels(t, labelsAt(tickLabel(m, "24h", 96), 96), map[int]string{
		0: "now", 3: "12:00", 15: "09:00", 27: "06:00", 39: "03:00", 51: "00:00", 63: "21:00", 75: "18:00", 87: "15:00",
	})
}

func TestTickLabelNowTakesTheNewestColumn(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	start := time.Date(2026, 10, 3, 12, 10, 0, 0, time.Local)
	m := azure.Monitoring{Start: start, End: start.Add(24 * time.Hour)}
	// 12:00 tomorrow falls in the newest column, [11:55, 12:10).
	got := labelsAt(tickLabel(m, "24h", 96), 96)
	if got[0] != "now" {
		t.Errorf("age 0 = %q, want now", got[0])
	}
	for age, s := range got {
		if s == "12:00" {
			t.Errorf("12:00 is labelled at age %d", age)
		}
	}
}

func TestTickLabelDays(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	start := time.Date(2026, 10, 3, 6, 0, 0, 0, time.Local)
	m := azure.Monitoring{Start: start, End: start.Add(7 * 24 * time.Hour)}
	// Width 84 gives 1-day ticks, 12 columns a day: Oct 4 midnight is 18 hours in, column 9.
	wantLabels(t, labelsAt(tickLabel(m, "7d", 84), 84), map[int]string{
		0: "now", 2: "Oct 10", 14: "Oct 9", 26: "Oct 8", 38: "Oct 7", 50: "Oct 6", 62: "Oct 5", 74: "Oct 4",
	})
}

func TestTickLabelWeeks(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	m := azure.Monitoring{Start: start, End: start.AddDate(0, 0, 30)}
	// Width 90 gives 7-day ticks, 3 columns a day: Monday Oct 5 is 4 days in, column 12.
	wantLabels(t, labelsAt(tickLabel(m, "30d", 90), 90), map[int]string{
		0: "now", 14: "Oct 26", 35: "Oct 19", 56: "Oct 12", 77: "Oct 5",
	})
}

func TestTickTimesAcrossDaylightSaving(t *testing.T) {
	stockholm, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		t.Fatal(err)
	}
	localZone(t, stockholm)
	format := func(ticks []time.Time) []string {
		var got []string
		for _, tk := range ticks {
			got = append(got, tk.Format("Jan 2 15:04"))
		}
		return got
	}
	// Clocks go back on Sunday 2026-10-25, so this day has 25 hours.
	start := time.Date(2026, 10, 24, 20, 0, 0, 0, time.Local)
	end := start.Add(24 * time.Hour)
	got := format(tickTimes(stepOf(t, 6*time.Hour), start, end))
	if want := []string{"Oct 25 00:00", "Oct 25 06:00", "Oct 25 12:00", "Oct 25 18:00"}; !slices.Equal(got, want) {
		t.Errorf("autumn ticks = %v, want %v", got, want)
	}
	// Clocks go forward on Sunday 2026-03-29, so 03:00 does not exist.
	start = time.Date(2026, 3, 28, 22, 0, 0, 0, time.Local)
	end = start.Add(24 * time.Hour)
	got = format(tickTimes(stepOf(t, 2*time.Hour), start, end))
	want := []string{"Mar 28 22:00", "Mar 29 00:00", "Mar 29 04:00", "Mar 29 06:00", "Mar 29 08:00", "Mar 29 10:00", "Mar 29 12:00", "Mar 29 14:00", "Mar 29 16:00", "Mar 29 18:00", "Mar 29 20:00", "Mar 29 22:00"}
	if !slices.Equal(got, want) {
		t.Errorf("spring ticks = %v, want %v", got, want)
	}
	// Lord Howe Island jumps from 02:00 to 02:30 on 2026-10-04: 02:30 is not a round time.
	lordHowe, err := time.LoadLocation("Australia/Lord_Howe")
	if err != nil {
		t.Fatal(err)
	}
	localZone(t, lordHowe)
	start = time.Date(2026, 10, 3, 20, 0, 0, 0, time.Local)
	got = format(tickTimes(stepOf(t, 2*time.Hour), start, start.Add(12*time.Hour)))
	want = []string{"Oct 3 20:00", "Oct 3 22:00", "Oct 4 00:00", "Oct 4 04:00", "Oct 4 06:00", "Oct 4 08:00"}
	if !slices.Equal(got, want) {
		t.Errorf("half-hour change ticks = %v, want %v", got, want)
	}
	// Midnights stay midnights after the change: a week across 2026-10-25 in Stockholm.
	localZone(t, stockholm)
	start = time.Date(2026, 10, 22, 17, 30, 0, 0, time.Local)
	got = format(tickTimes(stepOf(t, chartDay), start, start.Add(7*chartDay)))
	want = []string{"Oct 23 00:00", "Oct 24 00:00", "Oct 25 00:00", "Oct 26 00:00", "Oct 27 00:00", "Oct 28 00:00", "Oct 29 00:00"}
	if !slices.Equal(got, want) {
		t.Errorf("midnights across the change = %v, want %v", got, want)
	}
}

func TestTickTimesSkipMissingMidnight(t *testing.T) {
	havana, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatal(err)
	}
	localZone(t, havana)
	// Clocks go forward at midnight on 2026-03-08, so that day has no midnight.
	start := time.Date(2026, 3, 5, 12, 0, 0, 0, time.Local)
	end := time.Date(2026, 3, 12, 12, 0, 0, 0, time.Local)
	var got []string
	for _, tk := range tickTimes(stepOf(t, chartDay), start, end) {
		got = append(got, tk.Format("Jan 2 15:04"))
	}
	if want := []string{"Mar 6 00:00", "Mar 7 00:00", "Mar 9 00:00", "Mar 10 00:00", "Mar 11 00:00", "Mar 12 00:00"}; !slices.Equal(got, want) {
		t.Errorf("midnights = %v, want %v", got, want)
	}
	got = nil
	for _, tk := range tickTimes(stepOf(t, 6*time.Hour), start.AddDate(0, 0, 2), start.AddDate(0, 0, 4)) {
		got = append(got, tk.Format("Jan 2 15:04"))
	}
	if want := []string{"Mar 7 12:00", "Mar 7 18:00", "Mar 8 06:00", "Mar 8 12:00", "Mar 8 18:00", "Mar 9 00:00", "Mar 9 06:00"}; !slices.Equal(got, want) {
		t.Errorf("6-hour ticks = %v, want %v", got, want)
	}
}

func TestTickTimesTwoWeeksAcrossTheYear(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	// 2026 has 53 ISO weeks; the ticks must stay 14 days apart into 2027.
	ticks := tickTimes(stepOf(t, 14*chartDay), time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local), time.Date(2027, 3, 1, 0, 0, 0, 0, time.Local))
	if len(ticks) < 8 {
		t.Fatalf("ticks = %v, want one every 14 days", ticks)
	}
	for i := 1; i < len(ticks); i++ {
		if d := ticks[i].Sub(ticks[i-1]); d != 14*chartDay || !isMonday(ticks[i]) {
			t.Errorf("ticks %v and %v are %v apart", ticks[i-1], ticks[i], d)
		}
	}
}

func TestTickLabelColumnsOfUTCStart(t *testing.T) {
	localZone(t, time.FixedZone("W", -5*3600))
	start := time.Date(2026, 10, 16, 1, 5, 0, 0, time.UTC)
	m := azure.Monitoring{Start: start, End: start.Add(24 * time.Hour)}
	// Width 140 gives 3-hour ticks; the columns are [Start+i*span/w, Start+(i+1)*span/w).
	wantLabels(t, labelsAt(tickLabel(m, "24h", 140), 140), map[int]string{
		0: "now", 12: "18:00", 29: "15:00", 47: "12:00", 64: "09:00", 82: "06:00", 99: "03:00", 117: "00:00", 134: "21:00",
	})
}

func TestTickLabelUsesTheRealSpan(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	start := time.Date(2026, 10, 3, 13, 0, 0, 0, time.Local)
	// 24h at width 96 gives 3-hour ticks, but a minute more makes them just under 12 columns apart.
	m := azure.Monitoring{Start: start, End: start.Add(24*time.Hour + time.Minute)}
	wantLabels(t, labelsAt(tickLabel(m, "24h", 96), 96), map[int]string{0: "now", 4: "12:00", 28: "06:00", 52: "00:00", 76: "18:00"})
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
	axis := []rune(rows[chartHeight])
	drawn := false
	for _, r := range []int{1, 2, 4, 5} {
		cells := []rune(rows[r])
		for col := 30; col < 40; col++ {
			want := ' '
			if labels+col < len(axis) && axis[labels+col] == '┴' {
				want = '│'
			}
			if labels+col < len(cells) && cells[labels+col] != want {
				t.Errorf("row %d column %d = %q, want %q", r, col, cells[labels+col], want)
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

func TestChartLabelRow(t *testing.T) {
	localZone(t, time.FixedZone("T", 2*3600))
	start := time.Date(2026, 10, 3, 13, 0, 0, 0, time.Local)
	m := azure.Monitoring{Start: start, End: start.Add(24 * time.Hour), Interval: time.Hour}
	rows := plainAll(chartLines(m, "24h", []string{"a"}, [][]azure.Point{hourly(10, 20)}, 96))
	axis, labels := []rune(rows[chartHeight]), []rune(rows[chartHeight+1])
	if !strings.HasSuffix(strings.TrimRight(string(labels), " "), "now") {
		t.Errorf("label row = %q, want it to end with now", string(labels))
	}
	for _, old := range []string{"13:00", "13:15", "01:00"} {
		if strings.Contains(string(labels), old) {
			t.Errorf("label row has %s: %q", old, string(labels))
		}
	}
	marks := 0
	// 12:00 is dropped: it touches now.
	for _, text := range []string{"15:00", "18:00", "21:00", "00:00", "03:00", "06:00", "09:00"} {
		at := strings.Index(string(labels), text)
		if at < 0 {
			t.Errorf("label row has no %s: %q", text, string(labels))
			continue
		}
		at = len([]rune(string(labels)[:at]))
		if !slices.Contains(axis[at:at+len(text)], '┴') {
			t.Errorf("no ┴ under %s: %q", text, string(axis))
		}
	}
	for _, r := range axis {
		if r == '┴' {
			marks++
		}
	}
	if marks != 7 {
		t.Errorf("axis has %d ┴, want 7: %q", marks, string(axis))
	}
}

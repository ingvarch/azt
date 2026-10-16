package views

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

const gib = 1 << 30

const (
	diagnosticKey = "Diagnostic settings"
	// continuation indents the lines that continue a wrapped line.
	continuation = "  "
)

var periodNames = map[string]string{"24h": "24 hours", "7d": "7 days", "30d": "30 days"}

func aksMonitoringKey(row map[string]any, period string) data.Key {
	return data.Key{Kind: data.AKSMonitoring, Subscription: str(row, "subscriptionId"), ID: str(row, "id"), Period: period}
}

func aksMonitoring(row map[string]any, res tabData, width int) []string {
	switch {
	case res.loading:
		return []string{"Loading monitoring…"}
	case res.err != nil:
		return []string{failedLine(res.err)}
	}
	m, _ := res.data.(azure.Monitoring)
	secs := append([]section{setupSection(row, m, innerWidth(width))}, poolChartSections(row, m, res.period, innerWidth(width))...)
	secs = append(secs, poolsSection(row, m, innerWidth(width)), nodesSection(m), podsSection(m), controlPlaneSection(m))
	if autoscaling(row) {
		secs = append(secs, autoscalerSection(m))
	}
	secs = append(secs, alertsSection(m))
	return append([]string{periodLine(res.period, width), ""}, sections(width, secs...)...)
}

// periodLine is the period shown on the left and the key to the next one on the right.
func periodLine(period string, width int) string {
	left, right := "Last "+periodNames[period], "p: "+periodNames[azure.NextPeriod(period)]
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + strings.Repeat(" ", gap) + faintStyle.Render(right)
}

// failedLine is the line for a part that could not be read.
func failedLine(err error) string {
	if azure.Forbidden(err) {
		return errorStyle.Render("no access: needs Reader or Monitoring Reader on the cluster")
	}
	return errorStyle.Render("failed: " + azure.ErrorText(err))
}

// partSection draws the lines of a section that reads one part; a part with an error gives the error.
func partSection(title string, err error, lines func() []string) section {
	if err != nil {
		return section{title: title, lines: []string{failedLine(err)}}
	}
	return section{title: title, lines: lines()}
}

// listPairs puts lines under one key, the key beside the first line.
func listPairs(key string, lines []string) []pair {
	pairs := make([]pair, len(lines))
	for i, l := range lines {
		pairs[i].value = l
	}
	pairs[0].key = key
	return pairs
}

func setupSection(row map[string]any, m azure.Monitoring, width int) section {
	props := dig(row, "properties")
	prometheus, _ := flag(props, "azureMonitorProfile", "metrics", "enabled")
	pairs := []pair{
		{"Container insights", containerInsights(props)},
		{"Managed Prometheus", switchText(choose(prometheus, "on", "off"))},
	}
	if m.Diagnostics.Err != nil {
		pairs = append(pairs, pair{diagnosticKey, failedLine(m.Diagnostics.Err)})
	} else {
		pairs = append(pairs, listPairs(diagnosticKey, diagnosticLines(m.Diagnostics.Data, width-lipgloss.Width(diagnosticKey)-keyGap))...)
	}
	return section{title: "Setup", lines: block(pairs)}
}

// containerInsights is the omsagent add-on as on or off, with its workspace when on.
func containerInsights(props any) string {
	addons := dig(props, "addonProfiles")
	for _, name := range mapKeys(addons) {
		if on, _ := flag(addons, name, "enabled"); on && strings.EqualFold(name, "omsagent") {
			return addonState(dig(addons, name))
		}
	}
	return switchText("off")
}

// diagnosticLines draws each setting as "<name>: <logs> → <destinations>" in
// lines of up to width cells: the logs wrap and the destinations stay whole.
func diagnosticLines(settings []azure.DiagnosticSetting, width int) []string {
	if len(settings) == 0 {
		return []string{"none"}
	}
	var out []string
	for _, s := range settings {
		logs := slices.Clone(s.Logs)
		if s.Metrics {
			logs = append(logs, "metrics")
		}
		if len(logs) == 0 {
			logs = []string{"no logs"}
		}
		lines := wrapItems(s.Name+":", logs, width)
		tail := "→ " + strings.Join(s.Destinations, ", ")
		if last := len(lines) - 1; lipgloss.Width(lines[last])+1+lipgloss.Width(tail) <= width {
			lines[last] += " " + tail
		} else {
			lines = append(lines, continuation+tail)
		}
		out = append(out, lines...)
	}
	return out
}

// wrapItems lists the items comma separated after prefix, breaking lines
// between items to fit width; later lines start with continuation.
func wrapItems(prefix string, items []string, width int) []string {
	lines := []string{prefix}
	for i, it := range items {
		if i < len(items)-1 {
			it += ","
		}
		last := len(lines) - 1
		if cur := lines[last]; cur != prefix && lipgloss.Width(cur)+1+lipgloss.Width(it) > width {
			lines = append(lines, continuation+it)
		} else {
			lines[last] += " " + it
		}
	}
	return lines
}

// poolMetrics are the node CPU and memory series by pool and the pool names, sorted.
func poolMetrics(m azure.Monitoring) (cpu, mem []azure.Series, names []string) {
	cpu = seriesOf(m.Pools.Data, "node_cpu_usage_percentage")
	mem = seriesOf(m.Pools.Data, "node_memory_working_set_percentage")
	return cpu, mem, dimValues(slices.Concat(cpu, mem), "nodepool")
}

// poolLabels are the pool names as the table and the charts show them: a pool the cluster does not list is a Karpenter pool.
func poolLabels(row map[string]any, names []string) []string {
	own := make([]string, 0, len(names))
	for _, p := range pools(dig(row, "properties")) {
		own = append(own, str(p, "name"))
	}
	labels := make([]string, len(names))
	for i, name := range names {
		labels[i] = name + choose(containsFold(own, name), "", " (auto)")
	}
	return labels
}

// poolChartSections are the CPU and memory charts of the pools; with no pool data or room for a plot there are none.
func poolChartSections(row map[string]any, m azure.Monitoring, period string, width int) []section {
	cpu, mem, names := poolMetrics(m)
	if m.Pools.Err != nil || plotWidth(width) < chartMinPlot || len(names) == 0 || !hasData(slices.Concat(cpu, mem)...) {
		return nil
	}
	labels := poolLabels(row, names)
	chart := func(title string, metric []azure.Series) section {
		points := make([][]azure.Point, len(names))
		for i, name := range names {
			points[i] = byDim(metric, "nodepool", name).Points
		}
		return section{title: title, lines: chartLines(m, period, labels, points, plotWidth(width))}
	}
	return []section{chart("Node CPU", cpu), chart("Node memory", mem)}
}

func poolsSection(row map[string]any, m azure.Monitoring, width int) section {
	return partSection(overviewPools, m.Pools.Err, func() []string {
		cpu, mem, names := poolMetrics(m)
		switch {
		case len(names) == 0:
			return []string{"none"}
		case !hasData(slices.Concat(cpu, mem)...):
			return []string{"no data"}
		}
		rows := make([][]string, len(names))
		for i, label := range poolLabels(row, names) {
			c, mm := byDim(cpu, "nodepool", names[i]), byDim(mem, "nodepool", names[i])
			rows[i] = []string{label, percent(meanOf(c)), percent(peak(c)), percent(meanOf(mm)), percent(peak(mm))}
		}
		cols := []column{{title: "Pool"}, {title: "CPU avg"}, {title: "CPU max"}, {title: "Memory avg"}, {title: "Memory max"}}
		return summaryTable(cols, rows, width)
	})
}

func nodesSection(m azure.Monitoring) section {
	var pairs []pair
	if m.Ready.Err != nil {
		pairs = append(pairs, pair{"Ready", failedLine(m.Ready.Err)})
	} else {
		ready := allSeries(m.Ready.Data)
		for _, n := range []struct {
			label, status string
			warn          bool
		}{{"Ready", "Ready", false}, {"Not ready", "NotReady", true}, {"Unknown", "Unknown", true}} {
			s := byDim(ready, "status2", n.status)
			pairs = append(pairs, pair{n.label, warnIf(n.warn && positive(s), rounded(latest(s)))})
		}
	}
	if m.Conditions.Err != nil {
		pairs = append(pairs, pair{"Problems", failedLine(m.Conditions.Err)})
	} else {
		pairs = append(pairs, listPairs("Problems", problems(allSeries(m.Conditions.Data)))...)
	}
	cores := seriesOf(m.Cluster.Data, "kube_node_status_allocatable_cpu_cores")
	memory := seriesOf(m.Cluster.Data, "kube_node_status_allocatable_memory_bytes")
	if m.Cluster.Err != nil {
		pairs = append(pairs, pair{"Allocatable", failedLine(m.Cluster.Err)})
	} else {
		pairs = append(pairs, pair{"Allocatable", coreText(latest(firstSeries(cores))) + ", " + memoryText(latest(firstSeries(memory)))})
	}
	failed := m.Ready.Err != nil || m.Conditions.Err != nil || m.Cluster.Err != nil
	if !failed && !hasData(slices.Concat(allSeries(m.Ready.Data), allSeries(m.Conditions.Data), cores, memory)...) {
		return section{title: "Nodes", lines: []string{"no data"}}
	}
	return section{title: "Nodes", lines: block(pairs)}
}

// problems lists the node conditions other than Ready that hold on a node, one per line, or "none".
func problems(conditions []azure.Series) []string {
	conditions = slices.Clone(conditions)
	slices.SortFunc(conditions, func(a, b azure.Series) int { return strings.Compare(a.Dims["condition"], b.Dims["condition"]) })
	var out []string
	for _, s := range conditions {
		v, ok := latest(s)
		if n := int(math.Round(v)); ok && n > 0 && !strings.EqualFold(s.Dims["condition"], "Ready") {
			out = append(out, warnStyle.Render(fmt.Sprintf("%s on %d %s", s.Dims["condition"], n, choose(n == 1, "node", "nodes"))))
		}
	}
	if len(out) == 0 {
		return []string{"none"}
	}
	return out
}

func podsSection(m azure.Monitoring) section {
	return partSection("Pods", m.Pods.Err, func() []string {
		series := allSeries(m.Pods.Data)
		if !hasData(series...) {
			return []string{"no data"}
		}
		var labels []string
		var rows [][]string
		for _, p := range []struct {
			phase string
			warn  bool
		}{{"Running", false}, {"Pending", true}, {"Failed", true}, {"Unknown", true}, {"Succeeded", false}} {
			s := byDim(series, "phase", p.phase)
			cells := latestMax(s)
			cells[0] = warnIf(p.warn && positive(s), cells[0])
			labels, rows = append(labels, p.phase), append(rows, cells)
		}
		return block(valuePairs(labels, rows, true))
	})
}

func controlPlaneSection(m azure.Monitoring) section {
	return partSection("Control plane", m.Cluster.Err, func() []string {
		cpu := seriesOf(m.Cluster.Data, "apiserver_cpu_usage_percentage")
		mem := seriesOf(m.Cluster.Data, "apiserver_memory_usage_percentage")
		etcd := seriesOf(m.Cluster.Data, "etcd_database_usage_percentage")
		if !hasData(slices.Concat(cpu, mem, etcd)...) {
			return []string{"no data"}
		}
		pairs := valuePairs([]string{"API server CPU", "API server memory"},
			[][]string{avgMax(firstSeries(cpu)), avgMax(firstSeries(mem))}, false)
		return block(append(pairs, pair{"etcd database", percent(latest(firstSeries(etcd)))}))
	})
}

// autoscaling reports whether a node pool of the cluster has autoscaling on.
func autoscaling(row map[string]any) bool {
	return slices.ContainsFunc(pools(dig(row, "properties")), func(p map[string]any) bool {
		on, _ := flag(p, "enableAutoScaling")
		return on
	})
}

func autoscalerSection(m azure.Monitoring) section {
	return partSection("Autoscaler", m.Cluster.Err, func() []string {
		metric := func(name string) azure.Series { return firstSeries(seriesOf(m.Cluster.Data, name)) }
		pairs := valuePairs([]string{"Unneeded nodes", "Unschedulable pods"}, [][]string{
			latestMax(metric("cluster_autoscaler_unneeded_nodes_count")),
			latestMax(metric("cluster_autoscaler_unschedulable_pods_count")),
		}, true)
		return block(append(pairs,
			pair{"Safe to scale", yesNo(latest(metric("cluster_autoscaler_cluster_safe_to_autoscale")))},
			pair{"Scale-down cooldown", yesNo(latest(metric("cluster_autoscaler_scale_down_in_cooldown")))},
		))
	})
}

func alertsSection(m azure.Monitoring) section {
	rules := pair{"Rules", ""}
	if m.Rules.Err != nil {
		rules.value = failedLine(m.Rules.Err)
	} else {
		rules.value = ruleSummary(m.Rules.Data)
	}
	pairs := []pair{rules}
	if m.Alerts.Err != nil {
		pairs = append(pairs, pair{"Firing", failedLine(m.Alerts.Err)})
		return section{title: "Alerts", lines: block(pairs)}
	}
	var firing [][]string
	resolved := 0
	for _, a := range m.Alerts.Data {
		if !strings.EqualFold(a.Condition, "Fired") {
			resolved++
			continue
		}
		// The rule goes last, so a long name cannot push the time out of the box.
		firing = append(firing, []string{severityText(a.Severity), "since " + a.Started.Local().Format("Jan 2 15:04"), a.Rule})
	}
	lines := aligned(firing, false)
	if len(lines) == 0 {
		lines = []string{"none"}
	}
	pairs = append(pairs, listPairs("Firing", lines)...)
	if resolved > 0 {
		pairs = append(pairs, pair{"Resolved", fmt.Sprintf("%d in the last 30 days", resolved)})
	}
	return section{title: "Alerts", lines: block(pairs)}
}

// ruleSummary is the count of rules, by kind, and how many are disabled.
func ruleSummary(rules []azure.AlertRule) string {
	if len(rules) == 0 {
		return "none"
	}
	var kinds []string
	for _, kind := range []string{"metric", "log", "activity log", "Prometheus"} {
		if n := countRules(rules, func(r azure.AlertRule) bool { return r.Kind == kind }); n > 0 {
			kinds = append(kinds, fmt.Sprintf("%d %s", n, kind))
		}
	}
	out := fmt.Sprintf("%d: %s", len(rules), strings.Join(kinds, ", "))
	if n := countRules(rules, func(r azure.AlertRule) bool { return !r.Enabled }); n > 0 {
		out += fmt.Sprintf("; %d disabled", n)
	}
	return out
}

func countRules(rules []azure.AlertRule, match func(azure.AlertRule) bool) int {
	n := 0
	for _, r := range rules {
		if match(r) {
			n++
		}
	}
	return n
}

// severityText draws Sev0 and Sev1 as errors and the other severities as warnings.
func severityText(severity string) string {
	if severity == "Sev0" || severity == "Sev1" {
		return errorStyle.Render(severity)
	}
	return warnStyle.Render(severity)
}

// seriesOf returns the series of the metric with the name.
func seriesOf(metrics []azure.Metric, name string) []azure.Series {
	for _, m := range metrics {
		if m.Name == name {
			return m.Series
		}
	}
	return nil
}

// allSeries returns the series of every metric.
func allSeries(metrics []azure.Metric) []azure.Series {
	var out []azure.Series
	for _, m := range metrics {
		out = append(out, m.Series...)
	}
	return out
}

// firstSeries is the first series of a metric without dimensions; with none it has no points.
func firstSeries(series []azure.Series) azure.Series {
	if len(series) == 0 {
		return azure.Series{}
	}
	return series[0]
}

// byDim is the series whose dimension has the value, ignoring case; with none it has no points.
func byDim(series []azure.Series, dim, value string) azure.Series {
	for _, s := range series {
		if strings.EqualFold(s.Dims[dim], value) {
			return s
		}
	}
	return azure.Series{}
}

// dimValues lists the values of a dimension, sorted, each once.
func dimValues(series []azure.Series, dim string) []string {
	set := map[string]bool{}
	for _, s := range series {
		if v, ok := s.Dims[dim]; ok {
			set[v] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// hasData reports whether any series has a finite point.
func hasData(series ...azure.Series) bool {
	return slices.ContainsFunc(series, func(s azure.Series) bool {
		return slices.ContainsFunc(s.Points, func(p azure.Point) bool { return finite(p.Average) || finite(p.Maximum) })
	})
}

// latest is the last finite average.
func latest(s azure.Series) (float64, bool) {
	for _, p := range slices.Backward(s.Points) {
		if finite(p.Average) {
			return p.Average, true
		}
	}
	return 0, false
}

// meanOf is the mean of the finite averages.
func meanOf(s azure.Series) (float64, bool) {
	sum, n := 0.0, 0
	for _, p := range s.Points {
		if finite(p.Average) {
			sum, n = sum+p.Average, n+1
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// peak is the largest finite maximum.
func peak(s azure.Series) (float64, bool) {
	best, ok := 0.0, false
	for _, p := range s.Points {
		if finite(p.Maximum) && (!ok || p.Maximum > best) {
			best, ok = p.Maximum, true
		}
	}
	return best, ok
}

// positive reports whether the latest average rounds above 0.
func positive(s azure.Series) bool {
	v, ok := latest(s)
	return ok && math.Round(v) > 0
}

func rounded(v float64, ok bool) string {
	if !ok {
		return dash
	}
	return strconv.FormatFloat(math.Round(v), 'f', 0, 64)
}

func percent(v float64, ok bool) string {
	if !ok {
		return dash
	}
	return rounded(v, ok) + "%"
}

// latestMax is the cells "12" and "max 14": the latest average and the largest maximum.
func latestMax(s azure.Series) []string {
	v, ok := latest(s)
	if !ok {
		return []string{dash}
	}
	return []string{rounded(v, ok), "max " + rounded(peak(s))}
}

// avgMax is the cells "avg 3%" and "max 9%": the mean of the averages and the largest maximum.
func avgMax(s azure.Series) []string {
	mean, okMean := meanOf(s)
	top, okTop := peak(s)
	if !okMean && !okTop {
		return []string{dash}
	}
	return []string{"avg " + percent(mean, okMean), "max " + percent(top, okTop)}
}

// aligned joins the cells of each row, padding every column to its widest
// cell; right aligns the first column to the right. The padding is outside
// the cells, so their styling covers only their text.
func aligned(rows [][]string, right bool) []string {
	var widths []int
	for _, r := range rows {
		for i, c := range r {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(c))
		}
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		var b strings.Builder
		for j, c := range r {
			pad := strings.Repeat(" ", widths[j]-lipgloss.Width(c))
			switch {
			case j == 0 && right:
				b.WriteString(pad + c)
			case j < len(r)-1:
				b.WriteString(c + pad)
			default:
				b.WriteString(c)
			}
			if j < len(r)-1 {
				b.WriteString("  ")
			}
		}
		out[i] = b.String()
	}
	return out
}

// valuePairs is the labels with their rows of cells aligned in columns.
func valuePairs(labels []string, rows [][]string, right bool) []pair {
	lines := aligned(rows, right)
	pairs := make([]pair, len(labels))
	for i, l := range labels {
		pairs[i] = pair{l, lines[i]}
	}
	return pairs
}

func yesNo(v float64, ok bool) string {
	if !ok {
		return dash
	}
	return choose(math.Round(v) == 1, "yes", "no")
}

func coreText(v float64, ok bool) string {
	if !ok {
		return dash
	}
	v = math.Round(v*10) / 10
	return strconv.FormatFloat(v, 'f', -1, 64) + choose(v == 1, " core", " cores")
}

func memoryText(v float64, ok bool) string {
	if !ok {
		return dash
	}
	return strconv.FormatFloat(v/gib, 'f', 1, 64) + " GiB"
}

// warnIf draws s as a warning when warn is set.
func warnIf(warn bool, s string) string {
	if warn {
		return warnStyle.Render(s)
	}
	return s
}

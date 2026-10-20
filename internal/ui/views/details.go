package views

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

const (
	// margin is the empty cells left and right of the tab bar and the body.
	margin = 1
	// tabGap is the number of cells between two tabs.
	tabGap = 1
	// detailsHeader is the tab bar, its rule and the blank line for the refresh note.
	detailsHeader = 3
)

type details struct {
	loader
	title string
	spec  Spec
	// tab is the requested tab; current resolves it against the loaded type.
	tab           string
	row           map[string]any
	period        string // one of azure.Periods, for the tabs that load data
	keyed         *keyedTab
	top           int // first visible body line
	width, height int
}

// keyedTab is the data the current tab loads, when it has a key.
type keyedTab struct {
	key data.Key
	res tabData
	// refreshing is set from "r" until the next result for key.
	refreshing bool
}

// NewDetails shows one resource as tabs, opened on the first one.
func NewDetails(subscriptionID string, r azure.Resource) View {
	return NewDetailsAt(subscriptionID, r, "")
}

// NewDetailsAt is NewDetails opened on the named tab; a tab the resource
// type does not have opens the first one.
func NewDetailsAt(subscriptionID string, r azure.Resource, tab string) View {
	return details{
		loader: newLoader(data.Key{Kind: data.Details, Subscription: subscriptionID, ID: r.ID}, "details"),
		title:  r.Name,
		spec:   Spec{Kind: "details", Subscription: subscriptionID, ResourceID: r.ID, ResourceName: r.Name},
		tab:    tab,
		period: azure.Periods[0],
	}
}

func (d details) Title() string { return d.title }

func (d details) Init() tea.Cmd { return d.init() }

func (d details) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case data.ResultMsg:
		if k := d.keyed; k != nil && msg.Key == k.key {
			d.keyed = &keyedTab{key: k.key, res: tabData{data: msg.Data, err: msg.Err, period: k.res.period}}
			return d.scroll(d.top), nil
		}
		wasReady := d.ready()
		res, ok := d.result(msg)
		if !ok {
			return d, nil
		}
		if d.err == nil {
			d.row, d.err = parseRow(res)
		}
		var cmd tea.Cmd
		if !wasReady && d.ready() {
			d, cmd = d.loadTab()
		}
		return d.scroll(d.top), cmd
	case spinner.TickMsg:
		cmd := d.tick(msg)
		return d, cmd
	case tea.KeyPressMsg:
		return d.press(msg.String())
	}
	return d, nil
}

// parseRow takes a result as a row; it fails when the row does not encode as JSON.
func parseRow(res any) (map[string]any, error) {
	_, err := indentJSON(res)
	row, _ := res.(map[string]any)
	return row, err
}

func (d details) press(k string) (View, tea.Cmd) {
	if k == "r" {
		cmd := d.refresh()
		if d.keyed != nil {
			keyed := *d.keyed
			keyed.refreshing = true
			d.keyed = &keyed
			cmd = tea.Batch(cmd, data.Refresh(d.keyed.key))
		}
		return d, cmd
	}
	if !d.ready() {
		return d, nil
	}
	switch k {
	case "tab", "]":
		return d.step(1)
	case "shift+tab", "[":
		return d.step(-1)
	case "p":
		return d.nextPeriod()
	}
	return d.scroll(d.scrollTarget(k)), nil
}

// scrollTarget is the top line the scroll key k asks for.
func (d details) scrollTarget(k string) int {
	page := max(d.bodyHeight(), 1)
	switch k {
	case "up", "k":
		return d.top - 1
	case "down", "j":
		return d.top + 1
	case "pgup", "b":
		return d.top - page
	case "pgdown", "f":
		return d.top + page
	case "home", "g":
		return 0
	case "end", "G":
		return len(d.body())
	}
	return d.top
}

// scroll moves the top line, keeping the body filling the pane.
func (d details) scroll(top int) details {
	d.top = min(max(top, 0), max(len(d.body())-d.bodyHeight(), 0))
	return d
}

func (d details) bodyHeight() int { return max(d.height-detailsHeader, 0) }

// contentWidth is the width of the tab bar and the body between the margins.
func (d details) contentWidth() int { return max(d.width-2*margin, 0) }

// defs is the tabs of the loaded row's type; nil before the data is ready.
func (d details) defs() []tabDef {
	if !d.ready() {
		return nil
	}
	return tabDefs(str(d.row, "type"))
}

// index is the position of the current tab; the first when the type lacks it.
func (d details) index() int {
	for i, def := range d.defs() {
		if strings.EqualFold(def.name, d.tab) {
			return i
		}
	}
	return 0
}

// current is the name of the current tab.
func (d details) current() string {
	if defs := d.defs(); len(defs) > 0 {
		return defs[d.index()].name
	}
	return d.tab
}

// selectTab opens the named tab at its top; a tab that was not current starts loading its data.
func (d details) selectTab(name string) (details, tea.Cmd) {
	changed := !strings.EqualFold(name, d.current())
	d.tab = name
	var cmd tea.Cmd
	if changed {
		d, cmd = d.loadTab()
	}
	return d.scroll(0), cmd
}

// step moves to the next (delta 1) or previous (delta -1) tab, wrapping.
func (d details) step(delta int) (details, tea.Cmd) {
	defs := d.defs()
	return d.selectTab(defs[(d.index()+delta+len(defs))%len(defs)].name)
}

// nextPeriod steps the period of a tab that loads data; other tabs ignore it.
func (d details) nextPeriod() (details, tea.Cmd) {
	if d.keyed == nil {
		return d, nil
	}
	d.period = azure.NextPeriod(d.period)
	d, cmd := d.loadTab()
	return d.scroll(d.top), cmd
}

// loadTab asks for the data of the current tab and shows it as loading; a tab
// without a key keeps none.
func (d details) loadTab() (details, tea.Cmd) {
	def := d.defs()[d.index()]
	if def.key == nil {
		d.keyed = nil
		return d, nil
	}
	key := def.key(d.row, d.period)
	d.keyed = &keyedTab{key: key, res: tabData{loading: true, period: d.period}}
	return d, data.Fetch(key)
}

// body is the lines of the current tab, each cut to the content width.
func (d details) body() []string {
	defs := d.defs()
	if len(defs) == 0 {
		return nil
	}
	width := d.contentWidth()
	def := defs[d.index()]
	var out []string
	if def.draw != nil {
		out = def.draw(d.row, d.keyed.res, width)
	} else {
		out = def.render(d.row, width)
	}
	for i, l := range out {
		out[i] = cut(l, width)
	}
	return out
}

func (d details) Resize(width, height int) View {
	d.width, d.height = width, height
	return d.scroll(d.top)
}

func (d details) Render() string {
	if d.width <= 0 || d.height <= 0 {
		return ""
	}
	if s, ok := d.status(d.width, d.height); ok {
		return s
	}
	note := ""
	if d.refreshing || (d.keyed != nil && d.keyed.refreshing) {
		note = cut(faintStyle.Render(refreshingText), d.contentWidth())
	}
	body := d.body()
	out := []string{d.tabBar(), d.rule(), note}
	out = append(out, body[d.top:min(d.top+d.bodyHeight(), len(body))]...)
	pad := strings.Repeat(" ", margin)
	for i, l := range out {
		out[i] = pad + l
	}
	return clip(strings.Join(out, "\n"), d.width, d.height)
}

// tabSpan is the cells a tab or a scroll marker takes on the tab bar, a tab's
// padding included. A marker has mark set; name is the tab a click selects.
type tabSpan struct {
	name, mark string
	start, end int
}

// spans places the tabs from the left edge of the content. When they do not
// fit, it places a window of whole tabs that holds the current one, with a
// marker for the tabs hidden on each side.
func (d details) spans() []tabSpan {
	defs, width, cur := d.defs(), d.contentWidth(), d.index()
	if len(defs) == 0 {
		return nil
	}
	size := func(i int) int { return lipgloss.Width(defs[i].name) + 2 }
	marker := func(mark string, x, i int) tabSpan { return tabSpan{defs[i].name, mark, x, x + 1} }
	// window starts at tab first and takes the tabs after the current one that fit.
	window := func(first int) ([]tabSpan, bool) {
		var out []tabSpan
		x, last := 0, first-1
		if first > 0 {
			out, x = append(out, marker("‹", 0, first-1)), 1+tabGap
		}
		for i := first; i < len(defs); i++ {
			end := x + size(i)
			if i < len(defs)-1 {
				end += tabGap + 1 // room for ›
			}
			if end > width {
				if i <= cur {
					return nil, false
				}
				break
			}
			out = append(out, tabSpan{defs[i].name, "", x, x + size(i)})
			x, last = x+size(i)+tabGap, i
		}
		if last < len(defs)-1 {
			out = append(out, marker("›", x, last+1))
		}
		return out, true
	}
	for first := 0; first <= cur; first++ {
		if out, ok := window(first); ok {
			return out
		}
	}
	out := []tabSpan{{defs[cur].name, "", 0, size(cur)}}
	if x := size(cur) + tabGap; cur < len(defs)-1 && x+1 <= width {
		out = append(out, marker("›", x, cur+1))
	}
	return out
}

func (d details) tabBar() string {
	var b strings.Builder
	pos, cur := 0, d.current()
	for _, s := range d.spans() {
		b.WriteString(strings.Repeat(" ", s.start-pos))
		pos = s.end
		switch {
		case s.mark != "":
			b.WriteString(mutedStyle.Render(s.mark))
		case s.name == cur:
			b.WriteString(titleStyle.Render(" " + s.name + " "))
		default:
			b.WriteString(mutedStyle.Render(" " + s.name + " "))
		}
	}
	return cut(b.String(), d.contentWidth())
}

// rule is a line under the tab bar: heavy under the current tab, light elsewhere.
func (d details) rule() string {
	width := d.contentWidth()
	var start, end int
	cur := d.current()
	for _, s := range d.spans() {
		if s.name == cur {
			start, end = min(s.start, width), min(s.end, width)
		}
	}
	return mutedStyle.Render(strings.Repeat("─", start)) +
		accentStyle.Render(strings.Repeat("━", end-start)) +
		mutedStyle.Render(strings.Repeat("─", width-end))
}

// tabAt returns the tab a click at column x of the content selects.
func (d details) tabAt(x int) (string, bool) {
	for _, s := range d.spans() {
		if x >= s.start && x < s.end {
			return s.name, true
		}
	}
	return "", false
}

// Click selects the tab under x on the tab bar or its rule; a marker selects the nearest hidden tab.
func (d details) Click(x, y int) (View, tea.Cmd, bool) {
	if y != 0 && y != 1 {
		return d, nil, false
	}
	x -= margin
	name, ok := d.tabAt(x)
	if !ok || x >= d.contentWidth() {
		return d, nil, false
	}
	next, cmd := d.selectTab(name)
	return next, cmd, true
}

// Spec keeps the current tab unless it is the first.
func (d details) Spec() Spec {
	s := d.spec
	if name := d.current(); !strings.EqualFold(name, overviewTab) {
		s.Tab = name
	}
	return s
}

func (details) Open() (View, bool) { return nil, false }

func (d details) Selected() (Item, bool) {
	return Item{ID: d.spec.ResourceID, Name: d.spec.ResourceName}, true
}

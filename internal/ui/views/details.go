package views

import (
	"bytes"
	"encoding/json"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

// tabGap is the number of cells between two tab names.
const tabGap = 2

type details struct {
	loader
	title string
	spec  Spec
	// tab is the requested tab; current resolves it against the loaded type.
	tab           string
	row           map[string]any
	top           int // first visible body line
	width, height int
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
	}
}

func (d details) Title() string { return d.title }

func (d details) Init() tea.Cmd { return d.init() }

func (d details) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case data.ResultMsg:
		res, ok := d.result(msg)
		if !ok {
			return d, nil
		}
		if d.err == nil {
			d.row, d.err = parseRow(res)
		}
		return d.scroll(d.top), nil
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
		return d, cmd
	}
	if !d.ready() {
		return d, nil
	}
	switch k {
	case "tab", "]":
		return d.step(1), nil
	case "shift+tab", "[":
		return d.step(-1), nil
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

func (d details) bodyHeight() int { return max(d.height-1-lines(d.refreshing), 0) }

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

func (d details) selectTab(name string) details {
	d.tab = name
	return d.scroll(0)
}

// step moves to the next (delta 1) or previous (delta -1) tab, wrapping.
func (d details) step(delta int) details {
	defs := d.defs()
	return d.selectTab(defs[(d.index()+delta+len(defs))%len(defs)].name)
}

// body is the lines of the current tab.
func (d details) body() []string {
	defs := d.defs()
	if len(defs) == 0 {
		return nil
	}
	return defs[d.index()].render(d.row, d.width)
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
	out := []string{d.tabBar()}
	if d.refreshing {
		out = append(out, faintStyle.Render(refreshingText))
	}
	body := d.body()
	out = append(out, body[d.top:min(d.top+d.bodyHeight(), len(body))]...)
	return clip(strings.Join(out, "\n"), d.width, d.height)
}

func (d details) tabBar() string {
	defs := d.defs()
	names := make([]string, len(defs))
	for i, def := range defs {
		names[i] = def.name
		if i == d.index() {
			names[i] = lipgloss.NewStyle().Reverse(true).Render(def.name)
		}
	}
	return strings.Join(names, strings.Repeat(" ", tabGap))
}

// tabAt returns the tab drawn at column x of the tab bar.
func (d details) tabAt(x int) (string, bool) {
	start := 0
	for _, def := range d.defs() {
		end := start + lipgloss.Width(def.name)
		if x >= start && x < end {
			return def.name, true
		}
		start = end + tabGap
	}
	return "", false
}

// Click selects the tab under x on the tab bar.
func (d details) Click(x, y int) (View, tea.Cmd, bool) {
	if y != 0 {
		return d, nil, false
	}
	name, ok := d.tabAt(x)
	if !ok {
		return d, nil, false
	}
	return d.selectTab(name), nil, true
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

// indentJSON encodes v with two-space indents and keeps &, < and > as they are.
func indentJSON(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n"), err
}

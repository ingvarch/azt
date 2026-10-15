// Package ui is the Bubble Tea interface of azt.
package ui

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/views"
)

const (
	minPaneWidth  = 12
	minPaneHeight = 4
	resizeStep    = 0.05

	normalHints = "^w v/s split  ^w h/j/k/l focus  ^w q close  ^w z zoom  ^w t tab  q quit"
	prefixHints = "v/s split  h/j/k/l focus  q close  z zoom  = equal  <>+- resize  t tab  n/p/1-9 switch  esc cancel"
)

var (
	focusColor   = lipgloss.Color("12")
	borderColor  = lipgloss.Color("8")
	reverseStyle = lipgloss.NewStyle().Reverse(true)
	hintStyle    = lipgloss.NewStyle().Faint(true)
	border       = lipgloss.NormalBorder()
)

type tab struct {
	root   *layout.Node
	focus  layout.PaneID
	zoomed bool
}

// Model is the root model: tabs of split panes, each pane holding a view.
type Model struct {
	newView       func() views.View
	tabs          []tab
	active        int
	views         map[layout.PaneID]views.View
	nextID        layout.PaneID
	width, height int
	prefix        bool
}

func New(newView func() views.View) Model {
	return Model{
		newView: newView,
		tabs:    []tab{{root: layout.Leaf(0), focus: 0}},
		views:   map[layout.PaneID]views.View{0: newView()},
		nextID:  1,
	}
}

func (m Model) Init() tea.Cmd { return m.views[0].Init() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.key(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeViews()
		return m, nil
	}
	return m, m.broadcast(msg)
}

func (m Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.prefix {
		m.prefix = false
		return m.runPrefix(k)
	}
	switch k {
	case "q":
		return m, tea.Quit
	case "ctrl+w":
		m.prefix = true
		return m, nil
	}
	return m, m.send(m.tabs[m.active].focus, msg)
}

// send passes msg to one view.
func (m *Model) send(id layout.PaneID, msg tea.Msg) tea.Cmd {
	v, cmd := m.views[id].Update(msg)
	m.views[id] = v
	return cmd
}

// broadcast passes msg to every view of every tab.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range m.tabs {
		for _, id := range t.root.Panes() {
			cmds = append(cmds, m.send(id, msg))
		}
	}
	return tea.Batch(cmds...)
}

// runPrefix runs the action of key k after ctrl+w.
func (m Model) runPrefix(k string) (tea.Model, tea.Cmd) {
	t := &m.tabs[m.active]
	switch k {
	case "v", "s", "h", "j", "k", "l", "q", "=", "<", ">", "+", "-":
		t.zoomed = false
	}

	var cmd tea.Cmd
	switch k {
	case "v":
		cmd = m.split(t, layout.Horizontal)
	case "s":
		cmd = m.split(t, layout.Vertical)
	case "h":
		m.moveFocus(t, layout.Left)
	case "j":
		m.moveFocus(t, layout.Down)
	case "k":
		m.moveFocus(t, layout.Up)
	case "l":
		m.moveFocus(t, layout.Right)
	case "q":
		m.closePane(t)
	case "z":
		t.zoomed = !t.zoomed
	case "=":
		t.root = t.root.Equalize()
	case ">":
		t.root = t.root.Resize(t.focus, layout.Horizontal, resizeStep)
	case "<":
		t.root = t.root.Resize(t.focus, layout.Horizontal, -resizeStep)
	case "+":
		t.root = t.root.Resize(t.focus, layout.Vertical, resizeStep)
	case "-":
		t.root = t.root.Resize(t.focus, layout.Vertical, -resizeStep)
	case "t":
		cmd = m.newTab()
	case "n":
		m.active = (m.active + 1) % len(m.tabs)
	case "p":
		m.active = (m.active + len(m.tabs) - 1) % len(m.tabs)
	default:
		if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(m.tabs) {
			m.active = n - 1
		}
	}
	m.resizeViews()
	return m, cmd
}

func (m Model) paneArea() layout.Rect {
	return layout.Rect{X: 0, Y: 1, W: m.width, H: max(m.height-2, 0)}
}

// visible returns the rectangles of the panes drawn in the active tab.
func (m Model) visible() map[layout.PaneID]layout.Rect {
	t := m.tabs[m.active]
	if t.zoomed {
		return map[layout.PaneID]layout.Rect{t.focus: m.paneArea()}
	}
	return t.root.Rects(m.paneArea())
}

func (m *Model) resizeViews() {
	for id, r := range m.visible() {
		m.views[id] = m.views[id].Resize(max(r.W-2, 0), max(r.H-2, 0))
	}
}

func (m *Model) split(t *tab, o layout.Orientation) tea.Cmd {
	added := m.nextID
	root := t.root.SplitPane(t.focus, added, o)
	for _, r := range root.Rects(m.paneArea()) {
		if r.W < minPaneWidth || r.H < minPaneHeight {
			return nil
		}
	}
	m.nextID++
	t.root, t.focus = root, added
	m.views[added] = m.newView()
	return m.views[added].Init()
}

func (m *Model) moveFocus(t *tab, d layout.Direction) {
	if id, ok := t.root.Neighbor(t.focus, d, m.paneArea()); ok {
		t.focus = id
	}
}

func (m *Model) closePane(t *tab) {
	if len(t.root.Panes()) == 1 {
		m.closeTab()
		return
	}
	closed := t.root.Rects(m.paneArea())[t.focus]
	t.root = t.root.Close(t.focus)
	delete(m.views, t.focus)
	t.focus = t.root.Panes()[0]
	for id, r := range t.root.Rects(m.paneArea()) {
		if r.X <= closed.X && closed.X < r.X+r.W && r.Y <= closed.Y && closed.Y < r.Y+r.H {
			t.focus = id
		}
	}
}

// closeTab removes the active tab and its views; the last tab stays.
func (m *Model) closeTab() {
	if len(m.tabs) == 1 {
		return
	}
	for _, id := range m.tabs[m.active].root.Panes() {
		delete(m.views, id)
	}
	m.tabs = slices.Delete(m.tabs, m.active, m.active+1)
	m.active = max(m.active-1, 0)
}

func (m *Model) newTab() tea.Cmd {
	id := m.nextID
	m.nextID++
	m.views[id] = m.newView()
	m.tabs = slices.Insert(m.tabs, m.active+1, tab{root: layout.Leaf(id), focus: id})
	m.active++
	return m.views[id].Init()
}

func (m Model) View() tea.View {
	var content string
	if m.width > 0 && m.height > 0 {
		content = m.frame()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) frame() string {
	comp := lipgloss.NewCompositor(
		lipgloss.NewLayer(m.tabBar()),
		lipgloss.NewLayer(m.statusBar()).Y(m.height-1),
	)
	focus := m.tabs[m.active].focus
	for id, r := range m.visible() {
		if r.W < 2 || r.H < 2 {
			continue
		}
		pane := renderPane(m.views[id], r, id == focus)
		comp.AddLayers(lipgloss.NewLayer(pane).X(r.X).Y(r.Y))
	}
	return lipgloss.NewCanvas(m.width, m.height).Compose(comp).Render()
}

func (m Model) tabBar() string {
	var b strings.Builder
	for i, t := range m.tabs {
		label := " " + strconv.Itoa(i+1) + " " + m.views[t.focus].Title() + " "
		if i == m.active {
			label = reverseStyle.Render(label)
		}
		b.WriteString(label)
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(b.String())
}

func (m Model) statusBar() string {
	badge, hints := " NORMAL ", normalHints
	if m.prefix {
		badge, hints = " CTRL-W ", prefixHints
	}
	title := m.views[m.tabs[m.active].focus].Title()
	bar := reverseStyle.Render(badge) + " " + title + "  " + hintStyle.Render(hints)
	return lipgloss.NewStyle().MaxWidth(m.width).Render(bar)
}

// renderPane draws v inside a border, with its title in the top border.
// The result is exactly r.W x r.H cells; r must be at least 2x2.
func renderPane(v views.View, r layout.Rect, focused bool) string {
	color := borderColor
	title := lipgloss.NewStyle()
	if focused {
		color = focusColor
		title = title.Foreground(focusColor).Bold(true)
	}
	edge := lipgloss.NewStyle().Foreground(color)

	inner := r.W - 2
	label := ""
	if inner > 0 {
		label = title.MaxWidth(inner).Render(" " + v.Title() + " ")
	}
	top := edge.Render(border.TopLeft) + label +
		edge.Render(strings.Repeat(border.Top, inner-lipgloss.Width(label))+border.TopRight)
	bottom := edge.Render(border.BottomLeft + strings.Repeat(border.Bottom, inner) + border.BottomRight)

	body := strings.Split(v.Render(), "\n")
	fit := lipgloss.NewStyle().MaxWidth(inner)
	side := edge.Render(border.Left)
	lines := []string{top}
	for i := range r.H - 2 {
		line := ""
		if i < len(body) && inner > 0 {
			line = fit.Render(body[i])
		}
		line += strings.Repeat(" ", inner-lipgloss.Width(line))
		lines = append(lines, side+line+edge.Render(border.Right))
	}
	return strings.Join(append(lines, bottom), "\n")
}

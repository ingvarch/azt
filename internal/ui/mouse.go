package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/sidebar"
	"github.com/ingvarch/azt/internal/ui/views"
)

const doublePressWindow = 400 * time.Millisecond

// target is the thing under the pointer: a pane or the sidebar.
type target struct {
	pane    layout.PaneID
	sidebar bool
}

// drag is a split, or the explorer, whose boundary follows the pointer.
type drag struct {
	path    layout.Path
	orient  layout.Orientation
	sidebar bool
}

// lastPress is the previous press inside a target, kept to spot a double press.
type lastPress struct {
	target
	at  time.Time
	row int
}

// mouse handles a mouse message in any mode.
func (m Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mo := msg.Mouse()
	if _, ok := msg.(tea.MouseReleaseMsg); ok {
		m.drag = nil
	}
	switch {
	case m.mode == modeHelp || m.mode == modePrefix:
		if _, ok := msg.(tea.MouseClickMsg); ok {
			m.mode = modeNormal
		}
		return m, nil
	case m.mode == modeTabDialog:
		var cmd tea.Cmd
		if _, ok := msg.(tea.MouseClickMsg); ok && mo.Button == tea.MouseLeft {
			cmd = m.dialogPress(mo.X, mo.Y)
		}
		return m, cmd
	case m.typing() || m.mode == modePalette:
		return m, nil
	}
	var cmd tea.Cmd
	switch msg.(type) {
	case tea.MouseClickMsg:
		if mo.Button == tea.MouseLeft {
			cmd = m.leftPress(mo.X, mo.Y)
		}
	case tea.MouseMotionMsg:
		if mo.Button == tea.MouseLeft && m.drag != nil {
			m.dragTo(mo.X, mo.Y)
		}
	case tea.MouseWheelMsg:
		cmd = m.wheel(mo)
	}
	return m, cmd
}

// leftPress handles a left press in normal mode.
func (m *Model) leftPress(x, y int) tea.Cmd {
	m.drag = nil
	prev := m.last
	m.last = nil
	if i, ok := m.tabAt(x, y); ok {
		if i == len(m.tabs) {
			m.openTabDialog(true)
			return nil
		}
		m.active = i
		m.resizeViews()
		return nil
	}
	t := &m.tabs[m.active]
	if !t.zoomed {
		if p, ok := t.root.Boundary(m.paneArea(), x, y); ok {
			n, _ := t.root.SplitAt(p)
			m.drag = &drag{path: p, orient: n.Split}
			return nil
		}
	}
	if m.onSidebarBorder(x, y) {
		m.drag = &drag{sidebar: true}
		return nil
	}
	tg, r, ok := m.hit(x, y)
	if !ok {
		return nil
	}
	if tg.sidebar {
		m.focusSidebar(true)
	} else {
		t.focus = tg.pane
		m.focusSidebar(false)
	}
	return m.pressIn(prev, tg, r, x, y)
}

// pressIn handles a press at (x, y) inside tg, drawn in r, after the press
// prev. On the inner area a second press on the same row within the double
// press window acts as enter; any other press clicks the view and is kept
// as prev for the next one when it hit a row.
func (m *Model) pressIn(prev *lastPress, tg target, r layout.Rect, x, y int) tea.Cmd {
	ix, iy := x-r.X-1, y-r.Y-1
	if ix < 0 || iy < 0 || ix >= r.W-2 || iy >= r.H-2 {
		return nil
	}
	now := m.now()
	if prev != nil && prev.target == tg && prev.row == iy && now.Sub(prev.at) <= doublePressWindow && m.canOpen(tg) {
		if tg.sidebar {
			return m.openFromSidebar()
		}
		return m.open(tg.pane)
	}
	var cmd tea.Cmd
	var hit bool
	if tg.sidebar {
		var v views.View
		v, cmd, hit = m.sidebar.Click(ix, iy)
		m.sidebar = v.(sidebar.Model)
	} else {
		cmd, hit = m.clickTop(tg.pane, ix, iy)
	}
	if hit {
		m.last = &lastPress{target: tg, at: now, row: iy}
	}
	return cmd
}

// canOpen reports whether enter would open something in tg; a double press
// on a view with nothing to open is a second click.
func (m Model) canOpen(tg target) bool {
	if tg.sidebar {
		_, ok := m.sidebar.Stack()
		return ok
	}
	_, ok := m.top(tg.pane).Open()
	return ok
}

// onSidebarBorder reports whether (x, y) is on the explorer's right border
// column or the first pane's left border column, as for a split boundary.
func (m Model) onSidebarBorder(x, y int) bool {
	if !m.sidebarDrawn() {
		return false
	}
	area := m.paneArea()
	return y >= area.Y && y < area.Y+area.H && (x == area.X-1 || x == area.X)
}

// dragTo moves the dragged boundary to the pointer.
func (m *Model) dragTo(x, y int) {
	if m.drag.sidebar {
		m.setSidebarWidth(x + 1)
		m.resizeViews()
		return
	}
	t := &m.tabs[m.active]
	area, ok := t.root.SplitArea(m.drag.path, m.paneArea())
	if !ok {
		return
	}
	pos, origin, size := x, area.X, area.W
	if m.drag.orient == layout.Vertical {
		pos, origin, size = y, area.Y, area.H
	}
	if size <= 0 {
		return
	}
	// The half cell makes int(size*ratio) land exactly on pos.
	t.root = t.root.SetRatio(m.drag.path, (float64(pos-origin)+0.5)/float64(size))
	m.resizeViews()
}

// wheel scrolls the view under the pointer by one key press.
func (m *Model) wheel(mo tea.Mouse) tea.Cmd {
	var code rune
	switch mo.Button {
	case tea.MouseWheelUp:
		code = tea.KeyUp
	case tea.MouseWheelDown:
		code = tea.KeyDown
	default:
		return nil
	}
	tg, _, ok := m.hit(mo.X, mo.Y)
	if !ok {
		return nil
	}
	m.last = nil
	msg := tea.KeyPressMsg{Code: code}
	if tg.sidebar {
		return m.updateSidebar(msg)
	}
	return m.sendTop(tg.pane, msg)
}

// hit returns the pane or sidebar at (x, y) and its rectangle; false when
// nothing is there or the box is too small to be drawn.
func (m Model) hit(x, y int) (target, layout.Rect, bool) {
	for id, r := range m.visible() {
		if inside(r, x, y) {
			return target{pane: id}, r, drawn(r)
		}
	}
	if r := m.sidebarRect(); m.sidebarDrawn() && inside(r, x, y) {
		return target{sidebar: true}, r, drawn(r)
	}
	return target{}, layout.Rect{}, false
}

func drawn(r layout.Rect) bool { return r.W >= 2 && r.H >= 2 }

// tabAt returns the tab whose label is drawn at (x, y); len(m.tabs) is the plus button.
func (m Model) tabAt(x, y int) (int, bool) {
	if y != 0 {
		return 0, false
	}
	start := m.paneArea().X
	for i := range len(m.tabs) + 1 {
		label := plusLabel
		if i < len(m.tabs) {
			label = m.tabLabel(i)
		}
		end := start + lipgloss.Width(label)
		if x >= start && x < end {
			return i, true
		}
		start = end + lipgloss.Width(tabGap)
	}
	return 0, false
}

func inside(r layout.Rect, x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

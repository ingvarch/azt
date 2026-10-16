package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/layout"
)

const (
	dialogHints    = "enter save  ctrl-c clear  esc cancel"
	dialogMaxWidth = 56
	dialogHeight   = 5 // border, field, blank line, buttons, border
	dialogGap      = "  "
)

// dialogButtons are the buttons of the tab dialog from left to right; key is the key each one stands for.
var dialogButtons = [...]struct {
	label, key string
	primary    bool
}{
	{" ↵ save ", "enter", true},
	{" ctrl-c clear ", "ctrl+c", false},
	{" esc cancel ", "esc", false},
}

// openTabDialog opens the tab dialog: adding opens a tab on save, otherwise it renames the active tab.
func (m *Model) openTabDialog(adding bool) {
	m.newTabDialog = adding
	name := ""
	if !adding {
		name = m.tabs[m.active].name
	}
	m.startInput(modeTabDialog, name)
}

// dialogNumber is the number of the tab the dialog names: the next one, or the active one.
func (m Model) dialogNumber() int {
	if m.newTabDialog {
		return len(m.tabs) + 1
	}
	return m.active + 1
}

// dialogKey handles a key in the tab dialog; keys that are not a button's go to the field.
func (m Model) dialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	cmd, ok := m.dialogAct(msg.String())
	if !ok {
		cmd = m.editInput(msg)
	}
	return m, cmd
}

// dialogAct runs the button bound to key k; false when no button is.
func (m *Model) dialogAct(k string) (tea.Cmd, bool) {
	switch k {
	case "enter":
		return m.saveTabDialog(), true
	case "ctrl+c":
		m.input.SetValue("")
	case "esc":
		m.mode = modeNormal
	default:
		return nil, false
	}
	return nil, true
}

// saveTabDialog closes the dialog and applies the field. A new tab goes to the
// end and stays unnamed when the name is empty or its own number.
func (m *Model) saveTabDialog() tea.Cmd {
	name := strings.TrimSpace(m.input.Value())
	m.mode = modeNormal
	if !m.newTabDialog {
		m.tabs[m.active].name = name
		return nil
	}
	cmd := m.newTab(len(m.tabs))
	m.resizeViews()
	if name != strconv.Itoa(m.active+1) {
		m.tabs[m.active].name = name
	}
	return cmd
}

// dialogPress runs the button under (x, y); a press elsewhere does nothing.
func (m *Model) dialogPress(x, y int) tea.Cmd {
	r, ok := m.tabDialogRect()
	if !ok {
		return nil
	}
	for i, b := range buttonRects(r) {
		if inside(b, x, y) {
			cmd, _ := m.dialogAct(dialogButtons[i].key)
			return cmd
		}
	}
	return nil
}

// tabDialogRect is the dialog box; false when the window is too small for one.
func (m Model) tabDialogRect() (layout.Rect, bool) {
	return m.dialogRect(dialogMaxWidth, dialogHeight, buttonsWidth())
}

// buttonsWidth is the width of the button line.
func buttonsWidth() int {
	w := lipgloss.Width(dialogGap) * (len(dialogButtons) - 1)
	for _, b := range dialogButtons {
		w += lipgloss.Width(b.label)
	}
	return w
}

// buttonRects are the cells of the buttons in the dialog box r: centered on the last inner row.
func buttonRects(r layout.Rect) []layout.Rect {
	x := r.X + 1 + (r.W-2-buttonsWidth())/2
	rects := make([]layout.Rect, len(dialogButtons))
	for i, b := range dialogButtons {
		w := lipgloss.Width(b.label)
		rects[i] = layout.Rect{X: x, Y: r.Y + r.H - 2, W: w, H: 1}
		x += w + lipgloss.Width(dialogGap)
	}
	return rects
}

// tabDialogBox draws the dialog in r.
func (m Model) tabDialogBox(r layout.Rect) string {
	title := "Rename tab"
	if m.newTabDialog {
		title = "New tab"
	}
	inner := r.W - 2
	body := []string{m.dialogField(inner), "", m.dialogButtonLine(r)}
	return renderBox([]string{title}, "", strings.Join(body, "\n"), r, true)
}

// dialogField is the field line: a space and the input, as wide as the box.
// An empty field shows the faint number and the cursor after it.
func (m Model) dialogField(inner int) string {
	text := onBg(lipgloss.NewStyle(), m.tabShade)
	if m.input.Value() == "" {
		hint := strconv.Itoa(m.dialogNumber())
		pad := max(inner-2-lipgloss.Width(hint), 0)
		return text.Render(" ") + onBg(hintStyle, m.tabShade).Render(hint) +
			text.Reverse(true).Render(" ") + text.Render(strings.Repeat(" ", pad))
	}
	styles := m.input.Styles()
	styles.Focused.Text = text
	in := m.sizedInput(inner - 2) // the view is one cell wider than its width: the cursor
	in.SetStyles(styles)
	return text.Render(" ") + in.View()
}

// dialogButtonLine draws each button at its own rect in the box r.
func (m Model) dialogButtonLine(r layout.Rect) string {
	var b strings.Builder
	end := r.X + 1
	for i, rect := range buttonRects(r) {
		style := onBg(boldStyle, m.tabShade)
		if dialogButtons[i].primary {
			style = onBg(tabStyle, m.barShade)
		}
		b.WriteString(strings.Repeat(" ", rect.X-end))
		b.WriteString(style.Render(dialogButtons[i].label))
		end = rect.X + rect.W
	}
	return b.String()
}

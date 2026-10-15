package views

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// headerLines is the column titles plus the rule below them.
const headerLines = 2

type column struct {
	title string
	width int
}

// table draws rows under a header; cursor is -1 without rows.
type table struct {
	cols          []column
	rows          [][]string
	cursor        int
	top           int // first visible row
	width, height int
}

func newTable(cols []column) table { return table{cols: cols, cursor: -1} }

func (t *table) resize(width, height int) {
	t.width, t.height = width, height
	t.follow()
}

// setRows keeps the selection in range; it selects the first row when none was selected.
func (t *table) setRows(rows [][]string) {
	t.rows = rows
	t.selectRow(t.cursor)
}

// selectRow selects row i, clamped to the rows.
func (t *table) selectRow(i int) {
	t.cursor = min(max(i, 0), len(t.rows)-1)
	t.follow()
}

func (t *table) press(k tea.KeyPressMsg) {
	page := max(t.visible(), 1)
	switch k.String() {
	case "up", "k":
		t.selectRow(t.cursor - 1)
	case "down", "j":
		t.selectRow(t.cursor + 1)
	case "pgup", "b":
		t.selectRow(t.cursor - page)
	case "pgdown", "f":
		t.selectRow(t.cursor + page)
	case "home", "g":
		t.selectRow(0)
	case "end", "G":
		t.selectRow(len(t.rows) - 1)
	}
}

// visible is the number of rows that fit under the header.
func (t table) visible() int { return max(t.height-headerLines, 0) }

// follow scrolls so the selected row is visible and no blank rows show below the last.
func (t *table) follow() {
	vis := t.visible()
	t.top = max(min(t.top, len(t.rows)-vis, t.cursor), 0)
	t.top = max(t.top, t.cursor-vis+1)
}

// RowAt returns the row drawn on table line y; the header lines hold none.
func (t table) RowAt(y int) (int, bool) {
	i := t.top + y - headerLines
	if y < headerLines || y >= t.height || i >= len(t.rows) {
		return 0, false
	}
	return i, true
}

func (t table) view() string {
	if t.width <= 0 || t.height <= 0 {
		return ""
	}
	titles := make([]string, len(t.cols))
	widths := 0
	for i, c := range t.cols {
		titles[i] = c.title
		widths += c.width
	}
	rule := strings.Repeat("─", widths+max(len(t.cols)-1, 0))
	lines := []string{
		lipgloss.NewStyle().Bold(true).Render(t.line(titles)),
		cut(rule, t.width),
	}
	for i := t.top; i < min(t.top+t.visible(), len(t.rows)); i++ {
		text := t.line(t.rows[i])
		if i == t.cursor {
			text = lipgloss.NewStyle().Reverse(true).Render(text)
		}
		lines = append(lines, text)
	}
	return strings.Join(lines[:min(len(lines), t.height)], "\n")
}

// line joins the cells of one row, each fitted to its column, cut to the width.
func (t table) line(cells []string) string {
	fitted := make([]string, len(t.cols))
	for i, c := range t.cols {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		fitted[i] = fitCell(cell, c.width)
	}
	return cut(strings.Join(fitted, " "), t.width)
}

// fitCell pads s to width columns, or cuts it and ends it with "…".
func fitCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) > width {
		s = cut(s, width-1) + "…"
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

// cut keeps the first width columns of s on one line.
func cut(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(s)
}

package views

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// headerLines is the column titles plus the rule below them.
const headerLines = 2

type column struct {
	title string
	width int
	// state draws the cells of the column as states.
	state bool
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
		lipgloss.NewStyle().Bold(true).Render(t.line(titles, false)),
		cut(rule, t.width),
	}
	for i := t.top; i < min(t.top+t.visible(), len(t.rows)); i++ {
		text := t.line(t.rows[i], true)
		if i == t.cursor {
			text = reverse(text)
		}
		lines = append(lines, text)
	}
	return strings.Join(lines[:min(len(lines), t.height)], "\n")
}

// line is rowLine with a single space between the columns, cut to the width.
func (t table) line(cells []string, states bool) string {
	return cut(rowLine(t.cols, cells, " ", states), t.width)
}

// rowLine joins the cells of one row, each fitted to its column; with states
// the state columns are drawn as states.
func rowLine(cols []column, cells []string, sep string, states bool) string {
	fitted := make([]string, len(cols))
	for i, c := range cols {
		cell := cellAt(cells, i)
		if states && c.state {
			cell = stateText(cell)
		}
		fitted[i] = fitCell(cell, c.width)
	}
	return strings.Join(fitted, sep)
}

// cellAt is cell i of a row, empty past the last one.
func cellAt(cells []string, i int) string {
	if i < len(cells) {
		return cells[i]
	}
	return ""
}

// reverse draws s reversed, also behind the colors it has.
func reverse(s string) string {
	const on, off = "\x1b[7m", "\x1b[m"
	return on + strings.ReplaceAll(s, off, off+on) + off
}

// headerStyle draws the column titles of a summary table.
var headerStyle = lipgloss.NewStyle().Bold(true).Foreground(muted)

// summaryTable draws rows under their titles, each column as wide as its
// widest cell: no rule, no selection, two spaces between the columns, and no
// padding after the last one. A table wider than width cells gets its widest
// columns cut to one common width, see capWidths.
func summaryTable(cols []column, rows [][]string, width int) []string {
	cols = slices.Clone(cols)
	titles := make([]string, len(cols))
	for i := range cols {
		titles[i] = cols[i].title
		cols[i].width = lipgloss.Width(cols[i].title)
		for _, r := range rows {
			cols[i].width = max(cols[i].width, lipgloss.Width(cellAt(r, i)))
		}
	}
	capWidths(cols, width)
	lines := []string{headerStyle.Render(strings.TrimRight(rowLine(cols, titles, summaryGap, false), " "))}
	for _, r := range rows {
		lines = append(lines, strings.TrimRight(rowLine(cols, r, summaryGap, false), " "))
	}
	return lines
}

// summaryFloor is the narrowest width capWidths cuts a column to.
const summaryFloor = 8

// capWidths cuts the columns to the largest common width, at least
// summaryFloor, at which they and the gaps between them fit in width cells.
// Narrower columns keep their width.
func capWidths(cols []column, width int) {
	limit := 0
	for _, c := range cols {
		limit = max(limit, c.width)
	}
	for limit > summaryFloor && tableWidth(cols, limit) > width {
		limit--
	}
	for i := range cols {
		cols[i].width = min(cols[i].width, limit)
	}
}

// tableWidth is the width of the columns, each at most limit wide, and the gaps.
func tableWidth(cols []column, limit int) int {
	total := len(summaryGap) * max(len(cols)-1, 0)
	for _, c := range cols {
		total += min(c.width, limit)
	}
	return total
}

const summaryGap = "  "

// fitCell pads s to width columns, or cuts it and ends it with "…".
func fitCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ellipsize(s, width)
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

// cut keeps the first width columns of s on one line.
func cut(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(s)
}

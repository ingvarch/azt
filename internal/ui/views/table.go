package views

import (
	"math"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// headerLines is the column titles plus the rule below them.
const headerLines = 2

type column struct {
	title string
	// width is the content width, see sizeColumns; a table spreads the spare room over it.
	width int
	// state draws the cells of the column as states.
	state bool
	// fixed keeps the column at its content width: it takes no part of the spare room.
	fixed bool
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
	cols := t.layout()
	lines := []string{
		lipgloss.NewStyle().Bold(true).Render(t.line(cols, titles(cols), false)),
		cut(strings.Repeat("─", tableWidth(cols, math.MaxInt)), t.width),
	}
	for i := t.top; i < min(t.top+t.visible(), len(t.rows)); i++ {
		text := t.line(cols, t.rows[i], true)
		if i == t.cursor {
			text = selectedStyle.Render(ansi.Strip(text))
		}
		lines = append(lines, text)
	}
	return strings.Join(lines[:min(len(lines), t.height)], "\n")
}

// line is one row of cols cut at the table width.
func (t table) line(cols []column, cells []string, states bool) string {
	return cut(rowLine(cols, cells, summaryGap, states), t.width)
}

// titles are the column titles of cols.
func titles(cols []column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.title
	}
	return out
}

// layout is the columns at the widths they are drawn: the spare room of the
// table shared evenly by the columns that are not fixed, the first of them
// taking the remainder; a table that does not fit gets its widest columns cut,
// see capWidths.
func (t table) layout() []column {
	cols := slices.Clone(t.cols)
	if len(cols) == 0 {
		return cols
	}
	spare := t.width - tableWidth(cols, math.MaxInt)
	if spare < 0 {
		capWidths(cols, t.width)
		return cols
	}
	growing := 0
	for _, c := range cols {
		if !c.fixed {
			growing++
		}
	}
	n := 0
	for i := range cols {
		if cols[i].fixed {
			continue
		}
		cols[i].width += spare / growing
		if n < spare%growing {
			cols[i].width++
		}
		n++
	}
	return cols
}

// rowLine joins the cells of one row, each fitted to its column; with states
// the state columns are drawn as states.
func rowLine(cols []column, cells []string, sep string, states bool) string {
	fitted := make([]string, len(cols))
	for i, c := range cols {
		fitted[i] = fitCell(c.draw(cellAt(cells, i), states), c.width)
	}
	return strings.Join(fitted, sep)
}

// draw is cell as the column shows it: a state column draws states when asked.
func (c column) draw(cell string, states bool) string {
	if states && c.state {
		return stateText(cell)
	}
	return cell
}

// sizeColumns is cols, each as wide as its title and its widest cell as drawn.
func sizeColumns(cols []column, rows [][]string, states bool) []column {
	cols = slices.Clone(cols)
	for i := range cols {
		cols[i].width = lipgloss.Width(cols[i].title)
		for _, r := range rows {
			cols[i].width = max(cols[i].width, lipgloss.Width(cols[i].draw(cellAt(r, i), states)))
		}
	}
	return cols
}

// cellAt is cell i of a row, empty past the last one.
func cellAt(cells []string, i int) string {
	if i < len(cells) {
		return cells[i]
	}
	return ""
}

// selectedStyle draws the selected row: reversed, without the colors of its cells.
var selectedStyle = lipgloss.NewStyle().Reverse(true)

// headerStyle draws the column titles of a summary table.
var headerStyle = lipgloss.NewStyle().Bold(true).Foreground(muted)

// summaryTable draws rows under their titles, each column as wide as its
// widest cell: no rule, no selection, two spaces between the columns, and no
// padding after the last one. A table wider than width cells gets its widest
// columns cut to one common width, see capWidths.
func summaryTable(cols []column, rows [][]string, width int) []string {
	cols = sizeColumns(cols, rows, false)
	capWidths(cols, width)
	lines := []string{headerStyle.Render(strings.TrimRight(rowLine(cols, titles(cols), summaryGap, false), " "))}
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

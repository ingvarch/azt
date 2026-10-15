package views

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var sgrSeq = regexp.MustCompile("\x1b\\[[0-9;:]*m")

func plain(s string) string { return sgrSeq.ReplaceAllString(s, "") }

func testTable(width, height int, rows ...[]string) table {
	t := newTable([]column{{title: "Name", width: 6}, {title: "ID", width: 4}})
	t.resize(width, height)
	t.setRows(rows)
	return t
}

func numbered(n int) [][]string {
	rows := make([][]string, n)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("r%d", i), "x"}
	}
	return rows
}

func TestTableView(t *testing.T) {
	rows := [][]string{{"alpha", "1"}, {"toolongname", "22"}, {"b", "333333"}, {"d", "4"}}
	tests := []struct {
		name          string
		width, height int
		want          []string
	}{
		{"header, rule, rows", 40, 5, []string{
			"Name   ID  ",
			"───────────",
			"alpha  1   ",
			"toolo… 22  ",
			"b      333…",
		}},
		{"lines are cut to the width", 8, 4, []string{
			"Name   I",
			"────────",
			"alpha  1",
			"toolo… 2",
		}},
		{"rows are cut to the height", 40, 3, []string{
			"Name   ID  ",
			"───────────",
			"alpha  1   ",
		}},
		{"only the header and the rule", 40, 2, []string{"Name   ID  ", "───────────"}},
		{"only the header", 40, 1, []string{"Name   ID  "}},
		{"no height", 40, 0, nil},
		{"no width", 0, 5, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := testTable(tt.width, tt.height, rows...)
			got := plain(tb.view())
			if want := strings.Join(tt.want, "\n"); got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestTableStyles(t *testing.T) {
	tb := testTable(40, 5, []string{"alpha", "1"}, []string{"b", "2"})
	lines := strings.Split(tb.view(), "\n")
	want := []string{
		lipgloss.NewStyle().Bold(true).Render("Name   ID  "),
		"───────────",
		lipgloss.NewStyle().Reverse(true).Render("alpha  1   "),
		"b      2   ",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}

func TestTableWideAndEmptyCells(t *testing.T) {
	tb := newTable([]column{{title: "A", width: 4}, {title: "B", width: 1}, {title: "C"}})
	tb.resize(20, 4)
	tb.setRows([][]string{{"日本語", "xy", "z"}, {"", "", ""}})
	got := strings.Split(plain(tb.view()), "\n")
	for i, line := range got {
		if w := lipgloss.Width(line); w > 20 {
			t.Errorf("line %d is %d wide: %q", i, w, line)
		}
	}
	if !strings.HasPrefix(got[2], "日… ") {
		t.Errorf("wide cell cut = %q, want prefix %q", got[2], "日… ")
	}
	if !strings.Contains(got[2], "…") || strings.Contains(got[2], "xy") {
		t.Errorf("single-column cell not cut to the ellipsis: %q", got[2])
	}
}

var (
	kDown   = tea.KeyPressMsg{Code: tea.KeyDown}
	kUp     = tea.KeyPressMsg{Code: tea.KeyUp}
	kPgDown = tea.KeyPressMsg{Code: tea.KeyPgDown}
	kPgUp   = tea.KeyPressMsg{Code: tea.KeyPgUp}
	kHome   = tea.KeyPressMsg{Code: tea.KeyHome}
	kEnd    = tea.KeyPressMsg{Code: tea.KeyEnd}
)

func letter(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func TestTableKeys(t *testing.T) {
	// 10 rows, height 6: four rows are visible.
	tests := []struct {
		name      string
		keys      []tea.KeyPressMsg
		cursor    int
		top       int
		startLast bool
	}{
		{"down", []tea.KeyPressMsg{kDown}, 1, 0, false},
		{"j", []tea.KeyPressMsg{letter('j')}, 1, 0, false},
		{"up at the first row", []tea.KeyPressMsg{kUp}, 0, 0, false},
		{"k at the first row", []tea.KeyPressMsg{letter('k')}, 0, 0, false},
		{"down scrolls", []tea.KeyPressMsg{kDown, kDown, kDown, kDown}, 4, 1, false},
		{"up", []tea.KeyPressMsg{kUp}, 8, 6, true},
		{"k", []tea.KeyPressMsg{letter('k')}, 8, 6, true},
		{"down at the last row", []tea.KeyPressMsg{kDown}, 9, 6, true},
		{"pgdown", []tea.KeyPressMsg{kPgDown}, 4, 1, false},
		{"f", []tea.KeyPressMsg{letter('f')}, 4, 1, false},
		{"pgdown stops at the last row", []tea.KeyPressMsg{kPgDown, kPgDown, kPgDown}, 9, 6, false},
		{"pgup", []tea.KeyPressMsg{kPgUp}, 5, 5, true},
		{"b", []tea.KeyPressMsg{letter('b')}, 5, 5, true},
		{"pgup stops at the first row", []tea.KeyPressMsg{kPgUp}, 0, 0, false},
		{"end", []tea.KeyPressMsg{kEnd}, 9, 6, false},
		{"G", []tea.KeyPressMsg{letter('G')}, 9, 6, false},
		{"home", []tea.KeyPressMsg{kHome}, 0, 0, true},
		{"g", []tea.KeyPressMsg{letter('g')}, 0, 0, true},
		{"other keys", []tea.KeyPressMsg{letter('x'), {Code: tea.KeyEnter}}, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := testTable(40, 6, numbered(10)...)
			if tt.startLast {
				tb.press(kEnd)
			}
			for _, k := range tt.keys {
				tb.press(k)
			}
			if tb.cursor != tt.cursor || tb.top != tt.top {
				t.Errorf("cursor, top = %d, %d; want %d, %d", tb.cursor, tb.top, tt.cursor, tt.top)
			}
		})
	}
}

func TestTableKeysWithoutRows(t *testing.T) {
	tb := testTable(40, 6)
	for _, k := range []tea.KeyPressMsg{kDown, kUp, kPgDown, kPgUp, kHome, kEnd} {
		tb.press(k)
		if tb.cursor != -1 {
			t.Fatalf("%v selected row %d of an empty table", k, tb.cursor)
		}
	}
}

func TestTablePageOnTinyHeight(t *testing.T) {
	tb := testTable(40, 2, numbered(5)...)
	tb.press(kPgDown)
	if tb.cursor != 1 {
		t.Errorf("pgdown with no visible rows moved to %d, want 1", tb.cursor)
	}
}

func TestTableSelectionStaysVisible(t *testing.T) {
	tb := testTable(40, 12, numbered(10)...)
	tb.press(kEnd)
	tb.resize(40, 5) // three rows
	if tb.top != 7 {
		t.Errorf("after shrinking top = %d, want 7", tb.top)
	}
	tb.resize(40, 12)
	if tb.top != 0 {
		t.Errorf("after growing top = %d, want 0 (no blank rows below the last)", tb.top)
	}
	tb.resize(40, 5)
	tb.selectRow(0)
	if tb.top != 0 || tb.cursor != 0 {
		t.Errorf("selectRow(0): cursor, top = %d, %d", tb.cursor, tb.top)
	}
	tb.selectRow(99)
	if tb.cursor != 9 {
		t.Errorf("selectRow(99) cursor = %d, want 9", tb.cursor)
	}
	tb.selectRow(-3)
	if tb.cursor != 0 {
		t.Errorf("selectRow(-3) cursor = %d, want 0", tb.cursor)
	}
}

func TestTableSetRows(t *testing.T) {
	tb := testTable(40, 6)
	if tb.cursor != -1 {
		t.Errorf("empty table cursor = %d, want -1", tb.cursor)
	}
	tb.setRows(numbered(10))
	if tb.cursor != 0 {
		t.Errorf("first rows: cursor = %d, want 0", tb.cursor)
	}
	tb.press(kEnd)
	tb.setRows(numbered(3))
	if tb.cursor != 2 || tb.top != 0 {
		t.Errorf("fewer rows: cursor, top = %d, %d; want 2, 0", tb.cursor, tb.top)
	}
	tb.setRows(numbered(5))
	if tb.cursor != 2 {
		t.Errorf("more rows moved the cursor to %d, want 2", tb.cursor)
	}
	tb.setRows(nil)
	if tb.cursor != -1 || tb.view() == "" {
		t.Errorf("no rows: cursor = %d", tb.cursor)
	}
	if _, ok := tb.RowAt(2); ok {
		t.Error("RowAt found a row in an empty table")
	}
}

func TestTableRowAt(t *testing.T) {
	tb := testTable(40, 6, numbered(10)...)
	tb.press(kEnd) // top = 6: r6..r9 on lines 2..5
	few := testTable(40, 6, numbered(2)...)
	tests := []struct {
		name string
		tb   table
		y    int
		want int
		ok   bool
	}{
		{"header", tb, 0, 0, false},
		{"rule", tb, 1, 0, false},
		{"first visible row", tb, 2, 6, true},
		{"last visible row", tb, 5, 9, true},
		{"below the visible rows", tb, 6, 0, false},
		{"below the visible rows with more rows after", testTable(40, 6, numbered(10)...), 6, 0, false},
		{"negative", tb, -1, 0, false},
		{"below the last row of a short table", few, 4, 0, false},
		{"second row of a short table", few, 3, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.tb.RowAt(tt.y)
			if got != tt.want || ok != tt.ok {
				t.Errorf("RowAt(%d) = (%d, %v), want (%d, %v)", tt.y, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTableSelectedRowIsTheCursorRow(t *testing.T) {
	tb := testTable(40, 5, numbered(3)...)
	tb.press(kDown)
	lines := strings.Split(tb.view(), "\n")
	for i, selected := range []bool{false, true, false} {
		if got := strings.Contains(lines[i+2], "\x1b[7m"); got != selected {
			t.Errorf("row %d reversed = %v, want %v", i, got, selected)
		}
	}
}

func TestTableCellWithNewlineStaysOnOneLine(t *testing.T) {
	tb := testTable(40, 5, []string{"a\nb", "1"}, []string{"c", "2"})
	if got := strings.Count(tb.view(), "\n"); got != 3 {
		t.Errorf("view has %d line breaks, want 3:\n%s", got, tb.view())
	}
}

func stateTable(width, height int) table {
	t := newTable([]column{{title: "Name", width: 6}, {title: "State", width: 12, state: true}})
	t.resize(width, height)
	t.setRows([][]string{{"a", "Running"}, {"b", "Failed"}, {"c", "Updating"}})
	return t
}

func TestTableColorsTheStateColumn(t *testing.T) {
	lines := strings.Split(stateTable(40, 5).view(), "\n")
	if got, want := plain(lines[3]), "b      ● Failed    "; got != want {
		t.Errorf("line 3 = %q, want %q", got, want)
	}
	wantFg(t, lines[3], "Failed", fgBad)
	wantFg(t, lines[4], "Updating", fgWarn)
	wantFg(t, lines[3], "b", "")
	if attrsOf(t, lines[0], "State").fg != "" {
		t.Error("the header is colored")
	}
}

func TestTableSelectedRowStaysReversedWithColoredState(t *testing.T) {
	lines := strings.Split(stateTable(40, 5).view(), "\n")
	for _, sub := range []string{"a", "Running", "●", "   "} {
		if !attrsOf(t, lines[2], sub).reverse {
			t.Errorf("%q is not reversed in %q", sub, lines[2])
		}
	}
	wantFg(t, lines[2], "Running", fgGood)
	if attrsOf(t, lines[3], "Failed").reverse {
		t.Error("an unselected row is reversed")
	}
}

func TestTableCutsAColoredStateCell(t *testing.T) {
	tb := newTable([]column{{title: "State", width: 6, state: true}})
	tb.resize(20, 3)
	tb.setRows([][]string{{"Updating"}})
	if got := plain(strings.Split(tb.view(), "\n")[2]); got != "● Upd…" {
		t.Errorf("cell = %q", got)
	}
}

func TestSummaryTable(t *testing.T) {
	cols := []column{{title: "Name"}, {title: "State"}}
	lines := summaryTable(cols, [][]string{{"a", stateText("Running")}, {"bb", stateText("Stopped · Updating")}}, 200)
	want := []string{
		"Name  State",
		"a     ● Running",
		"bb    ● Stopped · Updating",
	}
	got := make([]string, len(lines))
	for i, l := range lines {
		got[i] = plain(l)
	}
	wantLines(t, got, want)
	if a := attrsOf(t, lines[0], "Name"); !a.bold || a.fg != fgMuted {
		t.Errorf("header = %+v, want bold muted", a)
	}
	wantFg(t, lines[1], "a", "")
	wantFg(t, lines[1], "Running", fgGood)
	wantFg(t, lines[2], "Stopped", fgBad)
	wantFg(t, lines[2], "Updating", fgWarn)
	for i, l := range lines {
		if strings.Contains(l, "\x1b[7m") {
			t.Errorf("line %d is highlighted", i)
		}
	}
}

func TestSummaryTableWithoutRows(t *testing.T) {
	lines := summaryTable([]column{{title: "Name"}}, nil, 200)
	if len(lines) != 1 || strings.TrimRight(plain(lines[0]), " ") != "Name" {
		t.Errorf("lines = %q", lines)
	}
}

func TestTableSelectedRowIsReversedAfterAState(t *testing.T) {
	tb := newTable([]column{{title: "State", width: 12, state: true}, {title: "Name", width: 4}})
	tb.resize(40, 3)
	tb.setRows([][]string{{"Running", "a"}})
	line := strings.Split(tb.view(), "\n")[2]
	for _, sub := range []string{"  ", " a"} {
		if !attrsOf(t, line, sub).reverse {
			t.Errorf("%q after the state is not reversed in %q", sub, line)
		}
	}
}

func TestSummaryTableShortRow(t *testing.T) {
	lines := summaryTable([]column{{title: "A"}, {title: "B"}}, [][]string{{"x"}}, 200)
	if got := plain(lines[1]); got != "x" {
		t.Errorf("row = %q", got)
	}
}

// summaryCols are columns named by their titles.
func summaryCols(titles ...string) []column {
	cols := make([]column, len(titles))
	for i, t := range titles {
		cols[i] = column{title: t}
	}
	return cols
}

func TestSummaryTableThatFitsIsUnchanged(t *testing.T) {
	cols := summaryCols("Name", "Note", "End")
	rows := [][]string{{"alpha", "bravo-charlie", "x"}}
	want := wantTable([]int{5, 13}, []string{"Name", "Note", "End"}, []string{"alpha", "bravo-charlie", "x"})
	for _, width := range []int{25, 26, 80} {
		wantLines(t, plainAll(summaryTable(cols, rows, width)), want)
	}
}

func TestSummaryTableCapsOnlyTheLongestColumn(t *testing.T) {
	cols := summaryCols("Name", "Note", "End")
	rows := [][]string{{"alpha", "bravo-charlie", "x"}}
	got := plainAll(summaryTable(cols, rows, 21))
	wantLines(t, got, wantTable([]int{5, 9}, []string{"Name", "Note", "End"}, []string{"alpha", "bravo-ch…", "x"}))
	wantMaxWidth(t, got, 21)
}

func TestSummaryTableCapsLongColumnsAtOneWidth(t *testing.T) {
	cols := summaryCols("Big", "Tag", "Huge", "Last")
	rows := [][]string{{"abcdefghijkl", "t1", "0123456789abcdefghij", "e"}}
	got := plainAll(summaryTable(cols, rows, 35))
	wantLines(t, got, wantTable([]int{11, 3, 11}, []string{"Big", "Tag", "Huge", "Last"}, []string{"abcdefghij…", "t1", "0123456789…", "e"}))
	wantMaxWidth(t, got, 35)
}

func TestSummaryTableStopsAtTheFloor(t *testing.T) {
	cols := summaryCols("Big", "Tag", "Huge", "Last")
	rows := [][]string{{"abcdefghijkl", "t1", "0123456789abcdefghij", "e"}}
	got := plainAll(summaryTable(cols, rows, 10))
	wantLines(t, got, wantTable([]int{8, 3, 8}, []string{"Big", "Tag", "Huge", "Last"}, []string{"abcdefg…", "t1", "0123456…", "e"}))
}

// wantMaxWidth fails for a line wider than width cells.
func wantMaxWidth(t *testing.T, lines []string, width int) {
	t.Helper()
	for _, l := range lines {
		if w := lipgloss.Width(l); w > width {
			t.Errorf("line %q is %d cells, want at most %d", l, w, width)
		}
	}
}

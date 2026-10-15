package views

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

// layoutTable has columns of natural widths 7, 4 and 5: 16 cells, plus 2 gaps of 2 cells.
func layoutTable(width int) table {
	t := newTable([]column{{title: "Name", width: 7}, {title: "ID", width: 4}, {title: "Code", width: 5}})
	t.resize(width, 5)
	t.setRows([][]string{{"alpha", "1", "abc"}, {"b", "22", "d"}})
	return t
}

// padded joins cells, each padded to its width, with two spaces.
func padded(widths []int, cells ...string) string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = c + strings.Repeat(" ", widths[i]-lipgloss.Width(c))
	}
	return strings.Join(out, "  ")
}

func TestTableSpreadsTheSpare(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		widths []int
	}{
		{"spare 9 shared by 3", 29, []int{10, 7, 8}},
		{"spare 10: the first column gets the extra cell", 30, []int{11, 7, 8}},
		{"spare 11: the first two get one", 31, []int{11, 8, 8}},
		{"no spare", 20, []int{7, 4, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Split(layoutTable(tt.width).view(), "\n")
			want := []string{
				padded(tt.widths, "Name", "ID", "Code"),
				strings.Repeat("─", tt.width),
				padded(tt.widths, "alpha", "1", "abc"),
				padded(tt.widths, "b", "22", "d"),
			}
			for i, w := range want {
				if plain(got[i]) != w {
					t.Errorf("line %d = %q, want %q", i, plain(got[i]), w)
				}
				if lipgloss.Width(got[i]) != tt.width {
					t.Errorf("line %d is %d wide, want %d", i, lipgloss.Width(got[i]), tt.width)
				}
			}
		})
	}
}

func TestTableSelectedRowRunsToTheEdge(t *testing.T) {
	line := strings.Split(layoutTable(31).view(), "\n")[2]
	if !strings.Contains(line, "\x1b[7m") || lipgloss.Width(line) != 31 {
		t.Errorf("selected row = %q (%d wide)", line, lipgloss.Width(line))
	}
	if got, want := plain(line), padded([]int{11, 8, 8}, "alpha", "1", "abc"); got != want {
		t.Errorf("selected row = %q, want %q", got, want)
	}
}

// wideTable has natural widths 12, 3, 20 and 4: 39 cells, plus 3 gaps of 2 cells.
func wideTable(width int) table {
	t := newTable([]column{{title: "Big", width: 12}, {title: "Tag", width: 3}, {title: "Huge", width: 20}, {title: "Last", width: 4}})
	t.resize(width, 3)
	t.setRows([][]string{{"abcdefghijkl", "t1", "0123456789abcdefghij", "e"}})
	return t
}

func TestTableCutsOnlyTheWidestColumns(t *testing.T) {
	got := strings.Split(plain(wideTable(35).view()), "\n")
	widths := []int{11, 3, 11, 4}
	want := []string{
		padded(widths, "Big", "Tag", "Huge", "Last"),
		strings.Repeat("─", 35),
		padded(widths, "abcdefghij…", "t1", "0123456789…", "e"),
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("line %d = %q, want %q", i, got[i], w)
		}
	}
}

func TestTableCutsAtTheFloor(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  []string
	}{
		{"a line wider than the table is cut", 10, []string{"Big       ", "──────────", "abcdefg…  "}},
		{"the floor of 8 holds the widest columns", 30, []string{
			padded([]int{8, 3, 8, 4}, "Big", "Tag", "Huge", "Last"),
			strings.Repeat("─", 29),
			padded([]int{8, 3, 8, 4}, "abcdefg…", "t1", "0123456…", "e"),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Split(plain(wideTable(tt.width).view()), "\n")
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("line %d = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestTableLayoutLeavesItsColumnsAlone(t *testing.T) {
	tb := wideTable(35)
	tb.view()
	tb.resize(100, 3)
	if got, want := tb.view(), wideTable(100).view(); got != want {
		t.Errorf("after a narrow pane, a wide one draws\n%s\nwant\n%s", got, want)
	}
	if got, want := tb.view(), wideTable(100).view(); got != want {
		t.Errorf("a second draw gives\n%s\nwant\n%s", got, want)
	}
}

func TestSizeColumns(t *testing.T) {
	cols := []column{{title: "A long title"}, {title: "S", state: true}, {title: "N"}}
	rows := [][]string{{"x", "Running", "wider cell"}, {"yy", "Up", "z"}}
	for _, tt := range []struct {
		states bool
		widths []int
	}{{true, []int{12, 9, 10}}, {false, []int{12, 7, 10}}} {
		got := sizeColumns(cols, rows, tt.states)
		for i, want := range tt.widths {
			if got[i].width != want {
				t.Errorf("states %v: column %d width = %d, want %d", tt.states, i, got[i].width, want)
			}
		}
	}
	if cols[0].width != 0 {
		t.Error("sizeColumns changed its input")
	}
	long := strings.Repeat("x", listColumnMax+10)
	if got := sizeColumns([]column{{title: "N"}}, [][]string{{long}}, false); got[0].width != listColumnMax+10 {
		t.Errorf("sizeColumns capped a summary column at %d", got[0].width)
	}
}

const nameBase = "VisualStudioOnline-2B85A62B591543EA99CEF6D9C1"

// longGroup is 90 cells wide.
const longGroup = nameBase + nameBase

func layoutGroups() []azure.ResourceGroup {
	return []azure.ResourceGroup{
		{Name: longGroup, Location: "westeurope", State: "Succeeded"},
		{Name: "rg-a", Location: "southeastasia", State: "Succeeded"},
	}
}

func groupList(t *testing.T, width int, groups []azure.ResourceGroup) View {
	t.Helper()
	return deliver(t, NewResourceGroups(azure.Subscription{ID: "id-1", Name: "Dev"}).Resize(width, 10), groups, nil)
}

// starts are the cells at which Location and State begin in the header line.
func starts(t *testing.T, v View) (loc, state int) {
	t.Helper()
	for _, line := range strings.Split(plain(v.Render()), "\n") {
		if strings.HasPrefix(line, "Name") {
			return strings.Index(line, "Location"), strings.Index(line, "State")
		}
	}
	t.Fatalf("no header in:\n%s", v.Render())
	return 0, 0
}

func TestListCutsALongNameAtTheCapAndSpreads(t *testing.T) {
	if lipgloss.Width(longGroup) != 90 {
		t.Fatalf("fixture name is %d wide", lipgloss.Width(longGroup))
	}
	v := groupList(t, 160, layoutGroups())
	out := plain(v.Render())
	// own widths 40, 13 and 11 with 4 of gap: spare 92 gives 31, 31 and 30 more.
	cut := longGroup[:70] + "…"
	for _, s := range []string{cut, "rg-a", "westeurope", "southeastasia", "● Succeeded"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q in:\n%s", s, out)
		}
	}
	if strings.Contains(out, longGroup[:71]) {
		t.Errorf("name is wider than 71 cells:\n%s", out)
	}
	if loc, state := starts(t, v); loc != 71+2 || state != 71+2+44+2 {
		t.Errorf("Location, State start at %d, %d, want 73, 119", loc, state)
	}
	for i, line := range strings.Split(v.Render(), "\n") {
		if lipgloss.Width(line) != 160 {
			t.Errorf("line %d is %d wide", i, lipgloss.Width(line))
		}
	}
}

func TestListOwnWidthIsTheLongestNameUnderTheCap(t *testing.T) {
	groups := layoutGroups()
	groups[0].Name = longGroup[:30]
	v := groupList(t, 100, groups)
	// own widths 30, 13 and 11 with 4 of gap: spare 42 gives 14 more to each.
	if loc, state := starts(t, v); loc != 44+2 || state != 44+2+27+2 {
		t.Errorf("Location, State start at %d, %d, want 46, 75", loc, state)
	}
	if out := plain(v.Render()); !strings.Contains(out, longGroup[:30]+" ") {
		t.Errorf("name is cut:\n%s", out)
	}
}

func TestListInANarrowPaneCutsOnlyTheLongest(t *testing.T) {
	out := plain(groupList(t, 45, layoutGroups()).Render())
	for _, s := range []string{"Location", "westeurope", "southeastasia", "● Succeeded"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q in:\n%s", s, out)
		}
	}
	if !strings.Contains(out, longGroup[:16]+"…") || strings.Contains(out, longGroup[:17]) {
		t.Errorf("name not cut to 17 cells:\n%s", out)
	}
}

func TestListFilterKeepsTheColumns(t *testing.T) {
	v := groupList(t, 100, layoutGroups())
	loc, state := starts(t, v)
	v = v.(interface{ SetFilter(string) View }).SetFilter("rg-a")
	if l, s := starts(t, v); l != loc || s != state {
		t.Errorf("filter moved the columns from %d, %d to %d, %d", loc, state, l, s)
	}
}

func TestListColumnsFollowDataAndResize(t *testing.T) {
	groups := layoutGroups()
	groups[0].Name = longGroup[:30]
	v := groupList(t, 100, groups)
	groups[0].Name = longGroup[:36]
	v, _ = v.Update(data.ResultMsg{Key: fetchKey(t, v), Data: groups})
	// own widths 36, 13 and 11: spare 36 gives 12 more to each.
	if loc, _ := starts(t, v); loc != 48+2 {
		t.Errorf("longer name: Location at %d, want 50", loc)
	}
	groups[0].Name = longGroup[:20]
	v, _ = v.Update(data.ResultMsg{Key: fetchKey(t, v), Data: groups})
	// own widths 20, 13 and 11: spare 52 gives 18, 17 and 17 more.
	if loc, _ := starts(t, v); loc != 38+2 {
		t.Errorf("shorter name: Location at %d, want 40", loc)
	}
	// spare 80 gives 27, 27 and 26 more.
	if loc, _ := starts(t, v.Resize(128, 10)); loc != 47+2 {
		t.Errorf("resize: Location at %d, want 49", loc)
	}
	// 48 cells do not fit in 40: the two widest columns are cut to 12.
	if loc, _ := starts(t, v.Resize(40, 10)); loc != 12+2 {
		t.Errorf("narrow: Location at %d, want 14", loc)
	}
}

func TestSummaryTableIgnoresTheListCap(t *testing.T) {
	long := strings.Repeat("y", listColumnMax+10)
	lines := plainAll(summaryTable([]column{{title: "A"}, {title: "B"}}, [][]string{{long, "z"}}, 200))
	if want := long + "  z"; lines[1] != want {
		t.Errorf("row = %q, want %q", lines[1], want)
	}
}

func TestListSizesOnlyTheKeptItems(t *testing.T) {
	data := []azure.Resource{
		{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites", Location: "westeurope"},
		{ID: "/r/kv", Name: "a-much-longer-vault-name-xx", Type: "microsoft.keyvault/vaults", Location: "northeurope"},
	}
	v := deliver(t, NewResourcesOfType("id-1", groups[0], "microsoft.web/sites").Resize(40, 10), data, nil)
	header := strings.Split(plain(v.Render()), "\n")[0]
	// Name 7, Type 19 and Location 10 with two gaps of 2 fill 40 cells with no spare.
	if ty, loc := strings.Index(header, "Type"), strings.Index(header, "Location"); ty != 9 || loc != 30 {
		t.Errorf("Type at %d, Location at %d in %q, want 9 and 30: sized to the kept web app only", ty, loc, header)
	}
}

package ui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/session"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

// bar is line 0 of the frame without styling codes.
func (h *harness) bar() string { return strings.Split(h.frame(), "\n")[0] }

// rawBar is line 0 of the frame with styling codes.
func (h *harness) rawBar() string { return strings.Split(h.raw(), "\n")[0] }

// barLayouts are the window setups where the tab bar starts in a different column.
var barLayouts = []struct {
	name  string
	build func() *harness
	start int // first column of the tab bar
	paneW int // width of a pane view
}{
	{"explorer shown", func() *harness { return newSidebarHarness(100, 30) }, sidebarWidth, 100 - sidebarWidth - 2},
	{"explorer hidden", func() *harness { return newHarness(100, 30) }, 0, 98},
	{"narrow window", func() *harness { return newSidebarHarness(sidebarWidth+minPaneWidth-1, 30) }, 0, sidebarWidth + minPaneWidth - 3},
}

func TestTabBarStartsWhereThePanesStart(t *testing.T) {
	for _, tt := range barLayouts {
		t.Run(tt.name, func(t *testing.T) {
			bar := []rune(tt.build().bar() + strings.Repeat(" ", 40)) // padded: a short bar fails instead of panicking
			if got := string(bar[tt.start : tt.start+8]); got != "   1    " {
				t.Errorf("bar from column %d = %q, want %q (bar %q)", tt.start, got, "   1    ", string(bar))
			}
			if got := strings.TrimSpace(string(bar[:tt.start])); tt.start > 0 && !strings.HasPrefix(got, "┌ Explorer ") {
				t.Errorf("left of the bar = %q, want the explorer border", got)
			}
		})
	}
}

func TestExplorerRunsFromLineZeroToTheLineAboveTheStatusBar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	lines := strings.Split(h.frame(), "\n")
	if !strings.HasPrefix(lines[0], "┌ Explorer ") {
		t.Errorf("line 0 = %q", lines[0])
	}
	for i := 1; i < len(lines)-2; i++ {
		if !strings.HasPrefix(lines[i], "│") {
			t.Errorf("line %d = %q, want the explorer side", i, lines[i])
		}
	}
	if !strings.HasPrefix(lines[len(lines)-2], "└") {
		t.Errorf("line above the status bar = %q", lines[len(lines)-2])
	}
	var subs []azure.Subscription
	for i := range 40 {
		subs = append(subs, azure.Subscription{ID: fmt.Sprint("sub-", i), Name: fmt.Sprint("S", i), State: "Enabled"})
	}
	h.send(data.ResultMsg{Key: data.Key{Kind: data.Subscriptions}, Data: subs})
	if got := lipgloss.Height(h.m.sidebar.Render()); got != 30-1-2 {
		t.Errorf("explorer inner height = %d, want %d", got, 30-1-2)
	}
	if r := h.pane(0); r.h != 26 {
		t.Errorf("pane inner height = %d, want 26", r.h)
	}
	if got := string([]rune(lines[1])[sidebarWidth]); got != "┌" {
		t.Errorf("pane does not start on line 1: %q", lines[1])
	}
}

func TestTabLabels(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.prefix("t")
	h.m.tabs[1].name = "ab"
	if got, want := strings.TrimRight(h.bar(), " "), "   1    "+" "+"  2 ab  "+" "+"   3    "+" "+" +"; got != want {
		t.Errorf("bar = %q, want %q", got, want)
	}
}

func TestTabLabelWidths(t *testing.T) {
	tests := []struct {
		name  string
		tab   string // name of the second tab
		label string
	}{
		{"number only", "", "   2    "},
		{"short name", "prod", "  2 prod  "},
		{"odd padding", "a", "  2 a   "},
		{"five cells", "abc", "  2 abc  "},
		{"long name", "verylongname", "  2 verylongname  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(100, 30)
			h.prefix("t")
			h.m.tabs[1].name = tt.tab
			if got := h.m.tabLabel(1); got != tt.label {
				t.Errorf("label = %q, want %q", got, tt.label)
			}
		})
	}
}

func TestTabLabelsIgnoreViews(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	check := func(step string) {
		t.Helper()
		if got, want := strings.TrimRight(h.bar(), " "), "   1    "+" "+"   2    "+" "+" +"; got != want {
			t.Errorf("after %s: bar = %q, want %q", step, got, want)
		}
	}
	h.prefix("v")
	check("split")
	h.send(key("enter"))
	check("drill down")
	h.prefix("h")
	check("focus change")
}

func TestTabBarStyles(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.prefix("t")
	h.prefix("2")
	line := h.rawBar()
	attrs := func(label string) []string {
		m := regexp.MustCompile("\x1b\\[([0-9;]*)m" + label).FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("label %q not found in %q", label, line)
		}
		return strings.Split(m[1], ";")
	}
	// 94 is the foreground color 12 (bright blue)
	if got := attrs("   2    "); !slices.Contains(got, "1") || !slices.Contains(got, "7") || !slices.Contains(got, "94") {
		t.Errorf("active label attributes = %v, want bold, reverse and the focus color", got)
	}
	for _, label := range []string{"   1    ", "   3    "} {
		got := attrs(label)
		if !slices.Equal(got, []string{"2"}) {
			t.Errorf("label %q attributes = %v, want faint only", label, got)
		}
	}
}

func TestPressOnTabLabels(t *testing.T) {
	// tabs: 0-7, 9-16, 18-28 (the third is named); gaps at 8, 17 and 29; plus 30-32
	tests := []struct {
		name string
		x    int
		from int // active tab before the press
		want int
	}{
		{"first start", 0, 2, 0},
		{"first end", 7, 2, 0},
		{"gap", 8, 2, 2},
		{"second start", 9, 0, 1},
		{"second end", 16, 2, 1},
		{"gap before the third", 17, 0, 0},
		{"third start", 18, 0, 2},
		{"third end", 28, 1, 2},
		{"gap before the plus", 29, 1, 1},
		{"past the plus", 33, 1, 1},
	}
	for _, shown := range []bool{true, false} {
		for _, tt := range tests {
			t.Run(fmt.Sprint("explorer ", shown, ", ", tt.name), func(t *testing.T) {
				h := newSidebarHarness(100, 30)
				start := sidebarWidth
				if !shown {
					h.prefix("e")
					start = 0
				}
				h.prefix("t")
				h.prefix("t")
				h.m.tabs[2].name = "abcde"
				h.m.active = tt.from
				h.press(start+tt.x, 0)
				if h.m.active != tt.want {
					t.Errorf("active = %d, want %d", h.m.active, tt.want)
				}
			})
		}
	}
}

func TestPressOnLineZeroOverExplorerGoesToExplorer(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("t")
	h.prefix("1")
	h.press(1, 0)
	if h.m.active != 0 || !h.m.sidebarFocused {
		t.Errorf("active %d, sidebar focused %v", h.m.active, h.m.sidebarFocused)
	}
	h.press(sidebarWidth+9, 0)
	if h.m.active != 1 {
		t.Errorf("active = %d, want 1", h.m.active)
	}
}

func TestFrameFitsWithManyNamedTabs(t *testing.T) {
	for _, shaded := range []bool{false, true} {
		for _, w := range []int{100, 44, 43, 13, 5} {
			for _, sidebar := range []bool{true, false} {
				h := newSidebarHarness(w, 10)
				if !sidebar {
					h.prefix("e")
				}
				if shaded {
					h.background("#1e1e2e")
				}
				for range 8 {
					h.prefix("t")
					h.m.tabs[h.m.active].name = "a long tab name"
				}
				f := h.frame()
				if got := lipgloss.Height(f); got != 10 {
					t.Errorf("shaded=%v w=%d sidebar=%v: height = %d", shaded, w, sidebar, got)
				}
				if got := lipgloss.Width(f); got > w {
					t.Errorf("shaded=%v w=%d sidebar=%v: width = %d", shaded, w, sidebar, got)
				}
			}
		}
	}
}

func TestRestoredSessionKeepsNamesInLabels(t *testing.T) {
	leaf := &session.Node{Stack: []views.Spec{{Kind: "subscriptions"}}}
	s := session.Session{Version: session.Version, Active: 1, Tabs: []session.Tab{
		{Name: "work", Layout: leaf},
		{Layout: leaf},
		{Name: "second", Layout: leaf},
	}}
	m, err := Restore(s, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	line := strings.Split(sgr.ReplaceAllString(next.(Model).View().Content, ""), "\n")[0]
	if want := "  1 work  " + " " + "   2    " + " " + "  3 second  "; !strings.Contains(line, want) {
		t.Errorf("bar = %q, want it to contain %q", line, want)
	}
}

// plusX is the column of the " + " label after n tabs without names, for a bar starting at start.
func plusX(start, n int) int { return start + n*9 }

func TestPlusLabelFollowsTheLastTab(t *testing.T) {
	plusStyle := regexp.MustCompile("\x1b\\[([0-9;]*)m \\+ ")
	for _, l := range barLayouts {
		for _, n := range []int{1, 3} {
			h := l.build()
			for range n - 1 {
				h.prefix("t")
			}
			x := plusX(l.start, n)
			bar := []rune(h.bar() + strings.Repeat(" ", 40)) // padded: a short bar fails instead of panicking
			if got := string(bar[x : x+3]); got != " + " {
				t.Errorf("%s n=%d: bar at %d = %q, want %q", l.name, n, x, got, " + ")
			}
			if got := string(bar[x-1]); got != " " {
				t.Errorf("%s n=%d: no gap before the plus: %q", l.name, n, got)
			}
			raw := h.rawBar()
			match := plusStyle.FindStringSubmatch(raw)
			if match == nil || !slices.Equal(strings.Split(match[1], ";"), []string{"2"}) {
				t.Errorf("%s n=%d: plus style = %v, want faint only", l.name, n, match)
			}
		}
	}
}

func TestPlusFollowsNamedTabs(t *testing.T) {
	for _, l := range barLayouts {
		h := l.build()
		h.prefix("t")
		h.prefix("t")
		h.m.tabs[1].name = "prod"
		bar := []rune(h.bar() + strings.Repeat(" ", 60))
		got := strings.TrimRight(string(bar[l.start:l.start+33]), " ")
		if want := "   1    " + " " + "  2 prod  " + " " + "   3    " + " " + " +"; got != want {
			t.Errorf("%s: bar = %q, want %q", l.name, got, want)
		}
	}
}

func TestPressOnPlusOpensNamedTabAtTheEnd(t *testing.T) {
	for _, l := range barLayouts {
		for dx := range 3 {
			h := l.build()
			h.prefix("t")
			h.prefix("p") // active is the first of two tabs
			cmd := h.press(plusX(l.start, 2)+dx, 0)
			if len(h.m.tabs) != 3 || h.m.active != 2 {
				t.Fatalf("%s dx=%d: tabs %d, active %d", l.name, dx, len(h.m.tabs), h.m.active)
			}
			if cmd == nil || cmd() != "init-3" || h.recs[2].inits != 1 {
				t.Errorf("%s dx=%d: the new view's Init did not run", l.name, dx)
			}
			if h.pane(h.focused()).rec.id != 3 {
				t.Errorf("%s dx=%d: the active tab does not hold the new view", l.name, dx)
			}
			if v := h.pane(h.focused()); v.w != l.paneW || v.h != 26 {
				t.Errorf("%s dx=%d: new view size = %dx%d, want %dx26", l.name, dx, v.w, v.h, l.paneW)
			}
			if h.m.mode != modeRename || h.m.input.Value() != "" {
				t.Errorf("%s dx=%d: mode %v, input %q", l.name, dx, h.m.mode, h.m.input.Value())
			}
		}
	}
}

func TestPlusTabNameFlow(t *testing.T) {
	tests := []struct {
		name  string
		typed string
		end   string
		label string
	}{
		{"typed name", " Prod ", "enter", "  2 Prod  "},
		{"enter with nothing typed", "", "enter", "   2    "},
		{"blank name", "   ", "enter", "   2    "},
		{"esc", "", "esc", "   2    "},
		{"esc after typing", "x", "esc", "   2    "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(100, 30)
			h.press(plusX(0, 1), 0)
			h.typ(tt.typed)
			h.send(key(tt.end))
			if h.m.mode != modeNormal || len(h.m.tabs) != 2 || h.m.active != 1 {
				t.Fatalf("mode %v, tabs %d, active %d", h.m.mode, len(h.m.tabs), h.m.active)
			}
			if got := h.m.tabLabel(1); got != tt.label {
				t.Errorf("label = %q, want %q", got, tt.label)
			}
		})
	}
}

func TestPressInTheGapAndPastPlusDoesNothing(t *testing.T) {
	for _, x := range []int{plusX(0, 1) - 1, plusX(0, 1) + 3, 40, 99} {
		h := newHarness(100, 30)
		if cmd := h.press(x, 0); cmd != nil {
			t.Errorf("x=%d: press returned a command", x)
		}
		if len(h.m.tabs) != 1 || h.m.mode != modeNormal {
			t.Errorf("x=%d: tabs %d, mode %v", x, len(h.m.tabs), h.m.mode)
		}
	}
}

func TestPressOnPlusIsIgnoredInOtherModes(t *testing.T) {
	enter := map[string]func(h *harness){
		"help":   func(h *harness) { h.send(key("?")) },
		"prefix": func(h *harness) { h.send(key("ctrl+w")) },
		"rename": func(h *harness) { h.prefix(",") },
		"filter": func(h *harness) { h.send(key("/")) },
	}
	for name, do := range enter {
		h, _ := newFilterHarness(100, 30)
		do(h)
		h.press(plusX(0, 1), 0)
		if len(h.m.tabs) != 1 {
			t.Errorf("%s: a tab was opened", name)
		}
	}
}

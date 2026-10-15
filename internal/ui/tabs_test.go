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

func TestTabBarStartsWhereThePanesStart(t *testing.T) {
	tests := []struct {
		name  string
		build func() *harness
		start int
	}{
		{"explorer shown", func() *harness { return newSidebarHarness(100, 30) }, sidebarWidth},
		{"explorer hidden", func() *harness { return newHarness(100, 30) }, 0},
		{"narrow window", func() *harness { return newSidebarHarness(sidebarWidth+minPaneWidth-1, 30) }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := []rune(tt.build().bar() + strings.Repeat(" ", 40)) // padded: a short bar fails instead of panicking
			if got := string(bar[tt.start : tt.start+3]); got != " 1 " {
				t.Errorf("bar from column %d = %q, want %q (bar %q)", tt.start, got, " 1 ", string(bar))
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
	if got, want := strings.TrimRight(h.bar(), " "), " 1   2 ab   3"; got != want {
		t.Errorf("bar = %q, want %q", got, want)
	}
}

func TestTabLabelsIgnoreViews(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	check := func(step string) {
		t.Helper()
		if got, want := strings.TrimRight(h.bar(), " "), " 1   2"; got != want {
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
	line := strings.Split(h.raw(), "\n")[0]
	attrs := func(label string) []string {
		m := regexp.MustCompile("\x1b\\[([0-9;]*)m" + label).FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("label %q not found in %q", label, line)
		}
		return strings.Split(m[1], ";")
	}
	// 94 is the foreground color 12 (bright blue)
	if got := attrs(" 2 "); !slices.Contains(got, "1") || !slices.Contains(got, "7") || !slices.Contains(got, "94") {
		t.Errorf("active label attributes = %v, want bold, reverse and the focus color", got)
	}
	for _, label := range []string{" 1 ", " 3 "} {
		got := attrs(label)
		if !slices.Equal(got, []string{"2"}) {
			t.Errorf("label %q attributes = %v, want faint only", label, got)
		}
	}
}

func TestPressOnTabLabels(t *testing.T) {
	// labels: " 1 " 0-2, " 2 " 4-6, " 3 ab " 8-13
	tests := []struct {
		name    string
		sidebar bool
		x       int
		active  int
	}{
		{"hidden, first start", false, 0, 0},
		{"hidden, first end", false, 2, 0},
		{"hidden, gap", false, 3, 1},
		{"hidden, second start", false, 4, 1},
		{"hidden, second end", false, 6, 1},
		{"hidden, gap before the third", false, 7, 1},
		{"hidden, third start", false, 8, 2},
		{"hidden, third end", false, 13, 2},
		{"hidden, past the last", false, 14, 1},
		{"shown, first start", true, sidebarWidth, 0},
		{"shown, second start", true, sidebarWidth + 4, 1},
		{"shown, third end", true, sidebarWidth + 13, 2},
		{"shown, gap", true, sidebarWidth + 3, 1},
		{"shown, past the last", true, sidebarWidth + 14, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newSidebarHarness(100, 30)
			if !tt.sidebar {
				h.prefix("e")
			}
			h.prefix("t")
			h.prefix("t")
			h.m.tabs[2].name = "ab"
			h.m.active = 1
			h.press(tt.x, 0)
			if h.m.active != tt.active {
				t.Errorf("active = %d, want %d", h.m.active, tt.active)
			}
		})
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
	h.press(sidebarWidth+4, 0)
	if h.m.active != 1 {
		t.Errorf("active = %d, want 1", h.m.active)
	}
}

func TestFrameFitsWithManyNamedTabs(t *testing.T) {
	for _, w := range []int{100, 44, 43, 13, 5} {
		for _, sidebar := range []bool{true, false} {
			h := newSidebarHarness(w, 10)
			if !sidebar {
				h.prefix("e")
			}
			for range 8 {
				h.prefix("t")
				h.m.tabs[h.m.active].name = "a long tab name"
			}
			f := h.frame()
			if got := lipgloss.Height(f); got != 10 {
				t.Errorf("w=%d sidebar=%v: height = %d", w, sidebar, got)
			}
			if got := lipgloss.Width(f); got > w {
				t.Errorf("w=%d sidebar=%v: width = %d", w, sidebar, got)
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
	if want := " 1 work   2   3 second "; !strings.Contains(line, want) {
		t.Errorf("bar = %q, want it to contain %q", line, want)
	}
}

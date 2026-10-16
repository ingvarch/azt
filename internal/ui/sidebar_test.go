package ui

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/data"
)

var testSubs = []azure.Subscription{
	{ID: "sub-1", Name: "Dev", State: "Enabled"},
	{ID: "sub-2", Name: "Prod", State: "Enabled"},
}

// loadSubs gives the sidebar two subscriptions.
func (h *harness) loadSubs() {
	h.send(data.ResultMsg{Key: data.Key{Kind: data.Subscriptions}, Data: testSubs})
}

func (h *harness) sidebarTitles() []string {
	stack, ok := h.m.sidebar.Stack()
	if !ok {
		return nil
	}
	return titles(stack)
}

func TestSidebarShownAtStart(t *testing.T) {
	h := newSidebarHarness(100, 30)
	lines := strings.Split(h.frame(), "\n")
	if !strings.HasPrefix(lines[0], "┌ Explorer ") {
		t.Errorf("top border = %q", lines[0])
	}
	if got := []rune(lines[1]); string(got[defaultSidebarWidth-1:defaultSidebarWidth+1]) != "│┌" {
		t.Errorf("sidebar does not end at column %d: %q", defaultSidebarWidth, lines[1])
	}
	if got := []rune(lines[len(lines)-2]); got[0] != '└' || string(got[defaultSidebarWidth-1:defaultSidebarWidth+1]) != "┘└" {
		t.Errorf("sidebar bottom border does not line up with the panes: %q", lines[len(lines)-2])
	}
	if r := h.pane(0); r.w != 66 || r.h != 26 {
		t.Errorf("pane inner size = %dx%d, want 66x26", r.w, r.h)
	}
	h.prefix("v")
	if a, b := h.pane(0), h.pane(1); a.w != 32 || b.w != 32 {
		t.Errorf("split inner widths = %d, %d, want 32 each", a.w, b.w)
	}
}

func TestSidebarNotDrawnInNarrowWindow(t *testing.T) {
	h := newSidebarHarness(minSidebarWidth+minPaneWidth-1, 30)
	if f := h.frame(); strings.Contains(f, "Explorer") {
		t.Errorf("narrow frame shows the sidebar:\n%s", f)
	}
	if got := h.pane(0).w; got != minSidebarWidth+minPaneWidth-3 {
		t.Errorf("pane inner width = %d, want the whole window", got)
	}
	h.send(tea.WindowSizeMsg{Width: minSidebarWidth + minPaneWidth, Height: 30})
	if f := h.frame(); !strings.Contains(f, "Explorer") {
		t.Errorf("frame at the minimum width lacks the sidebar:\n%s", f)
	}
	if got := h.pane(0).w; got != minPaneWidth-2 {
		t.Errorf("pane inner width = %d, want %d", got, minPaneWidth-2)
	}
}

func TestNarrowWindowReturnsFocusToPane(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	h.send(tea.WindowSizeMsg{Width: 20, Height: 30})
	h.send(key("x"))
	if got := h.pane(0).keys; len(got) != 1 {
		t.Errorf("pane keys = %v, want x", got)
	}
}

func TestPrefixEToggleResizesPanes(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("e")
	if f := h.frame(); strings.Contains(f, "Explorer") {
		t.Error("sidebar still drawn after hide")
	}
	if got := h.pane(0).w; got != 98 {
		t.Errorf("hidden: pane width = %d, want 98", got)
	}
	h.prefix("e")
	if f := h.frame(); !strings.Contains(f, "Explorer") {
		t.Error("sidebar not drawn after show")
	}
	if got := h.pane(0).w; got != 66 {
		t.Errorf("shown: pane width = %d, want 66", got)
	}
}

func TestHidingFocusedSidebarReturnsFocus(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	h.prefix("e")
	h.send(key("x"))
	if got := h.pane(0).keys; len(got) != 1 {
		t.Errorf("pane keys = %v, want x", got)
	}
}

func TestPrefixHFocusesSidebarFromLeftmostPane(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	h.send(key("x"))
	if got := h.pane(0).keys; len(got) != 0 {
		t.Errorf("pane got keys %v while the sidebar has focus", got)
	}
}

func TestPrefixHMovesToLeftNeighborFirst(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v") // focus on pane 1
	h.prefix("h")
	if got := h.m.tabs[0].focus; got != layout.PaneID(0) || h.m.sidebarFocused {
		t.Errorf("focus = pane %d, sidebar %v; want pane 0", got, h.m.sidebarFocused)
	}
	h.prefix("h")
	if !h.m.sidebarFocused {
		t.Error("second h did not focus the sidebar")
	}
}

func TestPrefixHDoesNothingWithHiddenSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("e")
	h.prefix("h")
	if h.m.sidebarFocused {
		t.Error("hidden sidebar got focus")
	}
}

func TestPrefixLReturnsToPane(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v")
	h.prefix("h")
	h.prefix("h") // sidebar
	h.prefix("l")
	if h.m.sidebarFocused || h.m.tabs[0].focus != 0 {
		t.Errorf("focus: sidebar %v, pane %d; want pane 0", h.m.sidebarFocused, h.m.tabs[0].focus)
	}
	h.send(key("x"))
	if got := h.pane(0).keys; len(got) != 1 {
		t.Errorf("pane keys = %v, want x", got)
	}
}

func TestKeysReachSidebarNotPane(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	h.prefix("h")
	if got := h.sidebarTitles(); len(got) != 2 || got[1] != "Dev" {
		t.Fatalf("selected = %v, want Dev", got)
	}
	h.send(key("j"))
	if got := h.sidebarTitles(); got[1] != "Prod" {
		t.Errorf("j did not move the selection: %v", got)
	}
	h.send(key("esc"))
	if got := h.pane(0).keys; len(got) != 0 {
		t.Errorf("pane got keys %v", got)
	}
}

func TestSidebarKeepsQuitAndPrefix(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	if !isQuit(h.send(key("q"))) || !isQuit(h.send(key("ctrl+c"))) {
		t.Error("q or ctrl+c lost its meaning in the sidebar")
	}
	h.send(key("ctrl+w"))
	if h.m.mode != modePrefix {
		t.Error("ctrl+w did not start the prefix")
	}
}

func TestEnterInSidebarReplacesPaneStack(t *testing.T) {
	h := newSidebarHarness(140, 30)
	h.send(key("enter")) // pane stack [1 2]
	h.loadSubs()
	h.prefix("h")
	h.send(key("j"))
	cmd := h.send(key("enter"))

	if h.m.sidebarFocused {
		t.Error("focus stayed in the sidebar")
	}
	if got := strings.Join(titles(h.m.stacks[0]), pathSep); got != "Subscriptions"+pathSep+"Prod" {
		t.Errorf("stack = %q", got)
	}
	want := []tea.Msg{
		data.FetchMsg{Key: data.Key{Kind: data.Subscriptions}},
		data.FetchMsg{Key: data.Key{Kind: data.ResourceGroups, Subscription: "sub-2"}},
	}
	var got []tea.Msg
	for _, msg := range collect(cmd) {
		if _, ok := msg.(data.FetchMsg); ok {
			got = append(got, msg)
		}
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Init messages = %v, want %v", got, want)
	}
	if !strings.Contains(h.frame(), "Subscriptions › Prod") {
		t.Errorf("pane border lacks the new path:\n%s", h.frame())
	}
}

func TestEnterInSidebarWithoutNodeDoesNothing(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	if cmd := h.send(key("enter")); cmd != nil || !h.m.sidebarFocused {
		t.Error("enter on an empty tree changed state")
	}
	if got := h.pane(0).keys; len(got) != 0 {
		t.Errorf("enter reached the pane: %v", got)
	}
}

func TestPrefixKeysLimitedInSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v")
	h.prefix("h")
	h.prefix("h") // sidebar
	for _, k := range []string{"v", "s", "q", "h", "j", "k", "z", "=", ">", "x"} {
		h.prefix(k)
		if len(h.m.stacks) != 2 || !h.m.sidebarFocused || h.m.tabs[0].zoomed {
			t.Fatalf("prefix %s changed the layout: panes %d, sidebar %v", k, len(h.m.stacks), h.m.sidebarFocused)
		}
	}
	h.prefix("t")
	if len(h.m.tabs) != 2 {
		t.Error("t does not work in the sidebar")
	}
	h.prefix("p")
	if h.m.active != 0 {
		t.Error("p does not work in the sidebar")
	}
	h.prefix("2")
	if h.m.active != 1 {
		t.Error("2 does not work in the sidebar")
	}
	h.prefix("n")
	if h.m.active != 0 {
		t.Error("n does not work in the sidebar")
	}
	h.send(key("ctrl+w"))
	h.send(key("esc"))
	if h.m.mode == modePrefix {
		t.Error("esc did not cancel the prefix")
	}
}

func TestResultReachesSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	if !strings.Contains(h.frame(), "Loading subscriptions") {
		t.Fatalf("no loading text:\n%s", h.frame())
	}
	h.loadSubs()
	if f := h.frame(); !strings.Contains(f, "▸ Dev") || !strings.Contains(f, "▸ Prod") {
		t.Errorf("sidebar lacks the subscriptions:\n%s", f)
	}
}

func TestSidebarFetchGoesThroughCache(t *testing.T) {
	h := newSidebarHarness(100, 30)
	k := data.Key{Kind: data.Subscriptions}
	for _, msg := range collect(h.m.Init()) {
		if f, ok := msg.(data.FetchMsg); ok {
			for _, m := range collect(h.send(f)) {
				h.send(m)
			}
		}
	}
	if len(h.calls) != 1 || h.calls[0] != k {
		t.Errorf("fetcher calls = %v, want one for %v", h.calls, k)
	}
}

func TestSidebarBorderColors(t *testing.T) {
	focus := strings.TrimSuffix(lipgloss.NewStyle().Foreground(focusColor).Render("┌"), "\x1b[m")
	gray := strings.TrimSuffix(lipgloss.NewStyle().Foreground(borderColor).Render("┌"), "\x1b[m")
	h := newSidebarHarness(100, 30)
	// the sidebar corner is on line 0, the pane corner on line 1
	corners := func() (side, pane string) {
		lines := strings.Split(h.raw(), "\n")
		return lines[0], lines[1]
	}
	check := func(step, sideColor, paneColor string) {
		t.Helper()
		side, pane := corners()
		if !strings.Contains(side, sideColor) || !strings.Contains(pane, paneColor) {
			t.Errorf("%s: sidebar line %q, pane line %q", step, side, pane)
		}
	}

	check("pane focused", gray, focus)
	h.prefix("h")
	check("sidebar focused", focus, gray)
	h.prefix("l")
	check("focus returned to the pane", gray, focus)
}

func TestSidebarSelectionStyleFollowsFocus(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	attrs := func() []string {
		m := regexp.MustCompile("\x1b\\[([0-9;]*)m▸ Dev").FindStringSubmatch(h.raw())
		if m == nil {
			t.Fatal("selected row not found")
		}
		return strings.Split(m[1], ";")
	}
	if got := attrs(); !slices.Contains(got, "1") || slices.Contains(got, "7") {
		t.Errorf("unfocused selection attributes = %v, want bold only", got)
	}
	h.prefix("h")
	if got := attrs(); !slices.Contains(got, "7") || slices.Contains(got, "1") {
		t.Errorf("focused selection attributes = %v, want reverse only", got)
	}
}

func TestSidebarStatusBar(t *testing.T) {
	h := newSidebarHarness(200, 30)
	h.prefix("h")
	lines := strings.Split(h.frame(), "\n")
	status := lines[len(lines)-1]
	for _, want := range []string{"NORMAL", "j/k move  l expand  h collapse  r refresh  enter open  ^w l panes  ^w e hide  : command  ? help  q quit"} {
		if !strings.Contains(status, want) {
			t.Errorf("status %q lacks %q", status, want)
		}
	}
	h.send(key("ctrl+w"))
	lines = strings.Split(h.frame(), "\n")
	status = lines[len(lines)-1]
	if !strings.Contains(status, "CTRL-W") || !strings.Contains(status, "l/→ panes  e hide  </> width  = default width  t tab  n/p/1-9 switch  , rename  ? help  esc cancel") {
		t.Errorf("prefix status = %q", status)
	}
	if strings.Contains(status, "v/s split") {
		t.Errorf("prefix status lists pane keys while the sidebar has focus: %q", status)
	}
}

func TestSidebarScrollsToLastRowInsideBorder(t *testing.T) {
	subs := make([]azure.Subscription, 40)
	for i := range subs {
		subs[i] = azure.Subscription{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("sub-name-%d", i)}
	}
	last := subs[len(subs)-1].Name
	h := newSidebarHarness(100, 20)
	h.send(data.ResultMsg{Key: data.Key{Kind: data.Subscriptions}, Data: subs})
	h.prefix("h")
	for range len(subs) - 1 {
		h.send(key("j"))
	}
	check := func(step string) {
		t.Helper()
		if f := h.frame(); !strings.Contains(f, last) {
			t.Errorf("%s: selected last row %q is not drawn:\n%s", step, last, f)
		}
	}
	check("start")
	h.send(tea.WindowSizeMsg{Width: 100, Height: 12})
	check("resized")
	h.prefix("e")
	h.prefix("e")
	check("hidden and shown")
}

func TestFrameFitsWithSidebar(t *testing.T) {
	setups := map[string]func(h *harness){
		"visible":  func(h *harness) { h.loadSubs() },
		"focused":  func(h *harness) { h.loadSubs(); h.prefix("h") },
		"splits":   func(h *harness) { h.prefix("v"); h.prefix("s") },
		"zoomed":   func(h *harness) { h.prefix("v"); h.prefix("z") },
		"prefix":   func(h *harness) { h.prefix("h"); h.send(key("ctrl+w")) },
		"expanded": func(h *harness) { h.loadSubs(); h.prefix("h"); h.send(key("l")) },
	}
	sizes := [][2]int{{140, 30}, {44, 30}, {43, 30}, {45, 31}, {44, 5}, {44, 3}, {30, 8}, {13, 5}, {5, 3}}
	for name, setup := range setups {
		for _, s := range sizes {
			h := newSidebarHarness(s[0], s[1])
			setup(h)
			f := h.frame()
			if got := lipgloss.Height(f); got != s[1] {
				t.Errorf("%s %v: height = %d", name, s, got)
			}
			if got := lipgloss.Width(f); got > s[0] {
				t.Errorf("%s %v: width = %d", name, s, got)
			}
			status := f[strings.LastIndex(f, "\n")+1:]
			if s[0] >= 8 && !strings.Contains(status, "NORMAL") && !strings.Contains(status, "CTRL-W") {
				t.Errorf("%s %v: last line %q is not the status bar", name, s, status)
			}
		}
	}
}

func TestSidebarWrapsTextInsideItsBorder(t *testing.T) {
	h := newSidebarHarness(100, 30)
	words := strings.Fields("alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima")
	h.send(data.ResultMsg{Key: data.Key{Kind: data.Subscriptions}, Err: errors.New(strings.Join(words, " "))})

	frame := h.frame()
	for _, w := range words {
		if !strings.Contains(frame, w) {
			t.Errorf("frame lacks %q:\n%s", w, frame)
		}
	}
}

func TestPrefixArrowsWorkLikeHJKL(t *testing.T) {
	h := newSidebarHarness(120, 40)
	h.prefix("v") // panes 0 | 1, focus 1
	steps := []struct {
		key     string
		pane    int
		sidebar bool
	}{
		{"left", 0, false},
		{"left", 0, true},
		{"right", 0, false},
		{"s", 2, false}, // pane 2 below pane 0
		{"up", 0, false},
		{"down", 2, false},
		{"right", 1, false},
	}
	for i, s := range steps {
		h.prefix(s.key)
		if got := int(h.m.tabs[0].focus); got != s.pane || h.m.sidebarFocused != s.sidebar {
			t.Fatalf("step %d (%s): focus pane %d sidebar %v, want pane %d sidebar %v",
				i, s.key, got, h.m.sidebarFocused, s.pane, s.sidebar)
		}
	}
}

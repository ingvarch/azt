package ui

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

func (h *harness) press(x, y int) tea.Cmd {
	return h.send(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func (h *harness) motion(x, y int) tea.Cmd {
	return h.send(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func (h *harness) release(x, y int) tea.Cmd {
	return h.send(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func (h *harness) wheel(x, y int, b tea.MouseButton) tea.Cmd {
	return h.send(tea.MouseWheelMsg{X: x, Y: y, Button: b})
}

// focused returns the focused pane of the active tab.
func (h *harness) focused() layout.PaneID { return h.m.tabs[h.m.active].focus }

func TestViewSetsMouseMode(t *testing.T) {
	h := newHarness(100, 30)
	if got := h.m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Errorf("MouseMode = %v, want cell motion", got)
	}
}

func TestPressOnTabResizesViews(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.send(tea.WindowSizeMsg{Width: 80, Height: 20})
	h.prefix("v")
	h.press(0, 0)
	if got := h.pane(0).w; got != 78 {
		t.Errorf("first tab pane width = %d, want 78", got)
	}
}

func TestHelpClosesOnPress(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v")
	h.send(key("?"))
	h.press(40, 5) // pane 0, away from the focused pane 1
	if h.m.mode != modeNormal {
		t.Errorf("mode = %v, want normal", h.m.mode)
	}
	if h.focused() != 1 {
		t.Errorf("focus = %d, want 1", h.focused())
	}
	if len(h.pane(0).clicks) != 0 {
		t.Error("the press reached the view under the help")
	}
}

func TestHelpStaysOnOtherMouse(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.send(key("?"))
	h.motion(40, 5)
	h.release(40, 5)
	h.wheel(40, 5, tea.MouseWheelDown)
	if h.m.mode != modeHelp {
		t.Errorf("mode = %v, want help", h.m.mode)
	}
	if len(h.pane(0).keys) != 0 {
		t.Errorf("wheel reached the view: %v", h.pane(0).keys)
	}
}

func TestTypingIgnoresMouse(t *testing.T) {
	for _, md := range []mode{modeFilter, modeRename} {
		h := newHarness(100, 30)
		h.prefix("t")
		h.prefix("v")
		h.m.startInput(md, "x")
		h.press(0, 0)
		h.press(10, 5)
		h.wheel(10, 5, tea.MouseWheelDown)
		if h.m.mode != md || h.m.active != 1 || h.m.input.Value() != "x" {
			t.Errorf("mode %v: mode=%v active=%d input=%q", md, h.m.mode, h.m.active, h.m.input.Value())
		}
		for _, f := range h.fakes() {
			if len(f.keys) != 0 || len(f.clicks) != 0 {
				t.Errorf("mode %v: view got keys %v clicks %v", md, f.keys, f.clicks)
			}
		}
	}
}

func TestPressCancelsPrefix(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.send(key("ctrl+w"))
	h.press(10, 5) // pane 0, not focused
	if h.m.mode != modeNormal {
		t.Errorf("mode = %v, want normal", h.m.mode)
	}
	if h.focused() != 1 || len(h.pane(0).clicks) != 0 {
		t.Errorf("the press did more than cancel: focus %d, clicks %v", h.focused(), h.pane(0).clicks)
	}
}

func TestPressFocusesPaneAndClicksInside(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v") // pane 0 at x 32..65, pane 1 at x 66..99, both from y 1
	h.press(40, 5)
	if h.focused() != 0 || h.m.sidebarFocused {
		t.Errorf("focus = %d, sidebar %v", h.focused(), h.m.sidebarFocused)
	}
	if got := h.pane(0).clicks; !slices.Equal(got, [][2]int{{7, 3}}) {
		t.Errorf("clicks = %v, want [[7 3]]", got)
	}
	h.press(90, 5)
	if h.focused() != 1 {
		t.Errorf("focus = %d, want 1", h.focused())
	}
}

func TestPressPaneLosesSidebarFocus(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	if !h.m.sidebarFocused {
		t.Fatal("setup: sidebar not focused")
	}
	h.press(50, 5)
	if h.m.sidebarFocused {
		t.Error("sidebar kept focus")
	}
}

func TestPressOnPaneBorderFocusesWithoutClick(t *testing.T) {
	tests := []struct {
		name         string
		x, y, wantAt int
	}{
		{"top", 5, 1, 0},
		{"left", 0, 5, 0},
		{"right", 99, 5, 0},
		{"bottom", 5, 28, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(100, 30)
			h.prefix("s") // pane 0 on top, full width
			h.press(tt.x, tt.y)
			if h.focused() != layout.PaneID(tt.wantAt) || len(h.pane(layout.PaneID(tt.wantAt)).clicks) != 0 {
				t.Errorf("focus = %d, clicks %v; want focus %d, no clicks", h.focused(), h.pane(layout.PaneID(tt.wantAt)).clicks, tt.wantAt)
			}
		})
	}
}

func TestBoundaryDragResizes(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v") // focus 1; A is x 0..49, B is x 50..99
	h.press(49, 5)
	if h.focused() != 1 {
		t.Errorf("focus = %d after a boundary press, want 1", h.focused())
	}
	if len(h.pane(0).clicks)+len(h.pane(1).clicks) != 0 {
		t.Error("boundary press clicked a view")
	}
	h.motion(30, 6)
	if a, b := h.pane(0).w, h.pane(1).w; a != 28 || b != 68 {
		t.Errorf("inner widths = %d, %d, want 28, 68", a, b)
	}
	h.release(30, 6)
	h.motion(10, 6)
	if a, b := h.pane(0).w, h.pane(1).w; a != 28 || b != 68 {
		t.Errorf("motion after release resized the panes to %d, %d", a, b)
	}
}

func TestBoundaryDragClamps(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.press(49, 5)
	h.motion(0, 5)
	if r := h.m.tabs[0].root.Ratio; r != layout.MinRatio {
		t.Errorf("ratio = %v, want %v", r, layout.MinRatio)
	}
}

func TestBoundaryDragWithSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v") // area x 32..99, A is 32..65
	h.press(65, 5)
	h.motion(32+17, 5)
	if a, b := h.pane(0).w, h.pane(1).w; a != 15 || b != 49 {
		t.Errorf("inner widths = %d, %d, want 15, 49 (boundary at x=49)", a, b)
	}
}

func TestBoundaryDragVertical(t *testing.T) {
	h := newHarness(100, 30) // area y 1..28
	h.prefix("s")            // A is y 1..14
	h.press(10, 14)
	h.motion(10, 8)
	if a, b := h.pane(0).h, h.pane(1).h; a != 5 || b != 19 {
		t.Errorf("inner heights = %d, %d, want 5, 19", a, b)
	}
}

func TestDragNestedSplitChangesOnlyIt(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.prefix("s") // B is split: pane 1 above pane 2, from x 50
	h.press(75, 14)
	h.motion(75, 8)
	if w := h.pane(0).w; w != 48 {
		t.Errorf("pane 0 inner width = %d, want 48 (outer split untouched)", w)
	}
	if a, b := h.pane(1).h, h.pane(2).h; a != 5 || b != 19 {
		t.Errorf("inner heights = %d, %d, want 5, 19", a, b)
	}
}

func TestNoBoundaryOnZoomedTab(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.prefix("z")
	h.press(49, 5)
	h.motion(30, 5)
	if r := h.m.tabs[0].root.Ratio; r != 0.5 {
		t.Errorf("ratio = %v, want 0.5", r)
	}
	if len(h.pane(1).clicks) != 1 {
		t.Errorf("zoomed press did not reach the pane: %v", h.pane(1).clicks)
	}
}

func TestMotionNeedsLeftDrag(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.motion(30, 5) // no press before
	h.press(49, 5)
	h.send(tea.MouseMotionMsg{X: 30, Y: 5, Button: tea.MouseRight})
	if r := h.m.tabs[0].root.Ratio; r != 0.5 {
		t.Errorf("ratio = %v, want 0.5", r)
	}
}

func TestOtherButtonsDoNothing(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.send(tea.MouseClickMsg{X: 10, Y: 5, Button: tea.MouseRight})
	h.send(tea.MouseClickMsg{X: 9, Y: 0, Button: tea.MouseMiddle})
	h.send(tea.MouseClickMsg{X: 49, Y: 5, Button: tea.MouseRight})
	h.motion(30, 5)
	if h.focused() != 1 || h.m.active != 0 || h.m.tabs[0].root.Ratio != 0.5 {
		t.Errorf("focus %d active %d ratio %v", h.focused(), h.m.active, h.m.tabs[0].root.Ratio)
	}
}

func TestDoublePress(t *testing.T) {
	const x, y = 10, 5
	tests := []struct {
		name       string
		x2, y2     int
		gap        time.Duration
		wantDepth  int // stack depth of pane 0 afterwards
		wantFocus0 bool
	}{
		{"same row within 400 ms", x + 3, y, 399 * time.Millisecond, 2, true},
		{"after 400 ms", x, y, 401 * time.Millisecond, 1, true},
		{"another row", x, y + 1, 100 * time.Millisecond, 1, true},
		{"another pane", 60, y, 100 * time.Millisecond, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(100, 30)
			h.prefix("v")
			h.press(x, y)
			h.clock = h.clock.Add(tt.gap)
			h.press(tt.x2, tt.y2)
			if got := len(h.m.stacks[0]); got != tt.wantDepth {
				t.Errorf("pane 0 stack depth = %d, want %d", got, tt.wantDepth)
			}
			if (h.focused() == 0) != tt.wantFocus0 {
				t.Errorf("focus = %d", h.focused())
			}
		})
	}
}

func TestTripleDoesNotOpenTwice(t *testing.T) {
	h := newHarness(100, 30)
	h.press(10, 5)
	h.press(10, 5)
	h.press(10, 5)
	if got := len(h.m.stacks[0]); got != 2 {
		t.Errorf("stack depth = %d, want 2", got)
	}
}

func TestDoublePressOnBorderDoesNothing(t *testing.T) {
	h := newHarness(100, 30)
	h.press(10, 1)
	h.press(10, 1)
	if got := len(h.m.stacks[0]); got != 1 {
		t.Errorf("stack depth = %d, want 1", got)
	}
}

func TestPressOnSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	h.press(5, 2) // second row: Prod
	if !h.m.sidebarFocused {
		t.Error("sidebar not focused")
	}
	if got := h.sidebarTitles(); !slices.Equal(got, []string{"Subscriptions", "Prod"}) {
		t.Errorf("selected stack = %v", got)
	}
	h.press(0, 2) // the border
	if !h.m.sidebarFocused {
		t.Error("border press dropped the sidebar focus")
	}
}

func TestSidebarMarkerClickFetches(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	cmd := h.press(1, 1) // marker cell of Dev
	var got []data.FetchMsg
	for _, msg := range collect(cmd) {
		if f, ok := msg.(data.FetchMsg); ok {
			got = append(got, f)
		}
	}
	want := data.FetchMsg{Key: data.Key{Kind: data.ResourceGroups, Subscription: "sub-1"}}
	if len(got) != 1 || got[0] != want {
		t.Errorf("fetches = %+v, want [%+v]", got, want)
	}
}

func TestDoublePressOnSidebarOpens(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	h.press(10, 2)
	h.press(10, 2)
	if h.m.sidebarFocused {
		t.Error("sidebar kept focus")
	}
	if got := titles(h.m.stacks[0]); !slices.Equal(got, []string{"Subscriptions", "Prod"}) {
		t.Errorf("pane stack = %v", got)
	}
}

func TestDoublePressSidebarThenPaneIsNotDouble(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	h.press(10, 2)
	h.press(40, 3)
	if got := len(h.m.stacks[0]); got != 1 {
		t.Errorf("stack depth = %d, want 1", got)
	}
}

func TestSidebarHiddenPressHitsPane(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("e")
	h.press(5, 5)
	if h.m.sidebarFocused || len(h.pane(0).clicks) != 1 {
		t.Errorf("sidebar %v clicks %v", h.m.sidebarFocused, h.pane(0).clicks)
	}
}

func TestWheelReachesViewUnderPointer(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v") // focus on pane 1
	h.wheel(10, 5, tea.MouseWheelDown)
	h.wheel(10, 5, tea.MouseWheelUp)
	if got := h.pane(0).keys; !slices.Equal(got, []string{"down", "up"}) {
		t.Errorf("pane 0 keys = %v", got)
	}
	if got := h.pane(1).keys; len(got) != 0 {
		t.Errorf("focused pane got keys %v", got)
	}
	if h.focused() != 1 {
		t.Errorf("focus = %d, want 1", h.focused())
	}
}

func TestWheelOverSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	h.wheel(5, 5, tea.MouseWheelDown)
	if got := h.sidebarTitles(); !slices.Equal(got, []string{"Subscriptions", "Prod"}) {
		t.Errorf("selected stack = %v", got)
	}
	if h.m.sidebarFocused || len(h.pane(0).keys) != 0 {
		t.Errorf("sidebar focused %v, pane keys %v", h.m.sidebarFocused, h.pane(0).keys)
	}
}

func TestWheelElsewhereDoesNothing(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.loadSubs()
	h.wheel(40, 0, tea.MouseWheelDown)  // tab bar
	h.wheel(40, 29, tea.MouseWheelDown) // status bar
	h.wheel(40, 5, tea.MouseWheelLeft)
	h.wheel(40, 5, tea.MouseWheelRight)
	if len(h.pane(0).keys) != 0 {
		t.Errorf("pane keys = %v", h.pane(0).keys)
	}
	if got := h.sidebarTitles(); !slices.Equal(got, []string{"Subscriptions", "Dev"}) {
		t.Errorf("sidebar moved: %v", got)
	}
}

func TestMouseReachesNoBroadcast(t *testing.T) {
	h := newHarness(100, 30)
	h.send(tea.MouseMotionMsg{X: 10, Y: 5})
	h.release(10, 5)
	if len(h.pane(0).keys) != 0 || h.pane(0).pings != 0 {
		t.Error("mouse message reached a view")
	}
}

func TestPressAfterLostReleaseEndsDrag(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.press(49, 5)
	h.press(10, 5) // the release never came
	h.motion(30, 5)
	if r := h.m.tabs[0].root.Ratio; r != 0.5 {
		t.Errorf("ratio = %v, want 0.5", r)
	}
}

func TestPressOnUndrawnSidebarDoesNothing(t *testing.T) {
	h := newSidebarHarness(100, 2) // the sidebar is one row high
	h.press(5, 0)
	if h.m.sidebarFocused {
		t.Error("the press focused the undrawn sidebar")
	}
}

func TestMouseOnUndrawnPaneDoesNothing(t *testing.T) {
	tests := []struct {
		name  string
		build func() *harness
		x, y  int
	}{
		{"too short", func() *harness {
			h := newSidebarHarness(100, 3)
			h.prefix("h") // focus the sidebar
			return h
		}, 50, 1},
		{"too narrow", func() *harness { return newHarness(1, 30) }, 0, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.build()
			h.press(tt.x, tt.y)
			h.wheel(tt.x, tt.y, tea.MouseWheelDown)
			if len(h.pane(0).clicks) != 0 || len(h.pane(0).keys) != 0 {
				t.Errorf("clicks %v, keys %v", h.pane(0).clicks, h.pane(0).keys)
			}
			if got := h.m.sidebarFocused; got != (tt.name == "too short") {
				t.Errorf("sidebar focused = %v", got)
			}
		})
	}
}

// TestDefaultClockDetectsDoublePress leaves the clock at its default: two
// immediate presses on one row are a double press.
func TestDefaultClockDetectsDoublePress(t *testing.T) {
	restored := func(t *testing.T, h *harness) Model {
		m, err := Restore(oneTab(leaf("subscriptions")), h.newFake, h.fetch)
		if err != nil {
			t.Fatal(err)
		}
		m.stacks[0] = []views.View{h.newFake()}
		return m
	}
	tests := []struct {
		name  string
		build func(t *testing.T, h *harness) Model
	}{
		{"New", func(_ *testing.T, h *harness) Model { return New(h.newFake, h.fetch) }},
		{"Restore", restored},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &harness{}
			h.m = tt.build(t, h)
			h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
			h.press(50, 5) // right of the sidebar, if shown
			h.press(50, 5)
			if got := len(h.m.stacks[0]); got != 2 {
				t.Errorf("stack depth = %d, want 2", got)
			}
		})
	}
}

func TestKeyEndsDrag(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.prefix("t")
	h.prefix("v")
	h.prefix("p") // tab 1
	h.press(49, 5)
	h.prefix("n") // tab 2 while the button is held
	h.motion(30, 5)
	for i, tb := range h.m.tabs {
		if tb.root.Ratio != 0.5 {
			t.Errorf("tab %d ratio = %v, want 0.5", i+1, tb.root.Ratio)
		}
	}
}

func TestReleaseEndsDragInAnyMode(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.press(49, 5)
	h.m.mode = modeHelp // a mode change that bypasses key()
	h.release(49, 5)
	h.press(10, 5) // closes help
	h.motion(30, 5)
	if r := h.m.tabs[0].root.Ratio; r != 0.5 {
		t.Errorf("ratio = %v, want 0.5", r)
	}
}

func TestDragIgnoresEmptyArea(t *testing.T) {
	tests := []struct {
		name  string
		split string
		press [2]int
		size  tea.WindowSizeMsg
		to    [2]int
	}{
		{"vertical", "s", [2]int{10, 14}, tea.WindowSizeMsg{Width: 100, Height: 2}, [2]int{10, 1}},
		{"horizontal", "v", [2]int{49, 5}, tea.WindowSizeMsg{Width: 0, Height: 30}, [2]int{0, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(100, 30)
			h.prefix(tt.split)
			h.press(tt.press[0], tt.press[1])
			h.send(tt.size)
			h.motion(tt.to[0], tt.to[1])
			if r := h.m.tabs[0].root.Ratio; r != 0.5 {
				t.Errorf("ratio = %v, want 0.5", r)
			}
		})
	}
}

func TestDoublePressNeedsARow(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.press(10, 2) // inner line 0 of pane 0: the fake's header
	h.clock = h.clock.Add(100 * time.Millisecond)
	h.press(10, 2)
	if got := len(h.m.stacks[0]); got != 1 {
		t.Errorf("double press off a row opened a view: depth %d", got)
	}
}

func TestOtherInputBreaksDoublePress(t *testing.T) {
	between := map[string]func(h *harness){
		"key":            func(h *harness) { h.send(key("x")) },
		"wheel":          func(h *harness) { h.wheel(10, 5, tea.MouseWheelDown) },
		"tab press":      func(h *harness) { h.press(1, 0) },
		"boundary press": func(h *harness) { h.press(49, 5); h.release(49, 5) },
	}
	for name, do := range between {
		h := newHarness(100, 30)
		h.prefix("v")
		h.press(10, 5)
		do(h)
		h.clock = h.clock.Add(100 * time.Millisecond)
		h.press(10, 5)
		if got := len(h.m.stacks[0]); got != 1 {
			t.Errorf("%s between presses: the second press opened a view (depth %d)", name, got)
		}
	}
}

func TestDragPutsBoundaryUnderPointer(t *testing.T) {
	h := newHarness(26, 10)
	h.prefix("v") // A is x 0..12, B is x 13..25
	h.press(12, 5)
	h.motion(15, 5) // 15/26 alone gives int(26*ratio) == 14
	if a, b := h.pane(0).w, h.pane(1).w; a != 13 || b != 9 {
		t.Errorf("inner widths = %d, %d, want 13, 9 (boundary at x=15)", a, b)
	}
}

func TestDoublePressWithNothingToOpenClicksAgain(t *testing.T) {
	h := newHarness(100, 30)
	h.recs[0].noOpen = true
	h.press(10, 5)
	h.clock = h.clock.Add(100 * time.Millisecond)
	h.press(20, 5)
	if got := len(h.pane(0).clicks); got != 2 {
		t.Errorf("clicks = %d, want 2: the second press must reach the view", got)
	}
	if got := len(h.m.stacks[0]); got != 1 {
		t.Errorf("stack depth = %d, want 1", got)
	}
}

package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/views"
)

// wantSidebarEdge checks that the explorer's box ends at column w, where the panes start.
func wantSidebarEdge(t *testing.T, h *harness, w int) {
	t.Helper()
	lines := strings.Split(h.frame(), "\n")
	if got := []rune(lines[1]); string(got[w-1:w+1]) != "│┌" {
		t.Errorf("explorer does not end at column %d: %q", w, lines[1])
	}
	if got := []rune(lines[len(lines)-2]); string(got[w-1:w+1]) != "┘└" {
		t.Errorf("explorer bottom does not end at column %d: %q", w, lines[len(lines)-2])
	}
}

func TestSidebarKeysResizeExplorer(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	h.prefix(">")
	if got := h.m.sidebarWidth(); got != 36 {
		t.Fatalf("width after > = %d, want 36", got)
	}
	if got := h.m.paneArea(); got.X != 36 || got.W != 64 {
		t.Errorf("pane area = %+v, want X 36 W 64", got)
	}
	if got := h.pane(0).w; got != 62 {
		t.Errorf("pane inner width = %d, want 62", got)
	}
	wantSidebarEdge(t, h, 36)
	h.prefix("<")
	h.prefix("<")
	if got := h.m.sidebarWidth(); got != 28 {
		t.Errorf("width after << = %d, want 28", got)
	}
	wantSidebarEdge(t, h, 28)
	h.prefix("=")
	if got := h.m.sidebarWidth(); got != defaultSidebarWidth {
		t.Errorf("width after = is %d, want %d", got, defaultSidebarWidth)
	}
	if !h.m.sidebarFocused {
		t.Error("explorer lost focus")
	}
}

func TestSidebarKeysStopAtLimits(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	for range 10 {
		h.prefix("<")
	}
	if got := h.m.sidebarWidth(); got != minSidebarWidth {
		t.Errorf("narrowest = %d, want %d", got, minSidebarWidth)
	}
	for range 30 {
		h.prefix(">")
	}
	if got := h.m.sidebarWidth(); got != 100-minPaneWidth {
		t.Errorf("widest = %d, want %d", got, 100-minPaneWidth)
	}
	if got := h.pane(0).w; got != minPaneWidth-2 {
		t.Errorf("pane inner width = %d, want %d", got, minPaneWidth-2)
	}
	if got := h.m.Session().SidebarWidth; got != 100-minPaneWidth {
		t.Errorf("saved widest = %d, want %d", got, 100-minPaneWidth)
	}
	h.send(tea.WindowSizeMsg{Width: 200, Height: 30})
	if got := h.m.sidebarWidth(); got != 100-minPaneWidth {
		t.Errorf("widest after the window grew = %d, want %d", got, 100-minPaneWidth)
	}
}

func TestPaneKeysDoNotResizeExplorer(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v")
	h.prefix(">")
	if got := h.m.sidebarWidth(); got != defaultSidebarWidth {
		t.Errorf("explorer width = %d, want %d", got, defaultSidebarWidth)
	}
	if r := h.m.tabs[0].root.Ratio; r == 0.5 {
		t.Error("the pane ratio did not change")
	}
	h.prefix("=")
	if r := h.m.tabs[0].root.Ratio; r != 0.5 {
		t.Errorf("pane ratio after = is %v, want 0.5", r)
	}
	if got := h.m.sidebarWidth(); got != defaultSidebarWidth {
		t.Errorf("explorer width = %d, want %d", got, defaultSidebarWidth)
	}
}

func TestSidebarKeysDoNotTouchPanes(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v")
	h.prefix("z")
	h.press(5, 5)
	h.prefix(">")
	if !h.m.sidebarFocused || h.m.sidebarWidth() != 36 {
		t.Fatalf("setup: focused %v, width %d", h.m.sidebarFocused, h.m.sidebarWidth())
	}
	if !h.m.tabs[0].zoomed {
		t.Error("a sidebar resize unzoomed the tab")
	}
	if r := h.m.tabs[0].root.Ratio; r != 0.5 {
		t.Errorf("pane ratio = %v, want 0.5", r)
	}
}

func TestSidebarBorderDrag(t *testing.T) {
	for _, x := range []int{defaultSidebarWidth - 1, defaultSidebarWidth} {
		h := newSidebarHarness(100, 30)
		h.press(x, 5)
		if h.m.drag == nil {
			t.Fatalf("press at column %d did not start a drag", x)
		}
		h.motion(40, 5)
		if got := h.m.sidebarWidth(); got != 41 {
			t.Errorf("press %d: width after motion to 40 = %d, want 41", x, got)
		}
		if got := h.pane(0).w; got != 100-41-2 {
			t.Errorf("press %d: pane inner width = %d", x, got)
		}
		wantSidebarEdge(t, h, 41)
		h.release(40, 5)
		h.motion(50, 5)
		if got := h.m.sidebarWidth(); got != 41 {
			t.Errorf("press %d: width after release and motion = %d, want 41", x, got)
		}
	}
}

func TestSidebarBorderDragClamps(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.press(defaultSidebarWidth-1, 5)
	h.motion(3, 5)
	if got := h.m.sidebarWidth(); got != minSidebarWidth {
		t.Errorf("left past the limit: %d, want %d", got, minSidebarWidth)
	}
	if got := h.m.Session().SidebarWidth; got != minSidebarWidth {
		t.Errorf("saved width after the left overshoot = %d, want %d", got, minSidebarWidth)
	}
	h.motion(99, 5)
	if got := h.m.sidebarWidth(); got != 100-minPaneWidth {
		t.Errorf("right past the limit: %d, want %d", got, 100-minPaneWidth)
	}
	if got := h.m.Session().SidebarWidth; got != 100-minPaneWidth {
		t.Errorf("saved width after the right overshoot = %d, want %d", got, 100-minPaneWidth)
	}
	h.send(tea.WindowSizeMsg{Width: 200, Height: 30})
	if got := h.m.sidebarWidth(); got != 100-minPaneWidth {
		t.Errorf("width after the window grew = %d, want %d", got, 100-minPaneWidth)
	}
}

func TestPressBesideSidebarBorderDoesNotDrag(t *testing.T) {
	for _, x := range []int{defaultSidebarWidth - 2, defaultSidebarWidth + 1} {
		h := newSidebarHarness(100, 30)
		h.press(x, 5)
		if h.m.drag != nil {
			t.Errorf("press at column %d started a drag", x)
		}
		h.motion(40, 5)
		if got := h.m.sidebarWidth(); got != defaultSidebarWidth {
			t.Errorf("press %d: width = %d", x, got)
		}
	}
}

func TestSidebarBorderDragKeepsFocus(t *testing.T) {
	for _, focused := range []bool{true, false} {
		for _, x := range []int{defaultSidebarWidth - 1, defaultSidebarWidth} {
			h := newSidebarHarness(100, 30)
			h.loadSubs()
			if focused {
				h.prefix("h")
			}
			h.press(x, 1)
			h.motion(40, 1)
			h.release(40, 1)
			if h.m.sidebarFocused != focused {
				t.Errorf("press %d, focused %v: focus is now %v", x, focused, h.m.sidebarFocused)
			}
			if len(h.pane(0).clicks) != 0 {
				t.Errorf("the drag clicked the pane: %v", h.pane(0).clicks)
			}
		}
	}
}

func TestSidebarBorderPressRows(t *testing.T) {
	for _, c := range []struct {
		x, y int
		drag bool
	}{
		{defaultSidebarWidth - 1, 1, true},
		{defaultSidebarWidth - 1, 28, true},
		{defaultSidebarWidth, 1, true},
		{defaultSidebarWidth, 28, true},
		{defaultSidebarWidth - 1, 0, false},
		{defaultSidebarWidth - 1, 29, false},
		{defaultSidebarWidth, 29, false},
	} {
		h := newSidebarHarness(100, 30)
		h.press(c.x, c.y)
		if got := h.m.drag != nil; got != c.drag {
			t.Errorf("press at (%d,%d): drag %v, want %v", c.x, c.y, got, c.drag)
		}
	}
	h := newSidebarHarness(100, 30)
	h.press(defaultSidebarWidth-1, 0)
	if !h.m.sidebarFocused {
		t.Error("a press on the tab bar above the explorer border did not focus the explorer")
	}
	h = newSidebarHarness(100, 30)
	h.press(defaultSidebarWidth, 0)
	if h.m.drag != nil {
		t.Error("a press on a tab label started a drag")
	}
}

func TestPaneSplitDragWinsAtSidebarCorner(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("s")
	for _, y := range []int{14, 15} {
		h.m.drag = nil
		h.press(defaultSidebarWidth, y)
		if h.m.drag == nil || h.m.drag.sidebar || h.m.drag.orient != layout.Vertical {
			t.Errorf("press at (%d,%d): drag %+v, want a vertical pane drag", defaultSidebarWidth, y, h.m.drag)
		}
	}
}

func TestSidebarKeyWidenStopsAtDrawnLimit(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	for range 3 {
		h.prefix(">")
	}
	h.send(tea.WindowSizeMsg{Width: 50, Height: 30})
	h.prefix(">")
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := h.m.sidebarWidth(); got != 44 {
		t.Errorf("width after > at the limit and growing = %d, want 44", got)
	}
}

func TestSidebarEqualsKeepsDefaultInNarrowWindow(t *testing.T) {
	h := newSidebarHarness(40, 30)
	h.prefix("h")
	h.prefix("=")
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := h.m.sidebarWidth(); got != defaultSidebarWidth {
		t.Errorf("width = %d, want %d", got, defaultSidebarWidth)
	}
	if got := h.m.Session().SidebarWidth; got != defaultSidebarWidth {
		t.Errorf("saved width = %d, want %d", got, defaultSidebarWidth)
	}
}

func TestSidebarBorderDragWorksOnZoomedTab(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v")
	h.prefix("z")
	h.press(defaultSidebarWidth-1, 5)
	h.motion(40, 5)
	if got := h.m.sidebarWidth(); got != 41 {
		t.Errorf("width = %d, want 41", got)
	}
}

func TestPaneDragStillWorksWithResizedSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("v")
	h.prefix("h") // the left pane
	h.prefix("h") // the explorer
	h.prefix(">") // explorer 36, area x 36..99, A is 36..67
	h.press(67, 5)
	if h.m.drag == nil || h.m.drag.sidebar {
		t.Fatal("press on the pane border did not start a pane drag")
	}
	h.motion(36+17, 5)
	if a := h.pane(0).w; a != 15 {
		t.Errorf("pane 0 inner width = %d, want 15", a)
	}
	if got := h.m.sidebarWidth(); got != 36 {
		t.Errorf("explorer width = %d, want 36", got)
	}
}

func TestSidebarWidthFollowsNarrowWindow(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	for range 3 {
		h.prefix(">")
	}
	if got := h.m.sidebarWidth(); got != 44 {
		t.Fatalf("width = %d, want 44", got)
	}
	h.send(tea.WindowSizeMsg{Width: 50, Height: 30})
	if got := h.m.sidebarWidth(); got != 50-minPaneWidth {
		t.Errorf("width in a window of 50 = %d, want %d", got, 50-minPaneWidth)
	}
	if got := h.pane(0).w; got != minPaneWidth-2 {
		t.Errorf("pane inner width = %d, want %d", got, minPaneWidth-2)
	}
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := h.m.sidebarWidth(); got != 44 {
		t.Errorf("width after the window grew = %d, want 44", got)
	}
	h.send(tea.WindowSizeMsg{Width: minSidebarWidth + minPaneWidth - 1, Height: 30})
	if h.m.sidebarDrawn() {
		t.Error("explorer drawn in a window too narrow for it")
	}
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := h.m.sidebarWidth(); got != 44 {
		t.Errorf("width after hiding = %d, want 44", got)
	}
}

func TestSidebarShrinksFromDrawnWidth(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	for range 3 {
		h.prefix(">")
	}
	h.send(tea.WindowSizeMsg{Width: 50, Height: 30})
	h.prefix("<")
	if got := h.m.sidebarWidth(); got != 50-minPaneWidth-sidebarResizeStep {
		t.Errorf("width = %d, want %d", got, 50-minPaneWidth-sidebarResizeStep)
	}
}

func TestSessionKeepsSidebarWidth(t *testing.T) {
	h := &harness{m: New(views.NewSubscriptions, nil)}
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.prefix("h")
	h.prefix(">")
	s := h.m.Session()
	if s.SidebarWidth != 36 {
		t.Fatalf("saved width = %d, want 36", s.SidebarWidth)
	}
	m, err := Restore(s, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.sidebarW != 36 {
		t.Errorf("restored width = %d, want 36", m.sidebarW)
	}
}

func TestRestoreSidebarWidthDefaultsAndClamps(t *testing.T) {
	for _, c := range []struct{ saved, want int }{
		{0, defaultSidebarWidth},
		{5, minSidebarWidth},
		{20, 20},
		{50, 50},
	} {
		s := oneTab(leaf("subscriptions"))
		s.SidebarWidth = c.saved
		m, err := Restore(s, views.NewSubscriptions, nil)
		if err != nil {
			t.Fatal(err)
		}
		if m.sidebarW != c.want {
			t.Errorf("saved %d: restored %d, want %d", c.saved, m.sidebarW, c.want)
		}
	}
}

func TestSidebarWidthHintsAndHelp(t *testing.T) {
	for _, want := range []string{"</> width", "= default width"} {
		if !strings.Contains(sidebarPrefixHints, want) {
			t.Errorf("hints lack %q: %q", want, sidebarPrefixHints)
		}
	}
	for _, keys := range []string{"ctrl-w < >", "ctrl-w ="} {
		if !helpHasKey("Explorer", keys) {
			t.Errorf("Explorer help lacks %q", keys)
		}
	}
}

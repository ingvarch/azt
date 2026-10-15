package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestInitAsksForTheBackgroundColor(t *testing.T) {
	h := newHarness(100, 30)
	if !slices.Contains(collect(h.m.Init()), tea.RequestBackgroundColor()) {
		t.Error("Init does not request the background color")
	}
}

// cellState is the styling of one cell, as the terminal would show it.
type cellState struct {
	bg, fg string
	flags  map[string]bool
}

// cellAt replays the escape codes of line up to column col.
func cellAt(t *testing.T, line string, col int) cellState {
	t.Helper()
	st := cellState{flags: map[string]bool{}}
	rs := []rune(line)
	seen := 0
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\x1b' {
			end := slices.Index(rs[i:], 'm')
			if rs[i+1] != '[' || end < 0 {
				t.Fatalf("unexpected escape in %q", line)
			}
			st.apply(strings.Split(string(rs[i+2:i+end]), ";"))
			i += end
			continue
		}
		if seen == col {
			return st
		}
		seen++
	}
	t.Fatalf("column %d is past the line %q", col, line)
	return st
}

func (s *cellState) apply(p []string) {
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == "" || c == "0":
			*s = cellState{flags: map[string]bool{}}
		case c == "22":
			delete(s.flags, "1")
			delete(s.flags, "2")
		case c == "27":
			delete(s.flags, "7")
		case c == "39":
			s.fg = ""
		case c == "49":
			s.bg = ""
		case c == "38" || c == "48":
			n := 2 // 5;n
			if p[i+1] == "2" {
				n = 4
			}
			code := strings.Join(p[i:i+n+1], ";")
			if c == "38" {
				s.fg = code
			} else {
				s.bg = code
			}
			i += n
		case slices.Contains([]string{"1", "2", "7"}, c):
			s.flags[c] = true
		default:
			if n, _ := strconv.Atoi(c); n >= 100 || (n >= 40 && n < 48) {
				s.bg = c
			} else {
				s.fg = c
			}
		}
	}
}

func rgbBg(r, g, b int) string { return fmt.Sprintf("48;2;%d;%d;%d", r, g, b) }

func (h *harness) background(hex string) {
	h.send(tea.BackgroundColorMsg{Color: lipgloss.Color(hex)})
}

// checkShadedBar checks the cells of line 0, given as columns from the bar start.
func checkShadedBar(t *testing.T, line string, start, last int, bar, tab string) {
	t.Helper()
	at := func(x int) cellState { return cellAt(t, line, start+x) }
	for _, x := range []int{8, 17, 26, 30, last} {
		if got := at(x).bg; got != bar {
			t.Errorf("x=%d: background %q, want the bar shade %q", x, got, bar)
		}
	}
	for _, x := range []int{0, 3, 7, 18, 25} {
		if c := at(x); c.bg != tab || !c.flags["2"] || c.flags["7"] || c.flags["1"] {
			t.Errorf("x=%d: inactive tab %+v, want the tab shade %q and faint", x, c, tab)
		}
	}
	for _, x := range []int{9, 12, 16} {
		if c := at(x); c.bg != bar || c.fg != "94" || !c.flags["7"] || !c.flags["1"] {
			t.Errorf("x=%d: active tab %+v, want bold, reverse, 94 and the bar shade %q", x, c, bar)
		}
	}
	for _, x := range []int{27, 28, 29} {
		if c := at(x); c.bg != bar || !c.flags["2"] || c.flags["7"] {
			t.Errorf("x=%d: plus %+v, want faint on the bar shade %q", x, c, bar)
		}
	}
}

func TestShadedTabBar(t *testing.T) {
	themes := []struct {
		name     string
		bg       string
		bar, tab string
	}{
		{"dark", "#1e1e2e", rgbBg(42, 42, 58), rgbBg(60, 60, 76)},
		{"light", "#eff1f5", rgbBg(227, 228, 232), rgbBg(210, 212, 215)},
	}
	for _, sh := range themes {
		for _, l := range barLayouts {
			t.Run(sh.name+", "+l.name, func(t *testing.T) {
				h := l.build()
				h.prefix("t")
				h.prefix("t")
				h.prefix("2") // tabs 0-7, 9-16, 18-25, plus 27-29 from the bar start
				h.background(sh.bg)
				checkShadedBar(t, h.rawBar(), l.start, h.m.width-1-l.start, sh.bar, sh.tab)
			})
		}
	}
}

func TestTabBarHasNoShadeBeforeTheBackgroundArrives(t *testing.T) {
	for _, l := range barLayouts {
		h := l.build()
		h.prefix("t")
		if line := h.rawBar(); strings.Contains(line, "48;") {
			t.Errorf("%s: background code in %q", l.name, line)
		}
	}
}

func TestShadesDoNotChangeTheBarText(t *testing.T) {
	for _, l := range barLayouts {
		h := l.build()
		h.prefix("t")
		before := h.bar()
		h.background("#1e1e2e")
		if got := strings.TrimRight(h.bar(), " "); got != strings.TrimRight(before, " ") {
			t.Errorf("%s: bar %q, want %q", l.name, got, before)
		}
	}
}

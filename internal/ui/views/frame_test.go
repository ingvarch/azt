package views

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

var aksTabNames = []string{"Overview", "Node pools", "Networking", "Identity", "Add-ons", "Monitoring", "JSON"}

// paddedTabs are the tab names with a space on each side.
func paddedTabs(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = " " + n + " "
	}
	return out
}

func TestTabBarText(t *testing.T) {
	line := func(name, tab string, width int) string {
		return plainLines(openedAt(t, name, tab, width, 10).Render())[0]
	}
	want := strings.TrimRight(" "+strings.Join(paddedTabs(aksTabNames), " "), " ")
	if got := line("aks", "", 100); got != want {
		t.Errorf("tab bar = %q, want %q", got, want)
	}
	if got := line("aks", "Networking", 100); got != want {
		t.Errorf("tab bar on another tab = %q, want %q", got, want)
	}
	if got, want := line("aks", "", 15), "  Overview   N"; got != want {
		t.Errorf("narrow tab bar = %q, want %q", got, want)
	}
	if got, want := line("webapp", "", 100), "  Overview   JSON"; got != want {
		t.Errorf("generic tab bar = %q, want %q", got, want)
	}
}

func TestTabBarStyles(t *testing.T) {
	for i, current := range aksTabNames {
		bar := strings.Split(openedAt(t, "aks", current, 100, 10).Render(), "\n")[0]
		for j, name := range aksTabNames {
			a := attrsOf(t, bar, name)
			if i == j && (!a.bold || a.fg != fgAccent) {
				t.Errorf("%s is current: %+v, want bold accent", name, a)
			}
			if i != j && (a.bold || a.fg != fgMuted) {
				t.Errorf("%s is not current: %+v, want muted", name, a)
			}
		}
	}
}

func TestTabRule(t *testing.T) {
	const content = 98
	for i, current := range aksTabNames {
		start := 0
		for _, p := range paddedTabs(aksTabNames[:i]) {
			start += lipgloss.Width(p) + 1
		}
		heavy := lipgloss.Width(paddedTabs(aksTabNames)[i])
		rule := strings.Split(openedAt(t, "aks", current, 100, 10).Render(), "\n")[1]
		want := " " + strings.Repeat("─", start) + strings.Repeat("━", heavy) + strings.Repeat("─", content-start-heavy)
		if got := plain(rule); got != want {
			t.Errorf("%s: rule = %q, want %q", current, got, want)
		}
		if a := attrsOf(t, rule, "━"); a.fg != fgAccent || a.bold {
			t.Errorf("%s: heavy part = %+v, want accent", current, a)
		}
		if i > 0 {
			if a := attrsOf(t, rule, "─"); a.fg != fgMuted || a.bold {
				t.Errorf("%s: light part = %+v, want muted", current, a)
			}
		}
		if i < len(aksTabNames)-1 {
			if a := lastAttrsOf(t, rule, "─"); a.fg != fgMuted || a.bold {
				t.Errorf("%s: light part on the right = %+v, want muted", current, a)
			}
		}
	}
}

func TestTabRuleIsAsWideAsTheBody(t *testing.T) {
	for _, c := range []struct {
		tab         string
		width, want int
	}{{"", 15, 14}, {"Networking", 15, 14}, {"", 40, 39}, {"", 2, 1}} {
		rule := strings.Split(openedAt(t, "aks", c.tab, c.width, 10).Render(), "\n")[1]
		if got := lipgloss.Width(rule); got != c.want {
			t.Errorf("tab %q width %d: rule is %d wide, want %d", c.tab, c.width, got, c.want)
		}
	}
	if got := plain(strings.Split(openedAt(t, "aks", "Networking", 15, 10).Render(), "\n")[1]); strings.Contains(got, "━") {
		t.Errorf("rule under a hidden tab has a heavy part: %q", got)
	}
}

func TestBodyStartsOnLineThree(t *testing.T) {
	got := plainLines(openedAt(t, "aks", "JSON", 40, 10).Render())
	if got[2] != "" || got[3] != " {" || !strings.HasPrefix(got[4], `   "id": "/subscriptions/`) {
		t.Errorf("render:\n%s", strings.Join(got, "\n"))
	}
}

func TestBodyHasAMarginOnBothSides(t *testing.T) {
	for _, width := range []int{20, 30, 60} {
		for _, tab := range []string{"JSON", "Overview", "Networking"} {
			lines := strings.Split(openedAt(t, "aks", tab, width, 60).Render(), "\n")
			for i, l := range lines[3:] {
				if p := plain(l); p != "" && !strings.HasPrefix(p, " ") {
					t.Errorf("%s width %d line %d has no left margin: %q", tab, width, i, p)
				}
				if w := lipgloss.Width(l); w > width-1 {
					t.Errorf("%s width %d line %d is %d wide, no right margin: %q", tab, width, i, w, plain(l))
				}
			}
		}
	}
	if got := plainLines(openedAt(t, "aks", "JSON", 20, 10).Render())[4]; got != `   "id": "/subscrip` {
		t.Errorf("a long JSON line is cut to the body: %q", got)
	}
}

func TestRefreshingShowsUnderTheTabBar(t *testing.T) {
	v, _ := press(openedAt(t, "aks", "JSON", 40, 8), keyR)
	got := plainLines(v.Render())
	if got[2] != " "+refreshingText || got[3] != " {" {
		t.Errorf("render:\n%s", strings.Join(got, "\n"))
	}
}

func TestRefreshingNoteIsCutToTheBody(t *testing.T) {
	v, _ := press(openedAt(t, "aks", "JSON", 8, 8), keyR)
	if got := plainLines(v.Render())[2]; got != " refres" {
		t.Errorf("note = %q", got)
	}
}

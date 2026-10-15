package views

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func plainAll(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = plain(l)
	}
	return out
}

func TestSectionBox(t *testing.T) {
	got := section{title: "Title", lines: []string{"one", "two words"}}.render(30)
	want := []string{
		"╭ Title ─────────────────────╮",
		"│ one                        │",
		"│ two words                  │",
		"╰────────────────────────────╯",
	}
	wantLines(t, plainAll(got), want)
	for i, l := range got {
		if w := lipgloss.Width(l); w != 30 {
			t.Errorf("line %d is %d wide", i, w)
		}
	}
}

func TestSectionBoxWithoutLines(t *testing.T) {
	got := section{title: "T"}.render(20)
	wantLines(t, plainAll(got), []string{"╭ T ───────────────╮", "╰──────────────────╯"})
}

func TestSectionBoxColors(t *testing.T) {
	got := section{title: "Title", lines: []string{"text"}}.render(30)
	for _, c := range []struct{ line, sub string }{{got[0], "╭"}, {got[0], "─"}, {got[0], "╮"}, {got[1], "│"}, {got[2], "╰"}, {got[2], "╯"}} {
		if a := attrsOf(t, c.line, c.sub); a.fg != fgMuted || a.bold {
			t.Errorf("%q = %+v, want muted", c.sub, a)
		}
	}
	if a := attrsOf(t, got[0], "Title"); a.fg != fgAccent || !a.bold {
		t.Errorf("title = %+v, want bold accent", a)
	}
	if a := attrsOf(t, got[1], "text"); a.fg != "" || a.bold {
		t.Errorf("text = %+v, want the default", a)
	}
	if a := lastAttrsOf(t, got[1], "│"); a.fg != fgMuted || a.bold {
		t.Errorf("right border = %+v", a)
	}
}

func TestSectionBoxTone(t *testing.T) {
	got := section{title: "Title", lines: []string{"text"}, tone: bad}.render(30)
	if a := attrsOf(t, got[0], "Title"); a.fg != fgBad || !a.bold {
		t.Errorf("title = %+v, want bold bad", a)
	}
	for _, c := range []struct{ line, sub string }{{got[0], "╭"}, {got[1], "│"}, {got[2], "╯"}} {
		if a := attrsOf(t, c.line, c.sub); a.fg != fgBad {
			t.Errorf("%q = %+v, want bad", c.sub, a)
		}
	}
	if a := lastAttrsOf(t, got[1], "│"); a.fg != fgBad {
		t.Errorf("right border = %+v, want bad", a)
	}
}

func TestSectionBoxCutsLongLines(t *testing.T) {
	exact := strings.Repeat("y", 20)
	got := section{title: "T", lines: []string{strings.Repeat("x", 30), exact}}.render(24)
	if g := plain(got[1]); g != "│ "+strings.Repeat("x", 19)+"… │" {
		t.Errorf("long line = %q", g)
	}
	if g := plain(got[2]); g != "│ "+exact+" │" {
		t.Errorf("fitting line = %q", g)
	}
}

func TestSectionBoxCutsStyledLines(t *testing.T) {
	long := stateText("Stopped · Updating · Updating")
	got := section{title: "T", lines: []string{long}}.render(24)
	if g := plain(got[1]); !strings.HasSuffix(g, "… │") || lipgloss.Width(got[1]) != 24 {
		t.Errorf("line = %q", g)
	}
	if a := lastAttrsOf(t, got[1], "│"); a.fg != fgMuted {
		t.Errorf("the border took the color of the cut text: %+v", a)
	}
	if a := attrsOf(t, got[1], "…"); a.fg != "" {
		t.Errorf("ellipsis = %+v", a)
	}
}

func TestSectionBoxCutsALongTitle(t *testing.T) {
	got := section{title: strings.Repeat("T", 40)}.render(20)
	if w := lipgloss.Width(got[0]); w != 20 {
		t.Errorf("top border is %d wide: %q", w, plain(got[0]))
	}
	if g := plain(got[0]); !strings.Contains(g, "…") || !strings.HasSuffix(g, " ─╮") {
		t.Errorf("top border = %q", g)
	}
}

func TestSectionNarrowHasNoBox(t *testing.T) {
	got := section{title: "Title", lines: []string{"one", "two"}}.render(19)
	wantLines(t, plainAll(got), []string{"Title", "one", "two"})
	if a := attrsOf(t, got[0], "Title"); !a.bold {
		t.Errorf("title = %+v, want bold", a)
	}
	for _, w := range []int{0, 1, 5} {
		if got := (section{title: "Title", lines: []string{"x"}}).render(w); len(got) != 2 {
			t.Errorf("width %d: %q", w, got)
		}
	}
	if got := (section{title: "T", lines: []string{"x"}}).render(20); len(got) != 3 {
		t.Errorf("width 20 is not boxed: %q", got)
	}
}

func TestSectionsAreSeparatedByABlankLine(t *testing.T) {
	got := sections(20, section{title: "A", lines: []string{"a"}}, section{title: "B", lines: []string{"b"}})
	if len(got) != 7 || got[3] != "" || !strings.HasPrefix(plain(got[4]), "╭ B") || got[6] == "" {
		t.Errorf("lines:\n%s", strings.Join(plainAll(got), "\n"))
	}
	narrow := sections(10, section{title: "A", lines: []string{"a"}}, section{title: "B"})
	wantLines(t, plainAll(narrow), []string{"A", "a", "", "B"})
}

func TestInnerWidth(t *testing.T) {
	for _, c := range []struct{ width, want int }{{100, 96}, {20, 16}, {19, 19}, {0, 0}} {
		if got := innerWidth(c.width); got != c.want {
			t.Errorf("innerWidth(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}

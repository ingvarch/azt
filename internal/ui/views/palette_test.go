package views

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// The foreground codes of the palette colors.
const (
	fgAccent  = "94"
	fgMuted   = "90"
	fgGood    = "92"
	fgWarn    = "93"
	fgBad     = "91"
	fgBoolean = "95"
)

// attrs are the styling attributes a piece of text is drawn with.
type attrs struct {
	fg            string
	bold, reverse bool
}

func (a *attrs) apply(seq string) {
	for _, p := range strings.Split(strings.Trim(seq, "\x1b[m"), ";") {
		switch {
		case p == "" || p == "0":
			*a = attrs{}
		case p == "1":
			a.bold = true
		case p == "7":
			a.reverse = true
		case p == "39":
			a.fg = ""
		case len(p) == 2 && (p[0] == '3' || p[0] == '9'):
			a.fg = p
		}
	}
}

// attrsOf returns the attributes the first occurrence of sub is drawn with in s.
func attrsOf(t *testing.T, s, sub string) attrs {
	t.Helper()
	return attrsAt(t, s, sub, strings.Index)
}

// lastAttrsOf is attrsOf for the last occurrence of sub.
func lastAttrsOf(t *testing.T, s, sub string) attrs {
	t.Helper()
	return attrsAt(t, s, sub, strings.LastIndex)
}

func attrsAt(t *testing.T, s, sub string, find func(s, sub string) int) attrs {
	t.Helper()
	at := plain(s)
	i := find(at, sub)
	if i < 0 {
		t.Fatalf("%q lacks %q", at, sub)
	}
	var states []attrs
	var cur attrs
	for rest := s; rest != ""; {
		if loc := sgrSeq.FindStringIndex(rest); loc != nil && loc[0] == 0 {
			cur.apply(rest[:loc[1]])
			rest = rest[loc[1]:]
			continue
		}
		states = append(states, cur)
		rest = rest[1:]
	}
	return states[i]
}

func wantFg(t *testing.T, s, sub, fg string) {
	t.Helper()
	if got := attrsOf(t, s, sub).fg; got != fg {
		t.Errorf("%q in %q: foreground %q, want %q", sub, s, got, fg)
	}
}

func TestPalette(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"accent", accent, lipgloss.Color("12")},
		{"muted", muted, lipgloss.Color("8")},
		{"good", good, lipgloss.Color("10")},
		{"warn", warn, lipgloss.Color("11")},
		{"bad", bad, lipgloss.Color("9")},
		{"boolean", boolean, lipgloss.Color("13")},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestStateTone(t *testing.T) {
	for _, c := range []struct {
		words []string
		want  any
	}{
		{[]string{"Running", "Succeeded", "Enabled", "running", "SUCCEEDED", "eNaBlEd", "available", "Available", "Ready", "ready", "READY"}, good},
		{[]string{"Stopped", "Failed", "Canceled", "Disabled", "Deleted", "stopped", "FAILED", "unavailable", "UNAVAILABLE", "Deallocated", "deallocated", "Dropping", "dropping", "DROPPING"}, bad},
		{[]string{
			"Creating", "Updating", "Upgrading", "Scaling", "Starting", "Stopping", "Deleting",
			"Migrating", "Warned", "PastDue", "updating", "Unknown", "", "Cancelled",
		}, warn},
	} {
		for _, w := range c.words {
			if got := stateTone(w); got != c.want {
				t.Errorf("stateTone(%q) = %v, want %v", w, got, c.want)
			}
		}
	}
}

func TestStateText(t *testing.T) {
	for _, c := range []struct {
		in, want string
	}{
		{"Running", "● Running"},
		{"Stopped · Failed", "● Stopped · Failed"},
		{"— · Succeeded", "● — · Succeeded"},
		{"", ""},
		{dash, dash},
	} {
		if got := plain(stateText(c.in)); got != c.want {
			t.Errorf("stateText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// One dot, in the tone of the most severe part; each word in its own tone.
	s := stateText("Running · Updating")
	wantFg(t, s, "●", fgWarn)
	wantFg(t, s, "Running", fgGood)
	wantFg(t, s, "Updating", fgWarn)
	wantFg(t, stateText("Running · Succeeded"), "●", fgGood)
	wantFg(t, stateText("Updating · Stopped"), "●", fgBad)
	if got := attrsOf(t, s, " · ").fg; got != "" {
		t.Errorf("separator foreground = %q, want the default", got)
	}
	wantFg(t, stateText("Failed"), "● Failed", fgBad)
	if got := stateText(dash); strings.Contains(got, "\x1b") {
		t.Errorf("dash is styled: %q", got)
	}
}

func TestStateToneOfNetworkStates(t *testing.T) {
	for _, c := range []struct {
		word string
		want any
	}{
		{"Connected", good}, {"connected", good}, {"CONNECTED", good},
		{"FullyInSync", good}, {"fullyinsync", good}, {"FULLYINSYNC", good},
		{"Disconnected", bad}, {"disconnected", bad}, {"DISCONNECTED", bad},
		{"Initiated", warn}, {"LocalNotInSync", warn}, {"RemoteNotInSync", warn},
	} {
		if got := stateTone(c.word); got != c.want {
			t.Errorf("stateTone(%q) = %v, want %v", c.word, got, c.want)
		}
	}
}

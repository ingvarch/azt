package ui

import (
	"slices"
	"testing"
)

// typed is a palette with text typed into it.
func typed(text string) paletteModel {
	p := newPalette()
	p.text = text
	return p.narrow()
}

func offered(p paletteModel) string {
	if p.at < 0 {
		return ""
	}
	return p.matches[p.at]
}

func TestNewPalette(t *testing.T) {
	p := newPalette()
	if p.text != "" || p.at != -1 || !slices.Equal(p.matches, allNames()) {
		t.Errorf("newPalette() = %+v", p)
	}
	if p.choice() != "" {
		t.Errorf("choice = %q, want none", p.choice())
	}
}

func TestPaletteNarrow(t *testing.T) {
	tests := []struct {
		text    string
		matches []string
		at      int
	}{
		{"", allNames(), -1},
		{" ", allNames(), -1},
		{"s", []string{"subs", "sql", "storage"}, 0},
		{"S", []string{"subs", "sql", "storage"}, 0},
		{"st", []string{"storage"}, 0},
		{"zzz", nil, -1},
		{"zzz arg", nil, -1},
		{"vm", []string{"vm"}, 0},
		{"type vm", []string{"type"}, 0},
		{"  v", []string{"vm", "vnet"}, 0},
	}
	for _, tt := range tests {
		p := typed(tt.text)
		if !slices.Equal(p.matches, tt.matches) || p.at != tt.at {
			t.Errorf("narrow(%q) = %v at %d, want %v at %d", tt.text, p.matches, p.at, tt.matches, tt.at)
		}
	}
}

func TestPaletteNarrowKeepsOffer(t *testing.T) {
	p := typed("s").walk(1) // sql
	if offered(p) != "sql" {
		t.Fatalf("offer = %q, want sql", offered(p))
	}
	p.text = "sq"
	p = p.narrow()
	if offered(p) != "sql" {
		t.Errorf("after more typing offer = %q, want sql", offered(p))
	}
	p.text = "s"
	p = p.narrow()
	if offered(p) != "sql" {
		t.Errorf("after erasing letters offer = %q, want sql", offered(p))
	}
	p.text = "s arg"
	p = p.narrow()
	if offered(p) != "sql" {
		t.Errorf("after an argument offer = %q, want sql", offered(p))
	}
	p.text = "su"
	if p = p.narrow(); offered(p) != "subs" {
		t.Errorf("offer that no longer fits: got %q, want subs", offered(p))
	}
}

func TestPaletteNarrowWholeName(t *testing.T) {
	saved := commandTable
	t.Cleanup(func() { commandTable = saved })
	commandTable = []commandInfo{{"ab", ""}, {"abc", ""}}
	p := typed("a").walk(1) // abc
	if offered(p) != "abc" {
		t.Fatalf("offer = %q, want abc", offered(p))
	}
	p.text = "ab"
	if p = p.narrow(); offered(p) != "ab" {
		t.Errorf("typed a whole name: offer = %q, want ab", offered(p))
	}
}

func TestPaletteWalk(t *testing.T) {
	p := typed("s") // subs sql storage, offering subs
	var got []string
	for range 4 {
		p = p.walk(1)
		got = append(got, offered(p))
	}
	if want := []string{"sql", "storage", "subs", "sql"}; !slices.Equal(got, want) {
		t.Errorf("down = %v, want %v", got, want)
	}
	got = nil
	for range 4 {
		p = p.walk(-1)
		got = append(got, offered(p))
	}
	if want := []string{"subs", "storage", "sql", "subs"}; !slices.Equal(got, want) {
		t.Errorf("up = %v, want %v", got, want)
	}
}

func TestPaletteWalkFromNothing(t *testing.T) {
	if got := offered(newPalette().walk(1)); got != "subs" {
		t.Errorf("down from nothing = %q, want subs", got)
	}
	if got := offered(newPalette().walk(-1)); got != "q" {
		t.Errorf("up from nothing = %q, want q", got)
	}
}

func TestPaletteWalkWithoutMatches(t *testing.T) {
	for _, by := range []int{1, -1} {
		if p := typed("zzz").walk(by); p.at != -1 {
			t.Errorf("walk(%d) at = %d, want -1", by, p.at)
		}
	}
}

func TestPaletteSplit(t *testing.T) {
	tests := []struct {
		name             string
		p                paletteModel
		word, tail, rest string
		choice, line     string
	}{
		{"nothing typed", newPalette(), "", "", "", "", ""},
		{"partial", typed("st"), "st", "orage", "", "storage", "storage"},
		{"partial in other case", typed("ST"), "ST", "orage", "", "storage", "STorage"},
		{"exact", typed("vm"), "vm", "", "", "vm", "vm"},
		{"argument", typed("ty microsoft.web/sites"), "ty", "pe", " microsoft.web/sites", "type", "type microsoft.web/sites"},
		{"letter that lowercases to ASCII", typed("\u212a"), "\u212a", "v", "", "kv", "\u212av"},
		{"unknown", typed("zzz"), "zzz", "", "", "", "zzz"},
		{"arrows without typing", newPalette().walk(1), "", "subs", "", "subs", "subs"},
		{"stale offer", paletteModel{text: "x", matches: []string{"subs"}, at: 0}, "x", "", "", "", "x"},
		{"offer past the end", paletteModel{text: "s", matches: []string{"subs"}, at: 3}, "s", "", "", "", "s"},
	}
	for _, tt := range tests {
		word, tail, rest := tt.p.split()
		if word != tt.word || tail != tt.tail || rest != tt.rest {
			t.Errorf("%s: split = %q %q %q, want %q %q %q", tt.name, word, tail, rest, tt.word, tt.tail, tt.rest)
		}
		if got := tt.p.choice(); got != tt.choice {
			t.Errorf("%s: choice = %q, want %q", tt.name, got, tt.choice)
		}
		if got := tt.p.line(); got != tt.line {
			t.Errorf("%s: line = %q, want %q", tt.name, got, tt.line)
		}
	}
}

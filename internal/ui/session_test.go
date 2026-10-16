package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/session"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

// shape describes the tabs of m from its internal state: names, zoom, focus
// index, split ratios, and the titles and filters of every stack.
func shape(m Model) []string {
	var out []string
	for _, t := range m.tabs {
		out = append(out, fmt.Sprintf("%q zoom=%v focus=%d %s",
			t.name, t.zoomed, slices.Index(t.root.Panes(), t.focus), m.tree(t.root)))
	}
	return append(out, fmt.Sprintf("active=%d sidebar=%v", m.active, m.sidebarVisible))
}

func (m Model) tree(n *layout.Node) string {
	if !n.IsLeaf() {
		return fmt.Sprintf("%d@%.3f(%s %s)", n.Split, n.Ratio, m.tree(n.A), m.tree(n.B))
	}
	var parts []string
	for _, v := range m.stacks[n.Pane] {
		p := v.Title()
		if f, ok := v.(views.Filterer); ok && f.Filter() != "" {
			p += "/" + f.Filter()
		}
		parts = append(parts, p)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// busyScenario builds two tabs: the first is zoomed, renamed and split twice
// with changed ratios and a drilled-down, filtered stack; the second shows a
// palette view. The sidebar is hidden and the second tab is active.
func busyScenario(t *testing.T) *scenario {
	t.Helper()
	s := newScenario(t)
	s.settle(s.m.Init())
	s.press("ctrl+w")
	s.press("v")
	s.press("ctrl+w")
	s.press(">")
	s.press("enter")
	s.press("/")
	s.typ("rg")
	s.press("enter")
	s.press("ctrl+w")
	s.press("s")
	s.press("ctrl+w")
	s.press("-")
	s.press("ctrl+w")
	s.press(",")
	s.typ("work")
	s.press("enter")
	s.press("ctrl+w")
	s.press("z")
	s.press("ctrl+w")
	s.press("t")
	s.press(":")
	s.typ("rg")
	s.press("enter")
	s.press("ctrl+w")
	s.press("e")
	return s
}

func TestSessionOfNewModel(t *testing.T) {
	m := New(views.NewSubscriptions, nil)
	want := session.Session{
		Version: session.Version,
		Tabs: []session.Tab{{Layout: &session.Node{
			Stack: []views.Spec{{Kind: "subscriptions"}},
		}}},
		SidebarVisible: true,
		SidebarWidth:   defaultSidebarWidth,
	}
	if got := m.Session(); !reflect.DeepEqual(got, want) {
		t.Errorf("Session = %+v, want %+v", got, want)
	}
}

func TestSessionOfBusyModel(t *testing.T) {
	s := busyScenario(t)
	got := s.m.Session()

	if got.Version != session.Version || got.Active != 1 || got.SidebarVisible {
		t.Errorf("Version %d, Active %d, SidebarVisible %v", got.Version, got.Active, got.SidebarVisible)
	}
	if len(got.Tabs) != 2 {
		t.Fatalf("%d tabs", len(got.Tabs))
	}
	first := got.Tabs[0]
	if first.Name != "work" || !first.Zoomed || first.Focus != 2 {
		t.Errorf("first tab: name %q, zoomed %v, focus %d", first.Name, first.Zoomed, first.Focus)
	}
	root := first.Layout
	if root.Split != "h" || root.Ratio != s.m.tabs[0].root.Ratio || root.Ratio == 0.5 {
		t.Errorf("root: split %q, ratio %v", root.Split, root.Ratio)
	}
	if root.A.Split != "" || len(root.A.Stack) != 1 {
		t.Errorf("A = %+v", root.A)
	}
	inner := root.B
	if inner.Split != "v" || inner.Ratio != s.m.tabs[0].root.B.Ratio || inner.Ratio == 0.5 {
		t.Errorf("B: split %q, ratio %v", inner.Split, inner.Ratio)
	}
	wantStack := []views.Spec{
		{Kind: "subscriptions"},
		{Kind: "resourceGroups", Subscription: "sub-1", SubscriptionName: "Dev", Filter: "rg"},
	}
	if !reflect.DeepEqual(inner.A.Stack, wantStack) {
		t.Errorf("stack = %+v, want %+v", inner.A.Stack, wantStack)
	}
	if want := []views.Spec{{Kind: "subscriptions"}, {Kind: "allResourceGroups"}}; !reflect.DeepEqual(got.Tabs[1].Layout.Stack, want) {
		t.Errorf("second tab stack = %+v", got.Tabs[1].Layout.Stack)
	}
}

func TestRestoreRebuildsTheSession(t *testing.T) {
	s := busyScenario(t)
	saved := s.m.Session()
	m, err := Restore(saved, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := shape(m), shape(s.m); !slices.Equal(got, want) {
		t.Errorf("shape:\n got %q\nwant %q", got, want)
	}
	if got := m.Session(); !reflect.DeepEqual(got, saved) {
		t.Errorf("Session after Restore = %+v, want %+v", got, saved)
	}
	if m.sidebarFocused {
		t.Error("sidebar has focus")
	}
}

func TestRestoredPanesHaveFreshIDs(t *testing.T) {
	s := busyScenario(t)
	m, err := Restore(s.m.Session(), views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[layout.PaneID]bool{}
	for _, tb := range m.tabs {
		for _, id := range tb.root.Panes() {
			if seen[id] {
				t.Errorf("pane %d appears twice", id)
			}
			seen[id] = true
			if id >= m.nextID {
				t.Errorf("pane %d >= nextID %d", id, m.nextID)
			}
		}
	}
	if len(m.stacks) != len(seen) {
		t.Errorf("%d stacks for %d panes", len(m.stacks), len(seen))
	}
}

func TestRestoredModelKeepsWorking(t *testing.T) {
	m, err := Restore(oneTab(leaf("subscriptions")), views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	next, _ = next.Update(key("ctrl+w"))
	next, _ = next.Update(key("v"))
	if got := len(next.(Model).stacks); got != 2 {
		t.Errorf("%d stacks after a split, want 2", got)
	}
}

func leaf(kinds ...string) *session.Node {
	n := &session.Node{}
	for _, k := range kinds {
		n.Stack = append(n.Stack, views.Spec{Kind: k})
	}
	return n
}

func oneTab(n *session.Node) session.Session {
	return session.Session{Version: session.Version, Tabs: []session.Tab{{Layout: n}}}
}

func split(o string, ratio float64, a, b *session.Node) *session.Node {
	return &session.Node{Split: o, Ratio: ratio, A: a, B: b}
}

func TestRestoreValidation(t *testing.T) {
	two := split("h", 0.5, leaf("subscriptions"), leaf("subscriptions"))
	tests := []struct {
		name string
		edit func(*session.Session)
	}{
		{"other version", func(s *session.Session) { s.Version = 2 }},
		{"no tabs", func(s *session.Session) { s.Tabs = nil }},
		{"negative active", func(s *session.Session) { s.Active = -1 }},
		{"active out of range", func(s *session.Session) { s.Active = 1 }},
		{"no layout", func(s *session.Session) { s.Tabs[0].Layout = nil }},
		{"bad split", func(s *session.Session) {
			s.Tabs[0].Layout = split("x", 0.5, leaf("subscriptions"), leaf("subscriptions"))
		}},
		{"split without A", func(s *session.Session) { s.Tabs[0].Layout = split("h", 0.5, nil, leaf("subscriptions")) }},
		{"split without B", func(s *session.Session) { s.Tabs[0].Layout = split("v", 0.5, leaf("subscriptions"), nil) }},
		{"nested split without B", func(s *session.Session) {
			s.Tabs[0].Layout = split("h", 0.5, leaf("subscriptions"), split("v", 0.5, leaf("subscriptions"), nil))
		}},
		{"empty stack", func(s *session.Session) { s.Tabs[0].Layout = leaf() }},
		{"empty stack in split", func(s *session.Session) { s.Tabs[0].Layout = split("h", 0.5, leaf("subscriptions"), leaf()) }},
		{"unbuildable view", func(s *session.Session) { s.Tabs[0].Layout = leaf("subscriptions", "nope") }},
		{"negative focus", func(s *session.Session) { s.Tabs[0].Focus = -1 }},
		{"focus out of range", func(s *session.Session) { s.Tabs[0].Layout = two; s.Tabs[0].Focus = 2 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oneTab(leaf("subscriptions"))
			tt.edit(&s)
			if _, err := Restore(s, views.NewSubscriptions, nil); err == nil {
				t.Error("no error")
			}
		})
	}
}

func TestRestoreAcceptsFocusOnLastPane(t *testing.T) {
	s := oneTab(split("h", 0.5, leaf("subscriptions"), leaf("subscriptions")))
	s.Tabs[0].Focus = 1
	m, err := Restore(s, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.tabs[0].focus != m.tabs[0].root.Panes()[1] {
		t.Errorf("focus = %d", m.tabs[0].focus)
	}
}

func TestRestoreClampsRatios(t *testing.T) {
	for ratio, want := range map[float64]float64{
		-1: layout.MinRatio, 0: layout.MinRatio, 0.01: layout.MinRatio,
		0.4: 0.4, 0.95: layout.MaxRatio, 3: layout.MaxRatio,
	} {
		s := oneTab(split("h", ratio, leaf("subscriptions"), split("v", ratio, leaf("subscriptions"), leaf("subscriptions"))))
		m, err := Restore(s, views.NewSubscriptions, nil)
		if err != nil {
			t.Fatal(err)
		}
		root := m.tabs[0].root
		if root.Ratio != want || root.B.Ratio != want {
			t.Errorf("ratio %v: got %v and %v, want %v", ratio, root.Ratio, root.B.Ratio, want)
		}
	}
}

func fetchKeys(cmd tea.Cmd) []data.Key {
	var keys []data.Key
	for _, msg := range collect(cmd) {
		if f, ok := msg.(data.FetchMsg); ok {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

func TestInitCoversEveryView(t *testing.T) {
	s := oneTab(split("h", 0.5,
		&session.Node{Stack: []views.Spec{
			{Kind: "subscriptions"},
			{Kind: "resourceGroups", Subscription: "sub-1", SubscriptionName: "Dev"},
		}},
		leaf("subscriptions")))
	s.Tabs = append(s.Tabs, session.Tab{Layout: leaf("allResourceGroups")})
	m, err := Restore(s, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := fetchKeys(m.Init())
	for _, want := range []data.Key{
		{Kind: data.ResourceGroups, Subscription: "sub-1"},
		{Kind: data.AllResourceGroups},
		{Kind: data.Subscriptions},
	} {
		if !slices.Contains(got, want) {
			t.Errorf("Init lacks a fetch of %+v; got %+v", want, got)
		}
	}
	// 2 stacked views + 1 + 1 + the sidebar.
	if len(got) != 5 {
		t.Errorf("%d fetches, want 5: %+v", len(got), got)
	}
}

func TestLoadOrNew(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.json")
	two := oneTab(leaf("subscriptions"))
	two.Tabs = append(two.Tabs, session.Tab{Name: "second", Layout: leaf("allResourceGroups")})
	if err := session.Save(valid, two); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.json")
	writeFile(t, broken, "{nope")
	invalid := filepath.Join(dir, "invalid.json")
	bad := oneTab(leaf("subscriptions"))
	bad.Active = 7
	if err := session.Save(invalid, bad); err != nil {
		t.Fatal(err)
	}

	t.Run("missing starts fresh without a notice", func(t *testing.T) {
		m := LoadOrNew(filepath.Join(dir, "none.json"), views.NewSubscriptions, nil)
		if len(m.tabs) != 1 || m.notice != "" || !m.sidebarVisible {
			t.Errorf("tabs %d, notice %q", len(m.tabs), m.notice)
		}
	})
	for _, path := range []string{broken, invalid} {
		t.Run(filepath.Base(path)+" starts fresh with a notice", func(t *testing.T) {
			m := LoadOrNew(path, views.NewSubscriptions, nil)
			if len(m.tabs) != 1 || !strings.HasPrefix(m.notice, "could not restore the session: ") {
				t.Errorf("tabs %d, notice %q", len(m.tabs), m.notice)
			}
			m.width, m.height = 140, 30
			if !strings.Contains(m.statusBar(), "could not restore the session") {
				t.Error("status bar lacks the notice")
			}
			next, _ := m.Update(key("x"))
			if next.(Model).notice != "" {
				t.Error("notice survives a key")
			}
		})
	}
	t.Run("valid is restored", func(t *testing.T) {
		m := LoadOrNew(valid, views.NewSubscriptions, nil)
		if len(m.tabs) != 2 || m.tabs[1].name != "second" || m.notice != "" {
			t.Errorf("tabs %d, notice %q", len(m.tabs), m.notice)
		}
	})
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreNoTabsSaysSo(t *testing.T) {
	_, err := Restore(session.Session{Version: session.Version}, views.NewSubscriptions, nil)
	if err == nil || !strings.Contains(err.Error(), "no tabs") {
		t.Errorf("err = %v", err)
	}
}

package ui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/views"
)

type pingMsg struct{}

// fake records what it receives. Copies share the record through rec.
type fake struct {
	rec *record
}

type record struct {
	id    int
	keys  []string
	inits int
	pings int
	w, h  int
}

func (f fake) Init() tea.Cmd {
	f.rec.inits++
	return func() tea.Msg { return fmt.Sprintf("init-%d", f.rec.id) }
}

func (f fake) Update(msg tea.Msg) (views.View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		f.rec.keys = append(f.rec.keys, msg.String())
		return f, f.reply("key")
	case pingMsg:
		f.rec.pings++
		return f, f.reply("pong")
	}
	return f, nil
}

func (f fake) reply(prefix string) tea.Cmd {
	return func() tea.Msg { return fmt.Sprintf("%s-%d", prefix, f.rec.id) }
}

func (f fake) Resize(w, h int) views.View {
	f.rec.w, f.rec.h = w, h
	return f
}

func (f fake) Render() string { return fmt.Sprintf("view %d", f.rec.id) }
func (f fake) Title() string  { return fmt.Sprintf("Fake%d", f.rec.id) }

// harness builds fake views and tracks them in creation order.
type harness struct {
	recs []*record
	m    Model
}

func newHarness(w, h int) *harness {
	hs := &harness{}
	hs.m = New(func() views.View {
		r := &record{id: len(hs.recs) + 1}
		hs.recs = append(hs.recs, r)
		return fake{r}
	})
	hs.send(tea.WindowSizeMsg{Width: w, Height: h})
	return hs
}

func (h *harness) send(msg tea.Msg) tea.Cmd {
	next, cmd := h.m.Update(msg)
	h.m = next.(Model)
	return cmd
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "ctrl+w":
		return tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

// prefix sends ctrl+w and then k.
func (h *harness) prefix(k string) tea.Cmd {
	h.send(key("ctrl+w"))
	return h.send(key(k))
}

// collect runs cmd, unwrapping batches, and returns the messages it produces.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, collect(c)...)
	}
	return msgs
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

var sgr = regexp.MustCompile("\x1b\\[[0-9;:]*m")

// raw is the frame with styling codes.
func (h *harness) raw() string { return h.m.View().Content }

// frame is the frame without styling codes.
func (h *harness) frame() string { return sgr.ReplaceAllString(h.raw(), "") }

func TestInitRunsFirstView(t *testing.T) {
	h := newHarness(100, 30)
	cmd := h.m.Init()
	if cmd == nil || cmd() != "init-1" {
		t.Errorf("Init did not return the first view's command")
	}
	if h.recs[0].inits != 1 {
		t.Errorf("Init calls = %d, want 1", h.recs[0].inits)
	}
}

func TestViewHasAltScreen(t *testing.T) {
	h := newHarness(100, 30)
	if !h.m.View().AltScreen {
		t.Error("AltScreen = false")
	}
}

func TestEmptyBeforeWindowSize(t *testing.T) {
	m := New(func() views.View { return fake{&record{id: 1}} })
	if got := m.View().Content; got != "" {
		t.Errorf("content = %q, want empty", got)
	}
}

func TestQuit(t *testing.T) {
	h := newHarness(100, 30)
	if !isQuit(h.send(key("q"))) {
		t.Error("q does not quit")
	}
	if !isQuit(h.send(key("ctrl+c"))) {
		t.Error("ctrl+c does not quit")
	}
	h.send(key("ctrl+w"))
	if !isQuit(h.send(key("ctrl+c"))) {
		t.Error("ctrl+c in prefix mode does not quit")
	}
}

func TestQuitDoesNotReachView(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("q"))
	if len(h.recs[0].keys) != 0 {
		t.Errorf("view got keys %v", h.recs[0].keys)
	}
}

func TestNormalKeysReachOnlyFocusedView(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	cmd := h.send(key("x"))
	if got := h.recs[1].keys; len(got) != 1 || got[0] != "x" {
		t.Errorf("focused keys = %v", got)
	}
	if got := collect(cmd); len(got) != 1 || got[0] != "key-2" {
		t.Errorf("returned messages = %v, want [key-2]", got)
	}
	if got := h.recs[0].keys; len(got) != 0 {
		t.Errorf("other view keys = %v", got)
	}
}

func TestSplitCreatesPane(t *testing.T) {
	tests := []struct {
		key          string
		w, h         int
		wantW, wantH int
	}{
		{"v", 100, 30, 48, 26},
		{"s", 100, 30, 98, 12},
	}
	for _, tt := range tests {
		h := newHarness(tt.w, tt.h)
		cmd := h.prefix(tt.key)
		if len(h.recs) != 2 {
			t.Fatalf("%s: views = %d, want 2", tt.key, len(h.recs))
		}
		if cmd == nil || cmd() != "init-2" {
			t.Errorf("%s: cmd is not the new view's Init", tt.key)
		}
		for i, r := range h.recs {
			if r.w != tt.wantW || r.h != tt.wantH {
				t.Errorf("%s: view %d size = %dx%d, want %dx%d", tt.key, i+1, r.w, r.h, tt.wantW, tt.wantH)
			}
		}
		h.send(key("x"))
		if len(h.recs[1].keys) != 1 {
			t.Errorf("%s: focus did not move to the new pane", tt.key)
		}
	}
}

func TestSinglePaneInnerSize(t *testing.T) {
	h := newHarness(100, 30)
	if r := h.recs[0]; r.w != 98 || r.h != 26 {
		t.Errorf("size = %dx%d, want 98x26", r.w, r.h)
	}
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	if r := h.recs[0]; r.w != 58 || r.h != 16 {
		t.Errorf("size after window change = %dx%d, want 58x16", r.w, r.h)
	}
}

func TestFocusMoves(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v") // focus: view 2 (right)
	h.prefix("s") // view 3 below view 2; focus: view 3
	steps := []struct {
		key  string
		want int
	}{
		{"k", 2}, {"j", 3}, {"h", 1}, {"l", 2}, {"l", 2}, {"k", 2},
	}
	for _, s := range steps {
		h.prefix(s.key)
		for _, r := range h.recs {
			r.keys = nil
		}
		h.send(key("x"))
		if got := h.recs[s.want-1].keys; len(got) != 1 {
			t.Errorf("after %s focus is not on view %d", s.key, s.want)
		}
	}
}

func TestClosePaneMovesFocus(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v") // 1 | 2, focus 2
	h.prefix("q")
	h.send(key("x"))
	if len(h.recs[0].keys) != 1 {
		t.Errorf("focus after close not on view 1")
	}
	if got := h.recs[0].w; got != 98 {
		t.Errorf("remaining pane width = %d, want 98", got)
	}
	if strings.Contains(h.frame(), "view 2") {
		t.Error("closed view still rendered")
	}
	if got := len(h.m.views); got != 1 {
		t.Errorf("views = %d, want 1", got)
	}
}

func TestCloseFocusMovesToCoverOfTopLeftCell(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v") // 1 | 2
	h.prefix("s") // 1 | (2 above 3), focus 3
	h.prefix("q") // pane 2 covers the closed cell; the first pane is 1
	h.send(key("x"))
	if len(h.recs[1].keys) != 1 || len(h.recs[0].keys) != 0 {
		t.Errorf("focus is not on view 2: %v %v", h.recs[0].keys, h.recs[1].keys)
	}
}

func TestCloseAfterShrinkKeepsFocusOnLivePane(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.send(tea.WindowSizeMsg{Width: 100, Height: 2})
	h.prefix("q")
	_ = h.frame()
	h.send(key("x"))
	if len(h.recs[0].keys) != 1 {
		t.Errorf("focus is not on the remaining view: %v", h.recs[0].keys)
	}
}

func TestCloseLastPaneClosesTab(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t") // tab 2 active
	h.prefix("q")
	f := h.frame()
	if strings.Contains(f, " 2 ") {
		t.Errorf("tab 2 still in tab bar:\n%s", f)
	}
	if got := len(h.m.views); got != 1 {
		t.Errorf("views = %d, want 1", got)
	}
	h.send(key("x"))
	if len(h.recs[0].keys) != 1 {
		t.Error("active tab is not tab 1 after closing tab 2")
	}
}

func TestCloseTabActivatesLeft(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.prefix("t") // tabs 1 2 3, active 3
	h.prefix("2")
	h.prefix("q") // close tab 2 -> tab 1
	h.send(key("x"))
	if len(h.recs[0].keys) != 1 {
		t.Error("active tab is not the one to the left")
	}
}

func TestCloseFirstTabActivatesFirst(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t") // active 2
	h.prefix("1")
	h.prefix("q") // closes tab 1; remaining tab holds view 2
	h.send(key("x"))
	if len(h.recs[1].keys) != 1 {
		t.Error("active tab is not the remaining one")
	}
}

func TestCloseLastPaneOfLastTabDoesNothing(t *testing.T) {
	h := newHarness(100, 30)
	cmd := h.prefix("q")
	if cmd != nil {
		t.Error("unexpected cmd")
	}
	h.send(key("x"))
	if len(h.recs[0].keys) != 1 {
		t.Error("view lost focus")
	}
	if !strings.Contains(h.frame(), "view 1") {
		t.Error("view not rendered")
	}
}

func TestZoom(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.prefix("z")
	if r := h.recs[1]; r.w != 98 || r.h != 26 {
		t.Errorf("zoomed size = %dx%d, want 98x26", r.w, r.h)
	}
	f := h.frame()
	if strings.Contains(f, "view 1") || !strings.Contains(f, "view 2") {
		t.Errorf("zoom does not hide other panes:\n%s", f)
	}
	h.prefix("z")
	if r := h.recs[1]; r.w != 48 {
		t.Errorf("width after unzoom = %d, want 48", r.w)
	}
	if !strings.Contains(h.frame(), "view 1") {
		t.Error("view 1 not back after unzoom")
	}
}

func TestLayoutActionsUnzoom(t *testing.T) {
	for _, k := range []string{"v", "s", "h", "j", "k", "l", "q", "=", "<", ">", "+", "-"} {
		h := newHarness(100, 30)
		h.prefix("v")
		h.prefix("v") // three columns, focus 3
		h.prefix("z")
		h.prefix(k)
		live := len(h.recs)
		if k == "q" {
			live-- // the last view is closed
		}
		f := h.frame()
		for i := 1; i <= live; i++ {
			if want := fmt.Sprintf("view %d", i); !strings.Contains(f, want) {
				t.Errorf("%s did not unzoom: %q missing\n%s", k, want, f)
			}
		}
	}
}

func TestEqualizeAndResize(t *testing.T) {
	// Pane area 100x28: a 0.05 step moves a divider 5 columns or 1.4 rows.
	h := newHarness(100, 30)
	steps := []struct {
		key          string
		view         int
		wantW, wantH int
	}{
		{"v", 2, 48, 26}, // 1 | 2, focus 2
		{">", 2, 53, 26},
		{"=", 2, 48, 26},
		{"<", 2, 43, 26},
		{"=", 2, 48, 26},
		{"s", 3, 48, 12}, // 2 above 3, focus 3
		{"+", 3, 48, 14},
		{"=", 3, 48, 12},
		{"-", 3, 48, 11},
	}
	for _, s := range steps {
		h.prefix(s.key)
		if r := h.recs[s.view-1]; r.w != s.wantW || r.h != s.wantH {
			t.Errorf("after %s view %d size = %dx%d, want %dx%d", s.key, s.view, r.w, r.h, s.wantW, s.wantH)
		}
	}
}

func TestTabs(t *testing.T) {
	h := newHarness(100, 30)
	cmd := h.prefix("t")
	if cmd == nil || cmd() != "init-2" {
		t.Error("t did not return the new view's Init")
	}
	h.send(key("x"))
	if len(h.recs[1].keys) != 1 {
		t.Error("new tab is not active")
	}
	if r := h.recs[1]; r.w != 98 || r.h != 26 {
		t.Errorf("new tab view size = %dx%d", r.w, r.h)
	}

	h.prefix("n") // wraps to 1
	h.send(key("y"))
	if len(h.recs[0].keys) != 1 {
		t.Error("n did not wrap to tab 1")
	}
	h.prefix("p") // wraps to 2
	h.send(key("z"))
	if len(h.recs[1].keys) != 2 {
		t.Error("p did not wrap to tab 2")
	}
	h.prefix("1")
	h.send(key("a"))
	if len(h.recs[0].keys) != 2 {
		t.Error("1 did not switch to tab 1")
	}
	h.prefix("9") // no such tab
	h.send(key("b"))
	if len(h.recs[0].keys) != 3 {
		t.Error("9 changed tab")
	}
}

func TestNewTabInsertsAfterActive(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t") // [1 2]
	h.prefix("1")
	h.prefix("t") // [1 3 2], active 3
	lines := strings.Split(h.frame(), "\n")
	bar := lines[0]
	i1, i3, i2 := strings.Index(bar, " 1 Fake1"), strings.Index(bar, " 2 Fake3"), strings.Index(bar, " 3 Fake2")
	if i1 < 0 || i2 < 0 || i3 < 0 || i1 >= i3 || i3 >= i2 {
		t.Errorf("tab bar = %q", bar)
	}
}

func TestTabBarAndStatusBar(t *testing.T) {
	h := newHarness(120, 30)
	h.prefix("v") // focus view 2
	lines := strings.Split(h.frame(), "\n")
	if !strings.Contains(lines[0], " 1 Fake2 ") {
		t.Errorf("tab bar = %q", lines[0])
	}
	status := lines[len(lines)-1]
	for _, want := range []string{"NORMAL", "Fake2", "^w v/s split", "q quit"} {
		if !strings.Contains(status, want) {
			t.Errorf("status %q lacks %q", status, want)
		}
	}
}

func TestPrefixStatus(t *testing.T) {
	h := newHarness(200, 30)
	h.send(key("ctrl+w"))
	lines := strings.Split(h.frame(), "\n")
	status := lines[len(lines)-1]
	for _, want := range []string{"CTRL-W", "esc cancel"} {
		if !strings.Contains(status, want) {
			t.Errorf("status %q lacks %q", status, want)
		}
	}
	h.send(key("v"))
	lines = strings.Split(h.frame(), "\n")
	if !strings.Contains(lines[len(lines)-1], "NORMAL") {
		t.Error("badge did not return to NORMAL")
	}
}

func TestPrefixCancel(t *testing.T) {
	for _, k := range []string{"esc", "x"} {
		h := newHarness(100, 30)
		h.send(key("ctrl+w"))
		h.send(key(k))
		if len(h.recs[0].keys) != 0 {
			t.Errorf("%s was forwarded: %v", k, h.recs[0].keys)
		}
		// back in normal mode: q quits
		if !isQuit(h.send(key("q"))) {
			t.Errorf("%s: not back in normal mode", k)
		}
	}
}

func TestOtherMessagesReachAllViews(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.prefix("v")
	cmd := h.send(pingMsg{})
	for i, r := range h.recs {
		if r.pings != 1 {
			t.Errorf("view %d pings = %d, want 1", i+1, r.pings)
		}
	}
	got := []string{}
	for _, msg := range collect(cmd) {
		got = append(got, msg.(string))
	}
	slices.Sort(got)
	if want := []string{"pong-1", "pong-2", "pong-3"}; !slices.Equal(got, want) {
		t.Errorf("batched messages = %v, want %v", got, want)
	}
}

func TestFrameFits(t *testing.T) {
	setups := map[string]func(h *harness){
		"one pane": func(h *harness) {},
		"splits": func(h *harness) {
			h.prefix("v")
			h.prefix("s")
			h.prefix("h")
			h.prefix("s")
		},
		"zoomed": func(h *harness) { h.prefix("v"); h.prefix("z") },
		"prefix": func(h *harness) { h.send(key("ctrl+w")) },
		"tabs": func(h *harness) {
			for range 12 {
				h.prefix("t")
			}
		},
	}
	sizes := [][2]int{{100, 30}, {101, 31}, {30, 8}, {13, 5}, {20, 3}}
	for name, setup := range setups {
		for _, s := range sizes {
			h := newHarness(s[0], s[1])
			setup(h)
			f := h.frame()
			if got := lipgloss.Height(f); got != s[1] {
				t.Errorf("%s %v: height = %d", name, s, got)
			}
			if got := lipgloss.Width(f); got > s[0] {
				t.Errorf("%s %v: width = %d", name, s, got)
			}
		}
	}
}

func TestFrameFitsAfterShrink(t *testing.T) {
	var sizes [][2]int
	for w := 1; w <= 10; w++ {
		sizes = append(sizes, [2]int{w, 30})
	}
	for ht := 1; ht <= 6; ht++ {
		sizes = append(sizes, [2]int{100, ht})
	}
	for _, s := range sizes {
		h := newHarness(100, 30)
		h.prefix("v")
		h.send(tea.WindowSizeMsg{Width: s[0], Height: s[1]})
		f := h.frame()
		if got := lipgloss.Height(f); got != s[1] {
			t.Errorf("%v: height = %d", s, got)
		}
		if got := lipgloss.Width(f); got > s[0] {
			t.Errorf("%v: width = %d", s, got)
		}
	}
}

func TestSplitGuard(t *testing.T) {
	// 20x10: pane area 20x8. Horizontal split gives 10 columns: too narrow.
	h := newHarness(20, 10)
	h.prefix("v")
	if len(h.recs) != 1 {
		t.Errorf("narrow split created a view")
	}
	// Vertical split gives 4 rows each: allowed.
	h.prefix("s")
	if len(h.recs) != 2 {
		t.Errorf("split with 4 rows was ignored")
	}
	// Splitting again would leave 2 rows.
	h.prefix("s")
	if len(h.recs) != 2 {
		t.Errorf("short split created a view")
	}
	h.send(key("x"))
	if len(h.recs[1].keys) != 1 {
		t.Error("focus changed by ignored split")
	}
}

// stub is a view with a fixed title and body.
type stub struct{ title, body string }

func (s stub) Init() tea.Cmd                        { return nil }
func (s stub) Update(tea.Msg) (views.View, tea.Cmd) { return s, nil }
func (s stub) Resize(int, int) views.View           { return s }
func (s stub) Render() string                       { return s.body }
func (s stub) Title() string                        { return s.title }

func TestRenderPaneClipsTitleAndBody(t *testing.T) {
	long := strings.Repeat("x", 50)
	v := stub{title: long, body: long + "\n" + long + "\n" + long}

	for _, r := range []layout.Rect{{W: 2, H: 2}, {W: 5, H: 3}, {W: 15, H: 6}} {
		lines := strings.Split(sgr.ReplaceAllString(renderPane(v, r, true), ""), "\n")
		if len(lines) != r.H {
			t.Errorf("%dx%d: %d lines, want %d", r.W, r.H, len(lines), r.H)
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w != r.W {
				t.Errorf("%dx%d: line %d width %d, want %d: %q", r.W, r.H, i, w, r.W, l)
			}
		}
		if !strings.HasSuffix(lines[0], "┐") {
			t.Errorf("%dx%d: top border lost its corner: %q", r.W, r.H, lines[0])
		}
		for _, l := range lines[1 : len(lines)-1] {
			if !strings.HasPrefix(l, "│") || !strings.HasSuffix(l, "│") {
				t.Errorf("%dx%d: body line lost a side border: %q", r.W, r.H, l)
			}
		}
	}
}

func TestWindowAndTabChangesResizeViews(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.prefix("t")

	steps := []struct {
		name string
		do   func()
		want map[int][2]int // view id -> inner size
	}{
		{"window change, tab 2", func() { h.send(tea.WindowSizeMsg{Width: 80, Height: 24}) }, map[int][2]int{3: {78, 20}}},
		{"p to tab 1", func() { h.prefix("p") }, map[int][2]int{1: {38, 20}, 2: {38, 20}}},
		{"window change, tab 1", func() { h.send(tea.WindowSizeMsg{Width: 70, Height: 24}) }, map[int][2]int{1: {33, 20}, 2: {33, 20}}},
		{"n to tab 2", func() { h.prefix("n") }, map[int][2]int{3: {68, 20}}},
		{"window change, tab 2 again", func() { h.send(tea.WindowSizeMsg{Width: 90, Height: 24}) }, map[int][2]int{3: {88, 20}}},
		{"1 to tab 1", func() { h.prefix("1") }, map[int][2]int{1: {43, 20}, 2: {43, 20}}},
	}
	for _, s := range steps {
		s.do()
		for id, size := range s.want {
			r := h.recs[id-1]
			if r.w != size[0] || r.h != size[1] {
				t.Errorf("%s: view %d is %dx%d, want %dx%d", s.name, id, r.w, r.h, size[0], size[1])
			}
		}
	}
}

func TestBorderTitle(t *testing.T) {
	h := newHarness(40, 10)
	lines := strings.Split(h.frame(), "\n")
	if !strings.HasPrefix(lines[1], "┌ Fake1 ") || !strings.HasSuffix(lines[1], "┐") {
		t.Errorf("top border = %q", lines[1])
	}
	if !strings.HasPrefix(lines[len(lines)-2], "└") {
		t.Errorf("bottom border = %q", lines[len(lines)-2])
	}
}

func TestStyles(t *testing.T) {
	h := newHarness(60, 10)
	lines := strings.Split(h.raw(), "\n")
	checks := []struct {
		name, line, want string
	}{
		{"active tab", lines[0], lipgloss.NewStyle().Reverse(true).Render(" 1 Fake1 ")},
		{"focused border", lines[1], strings.TrimSuffix(lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("┌"), "\x1b[m")},
		{"focused title", lines[1], strings.TrimSuffix(lipgloss.NewStyle().Bold(true).Render(" Fake1 "), "\x1b[m")},
		{"normal badge", lines[len(lines)-1], lipgloss.NewStyle().Reverse(true).Render(" NORMAL ")},
	}
	h.send(key("ctrl+w"))
	lines = strings.Split(h.raw(), "\n")
	checks = append(checks, struct{ name, line, want string }{
		"prefix badge", lines[len(lines)-1], lipgloss.NewStyle().Reverse(true).Render(" CTRL-W "),
	})
	h.send(key("v")) // the left pane loses focus
	lines = strings.Split(h.raw(), "\n")
	checks = append(checks, struct{ name, line, want string }{
		"unfocused border", lines[1], strings.TrimSuffix(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("┌"), "\x1b[m"),
	})
	for _, c := range checks {
		if !strings.Contains(c.line, c.want) {
			t.Errorf("%s: %q does not contain %q", c.name, c.line, c.want)
		}
	}
}

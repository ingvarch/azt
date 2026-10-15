package ui

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

type pingMsg struct{}

// fake records what it receives in its own fields, so a stack copied by a
// split can be told apart from the original. Copies share only rec, and Open
// returns a new fake, so a pane can hold a stack of them.
type fake struct {
	rec     *record
	next    func() views.View
	keys    []string
	pings   int
	w, h    int
	results []data.ResultMsg
	fetches int
	clicks  [][2]int
}

type record struct {
	id     int
	inits  int
	ask    *data.Key // when set, Init asks for this key
	noOpen bool
}

func (f fake) Init() tea.Cmd {
	f.rec.inits++
	if f.rec.ask != nil {
		return data.Fetch(*f.rec.ask)
	}
	return func() tea.Msg { return fmt.Sprintf("init-%d", f.rec.id) }
}

func (f fake) Update(msg tea.Msg) (views.View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		f.keys = append(f.keys[:len(f.keys):len(f.keys)], msg.String())
		return f, f.reply("key")
	case pingMsg:
		f.pings++
		return f, f.reply("pong")
	case data.ResultMsg:
		f.results = append(f.results[:len(f.results):len(f.results)], msg)
	case data.FetchMsg:
		f.fetches++
	}
	return f, nil
}

// Click records the click; line 0 counts as a header, every other line as a row.
func (f fake) Click(x, y int) (views.View, tea.Cmd, bool) {
	f.clicks = append(f.clicks[:len(f.clicks):len(f.clicks)], [2]int{x, y})
	return f, nil, y > 0
}

func (f fake) reply(prefix string) tea.Cmd {
	return func() tea.Msg { return fmt.Sprintf("%s-%d", prefix, f.rec.id) }
}

func (f fake) Resize(w, h int) views.View {
	f.w, f.h = w, h
	return f
}

func (f fake) Open() (views.View, bool) {
	if f.rec.noOpen {
		return nil, false
	}
	return f.next(), true
}

func (f fake) Render() string   { return fmt.Sprintf("view %d", f.rec.id) }
func (f fake) Title() string    { return fmt.Sprintf("Fake%d", f.rec.id) }
func (f fake) Spec() views.Spec { return views.Spec{Kind: "fake"} }

// harness builds fake views and tracks them in creation order.
type harness struct {
	recs  []*record
	calls []data.Key // keys the Fetcher was called with
	ask   *data.Key  // new views ask for this key in Init
	m     Model
	clock time.Time // what the model reads as now
}

func (h *harness) newFake() views.View {
	r := &record{id: len(h.recs) + 1, ask: h.ask}
	h.recs = append(h.recs, r)
	return fake{rec: r, next: h.newFake}
}

func (h *harness) fetch(k data.Key) (any, error) {
	h.calls = append(h.calls, k)
	return "data-" + k.ID, nil
}

// newHarness hides the sidebar, so pane geometry is the whole window.
func newHarness(w, h int) *harness {
	hs := newSidebarHarness(w, h)
	hs.prefix("e")
	return hs
}

// newSidebarHarness keeps the sidebar visible, as at start.
func newSidebarHarness(w, h int) *harness {
	hs := &harness{}
	hs.m = New(hs.newFake, hs.fetch)
	hs.m.now = func() time.Time { return hs.clock }
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
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

// view returns the view at depth in the stack of pane id.
func (h *harness) view(id layout.PaneID, depth int) fake { return h.m.stacks[id][depth].(fake) }

// pane returns the view shown in pane id.
func (h *harness) pane(id layout.PaneID) fake { return h.m.top(id).(fake) }

// fakes returns every view of every stack: panes in ID order, bottom first.
func (h *harness) fakes() []fake {
	var out []fake
	for _, id := range slices.Sorted(maps.Keys(h.m.stacks)) {
		for _, v := range h.m.stacks[id] {
			out = append(out, v.(fake))
		}
	}
	return out
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
	msgs := collect(h.m.Init())
	if !slices.Contains(msgs, any("init-1")) {
		t.Errorf("Init messages = %v, lack the first view's", msgs)
	}
	if !slices.Contains(msgs, any(data.FetchMsg{Key: data.Key{Kind: data.Subscriptions}})) {
		t.Errorf("Init messages = %v, lack the sidebar's fetch", msgs)
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
	m := New(func() views.View { return fake{rec: &record{id: 1}} }, nil)
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
	if got := h.pane(0).keys; len(got) != 0 {
		t.Errorf("view got keys %v", got)
	}
}

func TestNormalKeysReachOnlyFocusedView(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	cmd := h.send(key("x"))
	if got := h.pane(1).keys; len(got) != 1 || got[0] != "x" {
		t.Errorf("focused keys = %v", got)
	}
	if got := collect(cmd); len(got) != 1 || got[0] != "key-1" {
		t.Errorf("returned messages = %v, want [key-1]", got)
	}
	if got := h.pane(0).keys; len(got) != 0 {
		t.Errorf("other view keys = %v", got)
	}
}

func TestSplitSizesPanes(t *testing.T) {
	tests := []struct {
		key          string
		wantW, wantH int
	}{
		{"v", 48, 26},
		{"s", 98, 12},
	}
	for _, tt := range tests {
		h := newHarness(100, 30)
		h.prefix(tt.key)
		if len(h.m.stacks) != 2 {
			t.Fatalf("%s: panes = %d, want 2", tt.key, len(h.m.stacks))
		}
		for id := range layout.PaneID(2) {
			if r := h.pane(id); r.w != tt.wantW || r.h != tt.wantH {
				t.Errorf("%s: pane %d size = %dx%d, want %dx%d", tt.key, id, r.w, r.h, tt.wantW, tt.wantH)
			}
		}
		h.send(key("x"))
		if len(h.pane(1).keys) != 1 {
			t.Errorf("%s: focus did not move to the new pane", tt.key)
		}
	}
}

func TestSinglePaneInnerSize(t *testing.T) {
	h := newHarness(100, 30)
	if r := h.pane(0); r.w != 98 || r.h != 26 {
		t.Errorf("size = %dx%d, want 98x26", r.w, r.h)
	}
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	if r := h.pane(0); r.w != 58 || r.h != 16 {
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
		before := len(h.pane(layout.PaneID(s.want - 1)).keys)
		h.send(key("x"))
		if got := h.pane(layout.PaneID(s.want - 1)).keys; len(got) != before+1 {
			t.Errorf("after %s focus is not on view %d", s.key, s.want)
		}
	}
}

func TestClosePaneMovesFocus(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v") // 1 | 2, focus 2
	h.prefix("q")
	h.send(key("x"))
	if len(h.pane(0).keys) != 1 {
		t.Errorf("focus after close not on view 1")
	}
	if got := h.pane(0).w; got != 98 {
		t.Errorf("remaining pane width = %d, want 98", got)
	}
	if got := strings.Count(h.frame(), "view 1"); got != 1 {
		t.Errorf("closed view still rendered: %d views shown, want 1", got)
	}
	if got := len(h.m.stacks); got != 1 {
		t.Errorf("views = %d, want 1", got)
	}
}

func TestCloseFocusMovesToCoverOfTopLeftCell(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v") // 1 | 2
	h.prefix("s") // 1 | (2 above 3), focus 3
	h.prefix("q") // pane 2 covers the closed cell; the first pane is 1
	h.send(key("x"))
	if len(h.pane(1).keys) != 1 || len(h.pane(0).keys) != 0 {
		t.Errorf("focus is not on view 2: %v %v", h.pane(0).keys, h.pane(1).keys)
	}
}

func TestCloseAfterShrinkKeepsFocusOnLivePane(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.send(tea.WindowSizeMsg{Width: 100, Height: 2})
	h.prefix("q")
	_ = h.frame()
	h.send(key("x"))
	if got := h.pane(0).keys; len(got) != 1 {
		t.Errorf("focus is not on the remaining view: %v", got)
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
	if got := len(h.m.stacks); got != 1 {
		t.Errorf("views = %d, want 1", got)
	}
	h.send(key("x"))
	if len(h.pane(0).keys) != 1 {
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
	if len(h.pane(0).keys) != 1 {
		t.Error("active tab is not the one to the left")
	}
}

func TestCloseFirstTabActivatesFirst(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t") // active 2
	h.prefix("1")
	h.prefix("q") // closes tab 1; remaining tab holds view 2
	h.send(key("x"))
	if len(h.pane(1).keys) != 1 {
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
	if len(h.pane(0).keys) != 1 {
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
	if r := h.pane(1); r.w != 98 || r.h != 26 {
		t.Errorf("zoomed size = %dx%d, want 98x26", r.w, r.h)
	}
	if f := h.frame(); strings.Count(f, "view 1") != 1 {
		t.Errorf("zoom does not hide other panes:\n%s", f)
	}
	h.prefix("z")
	if r := h.pane(1); r.w != 48 {
		t.Errorf("width after unzoom = %d, want 48", r.w)
	}
	if got := strings.Count(h.frame(), "view 1"); got != 2 {
		t.Errorf("panes shown after unzoom = %d, want 2", got)
	}
}

func TestLayoutActionsUnzoom(t *testing.T) {
	for _, k := range []string{"v", "s", "h", "j", "k", "l", "q", "=", "<", ">", "+", "-"} {
		h := newHarness(100, 30)
		h.prefix("v")
		h.prefix("v") // three columns, focus 3
		h.prefix("z")
		h.prefix(k)
		f := h.frame()
		if got, want := strings.Count(f, "view 1"), len(h.m.stacks); got != want {
			t.Errorf("%s did not unzoom: %d panes shown, want %d\n%s", k, got, want, f)
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
		if r := h.pane(layout.PaneID(s.view - 1)); r.w != s.wantW || r.h != s.wantH {
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
	if len(h.pane(1).keys) != 1 {
		t.Error("new tab is not active")
	}
	if r := h.pane(1); r.w != 98 || r.h != 26 {
		t.Errorf("new tab view size = %dx%d", r.w, r.h)
	}

	h.prefix("n") // wraps to 1
	h.send(key("y"))
	if len(h.pane(0).keys) != 1 {
		t.Error("n did not wrap to tab 1")
	}
	h.prefix("p") // wraps to 2
	h.send(key("z"))
	if len(h.pane(1).keys) != 2 {
		t.Error("p did not wrap to tab 2")
	}
	h.prefix("1")
	h.send(key("a"))
	if len(h.pane(0).keys) != 2 {
		t.Error("1 did not switch to tab 1")
	}
	h.prefix("9") // no such tab
	h.send(key("b"))
	if len(h.pane(0).keys) != 3 {
		t.Error("9 changed tab")
	}
}

func TestNewTabInsertsAfterActive(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t") // [1 2], active 2
	h.m.tabs[1].name = "two"
	h.prefix("1")
	h.prefix("t") // [1 new two]
	if got, want := strings.TrimRight(h.bar(), " "), "   1    "+" "+"   2    "+" "+"  3 two  "+" "+" +"; got != want {
		t.Errorf("tab bar = %q, want %q", got, want)
	}
	if h.m.active != 1 || h.m.mode != modeNormal {
		t.Errorf("active = %d, mode %v, want 1 and normal", h.m.active, h.m.mode)
	}
}

func TestTabBarAndStatusBar(t *testing.T) {
	h := newHarness(200, 30)
	h.prefix("v")
	lines := strings.Split(h.frame(), "\n")
	if !strings.Contains(lines[0], " 1 ") {
		t.Errorf("tab bar = %q", lines[0])
	}
	status := lines[len(lines)-1]
	for _, want := range []string{"NORMAL", "^w v/s split", "q quit"} {
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
		if got := h.pane(0).keys; len(got) != 0 {
			t.Errorf("%s was forwarded: %v", k, got)
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
	for i, f := range h.fakes() {
		if f.pings != 1 {
			t.Errorf("view %d pings = %d, want 1", i+1, f.pings)
		}
	}
	got := []string{}
	for _, msg := range collect(cmd) {
		got = append(got, msg.(string))
	}
	slices.Sort(got)
	if want := []string{"pong-1", "pong-2", "pong-2"}; !slices.Equal(got, want) {
		t.Errorf("batched messages = %v, want %v", got, want)
	}
}

func TestFrameFits(t *testing.T) {
	setups := map[string]func(h *harness){
		"one pane": func(*harness) {},
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
	if len(h.m.stacks) != 1 {
		t.Errorf("narrow split created a pane")
	}
	// Vertical split gives 4 rows each: allowed.
	h.prefix("s")
	if len(h.m.stacks) != 2 {
		t.Errorf("split with 4 rows was ignored")
	}
	// Splitting again would leave 2 rows.
	h.prefix("s")
	if len(h.m.stacks) != 2 {
		t.Errorf("short split created a pane")
	}
	h.send(key("x"))
	if len(h.pane(1).keys) != 1 {
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
func (s stub) Open() (views.View, bool)             { return nil, false }
func (s stub) Spec() views.Spec                     { return views.Spec{Kind: "stub"} }

func TestRenderPaneClipsTitleAndBody(t *testing.T) {
	long := strings.Repeat("x", 50)
	v := stub{title: long, body: long + "\n" + long + "\n" + long}

	for _, r := range []layout.Rect{{W: 2, H: 2}, {W: 5, H: 3}, {W: 15, H: 6}} {
		lines := strings.Split(sgr.ReplaceAllString(renderPane([]views.View{v}, r, true), ""), "\n")
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
		want map[layout.PaneID][2]int // pane id -> inner size
	}{
		{"window change, tab 2", func() { h.send(tea.WindowSizeMsg{Width: 80, Height: 24}) }, map[layout.PaneID][2]int{2: {78, 20}}},
		{"p to tab 1", func() { h.prefix("p") }, map[layout.PaneID][2]int{0: {38, 20}, 1: {38, 20}}},
		{"window change, tab 1", func() { h.send(tea.WindowSizeMsg{Width: 70, Height: 24}) }, map[layout.PaneID][2]int{0: {33, 20}, 1: {33, 20}}},
		{"n to tab 2", func() { h.prefix("n") }, map[layout.PaneID][2]int{2: {68, 20}}},
		{"window change, tab 2 again", func() { h.send(tea.WindowSizeMsg{Width: 90, Height: 24}) }, map[layout.PaneID][2]int{2: {88, 20}}},
		{"1 to tab 1", func() { h.prefix("1") }, map[layout.PaneID][2]int{0: {43, 20}, 1: {43, 20}}},
	}
	for _, s := range steps {
		s.do()
		for id, size := range s.want {
			r := h.pane(id)
			if r.w != size[0] || r.h != size[1] {
				t.Errorf("%s: pane %d is %dx%d, want %dx%d", s.name, id, r.w, r.h, size[0], size[1])
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

func TestEnterPushesTopViewAndRunsInit(t *testing.T) {
	h := newHarness(100, 30)
	cmd := h.send(key("enter"))
	if len(h.recs) != 2 {
		t.Fatalf("views = %d, want 2", len(h.recs))
	}
	if cmd == nil || cmd() != "init-2" {
		t.Error("enter did not return the new view's Init")
	}
	if r := h.pane(0); r.w != 98 || r.h != 26 {
		t.Errorf("pushed view size = %dx%d, want 98x26", r.w, r.h)
	}
	if f := h.frame(); !strings.Contains(f, "view 2") || strings.Contains(f, "view 1") {
		t.Errorf("frame does not show only the pushed view:\n%s", f)
	}
	h.send(key("x"))
	if top, below := h.view(0, 1).keys, h.view(0, 0).keys; len(top) != 1 || len(below) != 0 {
		t.Errorf("keys: top %v, below %v", top, below)
	}
}

func TestEnterOpensInFocusedPaneOnly(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("v")
	h.send(key("enter"))
	if a, b := len(h.m.stacks[0]), len(h.m.stacks[1]); a != 1 || b != 2 {
		t.Errorf("stack depths = %d and %d, want 1 and 2", a, b)
	}
	if f := h.frame(); !strings.Contains(f, "view 1") || !strings.Contains(f, "view 2") {
		t.Errorf("frame:\n%s", f)
	}
}

func TestEnterAndEscAreNotForwarded(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("enter"))
	h.send(key("esc"))
	h.send(key("esc"))
	for i, f := range h.fakes() {
		if len(f.keys) != 0 {
			t.Errorf("view %d got keys %v", i+1, f.keys)
		}
	}
}

func TestEnterWithNothingToOpen(t *testing.T) {
	h := newHarness(100, 30)
	h.recs[0].noOpen = true
	if cmd := h.send(key("enter")); cmd != nil {
		t.Error("unexpected cmd")
	}
	if len(h.recs) != 1 || !strings.Contains(h.frame(), "view 1") {
		t.Error("a view was pushed")
	}
}

func TestEscPopsAndResizesRevealedView(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("enter"))
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20}) // only the top view is resized
	if r := h.view(0, 0); r.w != 98 {
		t.Fatalf("view below the top was resized to %d", r.w)
	}
	if cmd := h.send(key("esc")); cmd != nil {
		t.Error("unexpected cmd")
	}
	if r := h.pane(0); r.w != 58 || r.h != 16 {
		t.Errorf("revealed view size = %dx%d, want 58x16", r.w, r.h)
	}
	if f := h.frame(); !strings.Contains(f, "view 1") || strings.Contains(f, "view 2") {
		t.Errorf("frame after esc:\n%s", f)
	}
	h.send(key("x"))
	if len(h.pane(0).keys) != 1 {
		t.Error("keys do not reach the revealed view")
	}
}

func TestEscWithOneViewDoesNothing(t *testing.T) {
	h := newHarness(100, 30)
	if cmd := h.send(key("esc")); cmd != nil {
		t.Error("unexpected cmd")
	}
	if !strings.Contains(h.frame(), "view 1") {
		t.Error("view lost")
	}
}

func TestFetchGoesThroughCache(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("enter")) // stack [1 2]
	h.prefix("v")        // pane 1 holds a copy
	k := data.Key{Kind: data.Resources, Group: "g"}
	cmd := h.send(data.FetchMsg{Key: k})
	for i, f := range h.fakes() {
		if f.fetches != 0 || len(f.results) != 0 {
			t.Errorf("view %d got %d FetchMsgs and %d results, want none", i+1, f.fetches, len(f.results))
		}
	}
	if len(h.calls) != 0 {
		t.Fatal("fetched before the command ran")
	}
	msgs := collect(cmd)
	if len(msgs) != 1 || msgs[0] != (data.ResultMsg{Key: k, Data: "data-"}) {
		t.Fatalf("messages = %v", msgs)
	}
	h.send(msgs[0])
	if got := collect(h.send(data.FetchMsg{Key: k})); len(got) != 1 || len(h.calls) != 1 {
		t.Errorf("cached key: messages %v, fetcher calls %d, want 1 message and 1 call", got, len(h.calls))
	}
}

func TestPanesAskingForOneKeyFetchOnce(t *testing.T) {
	h := newHarness(100, 30)
	k := data.Key{Kind: data.Details, ID: "x"}
	h.ask = &k
	first := collect(h.send(key("enter"))) // pane 1, new view asks for k
	second := collect(h.prefix("t"))       // tab 2, new view asks for k
	var fetch tea.Cmd
	for _, msgs := range [][]tea.Msg{first, second} {
		if len(msgs) != 1 || msgs[0] != (data.FetchMsg{Key: k}) {
			t.Fatalf("messages = %v, want one FetchMsg", msgs)
		}
		if cmd := h.send(msgs[0]); cmd != nil {
			if fetch != nil {
				t.Fatal("two fetches started")
			}
			fetch = cmd
		}
	}
	for _, msg := range collect(fetch) {
		h.send(msg)
	}
	if len(h.calls) != 1 {
		t.Errorf("fetcher calls = %d, want 1", len(h.calls))
	}
	for _, i := range []int{1, 2} { // the views that asked
		if got := h.fakes()[i].results; len(got) != 1 || got[0].Data != "data-x" {
			t.Errorf("view %d results = %v", i+1, got)
		}
	}
}

func TestMessagesReachViewsBelowTopAndInHiddenTabs(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("enter")) // stack [1 2]
	h.prefix("t")        // view 3 in tab 2
	h.prefix("1")        // tab 2 is hidden
	result := data.ResultMsg{Key: data.Key{Kind: data.Details, ID: "x"}, Data: "d"}
	h.send(result)
	h.send(pingMsg{})
	for i, f := range h.fakes() {
		if len(f.results) != 1 || f.pings != 1 {
			t.Errorf("view %d: results %d, pings %d, want 1 and 1", i+1, len(f.results), f.pings)
		}
	}
	msgs := collect(h.send(data.FetchMsg{Key: result.Key}))
	if len(h.calls) != 0 || len(msgs) != 1 || msgs[0] != result {
		t.Errorf("ResultMsg was not stored in the cache: fetcher calls %d, messages %v", len(h.calls), msgs)
	}
}

func TestSplitCopiesStack(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("enter")) // stack [1 2]
	inits := h.recs[1].inits
	cmd := h.prefix("v")
	if cmd != nil || len(h.recs) != 2 {
		t.Errorf("split ran Init or created a view: cmd %v, views %d", cmd != nil, len(h.recs))
	}
	if h.recs[1].inits != inits {
		t.Error("Init ran for the copied views")
	}
	left, right := titles(h.m.stacks[0]), titles(h.m.stacks[1])
	if want := []string{"Fake1", "Fake2"}; !slices.Equal(left, want) || !slices.Equal(right, want) {
		t.Fatalf("stacks = %q and %q, want %q", left, right, want)
	}
	if r := h.pane(1); r.w != 48 {
		t.Errorf("copied view size = %d, want 48", r.w)
	}
	h.send(key("x"))
	if len(h.pane(1).keys) != 1 {
		t.Error("focus did not move to the new pane")
	}
}

func TestSplitStacksAreIndependent(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("enter")) // stack [1 2]
	h.prefix("v")        // pane 1 gets a copy, focus moves there
	h.send(key("esc"))
	if got, want := titles(h.m.stacks[0]), []string{"Fake1", "Fake2"}; !slices.Equal(got, want) {
		t.Errorf("esc in the new pane changed the old one: %q", got)
	}
	if got, want := titles(h.m.stacks[1]), []string{"Fake1"}; !slices.Equal(got, want) {
		t.Errorf("new pane stack = %q, want %q", got, want)
	}
	h.send(key("enter")) // pushes into the slot the old pane's top also occupied if shared
	if got, want := titles(h.m.stacks[0]), []string{"Fake1", "Fake2"}; !slices.Equal(got, want) {
		t.Errorf("enter in the new pane overwrote the old one: %q", got)
	}
	if got, want := titles(h.m.stacks[1]), []string{"Fake1", "Fake3"}; !slices.Equal(got, want) {
		t.Errorf("new pane stack = %q, want %q", got, want)
	}
}

func TestNewTabStartsWithNewView(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("enter"))
	h.prefix("t")
	if got, want := titles(h.m.stacks[1]), []string{"Fake3"}; !slices.Equal(got, want) {
		t.Errorf("new tab stack = %q, want %q", got, want)
	}
}

func TestFitPath(t *testing.T) {
	path := []string{"Prod", "prod-rg", "web-app"}
	tests := []struct {
		name  string
		path  []string
		width int
		want  string
	}{
		{"fits", path, 100, "Prod › prod-rg › web-app"},
		{"exact", path, 24, "Prod › prod-rg › web-app"},
		{"drops one", path, 23, "… › prod-rg › web-app"},
		{"drops two", path, 20, "… › web-app"},
		{"last alone", path, 10, "web-app"},
		{"last cut", path, 5, "web-a"},
		{"nothing fits", path, 0, ""},
		{"single cut", []string{"long-title"}, 4, "long"},
		{"single fits", []string{"title"}, 5, "title"},
		{"no path", nil, 5, ""},
	}
	for _, tt := range tests {
		if got := fitPath(tt.path, tt.width); got != tt.want {
			t.Errorf("%s: fitPath(%v, %d) = %q, want %q", tt.name, tt.path, tt.width, got, tt.want)
		}
	}
}

func TestBorderTitleShowsPath(t *testing.T) {
	h := newHarness(40, 10)
	h.send(key("enter"))
	h.send(key("enter"))
	lines := strings.Split(h.frame(), "\n")
	if !strings.HasPrefix(lines[1], "┌ Fake1 › Fake2 › Fake3 ") {
		t.Errorf("top border = %q", lines[1])
	}
	h.send(tea.WindowSizeMsg{Width: 24, Height: 10})
	lines = strings.Split(h.frame(), "\n")
	if !strings.HasPrefix(lines[1], "┌ … › Fake2 › Fake3 ") || lipgloss.Width(lines[1]) != 24 {
		t.Errorf("cut top border = %q", lines[1])
	}
}

func TestTabLabelIgnoresPathAndStatusBarShowsNoPath(t *testing.T) {
	h := newHarness(200, 10)
	h.send(key("enter"))
	h.send(key("enter"))
	lines := strings.Split(h.frame(), "\n")
	if got := strings.TrimRight(lines[0], " "); got != "   1    "+" "+" +" {
		t.Errorf("tab bar = %q", got)
	}
	if status := lines[len(lines)-1]; !strings.Contains(status, normalHints) || strings.Contains(status, "Fake") {
		t.Errorf("status %q: want the hints and no path", status)
	}
}

package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

type filtRec struct {
	filter string
	sets   []string
	keys   []string
}

// filt is a Filterer that records what it receives in a shared record.
type filt struct{ rec *filtRec }

func (f filt) Init() tea.Cmd { return nil }
func (f filt) Update(msg tea.Msg) (views.View, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		f.rec.keys = append(f.rec.keys, k.String())
	}
	return f, nil
}
func (f filt) Resize(int, int) views.View { return f }
func (f filt) Render() string             { return "filterable" }
func (f filt) Title() string              { return "Filt" }
func (f filt) Open() (views.View, bool)   { return nil, false }
func (f filt) Spec() views.Spec           { return views.Spec{Kind: "filt"} }
func (f filt) Filter() string             { return f.rec.filter }
func (f filt) SetFilter(q string) views.View {
	f.rec.filter = q
	f.rec.sets = append(f.rec.sets, q)
	return f
}

// newFilterHarness shows one filt in pane 0, with the sidebar hidden.
func newFilterHarness(w, h int) (*harness, *filtRec) {
	hs := newHarness(w, h)
	rec := &filtRec{}
	hs.m.stacks[0] = []views.View{filt{rec}}
	return hs, rec
}

func (h *harness) typ(s string) {
	for _, r := range s {
		h.send(key(string(r)))
	}
}

func (h *harness) status() string {
	lines := strings.Split(h.frame(), "\n")
	return lines[len(lines)-1]
}

func wantIn(t *testing.T, what, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s %q lacks %q", what, got, w)
		}
	}
}

func TestFilterModeEditsAndKeeps(t *testing.T) {
	h, rec := newFilterHarness(100, 30)
	h.send(key("/"))
	if h.m.mode != modeFilter {
		t.Fatalf("mode = %v, want filter", h.m.mode)
	}
	wantIn(t, "status", h.status(), "/", "enter apply  esc clear")
	h.typ("web")
	if got := strings.Join(rec.sets, ","); got != "w,we,web" {
		t.Errorf("SetFilter calls = %q", got)
	}
	wantIn(t, "status", h.status(), "/web")
	h.send(key("enter"))
	if h.m.mode != modeNormal || rec.filter != "web" || len(rec.sets) != 3 {
		t.Errorf("after enter: mode %v, filter %q, sets %v", h.m.mode, rec.filter, rec.sets)
	}
	if len(rec.keys) != 0 {
		t.Errorf("keys reached the view: %v", rec.keys)
	}
	if !isQuit(h.send(key("q"))) {
		t.Error("q does not quit in normal mode")
	}
}

func TestFilterStartsWithCurrentFilter(t *testing.T) {
	h, rec := newFilterHarness(100, 30)
	rec.filter = "abc"
	h.send(key("/"))
	wantIn(t, "status", h.status(), "/abc")
	h.typ("d")
	h.send(key("backspace"))
	h.send(key("backspace"))
	if got := strings.Join(rec.sets, ","); got != "abcd,abc,ab" {
		t.Errorf("SetFilter calls = %q (cursor not at the end?)", got)
	}
}

func TestInputStartsWithCursorAtEnd(t *testing.T) {
	h, rec := newFilterHarness(100, 30)
	h.send(key("/"))
	h.typ("abc")
	h.send(key("left"))
	h.send(key("enter"))
	h.send(key("/"))
	h.typ("d")
	if got := rec.sets[len(rec.sets)-1]; got != "abcd" {
		t.Errorf("filter = %q, want abcd", got)
	}
}

func TestFilterEscClears(t *testing.T) {
	h, rec := newFilterHarness(100, 30)
	h.send(key("/"))
	h.typ("web")
	h.send(key("esc"))
	if h.m.mode != modeNormal || rec.filter != "" {
		t.Errorf("after esc: mode %v, filter %q", h.m.mode, rec.filter)
	}
	h.send(key("/"))
	wantIn(t, "status", h.status(), "/")
	if strings.Contains(h.status(), "/web") {
		t.Error("input kept the cleared filter")
	}
}

func TestFilterDoesNothingWithoutFilterer(t *testing.T) {
	sidebar, srec := newSidebarHarness(100, 30), &filtRec{}
	sidebar.m.stacks[0] = []views.View{filt{srec}}
	sidebar.prefix("h")

	details := newHarness(100, 30)
	details.m.stacks[0] = []views.View{views.NewDetails("s", azure.Resource{Name: "x"})}

	plain := newHarness(100, 30)

	for name, h := range map[string]*harness{"sidebar focus": sidebar, "details": details, "fake view": plain} {
		h.send(key("/"))
		if h.m.mode != modeNormal {
			t.Errorf("%s: mode = %v", name, h.m.mode)
		}
	}
	if len(srec.keys)+len(srec.sets) != 0 {
		t.Errorf("sidebar focus: the view got %v %v", srec.keys, srec.sets)
	}
	if got := plain.pane(0).keys; len(got) != 0 {
		t.Errorf("fake view got %v", got)
	}
}

func TestFilterReachesFocusedViewOnly(t *testing.T) {
	h, left := newFilterHarness(100, 30)
	h.prefix("v")
	right := &filtRec{}
	h.m.stacks[1] = []views.View{filt{right}}
	h.send(key("/"))
	h.typ("ab")
	if len(left.sets) != 0 || len(right.sets) != 2 {
		t.Errorf("left %v, right %v", left.sets, right.sets)
	}
}

func TestPaletteModeEntryAndExit(t *testing.T) {
	h, rec := newFilterHarness(200, 30)
	h.send(key(":"))
	if h.m.mode != modePalette {
		t.Fatalf("mode = %v", h.m.mode)
	}
	if got := h.m.palette.text; got != "" {
		t.Errorf("text = %q, want empty", got)
	}
	h.typ("xyz")
	h.send(key("esc"))
	if h.m.mode != modeNormal || len(h.m.stacks[0]) != 1 {
		t.Errorf("esc: mode %v, stack %d", h.m.mode, len(h.m.stacks[0]))
	}
	h.send(key(":"))
	if got := h.m.palette.text; got != "" {
		t.Errorf("text = %q, want empty", got)
	}
	h.send(key("enter"))
	if h.m.mode != modeNormal || h.m.notice != "" || len(h.m.stacks[0]) != 1 {
		t.Errorf("empty input: mode %v, notice %q", h.m.mode, h.m.notice)
	}
	if len(rec.keys) != 0 {
		t.Errorf("keys reached the view: %v", rec.keys)
	}
}

func TestPaletteViewCommands(t *testing.T) {
	tests := []struct{ in, title string }{
		{"subs", "Subscriptions"},
		{"rg", "Resource groups"},
		{"vm", "compute/virtualmachines"},
		{"type microsoft.cache/redis", "cache/redis"},
	}
	for _, tt := range tests {
		h := newHarness(200, 30)
		h.send(key(":"))
		h.typ(tt.in)
		cmd := h.send(key("enter"))
		if h.m.mode != modeNormal {
			t.Errorf("%s: mode = %v", tt.in, h.m.mode)
		}
		stack := h.m.stacks[0]
		if len(stack) != 2 || stack[1].Title() != tt.title {
			t.Errorf("%s: stack titles = %v", tt.in, titles(stack))
		}
		fetches := 0
		for _, msg := range collect(cmd) {
			if _, ok := msg.(data.FetchMsg); ok {
				fetches++
			}
		}
		if fetches != 1 {
			t.Errorf("%s: Init asked for %d keys", tt.in, fetches)
		}
		h.send(key("esc"))
		if len(h.m.stacks[0]) != 1 {
			t.Errorf("%s: esc did not pop", tt.in)
		}
	}
}

func TestPaletteViewIsResized(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("rg")
	h.send(key("enter"))
	f := h.frame()
	if !strings.Contains(f, "Loading resource groups") || lipgloss.Height(f) != 30 {
		t.Errorf("frame:\n%s", f)
	}
}

func TestPaletteViewGoesToFocusedPaneAndFocusLeavesSidebar(t *testing.T) {
	h := newSidebarHarness(140, 30)
	h.prefix("s")
	h.prefix("h") // sidebar; the tab focus stays on pane 1
	if !h.m.sidebarFocused || h.m.tabs[0].focus == 0 {
		t.Fatalf("sidebar focused = %v, pane = %d", h.m.sidebarFocused, h.m.tabs[0].focus)
	}
	h.send(key(":"))
	h.typ("vm")
	h.send(key("enter"))
	if h.m.sidebarFocused {
		t.Error("focus stayed in the sidebar")
	}
	focus := h.m.tabs[0].focus
	if len(h.m.stacks[focus]) != 2 {
		t.Errorf("focused pane stack = %v", titles(h.m.stacks[focus]))
	}
	for id, s := range h.m.stacks {
		if id != focus && len(s) != 1 {
			t.Errorf("pane %d got the view", id)
		}
	}
}

func TestPaletteHelpAndQuit(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("help")
	h.send(key("enter"))
	if h.m.mode != modeHelp {
		t.Errorf(":help mode = %v", h.m.mode)
	}
	for _, in := range []string{"q", "quit"} {
		h := newHarness(100, 30)
		h.send(key(":"))
		h.typ(in)
		if !isQuit(h.send(key("enter"))) {
			t.Errorf(":%s does not quit", in)
		}
	}
}

func TestPaletteErrorShowsUntilNextKey(t *testing.T) {
	for in, msg := range map[string]string{
		"nope": "unknown command: nope",
		"type": "usage: type <resource type>",
	} {
		h := newHarness(200, 30)
		h.send(key(":"))
		h.typ(in)
		h.send(key("enter"))
		if h.m.mode != modeNormal {
			t.Errorf("%s: mode = %v", in, h.m.mode)
		}
		status := h.status()
		wantIn(t, "status", status, msg)
		if strings.Contains(status, "Fake1") {
			t.Errorf("%s: notice does not replace the path: %q", in, status)
		}
		red := strings.TrimSuffix(lipgloss.NewStyle().Foreground(lipgloss.Red).Render(msg), "\x1b[m")
		if lines := strings.Split(h.raw(), "\n"); !strings.Contains(lines[len(lines)-1], red) {
			t.Errorf("%s: notice is not red: %q", in, lines[len(lines)-1])
		}
		h.send(key("x"))
		if strings.Contains(h.status(), msg) || !strings.Contains(h.status(), "Fake1") {
			t.Errorf("%s: notice stayed after a key: %q", in, h.status())
		}
	}
}

func TestRenameFlow(t *testing.T) {
	h := newHarness(200, 30)
	h.prefix(",")
	if h.m.mode != modeRename {
		t.Fatalf("mode = %v", h.m.mode)
	}
	wantIn(t, "status", h.status(), "rename tab: ", "enter save  esc cancel")
	h.typ("  Prod ")
	wantIn(t, "status", h.status(), "rename tab:   Prod ")
	h.send(key("enter"))
	if h.m.mode != modeNormal || h.m.tabs[0].name != "Prod" {
		t.Errorf("after enter: mode %v, name %q", h.m.mode, h.m.tabs[0].name)
	}
	wantIn(t, "tab bar", strings.Split(h.frame(), "\n")[0], " 1 Prod ")

	h.prefix(",")
	wantIn(t, "status", h.status(), "rename tab: Prod")
	h.send(key("esc"))
	if h.m.mode != modeNormal || h.m.tabs[0].name != "Prod" {
		t.Errorf("after esc: mode %v, name %q", h.m.mode, h.m.tabs[0].name)
	}

	h.prefix(",")
	h.typ("x")
	h.send(key("esc"))
	if h.m.tabs[0].name != "Prod" {
		t.Errorf("esc saved the input: %q", h.m.tabs[0].name)
	}

	h.prefix(",")
	for range len("Prod") {
		h.send(key("backspace"))
	}
	h.send(key("enter"))
	if h.m.tabs[0].name != "" {
		t.Errorf("name = %q, want cleared", h.m.tabs[0].name)
	}
	wantIn(t, "tab bar", strings.Split(h.frame(), "\n")[0], " 1 Fake1 ")
}

func TestRenameAppliesToActiveTabOnly(t *testing.T) {
	h := newHarness(200, 30)
	h.prefix("t")
	h.prefix(",")
	h.typ("Two")
	h.send(key("enter"))
	bar := strings.Split(h.frame(), "\n")[0]
	wantIn(t, "tab bar", bar, " 1 Fake1 ", " 2 Two ")
	if h.m.tabs[0].name != "" {
		t.Errorf("first tab renamed: %q", h.m.tabs[0].name)
	}
}

func TestRenameKeysAreNotForwarded(t *testing.T) {
	h := newHarness(200, 30)
	h.prefix(",")
	h.typ("abc")
	h.send(key("enter"))
	if got := h.pane(0).keys; len(got) != 0 {
		t.Errorf("view got %v", got)
	}
}

func TestHelpOpensFromAllPlaces(t *testing.T) {
	opens := map[string]func(h *harness){
		"normal":         func(h *harness) { h.send(key("?")) },
		"prefix":         func(h *harness) { h.prefix("?") },
		"palette":        func(h *harness) { h.send(key(":")); h.typ("help"); h.send(key("enter")) },
		"sidebar":        func(h *harness) { h.prefix("h"); h.send(key("?")) },
		"sidebar prefix": func(h *harness) { h.prefix("h"); h.prefix("?") },
	}
	for name, open := range opens {
		h := newSidebarHarness(140, 42)
		open(h)
		if h.m.mode != modeHelp {
			t.Errorf("%s: mode = %v", name, h.m.mode)
			continue
		}
		wantIn(t, name+" frame", h.frame(), "┌ Help ", "Panes", "Layout (ctrl+w)", "Explorer", "Commands")
	}
}

func TestHelpClosesOnAnyKeyWithoutForwarding(t *testing.T) {
	for _, k := range []string{"x", "q", "esc", "enter", "?", ":", "/", "ctrl+w"} {
		h, rec := newFilterHarness(100, 30)
		h.send(key("?"))
		cmd := h.send(key(k))
		if h.m.mode != modeNormal || isQuit(cmd) {
			t.Errorf("%s: mode %v, quit %v", k, h.m.mode, isQuit(cmd))
		}
		if len(rec.keys) != 0 || len(h.m.stacks[0]) != 1 {
			t.Errorf("%s: forwarded: %v", k, rec.keys)
		}
		if strings.Contains(h.frame(), "┌ Help ") {
			t.Errorf("%s: overlay still drawn", k)
		}
	}
}

func TestCtrlCQuitsInEveryMode(t *testing.T) {
	enter := map[string][]string{
		"normal":  nil,
		"prefix":  {"ctrl+w"},
		"filter":  {"/"},
		"palette": {":"},
		"rename":  {"ctrl+w", ","},
		"help":    {"?"},
	}
	for name, keys := range enter {
		h, _ := newFilterHarness(100, 30)
		for _, k := range keys {
			h.send(key(k))
		}
		if !isQuit(h.send(key("ctrl+c"))) {
			t.Errorf("%s: ctrl+c does not quit", name)
		}
	}
}

func TestNewPrefixKeysWorkInSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	h.prefix(",")
	if h.m.mode != modeRename {
		t.Errorf(", mode = %v", h.m.mode)
	}
	h.send(key("esc"))
	h.prefix("?")
	if h.m.mode != modeHelp {
		t.Errorf("? mode = %v", h.m.mode)
	}
}

func TestQuestionAndColonInSidebar(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.prefix("h")
	h.send(key(":"))
	if h.m.mode != modePalette {
		t.Errorf(": mode = %v", h.m.mode)
	}
	h.send(key("esc"))
	h.send(key("?"))
	if h.m.mode != modeHelp {
		t.Errorf("? mode = %v", h.m.mode)
	}
}

func TestHelpBoxGeometry(t *testing.T) {
	lines := len(helpLines())
	tests := []struct {
		w, h, boxW, boxH int
	}{
		{140, 100, 76, lines + 2},
		{50, 100, 46, lines + 2},
		{140, 10, 76, 6},
	}
	for _, tt := range tests {
		h := newHarness(tt.w, tt.h)
		h.send(key("?"))
		frame := strings.Split(h.frame(), "\n")
		top := -1
		for i, l := range frame {
			if strings.Contains(l, "┌ Help ") {
				top = i
			}
		}
		if top < 0 {
			t.Errorf("%dx%d: no box", tt.w, tt.h)
			continue
		}
		l := []rune(frame[top])
		left := strings.Index(frame[top], "┌ Help ")
		left = len([]rune(frame[top][:left]))
		right := left
		for l[right] != '┐' {
			right++
		}
		if got := right - left + 1; got != tt.boxW {
			t.Errorf("%dx%d: box width = %d, want %d", tt.w, tt.h, got, tt.boxW)
		}
		if want := (tt.w - tt.boxW) / 2; left != want {
			t.Errorf("%dx%d: box x = %d, want %d", tt.w, tt.h, left, want)
		}
		if want := (tt.h - tt.boxH) / 2; top != want {
			t.Errorf("%dx%d: box y = %d, want %d", tt.w, tt.h, top, want)
		}
		bottom := top
		for !strings.Contains(frame[bottom], "└") || !strings.Contains(string([]rune(frame[bottom])[left:left+1]), "└") {
			bottom++
		}
		if got := bottom - top + 1; got != tt.boxH {
			t.Errorf("%dx%d: box height = %d, want %d", tt.w, tt.h, got, tt.boxH)
		}
	}
}

func TestFrameFitsWithOverlay(t *testing.T) {
	overlays := map[string][]string{"help": {"?"}, "dialog": {":", "s"}}
	for name, keys := range overlays {
		for _, s := range [][2]int{{140, 40}, {100, 30}, {30, 8}, {20, 6}, {14, 7}, {13, 5}, {5, 3}, {1, 1}, {80, 4}} {
			for _, sidebar := range []bool{false, true} {
				h := newSidebarHarness(s[0], s[1])
				if !sidebar {
					h.prefix("e")
				}
				for _, k := range keys {
					h.send(key(k))
				}
				f := h.frame()
				if got := lipgloss.Height(f); got != s[1] {
					t.Errorf("%s %v sidebar=%v: height = %d", name, s, sidebar, got)
				}
				if got := lipgloss.Width(f); got > s[0] {
					t.Errorf("%s %v sidebar=%v: width = %d", name, s, sidebar, got)
				}
			}
		}
	}
}

func TestInputStatusFitsNarrowWindow(t *testing.T) {
	for _, md := range inputModes {
		h, _ := newFilterHarness(20, 6)
		md.start(h)
		h.typ("a very long command line that does not fit")
		if got := lipgloss.Width(h.status()); got > 20 {
			t.Errorf("%s: status width = %d", md.name, got)
		}
	}
	h := newHarness(20, 6)
	h.send(key(":"))
	h.typ("a very long command line that does not fit")
	if got := lipgloss.Width(h.status()); got > 20 {
		t.Errorf("palette: status width = %d", got)
	}
}

func TestHintsText(t *testing.T) {
	want := map[string]string{
		normalHints:        "enter open  esc back  / filter  : command  r refresh  ? help  ^w v/s split  ^w hjkl/←↓↑→ focus  ^w t tab  q quit",
		sidebarHints:       "j/k move  l expand  h collapse  r refresh  enter open  ^w l panes  ^w e hide  : command  ? help  q quit",
		prefixHints:        "v/s split  hjkl/←↓↑→ focus  q close  z zoom  = equal  <>+- resize  t tab  n/p/1-9 switch  e explorer  , rename  ? help  esc cancel",
		sidebarPrefixHints: "l/→ panes  e hide  t tab  n/p/1-9 switch  , rename  ? help  esc cancel",
	}
	for got, w := range want {
		if got != w {
			t.Errorf("hints = %q, want %q", got, w)
		}
	}
}

// inputModes enters each input mode on a filter harness.
var inputModes = []struct {
	name, prompt string
	start        func(*harness)
}{
	{"filter", "/", func(h *harness) { h.send(key("/")) }},
	{"rename", renamePrompt, func(h *harness) { h.prefix(",") }},
}

func TestInputKeepsCursorInView(t *testing.T) {
	for _, md := range inputModes {
		for _, c := range []struct {
			lefts int
			want  string
		}{{0, "6789"}, {1, "6789"}, {31, "fghij"}} {
			h, _ := newFilterHarness(30, 10)
			md.start(h)
			h.typ("abcdefghijklmnopqrstuvwxyz0123456789")
			for range c.lefts {
				h.send(key("left"))
			}
			wantIn(t, fmt.Sprintf("%s after %d lefts: status", md.name, c.lefts), h.status(), c.want)
		}
	}
}

func TestInputUsesLineAfterPrompt(t *testing.T) {
	const text = "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, md := range inputModes {
		h, _ := newFilterHarness(30, 10)
		md.start(h)
		h.typ(text)
		tail := text[len(text)-(30-len(md.prompt)-1):] // one cell for the cursor
		wantIn(t, md.name+" status", h.status(), md.prompt+tail)
	}
}

func TestPasteGoesToInput(t *testing.T) {
	for _, md := range inputModes {
		h, rec := newFilterHarness(100, 30)
		md.start(h)
		h.send(tea.PasteMsg{Content: "web"})
		wantIn(t, md.name+" status", h.status(), md.prompt+"web")
		if md.name == "filter" && strings.Join(rec.sets, ",") != "web" {
			t.Errorf("SetFilter calls = %q, want web", rec.sets)
		}
	}
}

func TestHelpListsEveryKey(t *testing.T) {
	text := strings.Join(helpLines(), "\n")
	for _, want := range []string{"ctrl+c", "this help", "command"} {
		wantIn(t, "help", text, want)
	}
	for _, section := range []struct{ title, key string }{
		{"Panes", "?"}, {"Explorer", ":"}, {"Explorer", "?"}, {"Explorer", "↑ ↓"}, {"Explorer", "→ ←"},
	} {
		if !helpHasKey(section.title, section.key) {
			t.Errorf("%s lacks key %q", section.title, section.key)
		}
	}
}

func TestHelpDescribesTheCommandDialog(t *testing.T) {
	want := "command dialog (↑ ↓ choose, tab completes)"
	for _, s := range helpSections {
		if s.title != "Commands" {
			continue
		}
		for _, e := range s.entries {
			if e.keys == ":" && e.text == want {
				return
			}
		}
	}
	t.Errorf("the Commands section lacks %q for \":\"", want)
}

func helpHasKey(title, keys string) bool {
	for _, s := range helpSections {
		if s.title != title {
			continue
		}
		for _, e := range s.entries {
			if strings.Contains(" "+e.keys+" ", " "+keys+" ") {
				return true
			}
		}
	}
	return false
}

func TestCtrlVDoesNotReadClipboard(t *testing.T) {
	for _, md := range inputModes {
		h, _ := newFilterHarness(30, 10)
		md.start(h)
		h.typ("ab")
		if cmd := h.send(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}); cmd != nil {
			t.Errorf("%s: ctrl+v returned a command; its clipboard result would bypass the input", md.name)
		}
		if got := h.m.input.Value(); got != "ab" {
			t.Errorf("%s: input = %q, want ab", md.name, got)
		}
	}
}

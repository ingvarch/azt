package ui

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func ctrl(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl} }

// stackTitles are the titles of the focused pane's stack.
func (h *harness) stackTitles() []string { return titles(h.m.stacks[h.focused()]) }

func TestDialogOpensFromPaneAndSidebar(t *testing.T) {
	pane := newHarness(100, 30)
	sidebar := newSidebarHarness(140, 30)
	sidebar.prefix("h")
	for name, h := range map[string]*harness{"pane": pane, "sidebar": sidebar} {
		h.send(key(":"))
		if h.m.mode != modePalette {
			t.Errorf("%s: mode = %v", name, h.m.mode)
		}
		if p := h.m.palette; p.text != "" || p.at != -1 || len(p.matches) != len(commandTable) {
			t.Errorf("%s: palette = %+v", name, p)
		}
		wantIn(t, name+" frame", h.frame(), "┌ Command ")
	}
}

func TestDialogReopensEmpty(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("xyz")
	h.send(key("down"))
	h.send(key("esc"))
	h.send(key(":"))
	if p := h.m.palette; p.text != "" || p.at != -1 {
		t.Errorf("palette = %+v, want a fresh one", p)
	}
}

func TestDialogTypingNarrows(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("s")
	if got := strings.Join(h.m.palette.matches, " "); got != "subs sql storage" {
		t.Errorf("matches = %q", got)
	}
	h.typ("t")
	if h.m.palette.text != "st" || h.m.palette.choice() != "storage" {
		t.Errorf("palette = %+v", h.m.palette)
	}
}

func TestDialogArrowsMoveTheOffer(t *testing.T) {
	tests := []struct {
		name string
		keys []tea.KeyPressMsg
		want string
	}{
		{"down", []tea.KeyPressMsg{key("down")}, "subs"},
		{"ctrl+n", []tea.KeyPressMsg{ctrl('n')}, "subs"},
		{"up wraps to the last", []tea.KeyPressMsg{key("up")}, "q"},
		{"ctrl+p wraps to the last", []tea.KeyPressMsg{ctrl('p')}, "q"},
		{"down down", []tea.KeyPressMsg{key("down"), key("down")}, "rg"},
		{"down up", []tea.KeyPressMsg{ctrl('n'), ctrl('n'), ctrl('p')}, "subs"},
	}
	for _, tt := range tests {
		h := newHarness(100, 30)
		h.send(key(":"))
		for _, k := range tt.keys {
			h.send(k)
		}
		if got := h.m.palette.choice(); got != tt.want {
			t.Errorf("%s: offer = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDialogEnterRunsTheOfferedCommand(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("st")
	cmd := h.send(key("enter"))
	if h.m.mode != modeNormal {
		t.Errorf("mode = %v", h.m.mode)
	}
	if got := h.stackTitles(); len(got) != 2 || got[1] != "storage/storageaccounts" {
		t.Errorf("stack = %v", got)
	}
	if cmd == nil {
		t.Error("the new view was not started")
	}
}

func TestDialogEnterAfterArrowsRunsTheHighlightedCommand(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.send(key("down"))
	h.send(key("down"))
	h.send(key("enter"))
	if got := h.stackTitles(); len(got) != 2 || got[1] != "Resource groups" {
		t.Errorf("stack = %v", got)
	}
}

func TestDialogEnterOnEmptyLineOnlyCloses(t *testing.T) {
	h, rec := newFilterHarness(100, 30)
	h.send(key(":"))
	h.send(key("enter"))
	if h.m.mode != modeNormal || h.m.notice != "" || len(h.m.stacks[0]) != 1 || len(rec.keys) != 0 {
		t.Errorf("mode %v, notice %q, stack %v", h.m.mode, h.m.notice, h.stackTitles())
	}
}

func TestDialogEnterFromSidebarMovesFocus(t *testing.T) {
	h := newSidebarHarness(140, 30)
	h.prefix("s")
	h.prefix("h")
	h.send(key(":"))
	h.typ("vm")
	h.send(key("enter"))
	if h.m.sidebarFocused {
		t.Error("focus stayed in the sidebar")
	}
	if got := h.stackTitles(); len(got) != 2 {
		t.Errorf("focused stack = %v", got)
	}
}

func TestDialogHelpAndQuit(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("he")
	h.send(key("enter"))
	if h.m.mode != modeHelp {
		t.Errorf("mode = %v, want help", h.m.mode)
	}
	h = newHarness(100, 30)
	h.send(key(":"))
	h.typ("q")
	if !isQuit(h.send(key("enter"))) {
		t.Error("q does not quit")
	}
}

func TestDialogUnknownCommandShowsNotice(t *testing.T) {
	h := newHarness(200, 30)
	h.send(key(":"))
	h.typ("nope")
	h.send(key("enter"))
	if h.m.mode != modeNormal || h.m.notice != "unknown command: nope" {
		t.Errorf("mode %v, notice %q", h.m.mode, h.m.notice)
	}
	wantIn(t, "status", h.status(), "unknown command: nope")
}

func TestDialogTabCompletesAndKeepsOpen(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("ty")
	h.send(key("tab"))
	if h.m.mode != modePalette || h.m.palette.text != "type " {
		t.Fatalf("mode %v, text %q, want \"type \"", h.m.mode, h.m.palette.text)
	}
	h.send(key("tab"))
	if h.m.palette.text != "type " {
		t.Fatalf("a second tab changed the text to %q", h.m.palette.text)
	}
	h.typ("microsoft.web/sites")
	h.send(key("enter"))
	if got := h.stackTitles(); len(got) != 2 || got[1] != "web/sites" {
		t.Errorf("stack = %v", got)
	}
}

func TestDialogRightCompletesLikeTab(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("st")
	h.send(key("right"))
	if h.m.palette.text != "storage " {
		t.Errorf("text = %q", h.m.palette.text)
	}
}

func TestDialogTabKeepsAnArgumentAsTyped(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("ty microsoft.web/sites")
	h.send(key("tab"))
	if h.m.palette.text != "type microsoft.web/sites" {
		t.Errorf("text = %q", h.m.palette.text)
	}
}

func TestDialogKeepsKeysAndMouseFromTheSidebar(t *testing.T) {
	h := newSidebarHarness(140, 30)
	h.loadSubs()
	h.prefix("h")
	before := h.sidebarTitles()
	h.send(key(":"))
	// Each of these would move the selection off Dev if it reached the sidebar.
	for _, k := range []string{"j", "down", "j"} {
		h.send(key(k))
	}
	h.press(2, 3)
	h.wheel(2, 3, tea.MouseWheelDown)
	h.send(key("esc"))
	if got := h.sidebarTitles(); !slices.Equal(got, before) {
		t.Errorf("sidebar selection moved: %v, want %v", got, before)
	}
	if !h.m.sidebarFocused {
		t.Error("the sidebar lost focus")
	}
}

func TestDialogEditing(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("stx")
	h.send(key("backspace"))
	if h.m.palette.text != "st" || h.m.palette.choice() != "storage" {
		t.Errorf("after backspace: %+v", h.m.palette)
	}
	h.send(ctrl('h'))
	if h.m.palette.text != "s" || len(h.m.palette.matches) != 3 {
		t.Errorf("after ctrl+h: %+v", h.m.palette)
	}
	h.send(key("backspace"))
	h.send(key("backspace"))
	if h.m.palette.text != "" || h.m.palette.at != -1 {
		t.Errorf("backspace on empty text: %+v", h.m.palette)
	}
	h.typ("vm arg")
	h.send(ctrl('u'))
	if p := h.m.palette; p.text != "" || p.at != -1 || len(p.matches) != len(commandTable) {
		t.Errorf("after ctrl+u: %+v", p)
	}
	h.typ("é")
	h.send(key("backspace"))
	if h.m.palette.text != "" {
		t.Errorf("a letter is not erased whole: %q", h.m.palette.text)
	}
}

func TestDialogPaste(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.send(tea.PasteMsg{Content: "type\nmicrosoft.web/sites\tx"})
	if got := h.m.palette.text; got != "type microsoft.web/sites x" {
		t.Errorf("text = %q", got)
	}
	if h.m.palette.choice() != "type" {
		t.Errorf("paste did not narrow: %+v", h.m.palette)
	}
}

func TestDialogEscClosesWithoutRunning(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("vm")
	h.send(key("esc"))
	if h.m.mode != modeNormal || len(h.m.stacks[0]) != 1 {
		t.Errorf("mode %v, stack %v", h.m.mode, h.stackTitles())
	}
	if strings.Contains(h.frame(), "┌ Command ") {
		t.Error("dialog still drawn")
	}
}

func TestDialogKeepsKeysAndMouseFromThePanes(t *testing.T) {
	h := newSidebarHarness(140, 30)
	h.prefix("t")
	h.prefix("v")
	h.send(key(":"))
	for _, k := range []string{"j", "k", "up", "down", "left", "right", "ctrl+w", "x", "tab"} {
		h.send(key(k))
	}
	h.press(0, 0)
	h.press(60, 5)
	h.wheel(60, 5, tea.MouseWheelDown)
	h.motion(60, 5)
	h.release(60, 5)
	if h.m.mode != modePalette || h.m.active != 1 {
		t.Errorf("mode %v, active %d", h.m.mode, h.m.active)
	}
	for _, f := range h.fakes() {
		if len(f.keys) != 0 || len(f.clicks) != 0 {
			t.Errorf("view got keys %v clicks %v", f.keys, f.clicks)
		}
	}
	if h.m.sidebar.Title() == "" || h.m.sidebarFocused {
		t.Error("the sidebar changed")
	}
}

func TestDialogCtrlCQuits(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("vm")
	if !isQuit(h.send(key("ctrl+c"))) {
		t.Error("ctrl+c does not quit")
	}
}

func TestDialogClearsNoticeOnOpen(t *testing.T) {
	h := newHarness(200, 30)
	h.m.notice = "old"
	h.send(key(":"))
	if h.m.notice != "" {
		t.Errorf("notice = %q", h.m.notice)
	}
}

// boxAt finds the dialog box in the frame: its x, y, width and height.
func boxAt(t *testing.T, frame string) (x, y, w, hgt int) {
	t.Helper()
	lines := strings.Split(frame, "\n")
	y = -1
	for i, l := range lines {
		if at := strings.Index(l, "┌ Command "); at >= 0 {
			x, y = len([]rune(l[:at])), i
			break
		}
	}
	if y < 0 {
		t.Fatalf("no dialog in the frame:\n%s", frame)
	}
	top := []rune(lines[y])
	w = 1
	for top[x+w-1] != '┐' {
		w++
	}
	hgt = 1
	for !strings.HasPrefix(string([]rune(lines[y+hgt])[x:]), "└") {
		hgt++
	}
	return x, y, w, hgt + 1
}

func TestDialogBoxGeometry(t *testing.T) {
	tests := []struct {
		w, h int
		typ  string
		bw   int
		bh   int
	}{
		{100, 40, "", 64, 22},
		{100, 40, "st", 64, 5},
		{100, 40, "zzz", 64, 5},
		{50, 40, "", 46, 22},
		{100, 12, "", 64, 8},
		{100, 20, "", 64, 16},
	}
	for _, tt := range tests {
		h := newHarness(tt.w, tt.h)
		h.send(key(":"))
		h.typ(tt.typ)
		x, y, w, hgt := boxAt(t, h.frame())
		if w != tt.bw || hgt != tt.bh || x != (tt.w-tt.bw)/2 || y != min(tt.h/4, tt.h-1-tt.bh) {
			t.Errorf("%dx%d %q: box at %d,%d size %dx%d, want size %dx%d", tt.w, tt.h, tt.typ, x, y, w, hgt, tt.bw, tt.bh)
		}
	}
}

func dialogLines(t *testing.T, h *harness) []string {
	t.Helper()
	x, y, w, hgt := boxAt(t, h.frame())
	var out []string
	for _, l := range strings.Split(h.frame(), "\n")[y : y+hgt] {
		out = append(out, string([]rune(l)[x:x+w]))
	}
	return out
}

func TestDialogContent(t *testing.T) {
	h := newHarness(100, 40)
	h.send(key(":"))
	lines := dialogLines(t, h)
	wantIn(t, "input line", lines[1], "│:", " │")
	wantIn(t, "rule", lines[2], strings.Repeat("─", 62))
	wantIn(t, "first row", lines[3], "  subs     subscriptions")
	wantIn(t, "storage row", lines[11], "  storage  storage accounts in every subscription")
	wantIn(t, "managed identities row", lines[14], "  mi       managed identities in every subscription")
	wantIn(t, "registries row", lines[15], "  acr      container registries in every subscription")
	wantIn(t, "groups row", lines[16], "  nsg      network security groups in every subscription")
	wantIn(t, "dns row", lines[17], "  dns      public and private DNS zones in every subscription")
	wantIn(t, "last row", lines[20], "  q        quit")
	if strings.Contains(strings.Join(lines, ""), "pdns") {
		t.Error("the dialog offers pdns")
	}
	if strings.Contains(strings.Join(lines, ""), "▌") {
		t.Error("a row is offered before any key")
	}
	h.send(key("down"))
	h.send(key("down"))
	lines = dialogLines(t, h)
	wantIn(t, "offered row", lines[4], "▌ rg       resource groups in every subscription")
	if strings.Count(strings.Join(lines, ""), "▌") != 1 {
		t.Error("more than one row is marked")
	}
}

func TestDialogShowsTheTypedWordAndFaintTail(t *testing.T) {
	h := newHarness(100, 40)
	h.send(key(":"))
	h.typ("st")
	wantIn(t, "input line", dialogLines(t, h)[1], ":storage")
	faint := strings.TrimSuffix(hintStyle.Render("orage"), "\x1b[m")
	wantIn(t, "styled frame", h.raw(), faint)
	wantIn(t, "offered row", dialogLines(t, h)[3], "▌ storage")

	h.typ("x")
	wantIn(t, "no match", strings.Join(dialogLines(t, h), "\n"), ":stx", "no command starts with stx")
}

func TestDialogOfferedRowIsBold(t *testing.T) {
	h := newHarness(100, 40)
	h.send(key(":"))
	h.send(key("down"))
	bold := regexp.MustCompile("\x1b\\[(\\d+;)*1m▌")
	var row string
	for _, l := range strings.Split(h.raw(), "\n") {
		if strings.Contains(l, "▌") {
			row = l
		}
	}
	if !bold.MatchString(row) {
		t.Errorf("offered row is not bold: %q", row)
	}
}

func TestDialogNotDrawnOutsideThePaletteMode(t *testing.T) {
	h := newHarness(100, 40)
	if strings.Contains(h.frame(), "┌ Command ") {
		t.Error("dialog drawn in normal mode")
	}
	h.send(key("/"))
	h.send(key("esc"))
	h.send(key("?"))
	if strings.Contains(h.frame(), "┌ Command ") {
		t.Error("dialog drawn in help mode")
	}
}

func TestDialogRowsScrollToKeepTheOfferVisible(t *testing.T) {
	h := newHarness(100, 10) // box height 6: two rows fit
	h.send(key(":"))
	if got := strings.Join(dialogLines(t, h), "\n"); !strings.Contains(got, "subs") || strings.Contains(got, " q ") {
		t.Errorf("rows at the start:\n%s", got)
	}
	h.send(key("up"))
	got := strings.Join(dialogLines(t, h), "\n")
	wantIn(t, "dialog", got, "▌ q", "help")
	if strings.Contains(got, "subs") {
		t.Errorf("the window did not scroll:\n%s", got)
	}
	for range 17 {
		h.send(key("up"))
	}
	wantIn(t, "dialog", strings.Join(dialogLines(t, h), "\n"), "▌ subs")
}

func TestDialogNotDrawnInATinyWindowButKeysWork(t *testing.T) {
	for _, s := range [][2]int{{13, 30}, {100, 6}, {5, 3}} {
		h := newHarness(s[0], s[1])
		h.send(key(":"))
		h.typ("vm")
		if strings.Contains(h.frame(), "┌ C") {
			t.Errorf("%v: dialog drawn", s)
		}
		h.send(key("enter"))
		if got := h.stackTitles(); len(got) != 2 {
			t.Errorf("%v: stack = %v", s, got)
		}
	}
}

func TestDialogStatusBarShowsHints(t *testing.T) {
	h := newHarness(200, 40)
	h.send(key(":"))
	h.typ("st")
	status := h.status()
	wantIn(t, "status", status, "↑↓ choose  tab complete  enter run  esc close")
	if strings.Contains(status, ":st") {
		t.Errorf("status shows the input: %q", status)
	}
}

func TestDialogInNarrowWindowIsCutToTheBox(t *testing.T) {
	h := newHarness(24, 12)
	h.send(key(":"))
	h.typ(strings.Repeat("a", 40))
	lines := dialogLines(t, h)
	for _, l := range lines {
		if lipgloss.Width(l) != 20 {
			t.Errorf("line %q is %d cells wide, want 20", l, lipgloss.Width(l))
		}
	}
}

func TestDialogLeavesTheStatusBarAndTabBarVisible(t *testing.T) {
	for _, s := range [][2]int{{100, 20}, {100, 16}, {100, 17}, {100, 21}, {100, 12}} {
		h := newHarness(s[0], s[1])
		h.send(key(":"))
		_, y, _, hgt := boxAt(t, h.frame())
		if y+hgt > s[1]-1 || y < 1 {
			t.Errorf("%v: box rows %d..%d cover the tab bar or the status bar", s, y, y+hgt-1)
		}
		wantIn(t, "status", h.status(), paletteHints)
	}
}

func TestDialogIgnoresKeysWithoutText(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.typ("ab")
	if cmd := h.send(ctrl('v')); cmd != nil || h.m.palette.text != "ab" {
		t.Errorf("ctrl+v: cmd %v, text %q", cmd, h.m.palette.text)
	}
	h = newHarness(100, 30)
	h.send(key(":"))
	h.send(key("down"))
	cmd := h.send(ctrl('v'))
	h.send(key("left"))
	p := h.m.palette
	if cmd != nil || h.m.mode != modePalette || p.text != "" || p.choice() != "subs" {
		t.Errorf("cmd %v, mode %v, palette %+v", cmd, h.m.mode, p)
	}
}

func TestDialogPasteDropsControlCharacters(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key(":"))
	h.send(tea.PasteMsg{Content: "s\x1b[31mvm\x07"})
	if got := h.m.palette.text; got != "s[31mvm" {
		t.Errorf("text = %q", got)
	}
	if strings.Contains(h.raw(), "\x1b[31m") {
		t.Error("the escape reached the frame")
	}
	h.send(tea.PasteMsg{Content: "a\r\nb"})
	if got := h.m.palette.text; got != "s[31mvm"+"a b" {
		t.Errorf("text = %q", got)
	}
}

func TestDialogBoxSizeLimits(t *testing.T) {
	tests := []struct {
		w, h  int
		drawn bool
	}{
		{15, 30, false}, {16, 30, true}, {100, 8, false}, {100, 9, true},
	}
	for _, tt := range tests {
		h := newHarness(tt.w, tt.h)
		h.send(key(":"))
		if got := strings.Contains(h.frame(), "┌ C"); got != tt.drawn {
			t.Errorf("%dx%d: drawn = %v, want %v", tt.w, tt.h, got, tt.drawn)
		}
	}
}

func TestDialogStyles(t *testing.T) {
	h := newHarness(100, 40)
	h.send(key(":"))
	h.typ("st")
	if re := regexp.MustCompile("st\x1b\\[2morage\x1b\\[[\\d;]*7m \x1b\\[m"); !re.MatchString(h.raw()) {
		t.Error("faint tail and reverse cursor missing after the typed word")
	}
	h.typ("x")
	if re := regexp.MustCompile("\x1b\\[[\\d;]*2mno command starts with stx"); !re.MatchString(h.raw()) {
		t.Error("no-match row is not faint")
	}
}

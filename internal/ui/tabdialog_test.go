package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// dialogLayouts are windows where the tab dialog lands in different places.
// x, y and w are the box; save, clear and cancel are the first columns of the buttons; plus is the column of the plus.
var dialogLayouts = []struct {
	name                string
	build               func() *harness
	x, y, w             int
	save, clear, cancel int
	plus                int
}{
	{"explorer shown", func() *harness { return newSidebarHarness(100, 30) }, 22, 7, 56, 33, 43, 55, defaultSidebarWidth + 9},
	{"explorer hidden", func() *harness { return newHarness(100, 30) }, 22, 7, 56, 33, 43, 55, 9},
	{"odd width", func() *harness { return newHarness(41, 30) }, 2, 7, 37, 3, 13, 25, 9},
	{"short window", func() *harness { return newHarness(100, 9) }, 22, 2, 56, 33, 43, 55, 9},
}

const buttonsText = " ↵ save " + "  " + " ^c clear " + "  " + " esc cancel "

// cells is n cells of line from column x.
func cells(line string, x, n int) string {
	rs := []rune(line + strings.Repeat(" ", x+n))
	return string(rs[x : x+n])
}

// openNewTab presses the plus of the layout.
func (h *harness) openNewTab(plus int) tea.Cmd { return h.press(plus, 0) }

func (h *harness) frameLines() []string { return strings.Split(h.frame(), "\n") }

func TestPlusOpensTheNewTabDialog(t *testing.T) {
	for _, l := range dialogLayouts {
		t.Run(l.name, func(t *testing.T) {
			h := l.build()
			if cmd := h.openNewTab(l.plus); cmd != nil {
				t.Error("the plus returned a command; no tab opens yet")
			}
			if h.m.mode != modeTabDialog || len(h.m.tabs) != 1 || h.m.input.Value() != "" {
				t.Fatalf("mode %v, tabs %d, input %q", h.m.mode, len(h.m.tabs), h.m.input.Value())
			}
			inner := l.w - 2
			pad := (inner - 34) / 2
			lines := h.frameLines()
			want := []string{
				"┌ New tab " + strings.Repeat("─", inner-len([]rune(" New tab "))) + "┐",
				"│ 2" + strings.Repeat(" ", inner-2) + "│",
				"│" + strings.Repeat(" ", inner) + "│",
				"│" + strings.Repeat(" ", pad) + buttonsText + strings.Repeat(" ", inner-pad-34) + "│",
				"└" + strings.Repeat("─", inner) + "┘",
			}
			for i, w := range want {
				if got := cells(lines[l.y+i], l.x, l.w); got != w {
					t.Errorf("row %d = %q, want %q", i, got, w)
				}
			}
		})
	}
}

func TestPlaceholderIsTheNumberOfTheNewTab(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.prefix("t")
	h.prefix("2")
	h.press(plusX(0, 3), 0)
	if got := cells(h.frameLines()[8], 22, 5); got != "│ 4  " {
		t.Errorf("field = %q, want the placeholder 4", got)
	}
	h.typ("x")
	if got := cells(h.frameLines()[8], 22, 5); got != "│ x  " {
		t.Errorf("field = %q, want the typed x only", got)
	}
}

func TestNewTabSave(t *testing.T) {
	tests := []struct {
		name, typed, label string
	}{
		{"typed name", "prod", "  2 prod  "},
		{"padded name", "  prod ", "  2 prod  "},
		{"nothing typed", "", "   2    "},
		{"only spaces", "   ", "   2    "},
		{"the tab's own number", "2", "   2    "},
		{"its number with spaces", " 2 ", "   2    "},
		{"another number", "3", "  2 3   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(100, 30)
			h.openNewTab(9)
			h.typ(tt.typed)
			cmd := h.send(key("enter"))
			if h.m.mode != modeNormal || len(h.m.tabs) != 2 || h.m.active != 1 {
				t.Fatalf("mode %v, tabs %d, active %d", h.m.mode, len(h.m.tabs), h.m.active)
			}
			if cmd == nil || cmd() != "init-2" || h.recs[1].inits != 1 {
				t.Error("the new view's Init did not run")
			}
			if h.pane(h.focused()).rec.id != 2 {
				t.Error("the active tab does not hold the new view")
			}
			if got := h.m.tabLabel(1); got != tt.label {
				t.Errorf("label = %q, want %q", got, tt.label)
			}
		})
	}
}

func TestNewTabGoesToTheEnd(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.prefix("t")
	h.prefix("2")
	h.press(plusX(0, 3), 0)
	h.typ("last")
	h.send(key("enter"))
	if len(h.m.tabs) != 4 || h.m.active != 3 || h.m.tabs[3].name != "last" {
		t.Fatalf("tabs %d, active %d", len(h.m.tabs), h.m.active)
	}
	if h.m.tabs[1].name != "" || h.pane(h.focused()).rec.id != 4 {
		t.Error("the new tab is not a new view at the end")
	}
	if v := h.pane(h.focused()); v.w != 98 || v.h != 26 {
		t.Errorf("new view size = %dx%d, want 98x26", v.w, v.h)
	}
}

func TestNewTabCancelOpensNothing(t *testing.T) {
	for _, typed := range []string{"", "x"} {
		h := newHarness(100, 30)
		h.openNewTab(9)
		h.typ(typed)
		if cmd := h.send(key("esc")); cmd != nil {
			t.Errorf("%q: esc returned a command", typed)
		}
		if h.m.mode != modeNormal || len(h.m.tabs) != 1 || h.m.active != 0 || len(h.recs) != 1 {
			t.Errorf("%q: mode %v, tabs %d, views %d", typed, h.m.mode, len(h.m.tabs), len(h.recs))
		}
	}
}

func TestCtrlCClearsTheFieldAndStays(t *testing.T) {
	opens := map[string]func(h *harness){
		"new":    func(h *harness) { h.openNewTab(9) },
		"rename": func(h *harness) { h.prefix(",") },
	}
	for name, open := range opens {
		h := newHarness(100, 30)
		open(h)
		h.typ("ab")
		cmd := h.send(key("ctrl+c"))
		if cmd != nil {
			t.Errorf("%s: ctrl+c returned a command", name)
		}
		if h.m.mode != modeTabDialog || h.m.input.Value() != "" {
			t.Errorf("%s: mode %v, input %q", name, h.m.mode, h.m.input.Value())
		}
		h.typ("c")
		if h.m.input.Value() != "c" {
			t.Errorf("%s: input = %q after typing again", name, h.m.input.Value())
		}
	}
}

func TestRenameDialogShowsTheName(t *testing.T) {
	h := newHarness(100, 30)
	h.m.tabs[0].name = "Prod"
	h.prefix(",")
	if h.m.mode != modeTabDialog || h.m.input.Value() != "Prod" || h.m.input.Position() != 4 {
		t.Fatalf("mode %v, input %q at %d", h.m.mode, h.m.input.Value(), h.m.input.Position())
	}
	lines := h.frameLines()
	if got := cells(lines[7], 22, 14); got != "┌ Rename tab ─" {
		t.Errorf("top = %q", got)
	}
	if got := cells(lines[8], 22, 8); got != "│ Prod  " {
		t.Errorf("field = %q", got)
	}
}

func TestRenameDialogPlaceholderIsTheTabNumber(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix("t")
	h.prefix("t")
	h.prefix("2") // the second of three tabs: its number is neither the count nor the next
	h.prefix(",")
	if got := cells(h.frameLines()[8], 22, 5); got != "│ 2  " {
		t.Errorf("field = %q, want the placeholder 2", got)
	}
	if h.m.input.Value() != "" {
		t.Errorf("input = %q, want empty", h.m.input.Value())
	}
}

func TestRenameAfterTheNewTabDialog(t *testing.T) {
	h := newHarness(100, 30)
	h.press(plusX(0, 1), 0)
	h.send(key("esc"))
	h.m.tabs[0].name = "Prod"
	h.prefix(",")
	if top := h.frameLines()[7]; !strings.Contains(top, "┌ Rename tab ") {
		t.Errorf("top border = %q, want the Rename tab title", top)
	}
	h.typ("x")
	if cmd := h.send(key("enter")); cmd != nil {
		t.Error("enter returned a command; renaming opens no view")
	}
	if len(h.m.tabs) != 1 || h.m.tabs[0].name != "Prodx" {
		t.Errorf("tabs %d, name %q, want one tab named Prodx", len(h.m.tabs), h.m.tabs[0].name)
	}
}

func TestRenameSave(t *testing.T) {
	back := strings.Repeat("backspace ", 4)
	tests := []struct {
		name string
		keys string
		want string
	}{
		{"typed", back + "space D e v space enter", "Dev"},
		{"enter alone", "enter", "Prod"},
		{"emptied", back + "enter", ""},
		{"esc", "x esc", "Prod"},
		{"ctrl+c", "ctrl+c enter", ""},
		{"ctrl+c and typed", "ctrl+c Q enter", "Q"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(100, 30)
			h.prefix("t")
			h.m.tabs[0].name = "Keep"
			h.m.tabs[1].name = "Prod"
			h.prefix(",")
			for _, k := range strings.Fields(tt.keys) {
				if k == "space" {
					k = " "
				}
				h.send(key(k))
			}
			if h.m.mode != modeNormal || h.m.tabs[1].name != tt.want || h.m.tabs[0].name != "Keep" || len(h.m.tabs) != 2 {
				t.Errorf("mode %v, names %q %q, want the second %q", h.m.mode, h.m.tabs[0].name, h.m.tabs[1].name, tt.want)
			}
		})
	}
}

func TestPrefixTOpensATabWithoutTheDialog(t *testing.T) {
	h := newHarness(100, 30)
	cmd := h.prefix("t")
	if h.m.mode != modeNormal || len(h.m.tabs) != 2 || h.m.active != 1 || cmd == nil {
		t.Errorf("mode %v, tabs %d, active %d", h.m.mode, len(h.m.tabs), h.m.active)
	}
}

func TestDialogPasteAndNoClipboard(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix(",")
	h.send(tea.PasteMsg{Content: "web"})
	if h.m.input.Value() != "web" {
		t.Errorf("input = %q after the paste", h.m.input.Value())
	}
	if cmd := h.send(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}); cmd != nil || h.m.input.Value() != "web" {
		t.Errorf("ctrl+v: cmd %v, input %q", cmd != nil, h.m.input.Value())
	}
}

func TestDialogKeysAreNotForwarded(t *testing.T) {
	h, rec := newFilterHarness(100, 30)
	h.prefix(",")
	h.typ("abc")
	h.send(key("ctrl+c"))
	h.send(key("enter"))
	if len(rec.keys) != 0 {
		t.Errorf("view got %v", rec.keys)
	}
}

func TestDialogStatusBarShowsTheHints(t *testing.T) {
	for name, open := range map[string]func(h *harness){
		"new":    func(h *harness) { h.openNewTab(9) },
		"rename": func(h *harness) { h.prefix(",") },
	} {
		h := newHarness(100, 30)
		open(h)
		got := strings.TrimRight(h.status(), " ")
		if got != "enter save  ctrl+c clear  esc cancel" {
			t.Errorf("%s: status = %q", name, got)
		}
	}
}

func TestCtrlCQuitsOutsideTheDialog(t *testing.T) {
	h := newHarness(100, 30)
	h.prefix(",")
	h.send(key("esc"))
	if !isQuit(h.send(key("ctrl+c"))) {
		t.Error("ctrl+c does not quit after the dialog closed")
	}
}

func TestHelpSaysWhatCtrlCDoes(t *testing.T) {
	if !helpHasLine("Panes", "ctrl+c", "quit; clears the field in a tab dialog") {
		t.Errorf("help lacks the ctrl+c line:\n%s", strings.Join(helpLines(), "\n"))
	}
}

func helpHasLine(title, keys, text string) bool {
	for _, s := range helpSections {
		for _, e := range s.entries {
			if s.title == title && e.keys == keys && e.text == text {
				return true
			}
		}
	}
	return false
}

// dialogOpener opens a dialog on a harness; name is what the first tab is called after the dialog is cancelled.
type dialogOpener struct {
	open func(h *harness, plus int)
	name string
}

// dialogOpeners open each dialog; the rename one names the tab "Prod" first.
var dialogOpeners = map[string]dialogOpener{
	"new":    {func(h *harness, plus int) { h.openNewTab(plus) }, ""},
	"rename": {func(h *harness, _ int) { h.m.tabs[0].name = "Prod"; h.prefix(",") }, "Prod"},
}

func TestPressOnTheButtons(t *testing.T) {
	for _, l := range dialogLayouts {
		for name, o := range dialogOpeners {
			for dx, edge := range []string{"first", "last"} {
				t.Run(l.name+" "+name+" "+edge, func(t *testing.T) {
					press := func(start, w int) (*harness, tea.Cmd) {
						h := l.build()
						o.open(h, l.plus)
						h.typ("x")
						return h, h.press(start+dx*(w-1), l.y+3)
					}
					h, cmd := press(l.save, 8)
					checkSavePress(t, h, cmd, name)
					h, cmd = press(l.clear, 10)
					if cmd != nil || h.m.mode != modeTabDialog || h.m.input.Value() != "" {
						t.Errorf("clear: mode %v, input %q", h.m.mode, h.m.input.Value())
					}
					h, cmd = press(l.cancel, 12)
					want := o.name
					if cmd != nil || h.m.mode != modeNormal || len(h.m.tabs) != 1 || h.m.tabs[0].name != want {
						t.Errorf("cancel: mode %v, tabs %d, name %q", h.m.mode, len(h.m.tabs), h.m.tabs[0].name)
					}
				})
			}
		}
	}
}

// checkSavePress checks the state after a press on save with "x" typed.
func checkSavePress(t *testing.T, h *harness, cmd tea.Cmd, dialog string) {
	t.Helper()
	if h.m.mode != modeNormal {
		t.Fatalf("save: mode %v", h.m.mode)
	}
	if dialog == "new" && (len(h.m.tabs) != 2 || h.m.tabs[1].name != "x" || cmd == nil || cmd() != "init-2") {
		t.Errorf("save: tabs %d, cmd %v", len(h.m.tabs), cmd != nil)
	}
	if dialog == "rename" && (len(h.m.tabs) != 1 || h.m.tabs[0].name != "Prodx") {
		t.Errorf("save: name %q, want Prodx", h.m.tabs[0].name)
	}
}

func TestPressElsewhereInTheDialogDoesNothing(t *testing.T) {
	for _, l := range dialogLayouts {
		for name, o := range dialogOpeners {
			h := l.build()
			o.open(h, l.plus)
			h.typ("x")
			before := h.m.input.Value()
			spots := map[string][2]int{
				"field":         {l.x + 3, l.y + 1},
				"blank row":     {l.save, l.y + 2},
				"before save":   {l.save - 1, l.y + 3},
				"gap":           {l.clear - 1, l.y + 3},
				"after cancel":  {l.cancel + 12, l.y + 3},
				"left border":   {l.x, l.y + 3},
				"right border":  {l.x + l.w - 1, l.y + 3},
				"top border":    {l.x + 3, l.y},
				"bottom border": {l.x + 3, l.y + 4},
				"above":         {l.save, l.y - 1},
				"below":         {l.save, l.y + 5},
				"left":          {l.x - 1, l.y + 3},
				"right":         {l.x + l.w, l.y + 3},
				"corner":        {0, 0},
				"plus":          {l.plus, 0},
				"first tab":     {l.plus - 9, 0},
			}
			for where, p := range spots {
				if cmd := h.press(p[0], p[1]); cmd != nil {
					t.Errorf("%s %s %s: press returned a command", l.name, name, where)
				}
				if h.m.mode != modeTabDialog || h.m.input.Value() != before || len(h.m.tabs) != 1 || h.m.tabs[0].name != o.name {
					t.Errorf("%s %s %s: mode %v, input %q, tabs %d", l.name, name, where, h.m.mode, h.m.input.Value(), len(h.m.tabs))
				}
			}
		}
	}
}

func TestOtherMouseInTheDialogDoesNothing(t *testing.T) {
	h := newHarness(100, 30)
	h.openNewTab(9)
	h.typ("x")
	h.send(tea.MouseClickMsg{X: 33, Y: 10, Button: tea.MouseRight})
	h.send(tea.MouseReleaseMsg{X: 33, Y: 10, Button: tea.MouseLeft})
	h.send(tea.MouseMotionMsg{X: 33, Y: 10, Button: tea.MouseLeft})
	h.wheel(33, 10, tea.MouseWheelDown)
	if h.m.mode != modeTabDialog || h.m.input.Value() != "x" || len(h.m.tabs) != 1 {
		t.Errorf("mode %v, input %q, tabs %d", h.m.mode, h.m.input.Value(), len(h.m.tabs))
	}
}

func TestDialogShades(t *testing.T) {
	for _, l := range dialogLayouts {
		t.Run(l.name, func(t *testing.T) {
			h := l.build()
			h.openNewTab(l.plus)
			h.background("#1e1e2e")
			checkShadedField(t, strings.Split(h.raw(), "\n")[l.y+1], l.x, l.w, 1) // the placeholder "2"
			h.typ("ab")
			lines := strings.Split(h.raw(), "\n")
			checkShadedField(t, lines[l.y+1], l.x, l.w, 2)
			checkShadedButtons(t, lines[l.y+3], l.save, l.clear, l.cancel)
		})
	}
}

// checkShadedField checks the cells of the field line of the box at x with typed characters in the field.
func checkShadedField(t *testing.T, line string, x, w, typed int) {
	t.Helper()
	cursor := x + 2 + typed // after the border, the space and the text
	if c := cellAt(t, line, cursor); !c.flags["7"] {
		t.Errorf("cursor x=%d: %+v, want reverse", cursor, c)
	}
	for i := x + 1; i < x+w-1; i++ {
		if got := cellAt(t, line, i).bg; i != cursor && got != rgbBg(60, 60, 76) {
			t.Errorf("field x=%d: background %q, want the tab shade", i, got)
		}
	}
}

// checkShadedButtons checks the button cells of a line, given the first column of each button.
func checkShadedButtons(t *testing.T, line string, save, wipe, cancel int) {
	t.Helper()
	bar, tab := rgbBg(42, 42, 58), rgbBg(60, 60, 76)
	for _, x := range []int{save, save + 3, save + 7} {
		if c := cellAt(t, line, x); c.bg != bar || c.fg != "94" || !c.flags["7"] || !c.flags["1"] {
			t.Errorf("save x=%d: %+v, want bold, reverse, 94 on the bar shade", x, c)
		}
	}
	for _, x := range []int{wipe, wipe + 9, cancel, cancel + 11} {
		if c := cellAt(t, line, x); c.bg != tab || !c.flags["1"] || c.flags["7"] || c.flags["2"] {
			t.Errorf("x=%d: %+v, want bold on the tab shade", x, c)
		}
	}
	for _, x := range []int{save + 8, save + 9, wipe + 10} {
		if c := cellAt(t, line, x); c.bg != "" {
			t.Errorf("gap x=%d: background %q, want none", x, c.bg)
		}
	}
}

func TestPlaceholderIsFaint(t *testing.T) {
	for name, open := range map[string]func(h *harness){
		"new":    func(h *harness) { h.openNewTab(9) },
		"rename": func(h *harness) { h.prefix("t"); h.prefix(",") },
	} {
		h := newHarness(100, 30)
		open(h)
		line := strings.Split(h.raw(), "\n")[8]
		if c := cellAt(t, line, 24); !c.flags["2"] || c.flags["7"] || c.bg != "" {
			t.Errorf("%s unshaded: number %+v, want faint and no background", name, c)
		}
		if c := cellAt(t, line, 25); !c.flags["7"] {
			t.Errorf("%s: cell after the number %+v, want the cursor", name, c)
		}
		h.background("#1e1e2e")
		line = strings.Split(h.raw(), "\n")[8]
		if c := cellAt(t, line, 24); !c.flags["2"] || c.bg != rgbBg(60, 60, 76) {
			t.Errorf("%s shaded: number %+v, want faint on the tab shade", name, c)
		}
	}
}

func TestDialogBorderIsFocusedAndTitleBold(t *testing.T) {
	h := newHarness(100, 30)
	h.openNewTab(9)
	line := strings.Split(h.raw(), "\n")[7]
	if c := cellAt(t, line, 22); c.fg != "94" {
		t.Errorf("corner %+v, want the focus color", c)
	}
	if c := cellAt(t, line, 24); c.fg != "94" || !c.flags["1"] {
		t.Errorf("title %+v, want bold in the focus color", c)
	}
}

func TestDialogHasNoShadeBeforeTheBackgroundArrives(t *testing.T) {
	for _, l := range dialogLayouts {
		for name, o := range dialogOpeners {
			h := l.build()
			o.open(h, l.plus)
			lines := strings.Split(h.raw(), "\n")
			for i := l.y; i < l.y+5; i++ {
				if strings.Contains(lines[i], "48;") {
					t.Errorf("%s %s: background code in row %d: %q", l.name, name, i, lines[i])
				}
			}
			buttons := lines[l.y+3]
			if c := cellAt(t, buttons, l.save+1); c.fg != "94" || !c.flags["7"] || !c.flags["1"] {
				t.Errorf("%s %s: save %+v, want bold, reverse, 94", l.name, name, c)
			}
			if c := cellAt(t, buttons, l.clear+1); !c.flags["1"] || c.flags["7"] {
				t.Errorf("%s %s: clear %+v, want bold", l.name, name, c)
			}
		}
	}
}

func TestDialogIsNotDrawnInASmallWindow(t *testing.T) {
	for _, s := range [][2]int{{39, 30}, {100, 8}, {20, 6}, {5, 3}, {1, 1}} {
		for _, title := range []string{"New tab", "Rename tab"} {
			h := newHarness(s[0], s[1])
			if title == "New tab" {
				h.openNewTab(9)
			} else {
				h.prefix(",")
			}
			f := h.frame()
			if strings.Contains(f, "┌ "+title) || strings.Contains(f, " ↵ save ") {
				t.Errorf("%v %s: the dialog is drawn", s, title)
			}
			if lipgloss.Height(f) != s[1] || lipgloss.Width(f) > s[0] {
				t.Errorf("%v %s: frame is %dx%d", s, title, lipgloss.Width(f), lipgloss.Height(f))
			}
		}
	}
	for _, s := range [][2]int{{40, 30}, {100, 9}} {
		h := newHarness(s[0], s[1])
		h.prefix(",")
		if f := h.frame(); !strings.Contains(f, "┌ Rename tab ") || !strings.Contains(f, " esc cancel ") {
			t.Errorf("%v: the dialog is not drawn", s)
		}
	}
}

func TestDialogFitsTheWindowWithLongText(t *testing.T) {
	for _, s := range [][2]int{{40, 9}, {100, 30}, {41, 12}} {
		h := newHarness(s[0], s[1])
		h.prefix(",")
		h.typ(strings.Repeat("abcdefghij", 12))
		f := h.frame()
		if lipgloss.Height(f) != s[1] || lipgloss.Width(f) > s[0] {
			t.Errorf("%v: frame is %dx%d", s, lipgloss.Width(f), lipgloss.Height(f))
		}
		lines := strings.Split(f, "\n")
		y := min(s[1]/4, s[1]-6)
		if !strings.Contains(lines[y+1], "j │") {
			t.Errorf("%v: the end of the text is not in view: %q", s, lines[y+1])
		}
	}
}

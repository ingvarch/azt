package ui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/layout"
)

const (
	paletteMaxWidth  = 64
	paletteMinInner  = 10 // narrowest inner width a box is drawn at
	paletteMinHeight = 3  // fewest inner rows a box is drawn with
	paletteChrome    = 4  // border, input line and rule around the rows
)

var pasteToSpaces = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// paletteModel is the command dialog: what was typed, the commands that fit
// it, and the one it offers (at is its index in matches, -1 for none).
type paletteModel struct {
	text    string
	matches []string
	at      int
}

// newPalette opens the dialog with every command listed and none offered, so
// the key that opened it cannot choose one.
func newPalette() paletteModel {
	return paletteModel{matches: matchingCommands(""), at: -1}
}

// narrow keeps the commands that fit the text. A typed whole name is offered;
// otherwise the command offered before stays offered while it fits, or the
// first one is.
func (p paletteModel) narrow() paletteModel {
	chosen := p.choice()
	p.matches = matchingCommands(p.text)
	word, _ := firstWord(p.text)
	if word == "" || len(p.matches) == 0 {
		p.at = -1
		return p
	}
	if whole := slices.Index(p.matches, strings.ToLower(word)); whole >= 0 {
		p.at = whole
		return p
	}
	p.at = max(slices.Index(p.matches, chosen), 0)
	return p
}

// walk moves the offer through the matches and wraps at both ends. From no
// offer a step down goes to the first match and a step up to the last.
func (p paletteModel) walk(by int) paletteModel {
	n := len(p.matches)
	if n == 0 {
		p.at = -1
		return p
	}
	if p.at < 0 {
		p.at = n - 1
		if by > 0 {
			p.at = 0
		}
		return p
	}
	p.at = (p.at + by + n) % n
	return p
}

// choice is the offered command, empty when none is offered or it does not
// start with the typed word.
func (p paletteModel) choice() string {
	if p.at < 0 || p.at >= len(p.matches) {
		return ""
	}
	word, _ := firstWord(p.text)
	if offered := p.matches[p.at]; strings.HasPrefix(offered, strings.ToLower(word)) {
		return offered
	}
	return ""
}

// split is the typed first word, the rest of the offered command that was not
// typed yet, and the text after the first word.
func (p paletteModel) split() (word, tail, rest string) {
	word, rest = firstWord(p.text)
	if offered := p.choice(); offered != "" {
		tail = offered[len([]rune(word)):]
	}
	return word, tail, rest
}

// line is the text the dialog runs, with the offered command written in.
func (p paletteModel) line() string {
	word, tail, rest := p.split()
	return word + tail + rest
}

// edit applies a key that changes the offer or the text.
func (p paletteModel) edit(msg tea.KeyPressMsg) paletteModel {
	switch msg.String() {
	case "up", "ctrl+p":
		return p.walk(-1)
	case "down", "ctrl+n":
		return p.walk(1)
	case "tab", "right":
		p = p.complete()
	case "backspace", "ctrl+h":
		_, size := utf8.DecodeLastRuneInString(p.text)
		p.text = p.text[:len(p.text)-size]
	case "ctrl+u":
		p.text = ""
	default:
		if msg.Text == "" {
			return p
		}
		p.text += msg.Text
	}
	return p.narrow()
}

// complete writes the offered command into the text, with a space after it
// when nothing follows, so an argument can be typed next.
func (p paletteModel) complete() paletteModel {
	_, _, rest := p.split()
	offered := p.choice() != ""
	p.text = p.line()
	if offered && rest == "" {
		p.text += " "
	}
	return p
}

// paste appends pasted text as one line.
func (p paletteModel) paste(content string) paletteModel {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, pasteToSpaces.Replace(content))
	p.text += clean
	return p.narrow()
}

// paletteKey handles every key while the dialog is open: esc closes it, enter
// runs the line, the rest edits it.
func (m Model) paletteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "enter":
		m.mode = modeNormal
		cmd := m.runCommand(m.palette.line())
		return m, cmd
	}
	m.palette = m.palette.edit(msg)
	return m, nil
}

// paletteRect is the dialog box, a quarter down the window above the status
// bar; false when the window is too small for one.
func (m Model) paletteRect() (layout.Rect, bool) {
	w := min(m.width-4, paletteMaxWidth)
	h := min(m.height-4, max(len(m.palette.matches), 1)+paletteChrome)
	if w-2 < paletteMinInner || h-2 < paletteMinHeight {
		return layout.Rect{}, false
	}
	return layout.Rect{X: (m.width - w) / 2, Y: min(m.height/4, m.height-1-h), W: w, H: h}, true
}

// paletteBox draws the dialog in r.
func (m Model) paletteBox(r layout.Rect) string {
	return renderBox([]string{"Command"}, m.palette.body(r.W-2, r.H-2), r, true)
}

// body is the input line, a rule and as many rows as fit in height, scrolled
// so the offered one shows.
func (p paletteModel) body(width, height int) string {
	rows := p.rows()
	visible := height - 2
	start := max(p.at-visible+1, 0)
	rows = rows[start:min(start+visible, len(rows))]
	head := []string{p.inputLine(), strings.Repeat("─", width)}
	return strings.Join(append(head, rows...), "\n")
}

// inputLine is ":", the typed word, the faint rest of the offered command, the
// text after the word and the cursor.
func (p paletteModel) inputLine() string {
	word, tail, rest := p.split()
	return ":" + word + hintStyle.Render(tail) + rest + reverseStyle.Render(" ")
}

// rows is one line per match, the offered one marked and bold.
func (p paletteModel) rows() []string {
	if len(p.matches) == 0 {
		word, _ := firstWord(p.text)
		return []string{hintStyle.Render("no command starts with " + word)}
	}
	width := 0
	for _, name := range p.matches {
		width = max(width, len(name))
	}
	rows := make([]string, len(p.matches))
	for i, name := range p.matches {
		mark := "  "
		if i == p.at {
			mark = "▌ "
		}
		rows[i] = mark + fmt.Sprintf("%-*s", width, name) + "  " + describe(name)
		if i == p.at {
			rows[i] = boldStyle.Render(rows[i])
		}
	}
	return rows
}

// describe is the description of the command called name.
func describe(name string) string {
	for _, c := range commandTable {
		if c.name == name {
			return c.desc
		}
	}
	return ""
}

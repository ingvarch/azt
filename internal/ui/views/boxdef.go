package views

import (
	"image/color"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// format turns the value found at a field's path into text.
type format func(v any) string

func fmtText(v any) string { return orDash(text(v)) }

func fmtState(v any) string { return stateText(fmtText(v)) }

func fmtOnOff(v any) string { return switchText(onOff(v)) }

// fmtCount is the number of elements of a list or keys of a map, "0" when missing.
func fmtCount(v any) string {
	switch v := v.(type) {
	case []any:
		return strconv.Itoa(len(v))
	case map[string]any:
		return strconv.Itoa(len(v))
	}
	return "0"
}

func fmtLast(v any) string { return orDash(lastSegment(text(v))) }

func fmtJoin(v any) string { return joined(v) }

// fieldDef is a labeled line of a box: the value at path in the row shown in
// format (text when nil), or whatever value returns for what a path cannot say.
type fieldDef struct {
	label  string
	path   []string
	format format
	value  func(row map[string]any) string
}

func (f fieldDef) text(row map[string]any) string {
	if f.value != nil {
		return f.value(row)
	}
	if f.format == nil {
		return fmtText(dig(row, f.path...))
	}
	return f.format(dig(row, f.path...))
}

// boxDef is a titled box of fields. extra adds pairs the fields cannot list,
// drawn before them. split puts the fields in two columns, the first
// half on the left, when the box is wide enough.
type boxDef struct {
	title  string
	fields []fieldDef
	extra  func(row map[string]any) []pair
	split  bool
}

// lines draws the fields in a box width cells wide inside.
func (b boxDef) lines(row map[string]any, width int) []string {
	var pairs []pair
	if b.extra != nil {
		pairs = b.extra(row)
	}
	for _, f := range b.fields {
		pairs = append(pairs, pair{f.label, f.text(row)})
	}
	if !b.split {
		return block(pairs)
	}
	half := (len(pairs) + 1) / 2
	return columns(pairs[:half], pairs[half:], width)
}

func (b boxDef) asSection(row map[string]any, width int) section {
	return section{title: b.title, lines: b.lines(row, innerWidth(width))}
}

// boxesTab draws each box as a section.
func boxesTab(boxes ...boxDef) func(row map[string]any, width int) []string {
	return func(row map[string]any, width int) []string {
		secs := make([]section, len(boxes))
		for i, b := range boxes {
			secs[i] = b.asSection(row, width)
		}
		return sections(width, secs...)
	}
}

// withTail ends an Overview body with the Attention box, when attn returns
// any alert, and the Tags box.
func withTail(body func(row map[string]any, width int) []string, attn func(row map[string]any) []alert) func(row map[string]any, width int) []string {
	return func(row map[string]any, width int) []string {
		var tail []section
		if attn != nil {
			if alerts := attn(row); len(alerts) > 0 {
				tail = append(tail, alertSection(alerts))
			}
		}
		tail = append(tail, section{title: "Tags", lines: tagLines(row)})
		return append(append(body(row, width), ""), sections(width, tail...)...)
	}
}

const (
	// twoColumnWidth is the narrowest box inside that shows the essentials in two columns.
	twoColumnWidth = 90
	// columnGap is the number of cells between the two columns.
	columnGap = 4
)

// columns puts the left and the right pairs in two columns when the box is
// wide enough and no line has to be cut, else all of them in one column: an
// ID or an address cut short is of no use.
func columns(left, right []pair, width int) []string {
	l, r := block(left), block(right)
	leftWidth, rightWidth := columnWidths(width)
	if width < twoColumnWidth || !fits(l, leftWidth) || !fits(r, rightWidth) {
		return block(slices.Concat(left, right))
	}
	out := make([]string, max(len(l), len(r)))
	for i := range out {
		out[i] = fitCell(cellAt(l, i), leftWidth) + strings.Repeat(" ", columnGap) + cellAt(r, i)
	}
	return out
}

// columnWidths splits width into two columns and the gap between them.
func columnWidths(width int) (left, right int) {
	left = (width - columnGap) / 2
	return left, width - columnGap - left
}

// fits reports whether every line is at most width cells wide.
func fits(lines []string, width int) bool {
	return !slices.ContainsFunc(lines, func(l string) bool { return lipgloss.Width(l) > width })
}

// alert is an item of the Attention box: an error, or else a warning.
type alert struct {
	text   string
	severe bool
}

func (a alert) tone() color.Color {
	if a.severe {
		return bad
	}
	return warn
}

// line is the item with its symbol: "✗" for an error, "▲" for a warning.
func (a alert) line() string {
	if a.severe {
		return "✗ " + a.text
	}
	return "▲ " + a.text
}

func (a alert) render() string { return lipgloss.NewStyle().Foreground(a.tone()).Render(a.line()) }

// alertSection is the Attention box; it is bad when an item is an error, else warn.
func alertSection(alerts []alert) section {
	lines := make([]string, len(alerts))
	tone := warn
	for i, a := range alerts {
		lines[i] = a.render()
		if a.severe {
			tone = bad
		}
	}
	return section{title: "Attention", lines: lines, tone: tone}
}

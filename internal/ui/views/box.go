package views

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// boxMinWidth is the narrowest body that draws sections in boxes.
const boxMinWidth = 20

// boxFrame is the cells a box takes from its lines: a border and a space on each side.
const boxFrame = 4

// innerWidth is the width of the lines inside a section drawn at width.
func innerWidth(width int) int {
	if width < boxMinWidth {
		return width
	}
	return width - boxFrame
}

// section is a titled group of lines; the overview tabs draw it as a box.
type section struct {
	title string
	lines []string
	// tone colors the border and the title; nil keeps muted and accent.
	tone color.Color
}

// render draws the section width cells wide; a body narrower than boxMinWidth
// gets the bold title and the lines.
func (s section) render(width int) []string {
	if width < boxMinWidth {
		return append([]string{boldStyle.Render(s.title)}, s.lines...)
	}
	border, title := mutedStyle, titleStyle
	if s.tone != nil {
		border = lipgloss.NewStyle().Foreground(s.tone)
		title = title.Foreground(s.tone)
	}
	box := lipgloss.RoundedBorder()
	inner := width - boxFrame
	name := ellipsize(s.title, inner-1)
	top := border.Render(box.TopLeft+" ") + title.Render(name) +
		border.Render(" "+strings.Repeat(box.Top, inner-lipgloss.Width(name))+box.TopRight)
	out := []string{top}
	for _, l := range s.lines {
		out = append(out, border.Render(box.Left)+" "+fitCell(l, inner)+" "+border.Render(box.Right))
	}
	return append(out, border.Render(box.BottomLeft+strings.Repeat(box.Bottom, width-2)+box.BottomRight))
}

// sections draws the sections one under the other, a blank line between.
func sections(width int, secs ...section) []string {
	var out []string
	for i, s := range secs {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, s.render(width)...)
	}
	return out
}

// ellipsize cuts s to width cells and ends it with "…" when it is longer.
func ellipsize(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return cut(s, width-1) + "…"
}

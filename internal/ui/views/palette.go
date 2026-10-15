package views

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// The ANSI colors of the views, so they follow the terminal theme.
var (
	accent  = lipgloss.Color("12")
	muted   = lipgloss.Color("8")
	good    = lipgloss.Color("10")
	warn    = lipgloss.Color("11")
	bad     = lipgloss.Color("9")
	boolean = lipgloss.Color("13")
)

var (
	accentStyle = lipgloss.NewStyle().Foreground(accent)
	mutedStyle  = lipgloss.NewStyle().Foreground(muted)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	goodStyle   = lipgloss.NewStyle().Foreground(good)
	warnStyle   = lipgloss.NewStyle().Foreground(warn)
)

// stateSeparator joins the parts of a combined status such as "Running · Succeeded".
const stateSeparator = " · "

// stateTone is the color of a state word: good, bad, or warn for any other.
func stateTone(word string) color.Color {
	switch strings.ToLower(word) {
	case "running", "succeeded", "enabled", "available", "connected", "fullyinsync":
		return good
	case "stopped", "failed", "canceled", "disabled", "deleted", "unavailable", "disconnected":
		return bad
	}
	return warn
}

// stateText draws a status as one dot in the tone of its most severe part,
// then each part in its own tone; an empty status and a dash stay as they are.
func stateText(s string) string {
	parts := strings.Split(s, stateSeparator)
	worst, found := 0, false
	for i, p := range parts {
		if p == "" || p == dash {
			continue
		}
		tone := stateTone(p)
		worst, found = max(worst, severity(tone)), true
		parts[i] = lipgloss.NewStyle().Foreground(tone).Render(p)
	}
	if !found {
		return s
	}
	dot := lipgloss.NewStyle().Foreground([]color.Color{good, warn, bad}[worst]).Render("●")
	return dot + " " + strings.Join(parts, stateSeparator)
}

// severity ranks a state tone: good 0, warn 1, bad 2.
func severity(tone color.Color) int {
	switch tone {
	case bad:
		return 2
	case warn:
		return 1
	}
	return 0
}

// switchText draws "on" in good and "off" in muted; any other text stays as it is.
func switchText(s string) string {
	switch s {
	case "on":
		return goodStyle.Render(s)
	case "off":
		return mutedStyle.Render(s)
	}
	return s
}

// withDays is state drawn with the retention in days after "on"; days may be empty.
func withDays(state, days string) string {
	if state != "on" || days == "" {
		return switchText(state)
	}
	return join(stateSeparator, switchText(state), days+" days")
}

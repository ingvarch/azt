package views

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/ui/data"
)

var (
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red)
	faintStyle = lipgloss.NewStyle().Faint(true)
)

const refreshingText = "refreshing…"

// loader tracks the load state of the data behind one view.
type loader struct {
	key     data.Key
	noun    string
	loading bool
	// refreshing is set from "r" until the next result for key.
	refreshing bool
	err        error
	spinner    spinner.Model
}

func newLoader(key data.Key, noun string) loader {
	return loader{key: key, noun: noun, loading: true, spinner: spinner.New()}
}

func (l loader) init() tea.Cmd { return tea.Batch(l.spinner.Tick, data.Fetch(l.key)) }

// result takes the result for the loader's own key and returns its data.
func (l *loader) result(msg data.ResultMsg) (any, bool) {
	if msg.Key != l.key {
		return nil, false
	}
	l.loading, l.refreshing, l.err = false, false, msg.Err
	return msg.Data, true
}

// refresh marks the loader as refreshing and asks for fresh data.
func (l *loader) refresh() tea.Cmd {
	l.refreshing = true
	return data.Refresh(l.key)
}

// tick advances the spinner while loading.
func (l *loader) tick(msg spinner.TickMsg) tea.Cmd {
	if !l.loading {
		return nil
	}
	var cmd tea.Cmd
	l.spinner, cmd = l.spinner.Update(msg)
	return cmd
}

// ready reports whether data has arrived without an error.
func (l loader) ready() bool { return !l.loading && l.err == nil }

// status renders the loading or error text; false once the data is ready.
func (l loader) status(width, height int) (string, bool) {
	text := lipgloss.NewStyle().MaxHeight(height).Width(width)
	switch {
	case l.loading:
		return text.Render(l.spinner.View() + " Loading " + l.noun + "…"), true
	case l.err != nil:
		return text.Inherit(errorStyle).Render(fmt.Sprintf("Failed to load %s: %v", l.noun, l.err)), true
	}
	return "", false
}

// lines is the height of an optional one-line part of a view.
func lines(present bool) int {
	if present {
		return 1
	}
	return 0
}

// clip cuts s to at most width columns and height lines.
func clip(s string, width, height int) string {
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(s)
}

// wrap breaks s into lines of at most width columns.
func wrap(s string, width int) string { return lipgloss.NewStyle().Width(width).Render(s) }

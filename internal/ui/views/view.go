// Package views holds the views a pane can show.
package views

import tea "charm.land/bubbletea/v2"

// View is the content of one pane.
type View interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (View, tea.Cmd)
	// Resize sets the inner pane size; called on every layout change.
	Resize(width, height int) View
	// Render fits into the last Resize size: at most height lines, no line wider than width.
	Render() string
	// Title is shown in the pane border, the tab bar and the status bar.
	Title() string
	// Open returns the view for the selected item; false if nothing opens.
	Open() (View, bool)
	// Spec describes the view so Build can make it again.
	Spec() Spec
}

// Filterer is a View whose rows can be filtered.
type Filterer interface {
	View
	Filter() string
	// SetFilter keeps the rows with a cell that contains query, ignoring case.
	SetFilter(query string) View
}

// Counter is a View that reports its number of rows once its data has loaded.
type Counter interface {
	View
	Count() (n int, ok bool)
}

// Clicker is a View that reacts to a mouse click.
type Clicker interface {
	View
	// Click takes coordinates inside the view's inner area; hit reports
	// whether they fell on a row.
	Click(x, y int) (v View, cmd tea.Cmd, hit bool)
}

// ScopeMsg tells the views of a tab which subscription the tab shows; an empty ID means all.
// Lists over rows of every subscription keep the rows of that subscription; other views ignore it.
type ScopeMsg struct{ ID, Name string }

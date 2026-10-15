package views

import (
	"fmt"
	"sync/atomic"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
)

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red)

// LoadFunc fetches the subscriptions to show.
type LoadFunc func() ([]azure.Subscription, error)

// lastID numbers the views: the root model sends results to every view,
// and each view takes only its own.
var lastID atomic.Int64

type loadedMsg struct {
	id   int64
	subs []azure.Subscription
	err  error
}

// Subscriptions lists the Azure subscriptions of the signed-in account.
type Subscriptions struct {
	id            int64
	load          LoadFunc
	loading       bool
	err           error
	spinner       spinner.Model
	table         table.Model
	width, height int
}

func NewSubscriptions(load LoadFunc) Subscriptions {
	return Subscriptions{
		id:      lastID.Add(1),
		load:    load,
		loading: true,
		spinner: spinner.New(),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "Name", Width: 32},
				{Title: "Subscription ID", Width: 36},
				{Title: "State", Width: 10},
			}),
			table.WithFocused(true),
		),
	}
}

func (s Subscriptions) Title() string { return "Subscriptions" }

func (s Subscriptions) Init() tea.Cmd {
	return tea.Batch(s.spinner.Tick, s.fetch)
}

func (s Subscriptions) fetch() tea.Msg {
	subs, err := s.load()
	return loadedMsg{id: s.id, subs: subs, err: err}
}

func (s Subscriptions) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		if msg.id != s.id {
			return s, nil
		}
		s.loading = false
		s.err = msg.err
		s.table.SetRows(rows(msg.subs))
		return s, nil
	case spinner.TickMsg:
		if !s.loading {
			return s, nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	}

	var cmd tea.Cmd
	s.table, cmd = s.table.Update(msg)
	return s, cmd
}

func (s Subscriptions) Resize(width, height int) View {
	s.width, s.height = width, height
	s.table.SetWidth(width)
	s.table.SetHeight(height)
	return s
}

func (s Subscriptions) Render() string {
	if s.width <= 0 || s.height <= 0 {
		return ""
	}
	fit := lipgloss.NewStyle().MaxHeight(s.height)
	text := fit.Width(s.width)
	switch {
	case s.loading:
		return text.Render(s.spinner.View() + " Loading subscriptions…")
	case s.err != nil:
		return text.Inherit(errorStyle).Render(fmt.Sprintf("Failed to load subscriptions: %v", s.err))
	case len(s.table.Rows()) == 0:
		return text.Render("No subscriptions found. Check the account you signed in with az login.")
	default:
		return fit.MaxWidth(s.width).Render(s.table.View())
	}
}

func rows(subs []azure.Subscription) []table.Row {
	r := make([]table.Row, len(subs))
	for i, s := range subs {
		r[i] = table.Row{s.Name, s.ID, s.State}
	}
	return r
}

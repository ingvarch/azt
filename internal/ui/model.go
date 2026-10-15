// Package ui is the Bubble Tea interface of azt.
package ui

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
)

// chromeHeight is the lines around the body: title, blank, blank, help.
const chromeHeight = 4

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red)
	helpStyle  = lipgloss.NewStyle().Faint(true)
)

// LoadFunc fetches the subscriptions to show.
type LoadFunc func() ([]azure.Subscription, error)

type loadedMsg struct {
	subs []azure.Subscription
	err  error
}

type Model struct {
	load    LoadFunc
	loading bool
	err     error
	spinner spinner.Model
	table   table.Model
}

func New(load LoadFunc) Model {
	return Model{
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetch)
}

func (m Model) fetch() tea.Msg {
	subs, err := m.load()
	return loadedMsg{subs: subs, err: err}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - chromeHeight)
		return m, nil
	case loadedMsg:
		m.loading = false
		m.err = msg.err
		m.table.SetRows(rows(msg.subs))
		return m, nil
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	v := tea.NewView(titleStyle.Render("Azure subscriptions") + "\n\n" +
		m.body() + "\n\n" +
		helpStyle.Render("↑/↓ move • q quit"))
	v.AltScreen = true
	return v
}

func (m Model) body() string {
	switch {
	case m.loading:
		return m.spinner.View() + " Loading subscriptions…"
	case m.err != nil:
		return errorStyle.Render(fmt.Sprintf("Failed to load subscriptions: %v", m.err))
	case len(m.table.Rows()) == 0:
		return "No subscriptions found. Check the account you signed in with az login."
	default:
		return m.table.View()
	}
}

func rows(subs []azure.Subscription) []table.Row {
	r := make([]table.Row, len(subs))
	for i, s := range subs {
		r[i] = table.Row{s.Name, s.ID, s.State}
	}
	return r
}

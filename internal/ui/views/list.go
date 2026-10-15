package views

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

// listConfig describes one table view over items of type T.
type listConfig[T any] struct {
	key     data.Key
	title   string
	noun    string
	empty   string
	columns []table.Column
	row     func(T) table.Row
	open    func(T) View
}

type list[T any] struct {
	loader
	cfg           listConfig[T]
	items         []T
	table         table.Model
	width, height int
}

func newList[T any](cfg listConfig[T]) View {
	return list[T]{
		loader: newLoader(cfg.key, cfg.noun),
		cfg:    cfg,
		table:  table.New(table.WithColumns(cfg.columns), table.WithFocused(true)),
	}
}

func (l list[T]) Title() string { return l.cfg.title }

func (l list[T]) Init() tea.Cmd { return l.init() }

func (l list[T]) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case data.ResultMsg:
		d, ok := l.result(msg)
		if ok {
			l.items, _ = d.([]T)
			l.table.SetRows(mapRows(l.items, l.cfg.row))
			if l.table.Cursor() < 0 {
				l.table.SetCursor(0)
			}
		}
		return l, nil
	case spinner.TickMsg:
		return l, l.tick(msg)
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			return l, data.Refresh(l.key)
		}
	}
	var cmd tea.Cmd
	l.table, cmd = l.table.Update(msg)
	return l, cmd
}

func (l list[T]) Resize(width, height int) View {
	l.width, l.height = width, height
	l.table.SetWidth(width)
	l.table.SetHeight(height)
	return l
}

func (l list[T]) Render() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}
	if s, ok := l.status(l.width, l.height); ok {
		return s
	}
	if len(l.items) == 0 {
		return lipgloss.NewStyle().MaxHeight(l.height).Width(l.width).Render(l.cfg.empty)
	}
	return lipgloss.NewStyle().MaxHeight(l.height).MaxWidth(l.width).Render(l.table.View())
}

func (l list[T]) Open() (View, bool) {
	if !l.ready() || len(l.items) == 0 {
		return nil, false
	}
	return l.cfg.open(l.items[l.table.Cursor()]), true
}

func NewSubscriptions() View {
	return newList(listConfig[azure.Subscription]{
		key:   data.Key{Kind: data.Subscriptions},
		title: "Subscriptions",
		noun:  "subscriptions",
		empty: "No subscriptions found. Check the account you signed in with az login.",
		columns: []table.Column{
			{Title: "Name", Width: 32},
			{Title: "Subscription ID", Width: 36},
			{Title: "State", Width: 10},
		},
		row:  func(s azure.Subscription) table.Row { return table.Row{s.Name, s.ID, s.State} },
		open: func(s azure.Subscription) View { return NewResourceGroups(s) },
	})
}

func NewResourceGroups(sub azure.Subscription) View {
	return newList(listConfig[azure.ResourceGroup]{
		key:   data.Key{Kind: data.ResourceGroups, Subscription: sub.ID},
		title: sub.Name,
		noun:  "resource groups",
		empty: "No resource groups in this subscription.",
		columns: []table.Column{
			{Title: "Name", Width: 40},
			{Title: "Location", Width: 16},
			{Title: "State", Width: 12},
		},
		row:  func(g azure.ResourceGroup) table.Row { return table.Row{g.Name, g.Location, g.State} },
		open: func(g azure.ResourceGroup) View { return NewResources(sub.ID, g) },
	})
}

func NewResources(subscriptionID string, group azure.ResourceGroup) View {
	return newList(listConfig[azure.Resource]{
		key:   data.Key{Kind: data.Resources, Subscription: subscriptionID, Group: group.Name},
		title: group.Name,
		noun:  "resources",
		empty: "No resources in this resource group.",
		columns: []table.Column{
			{Title: "Name", Width: 36},
			{Title: "Type", Width: 44},
			{Title: "Location", Width: 16},
		},
		row:  func(r azure.Resource) table.Row { return table.Row{r.Name, r.Type, r.Location} },
		open: func(r azure.Resource) View { return NewDetails(subscriptionID, r) },
	})
}

func mapRows[T any](items []T, row func(T) table.Row) []table.Row {
	rows := make([]table.Row, len(items))
	for i, it := range items {
		rows[i] = row(it)
	}
	return rows
}

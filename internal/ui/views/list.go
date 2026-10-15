package views

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

// listConfig describes one table view over items of type T.
type listConfig[T any] struct {
	key     data.Key
	spec    Spec
	title   string
	noun    string
	empty   string
	columns []column
	row     func(T) []string
	open    func(T) View
	// keep, when set, drops the items it rejects.
	keep func(T) bool
}

type list[T any] struct {
	loader
	cfg           listConfig[T]
	items         []T // after cfg.keep
	shown         []T // items that match filter
	filter        string
	table         table
	width, height int
}

func newList[T any](cfg listConfig[T]) View {
	return list[T]{
		loader: newLoader(cfg.key, cfg.noun),
		cfg:    cfg,
		table:  newTable(cfg.columns),
	}
}

func (l list[T]) Title() string { return l.cfg.title }

func (l list[T]) Init() tea.Cmd { return l.init() }

func (l list[T]) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case data.ResultMsg:
		d, ok := l.result(msg)
		if !ok {
			return l, nil
		}
		l.items, _ = d.([]T)
		if l.cfg.keep != nil {
			l.items = slices.DeleteFunc(slices.Clone(l.items), func(it T) bool { return !l.cfg.keep(it) })
		}
		l.table.cols = listColumns(l.cfg.columns, mapRows(l.items, l.cfg.row))
		l.applyFilter()
		return l.fit(), nil
	case spinner.TickMsg:
		cmd := l.tick(msg)
		return l, cmd
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			cmd := l.refresh()
			return l.fit(), cmd
		}
		l.table.press(msg)
	}
	return l, nil
}

func (l list[T]) Resize(width, height int) View {
	l.width, l.height = width, height
	return l.fit()
}

// fit sizes the table, leaving a line for the info line.
func (l list[T]) fit() list[T] {
	l.table.resize(l.width, l.height-lines(l.hasInfo()))
	return l
}

func (l list[T]) hasInfo() bool { return l.filter != "" || l.refreshing }

func (l list[T]) Filter() string { return l.filter }

func (l list[T]) Spec() Spec {
	s := l.cfg.spec
	s.Filter = l.filter
	return s
}

func (l list[T]) SetFilter(query string) View {
	if query == l.filter {
		return l
	}
	l.filter = query
	l.applyFilter()
	l.table.selectRow(0)
	return l.fit()
}

// applyFilter recomputes the shown items and the table rows from items and filter.
func (l *list[T]) applyFilter() {
	query := strings.ToLower(l.filter)
	l.shown = slices.DeleteFunc(slices.Clone(l.items), func(it T) bool {
		return !slices.ContainsFunc(l.cfg.row(it), func(cell string) bool {
			return strings.Contains(strings.ToLower(cell), query)
		})
	})
	l.table.setRows(mapRows(l.shown, l.cfg.row))
}

func (l list[T]) info() string {
	var parts []string
	if l.filter != "" {
		parts = append(parts, "/"+l.filter, fmt.Sprintf("%d of %d", len(l.shown), len(l.items)))
	}
	if l.refreshing {
		parts = append(parts, refreshingText)
	}
	return clip(faintStyle.Render(strings.Join(parts, "  ")), l.width, 1)
}

func (l list[T]) Render() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}
	if s, ok := l.status(l.width, l.height); ok {
		return s
	}
	var body string
	switch {
	case len(l.items) == 0:
		body = wrap(l.cfg.empty, l.width)
	case len(l.shown) == 0:
		body = wrap("No matches.", l.width)
	default:
		body = l.table.view()
	}
	if l.hasInfo() {
		body = lipgloss.JoinVertical(lipgloss.Left, l.info(), body)
	}
	return clip(body, l.width, l.height)
}

// hasRows reports whether the table is drawn.
func (l list[T]) hasRows() bool { return l.ready() && len(l.shown) > 0 }

func (l list[T]) Open() (View, bool) {
	if !l.hasRows() {
		return nil, false
	}
	return l.cfg.open(l.shown[l.table.cursor]), true
}

// Click selects the row drawn on line y; the info line is line 0 when shown.
func (l list[T]) Click(_, y int) (View, tea.Cmd, bool) {
	i, ok := l.table.RowAt(y - lines(l.hasInfo()))
	if ok {
		l.table.selectRow(i)
	}
	return l, nil, ok
}

// NewSubscriptions lists the subscriptions of the signed-in account.
func NewSubscriptions() View {
	return newList(listConfig[azure.Subscription]{
		key:   data.Key{Kind: data.Subscriptions},
		spec:  Spec{Kind: "subscriptions"},
		title: "Subscriptions",
		noun:  "subscriptions",
		empty: "No subscriptions found. Check the account you signed in with az login.",
		columns: []column{
			{title: "Name"},
			{title: "Subscription ID"},
			{title: "State", state: true},
		},
		row:  func(s azure.Subscription) []string { return []string{s.Name, s.ID, s.State} },
		open: NewResourceGroups,
	})
}

// NewResourceGroups lists the resource groups of sub.
func NewResourceGroups(sub azure.Subscription) View {
	return newList(listConfig[azure.ResourceGroup]{
		key:   data.Key{Kind: data.ResourceGroups, Subscription: sub.ID},
		spec:  Spec{Kind: "resourceGroups", Subscription: sub.ID, SubscriptionName: sub.Name},
		title: sub.Name,
		noun:  "resource groups",
		empty: "No resource groups in this subscription.",
		columns: []column{
			{title: "Name"},
			{title: "Location"},
			{title: "State", state: true},
		},
		row:  func(g azure.ResourceGroup) []string { return []string{g.Name, g.Location, g.State} },
		open: func(g azure.ResourceGroup) View { return NewResources(sub.ID, g) },
	})
}

// NewResources lists the resources of one resource group.
func NewResources(subscriptionID string, group azure.ResourceGroup) View {
	return newList(resourcesConfig(subscriptionID, group))
}

// NewResourcesOfType lists the resources of one type in a resource group; it
// shares its data with NewResources.
func NewResourcesOfType(subscriptionID string, group azure.ResourceGroup, resourceType string) View {
	cfg := resourcesConfig(subscriptionID, group)
	cfg.spec.Kind, cfg.spec.Type = "resourcesOfType", resourceType
	cfg.title = ShortType(resourceType)
	cfg.empty = "No resources of this type in this resource group."
	cfg.keep = func(r azure.Resource) bool { return strings.EqualFold(r.Type, resourceType) }
	return newList(cfg)
}

// ShortType drops the leading "microsoft." from a resource type.
func ShortType(t string) string {
	const prefix = "microsoft."
	if len(t) >= len(prefix) && strings.EqualFold(t[:len(prefix)], prefix) {
		return t[len(prefix):]
	}
	return t
}

func resourcesConfig(subscriptionID string, group azure.ResourceGroup) listConfig[azure.Resource] {
	return listConfig[azure.Resource]{
		key:   data.Key{Kind: data.Resources, Subscription: subscriptionID, Group: group.Name},
		spec:  Spec{Kind: "resources", Subscription: subscriptionID, Group: group.Name},
		title: group.Name,
		noun:  "resources",
		empty: "No resources in this resource group.",
		columns: []column{
			{title: "Name"},
			{title: "Type"},
			{title: "Location"},
		},
		row:  func(r azure.Resource) []string { return []string{r.Name, r.Type, r.Location} },
		open: func(r azure.Resource) View { return NewDetails(subscriptionID, r) },
	}
}

// NewAllResourceGroups lists the resource groups of every subscription.
func NewAllResourceGroups() View {
	return newList(listConfig[azure.ResourceGroup]{
		key:   data.Key{Kind: data.AllResourceGroups},
		spec:  Spec{Kind: "allResourceGroups"},
		title: "Resource groups",
		noun:  "resource groups",
		empty: "No resource groups found.",
		columns: []column{
			{title: "Name"},
			{title: "Subscription"},
			{title: "Location"},
			{title: "State", state: true},
		},
		row:  func(g azure.ResourceGroup) []string { return []string{g.Name, g.Subscription, g.Location, g.State} },
		open: func(g azure.ResourceGroup) View { return NewResources(g.SubscriptionID, g) },
	})
}

// NewResourcesByType lists the resources of the given types in every subscription.
// With several types the list adds a Type column.
func NewResourcesByType(resourceTypes ...string) View {
	joined := strings.Join(resourceTypes, ",")
	short := make([]string, len(resourceTypes))
	for i, t := range resourceTypes {
		short[i] = ShortType(t)
	}
	several := len(resourceTypes) > 1
	empty := "No resources of this type."
	columns := []column{
		{title: "Name"},
		{title: "Resource group"},
		{title: "Subscription"},
		{title: "Location"},
	}
	if several {
		empty = "No resources of these types."
		columns = slices.Insert(columns, 1, column{title: "Type"})
	}
	return newList(listConfig[azure.Resource]{
		key:     data.Key{Kind: data.ResourcesOfType, Type: joined},
		spec:    Spec{Kind: "resourcesByType", Type: joined},
		title:   strings.Join(short, ", "),
		noun:    "resources",
		empty:   empty,
		columns: columns,
		row: func(r azure.Resource) []string {
			cells := []string{r.Name, r.ResourceGroup, r.Subscription, r.Location}
			if several {
				cells = slices.Insert(cells, 1, ShortType(r.Type))
			}
			return cells
		},
		open: func(r azure.Resource) View { return NewDetails(r.SubscriptionID, r) },
	})
}

func mapRows[T any](items []T, row func(T) []string) [][]string {
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = row(it)
	}
	return rows
}

// listColumnMax is the widest a list column is by its own content.
const listColumnMax = 40

// listColumns is cols sized to the rows, none wider than listColumnMax.
func listColumns(cols []column, rows [][]string) []column {
	cols = sizeColumns(cols, rows, true)
	for i := range cols {
		cols[i].width = min(cols[i].width, listColumnMax)
	}
	return cols
}

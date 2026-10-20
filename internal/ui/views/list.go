package views

import (
	"fmt"
	"slices"
	"strconv"
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
	item    func(T) Item
	// keep, when set, drops the items it rejects.
	keep func(T) bool
	// subscription, when set, gives the subscription ID of an item; the list then
	// follows ScopeMsg and keeps the items of the scope's subscription.
	subscription func(T) string
	// numbered adds a first column with the position, 1 to 9, of the first nine items.
	numbered bool
}

type list[T any] struct {
	loader
	cfg           listConfig[T]
	raw           []T // as loaded
	items         []T // after cfg.keep and the scope
	shown         []T // items that match filter
	scope         ScopeMsg
	filter        string
	table         table
	width, height int
}

func newList[T any](cfg listConfig[T]) View {
	return list[T]{
		loader: newLoader(cfg.key, cfg.noun),
		cfg:    cfg,
		table:  newTable(configColumns(cfg)),
	}
}

func (l list[T]) Title() string {
	if l.scope.Name == "" {
		return l.cfg.title
	}
	return l.cfg.title + " (" + l.scope.Name + ")"
}

// Count is the number of items after keep and the scope; false while loading or after an error.
func (l list[T]) Count() (int, bool) { return len(l.items), l.ready() }

func (l list[T]) Init() tea.Cmd { return l.init() }

func (l list[T]) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case data.ResultMsg:
		d, ok := l.result(msg)
		if !ok {
			return l, nil
		}
		l.raw, _ = d.([]T)
		l.refill()
		return l.fit(), nil
	case ScopeMsg:
		if l.cfg.subscription == nil {
			return l, nil
		}
		if msg != l.scope {
			l.scope = msg
			l.refill()
			l.table.selectRow(0)
		}
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

// refill recomputes the items, the column widths and the shown rows from the loaded items.
func (l *list[T]) refill() {
	l.items = slices.DeleteFunc(slices.Clone(l.raw), func(it T) bool { return !l.wanted(it) })
	l.table.cols = listColumns(configColumns(l.cfg), l.rows())
	l.applyFilter()
}

// wanted reports whether it passes keep and the scope.
func (l list[T]) wanted(it T) bool {
	if l.cfg.keep != nil && !l.cfg.keep(it) {
		return false
	}
	return l.scope.ID == "" || l.cfg.subscription == nil || strings.EqualFold(l.cfg.subscription(it), l.scope.ID)
}

// applyFilter recomputes the shown items and the table rows from items and filter.
// The filter does not match the row number.
func (l *list[T]) applyFilter() {
	query := strings.ToLower(l.filter)
	l.shown = nil
	var rows [][]string
	for i, it := range l.items {
		cells := l.cells(i)
		own := cells
		if l.cfg.numbered {
			own = cells[1:]
		}
		if slices.ContainsFunc(own, func(cell string) bool {
			return strings.Contains(strings.ToLower(cell), query)
		}) {
			l.shown = append(l.shown, it)
			rows = append(rows, cells)
		}
	}
	l.table.setRows(rows)
}

// cells is the table row of items[i].
func (l list[T]) cells(i int) []string {
	cells := l.cfg.row(l.items[i])
	if !l.cfg.numbered {
		return cells
	}
	number := ""
	if i < 9 {
		number = strconv.Itoa(i + 1)
	}
	return slices.Insert(cells, 0, number)
}

// rows is the table rows of all items.
func (l list[T]) rows() [][]string {
	rows := make([][]string, len(l.items))
	for i := range l.items {
		rows[i] = l.cells(i)
	}
	return rows
}

// configColumns is the columns of cfg, with the number column when it has one.
func configColumns[T any](cfg listConfig[T]) []column {
	if cfg.numbered {
		return slices.Insert(slices.Clone(cfg.columns), 0, column{title: "#", fixed: true})
	}
	return cfg.columns
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

// current is the item of the row under the cursor; false when no row is drawn.
func (l list[T]) current() (T, bool) {
	if !l.hasRows() {
		var zero T
		return zero, false
	}
	return l.shown[l.table.cursor], true
}

func (l list[T]) Open() (View, bool) {
	it, ok := l.current()
	if !ok {
		return nil, false
	}
	return l.cfg.open(it), true
}

func (l list[T]) Selected() (Item, bool) {
	it, ok := l.current()
	if !ok {
		return Item{}, false
	}
	return l.cfg.item(it), true
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
		row:      func(s azure.Subscription) []string { return []string{s.Name, s.ID, s.State} },
		open:     NewResourceGroups,
		item:     SubscriptionItem,
		numbered: true,
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
		item: GroupItem,
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
	cfg.title = KindName(resourceType)
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

// SubscriptionItem is s by its subscription ID and name.
func SubscriptionItem(s azure.Subscription) Item { return Item{ID: s.ID, Name: s.Name} }

// GroupItem is g by its resource ID and name.
func GroupItem(g azure.ResourceGroup) Item { return Item{ID: g.ID(), Name: g.Name} }

// ResourceItem is r by its resource ID and name.
func ResourceItem(r azure.Resource) Item { return Item{ID: r.ID, Name: r.Name} }

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
		item: ResourceItem,
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
		row:          func(g azure.ResourceGroup) []string { return []string{g.Name, g.Subscription, g.Location, g.State} },
		open:         func(g azure.ResourceGroup) View { return NewResources(g.SubscriptionID, g) },
		item:         GroupItem,
		subscription: func(g azure.ResourceGroup) string { return g.SubscriptionID },
	})
}

// NewResourcesByType lists the resources of the given types in every subscription.
// With several types the list adds a Type column.
func NewResourcesByType(resourceTypes ...string) View {
	joined := strings.Join(resourceTypes, ",")
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
		title:   KindName(resourceTypes[0]),
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
		open:         func(r azure.Resource) View { return NewDetails(r.SubscriptionID, r) },
		item:         ResourceItem,
		subscription: func(r azure.Resource) string { return r.SubscriptionID },
	})
}

// ofType makes cfg a list of the resources of type typ in every subscription;
// resource is the resource an item holds.
func ofType[T any](cfg listConfig[T], typ string, resource func(T) azure.Resource) listConfig[T] {
	cfg.spec = Spec{Kind: "resourcesByType", Type: typ}
	cfg.title = KindName(typ)
	cfg.noun = "resources"
	cfg.empty = "No resources of this type."
	cfg.open = func(it T) View {
		r := resource(it)
		return NewDetails(r.SubscriptionID, r)
	}
	cfg.item = func(it T) Item { return ResourceItem(resource(it)) }
	cfg.subscription = func(it T) string { return resource(it).SubscriptionID }
	return cfg
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

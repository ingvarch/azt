// Package sidebar holds the explorer tree: subscriptions, resource groups and
// resource types.
package sidebar

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

type kind int

const (
	subscription kind = iota
	group
	resourceType
)

type node struct {
	kind     kind
	id       string
	label    string
	key      data.Key // data behind the children; empty for types
	sub      azure.Subscription
	group    azure.ResourceGroup
	typ      string
	count    int
	expanded bool
	loaded   bool
	loading  bool
	err      error
	children []*node
}

func (n *node) clone() *node {
	c := *n
	c.children = cloneAll(n.children)
	return &c
}

func cloneAll(nodes []*node) []*node {
	out := make([]*node, len(nodes))
	for i, n := range nodes {
		out[i] = n.clone()
	}
	return out
}

// row is one visible line; n is nil for a status row.
type row struct {
	id   string
	text string
	path []*node // from the top-level node down to the row's node or, for a status row, its parent
	n    *node
}

// Model is the explorer tree.
type Model struct {
	nodes         []*node
	loaded        bool
	err           error
	sel           string
	selPath       []string // ids on the selected row's path, to fall back to an ancestor
	idx, top      int
	width, height int
	focused       bool
}

// New returns an empty explorer.
func New() Model { return Model{} }

// SetFocused sets whether the explorer has keyboard focus.
func (m Model) SetFocused(f bool) Model {
	m.focused = f
	return m
}

// Title is the pane border title.
func (m Model) Title() string { return "Explorer" }

// Init requests the subscriptions that form the top of the tree.
func (m Model) Init() tea.Cmd { return data.Fetch(data.Key{Kind: data.Subscriptions}) }

// Open never opens a view: the tree opens items itself.
func (m Model) Open() (views.View, bool) { return nil, false }

// Spec is never saved: the sidebar is not part of a pane stack.
func (m Model) Spec() views.Spec { return views.Spec{Kind: "explorer"} }

// Update applies fetched data and key presses to the tree.
func (m Model) Update(msg tea.Msg) (views.View, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case data.ResultMsg:
		m.nodes = cloneAll(m.nodes)
		m.apply(msg)
	case tea.KeyPressMsg:
		m.nodes = cloneAll(m.nodes)
		cmd = m.press(msg.String())
	}
	return m.fix(), cmd
}

// Click selects the row at y and toggles it when x is on its marker; false when no row is hit.
func (m Model) Click(x, y int) (views.View, tea.Cmd, bool) {
	m.nodes = cloneAll(m.nodes)
	cmd, hit := m.click(x, y)
	return m.fix(), cmd, hit
}

// Resize sets the inner size and keeps the cursor visible.
func (m Model) Resize(width, height int) views.View {
	m.width, m.height = width, height
	return m.fix()
}

// Render draws the tree, or a message while loading or after an error.
func (m Model) Render() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	switch {
	case m.err != nil:
		return m.message("Failed to load subscriptions: " + m.err.Error())
	case !m.loaded:
		return m.message("Loading subscriptions…")
	case len(m.nodes) == 0:
		return m.message("No subscriptions found.")
	}
	rows := m.rows()
	rows = rows[m.top:min(m.top+m.height, len(rows))]
	out := make([]string, len(rows))
	for i, r := range rows {
		style := lipgloss.NewStyle().MaxWidth(m.width)
		if r.id == m.sel {
			style = style.Reverse(m.focused).Bold(!m.focused)
		}
		out[i] = style.Render(r.text)
	}
	return strings.Join(out, "\n")
}

func (m Model) message(text string) string {
	return lipgloss.NewStyle().MaxHeight(m.height).Width(m.width).Render(text)
}

// Stack returns the pane stack that leads to the selected node.
func (m Model) Stack() ([]views.View, bool) {
	r, ok := m.selected()
	if !ok || r.n == nil {
		return nil, false
	}
	sub := r.path[0].sub
	stack := []views.View{views.NewSubscriptions(), views.NewResourceGroups(sub)}
	if r.n.kind == subscription {
		return stack, true
	}
	g := r.path[1].group
	stack = append(stack, views.NewResources(sub.ID, g))
	if r.n.kind == resourceType {
		stack = append(stack, views.NewResourcesOfType(sub.ID, g, r.n.typ))
	}
	return stack, true
}

func (m Model) selected() (row, bool) {
	rows := m.rows()
	if m.idx < 0 || m.idx >= len(rows) {
		return row{}, false
	}
	return rows[m.idx], true
}

// rows lists the visible rows; none until the subscriptions are loaded.
func (m Model) rows() []row {
	if !m.loaded || m.err != nil {
		return nil
	}
	var rows []row
	for _, n := range m.nodes {
		rows = appendRows(rows, nil, n)
	}
	return rows
}

func appendRows(rows []row, parents []*node, n *node) []row {
	path := append(slices.Clone(parents), n)
	depth := len(parents)
	marker := "  "
	if n.kind != resourceType {
		marker = "▸ "
		if n.expanded {
			marker = "▾ "
		}
	}
	rows = append(rows, row{id: n.id, text: line(depth, marker, n.label), path: path, n: n})
	if !n.expanded {
		return rows
	}
	status := func(s string) row {
		return row{id: n.id + "#status", text: line(depth+1, "  ", s), path: path}
	}
	switch {
	case n.loading:
		return append(rows, status("loading…"))
	case n.err != nil:
		return append(rows, status("failed: "+n.err.Error()))
	case len(n.children) == 0:
		return append(rows, status("empty"))
	}
	for _, c := range n.children {
		rows = appendRows(rows, path, c)
	}
	return rows
}

var oneLine = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

// Cells of the indent per depth level and of the marker after it.
const indentWidth, markerWidth = 2, 2

func line(depth int, marker, label string) string {
	return strings.Repeat(" ", indentWidth*depth) + marker + oneLine.Replace(label)
}

// fix keeps the selection on its node, or on its nearest ancestor row when the node is gone,
// or at the same position when there is none, and scrolls it into view.
// Without rows the selection is kept for when they come back.
func (m Model) fix() Model {
	rows := m.rows()
	if len(rows) == 0 {
		return m
	}
	indexOf := func(id string) int { return slices.IndexFunc(rows, func(r row) bool { return r.id == id }) }
	i := indexOf(m.sel)
	for j := len(m.selPath) - 1; i < 0 && j >= 0; j-- {
		i = indexOf(m.selPath[j])
	}
	if i >= 0 {
		m.idx = i
	}
	m.idx = max(0, min(m.idx, len(rows)-1))
	m.sel = rows[m.idx].id
	m.selPath = rows[m.idx].pathIDs()
	m.top = max(0, min(m.top, len(rows)-m.height, m.idx))
	m.top = max(m.top, m.idx-m.height+1)
	return m
}

func (m *Model) apply(msg data.ResultMsg) {
	k := msg.Key
	switch k.Kind {
	case data.Subscriptions:
		m.err = msg.Err
		if msg.Err != nil {
			return
		}
		subs, _ := msg.Data.([]azure.Subscription)
		m.loaded = true
		m.nodes = mergeNodes(m.nodes, subs, func(s azure.Subscription) *node {
			return &node{
				kind: subscription, id: "s/" + s.ID, label: s.Name, sub: s,
				key: data.Key{Kind: data.ResourceGroups, Subscription: s.ID},
			}
		})
	case data.ResourceGroups:
		n := m.find(k.Subscription, "")
		if n == nil || !n.finish(msg) {
			return
		}
		groups, _ := msg.Data.([]azure.ResourceGroup)
		n.children = mergeNodes(n.children, groups, func(g azure.ResourceGroup) *node {
			return &node{
				kind: group, id: "g/" + k.Subscription + "/" + g.Name, label: g.Name, group: g,
				key: data.Key{Kind: data.Resources, Subscription: k.Subscription, Group: g.Name},
			}
		})
	case data.Resources:
		if k.Group == "" {
			return
		}
		n := m.find(k.Subscription, k.Group)
		if n == nil || !n.finish(msg) {
			return
		}
		resources, _ := msg.Data.([]azure.Resource)
		n.children = typeNodes(n, resources)
	}
}

// find returns the subscription node, or its group node when groupName is set.
func (m *Model) find(subscriptionID, groupName string) *node {
	i := slices.IndexFunc(m.nodes, func(n *node) bool { return n.sub.ID == subscriptionID })
	if i < 0 {
		return nil
	}
	n := m.nodes[i]
	if groupName == "" {
		return n
	}
	j := slices.IndexFunc(n.children, func(c *node) bool { return c.group.Name == groupName })
	if j < 0 {
		return nil
	}
	return n.children[j]
}

// finish records the end of a load; false when it failed.
func (n *node) finish(msg data.ResultMsg) bool {
	n.loading, n.err = false, msg.Err
	n.loaded = msg.Err == nil
	return msg.Err == nil
}

// mergeNodes builds the nodes for items in order, reusing the existing node with the same id.
func mergeNodes[T any](old []*node, items []T, build func(T) *node) []*node {
	nodes := make([]*node, len(items))
	for i, it := range items {
		n := build(it)
		if j := slices.IndexFunc(old, func(o *node) bool { return o.id == n.id }); j >= 0 {
			o := old[j]
			o.label, o.sub, o.group = n.label, n.sub, n.group
			n = o
		}
		nodes[i] = n
	}
	return nodes
}

// typeNodes groups resources by type, ignoring case, sorted by short type.
func typeNodes(parent *node, resources []azure.Resource) []*node {
	types := map[string]*node{}
	for _, r := range resources {
		lower := strings.ToLower(r.Type)
		if types[lower] == nil {
			types[lower] = &node{kind: resourceType, id: "t/" + parent.id + "/" + lower, typ: r.Type}
		}
		types[lower].count++
	}
	nodes := slices.Collect(maps.Values(types))
	for _, n := range nodes {
		n.label = views.ShortType(n.typ) + " " + strconv.Itoa(n.count)
	}
	slices.SortFunc(nodes, func(a, b *node) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(views.ShortType(a.typ)), strings.ToLower(views.ShortType(b.typ))),
			cmp.Compare(strings.ToLower(a.typ), strings.ToLower(b.typ)),
		)
	})
	return nodes
}

// press handles one key; it returns the fetch the key asks for.
func (m *Model) press(k string) tea.Cmd {
	rows := m.rows()
	if k == "r" {
		return refresh(rows, m.idx)
	}
	if len(rows) == 0 {
		return nil
	}
	i := m.idx
	r := rows[i]
	switch k {
	case "j", "down":
		m.sel = rows[min(i+1, len(rows)-1)].id
	case "k", "up":
		m.sel = rows[max(i-1, 0)].id
	case "l", "right":
		return m.expand(rows, i)
	case "h", "left":
		if r.n != nil && r.n.expanded {
			r.n.expanded = false
		} else if p := r.parent(); p != nil {
			m.sel = p.id
		}
	}
	return nil
}

// click selects the row on line y; a click on the marker of a subscription or
// group also toggles it. It returns the fetch that expanding asks for.
func (m *Model) click(x, y int) (tea.Cmd, bool) {
	rows := m.rows()
	i := m.top + y
	if y < 0 || y >= m.height || i >= len(rows) {
		return nil, false
	}
	r := rows[i]
	m.sel = r.id
	if r.n == nil || x < r.indent() || x >= r.indent()+markerWidth {
		return nil, true
	}
	if r.n.expanded {
		r.n.expanded = false
		return nil, true
	}
	return m.expand(rows, i), true
}

// refresh reloads the children of the selected node, or of its parent for a
// type or status row; without rows it reloads the subscriptions.
func refresh(rows []row, i int) tea.Cmd {
	if i < 0 || i >= len(rows) {
		return data.Refresh(data.Key{Kind: data.Subscriptions})
	}
	n := rows[i].n
	if n == nil || n.kind == resourceType {
		n = rows[i].parent()
	}
	return data.Refresh(n.key)
}

// expand opens a collapsed node, fetching its children when it has none, or moves into an expanded one.
func (m *Model) expand(rows []row, i int) tea.Cmd {
	n := rows[i].n
	switch {
	case n == nil || n.kind == resourceType:
		return nil
	case n.expanded:
		m.sel = rows[i+1].id
		return nil
	}
	n.expanded = true
	if n.loaded {
		return nil
	}
	n.loading = true
	return data.Fetch(n.key)
}

// indent is the number of cells before the marker.
func (r row) indent() int { return indentWidth * (len(r.path) - 1) }

func (r row) pathIDs() []string {
	ids := make([]string, len(r.path))
	for i, n := range r.path {
		ids[i] = n.id
	}
	return ids
}

func (r row) parent() *node {
	p := r.path
	if r.n != nil {
		p = p[:len(p)-1]
	}
	if len(p) == 0 {
		return nil
	}
	return p[len(p)-1]
}

package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/session"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

// Session describes the layout of m: tabs, split trees, view stacks, focus.
func (m Model) Session() session.Session {
	s := session.Session{Version: session.Version, Active: m.active, SidebarVisible: m.sidebarVisible, SidebarWidth: m.sidebarW}
	for _, t := range m.tabs {
		s.Tabs = append(s.Tabs, session.Tab{
			Name:             t.name,
			Zoomed:           t.zoomed,
			Subscription:     t.scope.ID,
			SubscriptionName: t.scope.Name,
			Focus:            slices.Index(t.root.Panes(), t.focus),
			Layout:           m.sessionNode(t.root),
		})
	}
	return s
}

func (m Model) sessionNode(n *layout.Node) *session.Node {
	if n.IsLeaf() {
		var stack []views.Spec
		for _, v := range m.stacks[n.Pane] {
			stack = append(stack, v.Spec())
		}
		return &session.Node{Stack: stack}
	}
	split := "h"
	if n.Split == layout.Vertical {
		split = "v"
	}
	return &session.Node{Split: split, Ratio: n.Ratio, A: m.sessionNode(n.A), B: m.sessionNode(n.B)}
}

// Restore builds a model from s. Panes get new IDs, the sidebar has no focus.
func Restore(s session.Session, newView func() views.View, fetch data.Fetcher) (Model, error) {
	if s.Version != session.Version {
		return Model{}, fmt.Errorf("session version %d, want %d", s.Version, session.Version)
	}
	if len(s.Tabs) == 0 {
		return Model{}, errors.New("session has no tabs")
	}
	if s.Active < 0 || s.Active >= len(s.Tabs) {
		return Model{}, fmt.Errorf("active tab %d out of range", s.Active)
	}
	m := newModel(newView, fetch)
	for i, t := range s.Tabs {
		root, err := m.restoreNode(t.Layout)
		if err != nil {
			return Model{}, fmt.Errorf("tab %d: %w", i+1, err)
		}
		panes := root.Panes()
		if t.Focus < 0 || t.Focus >= len(panes) {
			return Model{}, fmt.Errorf("tab %d: focus %d out of range", i+1, t.Focus)
		}
		restored := tab{root: root, focus: panes[t.Focus], zoomed: t.Zoomed, name: t.Name, scope: views.ScopeMsg{ID: t.Subscription, Name: t.SubscriptionName}}
		m.scopeViews(restored)
		m.tabs = append(m.tabs, restored)
	}
	m.active = s.Active
	m.sidebarVisible = s.SidebarVisible
	if s.SidebarWidth != 0 {
		m.sidebarW = max(s.SidebarWidth, minSidebarWidth)
	}
	return m, nil
}

// restoreNode builds the layout of n and the stacks of its panes.
func (m *Model) restoreNode(n *session.Node) (*layout.Node, error) {
	if n == nil {
		return nil, errors.New("missing layout")
	}
	var o layout.Orientation
	switch n.Split {
	case "":
		return m.restorePane(n.Stack)
	case "h":
		o = layout.Horizontal
	case "v":
		o = layout.Vertical
	default:
		return nil, fmt.Errorf("unknown split %q", n.Split)
	}
	a, err := m.restoreNode(n.A)
	if err != nil {
		return nil, err
	}
	b, err := m.restoreNode(n.B)
	if err != nil {
		return nil, err
	}
	return (&layout.Node{Split: o, A: a, B: b}).SetRatio(layout.Path{}, n.Ratio), nil
}

func (m *Model) restorePane(specs []views.Spec) (*layout.Node, error) {
	if len(specs) == 0 {
		return nil, errors.New("pane without views")
	}
	stack := make([]views.View, len(specs))
	for i, spec := range specs {
		v, err := views.Build(spec)
		if err != nil {
			return nil, err
		}
		stack[i] = v
	}
	id := m.nextID
	m.nextID++
	m.stacks[id] = stack
	return layout.Leaf(id), nil
}

// LoadOrNew restores the session saved at path. A missing file starts a new
// session; any other failure does too, with a notice.
func LoadOrNew(path string, newView func() views.View, fetch data.Fetcher) Model {
	s, err := session.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(newView, fetch)
	}
	var m Model
	if err == nil {
		m, err = Restore(s, newView, fetch)
	}
	if err != nil {
		m = New(newView, fetch)
		m.notice = "could not restore the session: " + err.Error()
	}
	return m
}

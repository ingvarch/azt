package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

// session runs the root model with real views and a Fetcher returning fixed data.
type session struct{ *harness }

func newSession(t *testing.T) *session {
	t.Helper()
	s := &session{&harness{}}
	fetch := func(k data.Key) (any, error) {
		s.calls = append(s.calls, k)
		switch k.Kind {
		case data.Subscriptions:
			return []azure.Subscription{{ID: "sub-1", Name: "Dev", State: "Enabled"}}, nil
		case data.ResourceGroups:
			return []azure.ResourceGroup{{Name: "rg-a", Location: "westeurope", State: "Succeeded"}}, nil
		case data.Resources:
			return []azure.Resource{{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites", Location: "westeurope"}}, nil
		case data.Details:
			return map[string]any{"name": "web-app", "kind": "app,linux"}, nil
		}
		t.Fatalf("unexpected key %+v", k)
		return nil, nil
	}
	s.m = New(views.NewSubscriptions, fetch)
	s.send(tea.WindowSizeMsg{Width: 140, Height: 30})
	return s
}

// settle runs cmd and feeds fetch and result messages back to the model until none are left.
func (s *session) settle(cmd tea.Cmd) {
	for _, msg := range collect(cmd) {
		switch msg.(type) {
		case data.FetchMsg, data.ResultMsg:
			s.settle(s.send(msg))
		}
	}
}

func (s *session) press(k string) { s.settle(s.send(key(k))) }

func (s *session) want(t *testing.T, step string, texts ...string) {
	t.Helper()
	f := s.frame()
	for _, text := range texts {
		if !strings.Contains(f, text) {
			t.Errorf("%s: frame lacks %q:\n%s", step, text, f)
		}
	}
}

func TestDrillDownAndBack(t *testing.T) {
	s := newSession(t)
	s.settle(s.m.Init())
	s.want(t, "start", "Subscription ID", "Dev", "sub-1")

	s.press("enter")
	s.want(t, "resource groups", "Dev", "rg-a", "westeurope", "Subscriptions › Dev")

	s.press("enter")
	s.want(t, "resources", "web-app", "microsoft.web/sites", "Subscriptions › Dev › rg-a")

	s.press("enter")
	s.want(t, "details", `"kind": "app,linux"`, "Subscriptions › Dev › rg-a › web-app")

	for range 3 {
		s.press("esc")
	}
	s.want(t, "back", "Subscription ID", "sub-1")
	if f := s.frame(); strings.Contains(f, "rg-a") {
		t.Errorf("frame still shows the resource groups:\n%s", f)
	}
}

func TestSplitSharesTheCache(t *testing.T) {
	s := newSession(t)
	s.settle(s.m.Init())
	for range 3 {
		s.press("enter")
	}
	s.send(key("ctrl+w"))
	s.settle(s.send(key("v"))) // the new pane starts at the details
	for range 3 {
		s.press("esc")
	}
	for range 3 {
		s.press("enter")
	}
	s.want(t, "second pane", `"kind": "app,linux"`)

	if len(s.calls) != 4 {
		t.Errorf("fetched keys = %v, want 4", s.calls)
	}
	seen := map[data.Key]bool{}
	for _, k := range s.calls {
		if seen[k] {
			t.Errorf("Fetcher called more than once for %+v", k)
		}
		seen[k] = true
	}
}

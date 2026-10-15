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
			return []azure.Resource{
				{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites", Location: "westeurope"},
				{ID: "/r/api", Name: "web-api", Type: "Microsoft.Web/sites", Location: "westeurope"},
				{ID: "/r/sql", Name: "sql-db", Type: "microsoft.sql/servers", Location: "westeurope"},
			}, nil
		case data.ResourcesOfType:
			if k.Type != "microsoft.compute/virtualmachines" {
				t.Fatalf("unexpected type %q", k.Type)
			}
			return []azure.Resource{
				{ID: "/v/web", Name: "vm-web", ResourceGroup: "rg-a", SubscriptionID: "sub-1", Subscription: "Dev", Location: "westeurope"},
				{ID: "/v/db", Name: "vm-db", ResourceGroup: "rg-b", SubscriptionID: "sub-2", Subscription: "Prod", Location: "westeurope"},
				{ID: "/v/cache", Name: "vm-cache", ResourceGroup: "rg-a", SubscriptionID: "sub-1", Subscription: "Dev", Location: "westeurope"},
			}, nil
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

func (s *session) typ(text string) {
	for _, r := range text {
		s.press(string(r))
	}
}

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

func TestSidebarOpensFilteredResources(t *testing.T) {
	s := newSession(t)
	s.settle(s.m.Init())
	s.want(t, "start", "Explorer", "▸ Dev")

	s.press("ctrl+w")
	s.press("h")
	s.press("l")
	s.want(t, "subscription expanded", "▾ Dev", "▸ rg-a")

	s.press("j")
	s.press("l")
	s.want(t, "group expanded", "▾ rg-a", "sql/servers 1", "web/sites 2")

	s.press("j")
	s.press("j")
	s.press("enter")
	s.want(t, "type opened", "Subscriptions › Dev › rg-a › web/sites", "web-app", "web-api")
	if f := s.frame(); strings.Contains(f, "sql-db") {
		t.Errorf("pane lists another type:\n%s", f)
	}
	if s.m.sidebarFocused {
		t.Error("focus stayed in the sidebar")
	}

	s.press("esc")
	s.want(t, "group resources", "Subscriptions › Dev › rg-a ", "web-app", "web-api", "sql-db")

	seen := map[data.Key]bool{}
	for _, k := range s.calls {
		if seen[k] {
			t.Errorf("Fetcher called more than once for %+v", k)
		}
		seen[k] = true
	}
	if len(s.calls) != 3 {
		t.Errorf("fetched keys = %v, want 3", s.calls)
	}
}

func TestPaletteFilterRenameHelp(t *testing.T) {
	s := newSession(t)
	s.settle(s.m.Init())

	s.press(":")
	s.typ("vm")
	s.press("enter")
	s.want(t, ":vm", "compute/virtualmachines", "vm-web", "vm-db", "vm-cache", "Dev", "Prod")

	s.press("esc")
	s.want(t, "back", "Subscription ID", "sub-1")
	if f := s.frame(); strings.Contains(f, "vm-web") {
		t.Errorf("frame still shows the VMs:\n%s", f)
	}

	s.press(":")
	s.typ("vm")
	s.press("enter")
	s.press("/")
	s.typ("prod")
	s.want(t, "filter", "/prod", "1 of 3", "vm-db")
	if f := s.frame(); strings.Contains(f, "vm-web") {
		t.Errorf("filter left a row out of the match:\n%s", f)
	}
	s.press("esc")
	s.want(t, "filter cleared", "vm-web", "vm-db", "vm-cache")
	if f := s.frame(); strings.Contains(f, "1 of 3") {
		t.Errorf("info line stayed after esc:\n%s", f)
	}

	s.press("ctrl+w")
	s.press(",")
	s.typ("Work")
	s.press("enter")
	s.want(t, "rename", " 1 Work ")

	s.press("?")
	s.want(t, "help", "┌ Help ", "Layout (ctrl+w)")
	s.press("x")
	if f := s.frame(); strings.Contains(f, "┌ Help ") {
		t.Errorf("help stayed open:\n%s", f)
	}

	s.press(":")
	s.typ("nope")
	s.press("enter")
	s.want(t, "unknown command", "unknown command: nope")

	s.press(":")
	s.typ("st")
	s.press("tab")
	s.want(t, "completion", ":storage")
}

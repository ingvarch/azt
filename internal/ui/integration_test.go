package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/session"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

// scenario runs the root model with real views and a Fetcher returning fixed data.
type scenario struct {
	*harness
	fetcher data.Fetcher
}

func newScenario(t *testing.T) *scenario {
	t.Helper()
	s := &scenario{harness: &harness{}}
	fetch := func(k data.Key) (any, error) {
		s.calls = append(s.calls, k)
		switch k.Kind {
		case data.Subscriptions:
			return []azure.Subscription{{ID: "sub-1", Name: "Dev", State: "Enabled"}}, nil
		case data.ResourceGroups:
			return []azure.ResourceGroup{{Name: "rg-a", Location: "westeurope", State: "Succeeded"}}, nil
		case data.AllResourceGroups:
			return []azure.ResourceGroup{{Name: "rg-a", Subscription: "Dev", SubscriptionID: "sub-1"}}, nil
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
	s.fetcher = fetch
	s.m = New(views.NewSubscriptions, fetch)
	s.m.now = func() time.Time { return s.clock }
	s.send(tea.WindowSizeMsg{Width: 140, Height: 30})
	return s
}

// settle runs cmd and feeds fetch and result messages back to the model until none are left.
func (s *scenario) settle(cmd tea.Cmd) {
	for _, msg := range collect(cmd) {
		switch msg.(type) {
		case data.FetchMsg, data.ResultMsg:
			s.settle(s.send(msg))
		}
	}
}

func (s *scenario) press(k string) { s.settle(s.send(key(k))) }

func (s *scenario) typ(text string) {
	for _, r := range text {
		s.press(string(r))
	}
}

func (s *scenario) want(t *testing.T, step string, texts ...string) {
	t.Helper()
	f := s.frame()
	for _, text := range texts {
		if !strings.Contains(f, text) {
			t.Errorf("%s: frame lacks %q:\n%s", step, text, f)
		}
	}
}

func TestDrillDownAndBack(t *testing.T) {
	s := newScenario(t)
	s.settle(s.m.Init())
	s.want(t, "start", "Subscription ID", "Dev", "sub-1")

	s.press("enter")
	s.want(t, "resource groups", "Dev", "rg-a", "westeurope", "Subscriptions › Dev")

	s.press("enter")
	s.want(t, "resources", "web-app", "microsoft.web/sites", "Subscriptions › Dev › rg-a")

	s.press("enter")
	s.want(t, "details", "Essentials", "app,linux", "Subscriptions › Dev › rg-a › web-app")

	for range 3 {
		s.press("esc")
	}
	s.want(t, "back", "Subscription ID", "sub-1")
	if f := s.frame(); strings.Contains(f, "rg-a") {
		t.Errorf("frame still shows the resource groups:\n%s", f)
	}
}

func TestSplitSharesTheCache(t *testing.T) {
	s := newScenario(t)
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
	s.want(t, "second pane", "Essentials", "app,linux")

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
	s := newScenario(t)
	s.settle(s.m.Init())
	s.want(t, "start", "Explorer", "▸ Dev")

	s.press("ctrl+w")
	s.press("h")
	s.press("l")
	s.want(t, "subscription expanded", "▾ Dev", "▸ rg-a")

	s.press("j")
	s.press("l")
	s.want(t, "group expanded", "▾ rg-a", "SQL servers 1", "Web apps 2")

	s.press("j")
	s.press("j")
	s.press("enter")
	s.want(t, "type opened", "Subscriptions › Dev › rg-a › Web apps [2]", "web-app", "web-api")
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
	s := newScenario(t)
	s.settle(s.m.Init())

	s.press(":")
	s.typ("vm")
	s.press("enter")
	s.want(t, ":vm", "Virtual machines", "vm-web", "vm-db", "vm-cache", "Dev", "Prod")

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

func TestCommandDialogArrowsOpenTheChosenView(t *testing.T) {
	s := newScenario(t)
	s.settle(s.m.Init())

	s.press(":")
	s.want(t, "dialog", "┌ Command ", "subscriptions", "resource groups in every subscription")
	s.press("down")
	s.press("down")
	s.press("enter")
	s.want(t, "second command", "Resource groups", "rg-a", "Dev")
	if f := s.frame(); strings.Contains(f, "┌ Command ") {
		t.Errorf("dialog stayed open:\n%s", f)
	}
}

func TestCommandDialogTypedCommandShowsTheViews(t *testing.T) {
	s := newScenario(t)
	s.settle(s.m.Init())

	s.press(":")
	s.typ("vm")
	s.press("enter")
	s.want(t, ":vm", "Virtual machines", "vm-web", "vm-db", "vm-cache")
}

func TestMouseSessionRoundTrip(t *testing.T) {
	s := newScenario(t)
	s.settle(s.m.Init())

	s.settle(s.harness.press(1, 1)) // marker of Dev
	s.want(t, "subscription expanded", "▾ Dev", "▸ rg-a")

	s.settle(s.harness.press(10, 2))
	s.settle(s.harness.press(10, 2)) // double press on rg-a
	s.want(t, "group opened", "Subscriptions › Dev › rg-a", "web-app", "sql-db")
	if s.m.sidebarFocused {
		t.Error("sidebar kept focus after the double press")
	}

	s.prefix("v")
	s.harness.press(32+54-1, 5) // boundary of the split
	s.motion(32+27, 5)
	s.release(32+27, 5)

	saved := s.m.Session()
	node := saved.Tabs[0].Layout
	if node.Split != "h" || node.Ratio != s.m.tabs[0].root.Ratio {
		t.Fatalf("saved layout = %q %v, want h %v", node.Split, node.Ratio, s.m.tabs[0].root.Ratio)
	}
	if r := s.m.tabs[0].root.Rects(s.m.paneArea()); r[s.m.tabs[0].root.Panes()[0]].W != 27 {
		t.Fatalf("dragged pane width = %d, want 27", r[s.m.tabs[0].root.Panes()[0]].W)
	}
	want := []string{"Subscriptions", "Dev", "rg-a"}
	for _, pane := range []*session.Node{node.A, node.B} {
		var got []string
		for _, sp := range pane.Stack {
			v, err := views.Build(sp)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, v.Title())
		}
		if !slices.Equal(got, want) {
			t.Errorf("saved stack titles = %v, want %v", got, want)
		}
	}

	restored, err := Restore(saved, views.NewSubscriptions, s.fetcher)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Session(), saved) {
		t.Errorf("restored session differs:\n got %+v\nwant %+v", restored.Session(), saved)
	}
}

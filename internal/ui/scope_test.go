package ui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/session"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

const aksType = "microsoft.containerservice/managedclusters"

var (
	keySubs   = data.Key{Kind: data.Subscriptions}
	keyAKS    = data.Key{Kind: data.AKSClusters}
	scopeDev  = views.ScopeMsg{ID: "id-1", Name: "Dev"}
	scopeAll  = views.ScopeMsg{}
	devProd   = []azure.Subscription{{ID: "id-1", Name: "Dev"}, {ID: "id-2", Name: "Prod"}}
	scopeProd = views.ScopeMsg{ID: "id-2", Name: "Prod"}
)

// scopedHarness is a harness whose subscriptions have loaded.
func scopedHarness(w, h int) *harness {
	hs := newHarness(w, h)
	hs.send(data.ResultMsg{Key: keySubs, Data: devProd})
	return hs
}

func (h *harness) scope() views.ScopeMsg { return h.m.tabs[h.m.active].scope }

func (h *harness) explorer() string { return sgr.ReplaceAllString(h.m.sidebar.Render(), "") }

func TestNumberKeysChooseTheSubscriptionOfTheTab(t *testing.T) {
	h := scopedHarness(100, 30)
	for _, c := range []struct {
		key  string
		want views.ScopeMsg
	}{{"2", scopeProd}, {"1", scopeDev}, {"0", scopeAll}, {"2", scopeProd}} {
		h.send(key(c.key))
		if got := h.scope(); got != c.want {
			t.Errorf("after %s the scope is %+v, want %+v", c.key, got, c.want)
		}
		scopes := h.pane(0).scopes
		if got := scopes[len(scopes)-1]; got != c.want {
			t.Errorf("after %s the view got %+v, want %+v", c.key, got, c.want)
		}
	}
	if got := h.pane(0).keys; len(got) != 0 {
		t.Errorf("the view got the keys %v", got)
	}
}

func TestNumberKeysWorkWithTheExplorerFocused(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.send(data.ResultMsg{Key: keySubs, Data: devProd})
	h.prefix("h")
	if !h.m.sidebarFocused {
		t.Fatal("the explorer has no focus")
	}
	h.send(key("2"))
	if got := h.scope(); got != scopeProd {
		t.Errorf("scope = %+v, want %+v", got, scopeProd)
	}
	if got := h.explorer(); !strings.Contains(got, "Prod") || strings.Contains(got, "Dev") {
		t.Errorf("explorer:\n%s", got)
	}
	h.send(key("0"))
	if got := h.explorer(); !strings.Contains(got, "Prod") || !strings.Contains(got, "Dev") {
		t.Errorf("explorer after 0:\n%s", got)
	}
	if !h.m.sidebarFocused {
		t.Error("the explorer lost focus")
	}
}

func TestNumberKeyWithoutSubscriptionShowsNotice(t *testing.T) {
	h := scopedHarness(100, 30)
	h.send(key("3"))
	if got := h.scope(); got != scopeAll {
		t.Errorf("scope = %+v, want all", got)
	}
	wantIn(t, "status", h.status(), "no subscription 3")
	if got := h.pane(0).scopes; len(got) != 0 {
		t.Errorf("the view got %v", got)
	}
	h.send(key("j"))
	if strings.Contains(h.status(), "no subscription") {
		t.Error("the notice stays after the next key")
	}
}

func TestNumberKeyWithoutLoadedSubscriptionsShowsNotice(t *testing.T) {
	h := newHarness(100, 30)
	h.send(key("1"))
	wantIn(t, "status", h.status(), "subscriptions are still loading")
	h.send(data.FetchMsg{Key: keySubs})
	h.send(key("1"))
	wantIn(t, "status", h.status(), "subscriptions are still loading")
	if got := h.scope(); got != scopeAll {
		t.Errorf("scope = %+v, want all", got)
	}
	h.send(key("0"))
	if strings.Contains(h.status(), "loading") {
		t.Errorf("0 shows a notice: %q", h.status())
	}
}

func TestNumberKeysStillTypeInInputs(t *testing.T) {
	t.Run("filter", func(t *testing.T) {
		h, rec := newFilterHarness(100, 30)
		h.send(data.ResultMsg{Key: keySubs, Data: devProd})
		h.send(key("/"))
		h.typ("012")
		if rec.filter != "012" || h.scope() != scopeAll {
			t.Errorf("filter %q, scope %+v", rec.filter, h.scope())
		}
	})
	t.Run("palette", func(t *testing.T) {
		h := scopedHarness(100, 30)
		h.send(key(":"))
		h.typ("12")
		if h.m.palette.text != "12" || h.scope() != scopeAll {
			t.Errorf("palette %q, scope %+v", h.m.palette.text, h.scope())
		}
	})
	t.Run("tab dialog", func(t *testing.T) {
		h := scopedHarness(100, 30)
		h.prefix(",")
		h.typ("12")
		if got := h.m.input.Value(); got != "12" || h.scope() != scopeAll {
			t.Errorf("field %q, scope %+v", got, h.scope())
		}
	})
}

func TestNumberKeysAfterTheTabPrefixStillSwitchTabs(t *testing.T) {
	h := scopedHarness(100, 30)
	h.prefix("t")
	h.prefix("1")
	h.prefix("2")
	if h.m.active != 1 || h.scope() != scopeAll {
		t.Errorf("active %d, scope %+v", h.m.active, h.scope())
	}
}

func TestEachTabHasItsSubscription(t *testing.T) {
	h := scopedHarness(100, 30)
	h.send(key("1"))
	h.prefix("t")
	if got := h.scope(); got != scopeAll {
		t.Errorf("a new tab has the scope %+v, want all", got)
	}
	h.send(key("2"))
	h.prefix("1")
	if got := h.scope(); got != scopeDev {
		t.Errorf("tab 1 has the scope %+v, want %+v", got, scopeDev)
	}
	if got := h.m.tabs[1].scope; got != scopeProd {
		t.Errorf("tab 2 has the scope %+v, want %+v", got, scopeProd)
	}
	if got := h.m.stacks[1][0].(fake).scopes; len(got) != 1 || got[0] != scopeProd {
		t.Errorf("the view of tab 2 got %v, want only %+v", got, scopeProd)
	}
	if got := h.m.stacks[0][0].(fake).scopes; len(got) != 1 || got[0] != scopeDev {
		t.Errorf("the view of tab 1 got %v, want only %+v", got, scopeDev)
	}
}

func TestNewTabViewStaysUnscopedWhileTheOldOneIs(t *testing.T) {
	h := scopedHarness(100, 30)
	h.send(key("2"))
	h.prefix("t")
	if got := h.pane(1).scopes; len(got) != 0 {
		t.Errorf("the new tab's view got %v", got)
	}
}

func TestExplorerFollowsTheTab(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.send(data.ResultMsg{Key: keySubs, Data: devProd})
	h.send(key("1"))
	h.prefix("t")
	if got := h.explorer(); !strings.Contains(got, "Prod") || !strings.Contains(got, "Dev") {
		t.Errorf("explorer of the new tab:\n%s", got)
	}
	h.prefix("1")
	if got := h.explorer(); !strings.Contains(got, "Dev") || strings.Contains(got, "Prod") {
		t.Errorf("explorer of tab 1:\n%s", got)
	}
	h.prefix("n")
	h.prefix("q") // close tab 2
	if got := h.explorer(); !strings.Contains(got, "Dev") || strings.Contains(got, "Prod") {
		t.Errorf("explorer after closing a tab:\n%s", got)
	}
}

func TestExplorerKeepsExpansionOfHiddenSubscriptions(t *testing.T) {
	h := newSidebarHarness(100, 30)
	h.send(data.ResultMsg{Key: keySubs, Data: devProd})
	h.prefix("h")
	h.send(key("l")) // expand Dev
	h.send(data.ResultMsg{Key: data.Key{Kind: data.ResourceGroups, Subscription: "id-1"}, Data: []azure.ResourceGroup{{Name: "rg-a"}}})
	want := h.explorer()
	h.send(key("2"))
	h.send(key("0"))
	if got := h.explorer(); got != want {
		t.Errorf("explorer:\n%s\nwant\n%s", got, want)
	}
}

func TestSplitSharesTheSubscription(t *testing.T) {
	h := scopedHarness(100, 30)
	h.send(key("2"))
	h.prefix("v")
	if got := h.pane(1).scopes; len(got) != 1 || got[0] != scopeProd {
		t.Errorf("the new pane's view has %v, want %+v", got, scopeProd)
	}
	h.send(key("1"))
	for id := range layout.PaneID(2) {
		scopes := h.pane(id).scopes
		if got := scopes[len(scopes)-1]; got != scopeDev {
			t.Errorf("pane %d got %+v last, want %+v", id, got, scopeDev)
		}
	}
	if got := h.scope(); got != scopeDev {
		t.Errorf("scope = %+v", got)
	}
}

func TestOpenedViewGetsTheSubscription(t *testing.T) {
	h := scopedHarness(100, 30)
	h.send(key("1"))
	h.send(key("enter"))
	if got := h.view(0, 1).scopes; len(got) != 1 || got[0] != scopeDev {
		t.Errorf("the opened view got %v, want %+v", got, scopeDev)
	}
}

func TestCommandViewGetsTheSubscription(t *testing.T) {
	h := scopedHarness(140, 30)
	h.send(key("2"))
	h.send(key(":"))
	h.typ("aks")
	h.send(key("enter"))
	if got, want := h.m.top(0).Title(), "AKS clusters (Prod)"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
	h.send(data.ResultMsg{Key: keyAKS, Data: []azure.Cluster{
		{Resource: azure.Resource{ID: "/a", Name: "aks-dev", SubscriptionID: "id-1", Subscription: "Dev"}},
		{Resource: azure.Resource{ID: "/b", Name: "aks-prod", SubscriptionID: "id-2", Subscription: "Prod"}},
		{Resource: azure.Resource{ID: "/c", Name: "aks-prod2", SubscriptionID: "id-2", Subscription: "Prod"}},
	}})
	f := h.frame()
	wantIn(t, "frame", f, "AKS clusters (Prod) [2]", "aks-prod")
	if strings.Contains(f, "aks-dev") {
		t.Errorf("frame shows a row of another subscription:\n%s", f)
	}
	h.send(key("0"))
	wantIn(t, "frame", h.frame(), "AKS clusters [3]", "aks-dev")
}

func TestTabBarShowsTheSubscriptionName(t *testing.T) {
	h := scopedHarness(100, 30)
	if bar := h.bar(); strings.Contains(bar, "Dev") || strings.Contains(bar, "Prod") {
		t.Errorf("bar of a tab with all subscriptions: %q", bar)
	}
	h.send(key("2"))
	bar := h.bar()
	col := strings.Index(bar, "Prod")
	if want := 100 - 1 - len("Prod"); col != want {
		t.Errorf("the name starts at column %d, want %d (one cell before the edge): %q", col, want, bar)
	}
	if !cellAt(t, h.rawBar(), col).flags["2"] {
		t.Errorf("the name is not faint: %q", h.rawBar())
	}
	h.prefix("t")
	if bar := h.bar(); strings.Contains(bar, "Prod") {
		t.Errorf("bar of the new tab: %q", bar)
	}
	h.prefix("1")
	wantIn(t, "bar", h.bar(), "Prod")
}

func TestTabBarDropsTheNameWithoutRoom(t *testing.T) {
	h := newHarness(20, 10)
	h.send(data.ResultMsg{Key: keySubs, Data: []azure.Subscription{{ID: "id-1", Name: "A very long subscription name"}}})
	h.send(key("1"))
	if bar := h.bar(); strings.Contains(bar, "A very") || lipgloss.Width(bar) > 20 {
		t.Errorf("bar = %q", bar)
	}
}

func TestSessionSavesTheSubscriptionOfEachTab(t *testing.T) {
	h := scopedHarness(100, 30)
	h.send(key("2"))
	h.prefix("t")
	saved := h.m.Session()
	if got := saved.Tabs[0]; got.Subscription != "id-2" || got.SubscriptionName != "Prod" {
		t.Errorf("tab 1 saved %q %q", got.Subscription, got.SubscriptionName)
	}
	if got := saved.Tabs[1]; got.Subscription != "" || got.SubscriptionName != "" {
		t.Errorf("tab 2 saved %q %q, want empty", got.Subscription, got.SubscriptionName)
	}
}

func TestRestoreKeepsTheSubscriptionOfEachTab(t *testing.T) {
	s := oneTab(leaf("subscriptions"))
	s.Tabs = append(s.Tabs, session.Tab{Layout: leaf("subscriptions")}, session.Tab{Subscription: "id-1", SubscriptionName: "Dev", Layout: leaf("subscriptions")})
	s.Tabs[0].Subscription, s.Tabs[0].SubscriptionName = "id-2", "Prod"
	m, err := Restore(s, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []views.ScopeMsg{scopeProd, scopeAll, scopeDev} {
		if got := m.tabs[i].scope; got != want {
			t.Errorf("tab %d restored with %+v, want %+v", i+1, got, want)
		}
	}
	if got := m.Session(); !reflect.DeepEqual(got.Tabs, s.Tabs) {
		t.Errorf("Session after Restore = %+v, want %+v", got.Tabs, s.Tabs)
	}
}

func TestRestoreDeliversTheSubscriptionToTheViews(t *testing.T) {
	aks := &session.Node{Stack: []views.Spec{{Kind: "resourcesByType", Type: aksType}}}
	s := oneTab(aks)
	s.Tabs[0].Subscription, s.Tabs[0].SubscriptionName = "id-2", "Prod"
	m, err := Restore(s, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.top(0).Title(), "AKS clusters (Prod)"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
}

func TestRestoreWithoutSubscriptionFieldsShowsAll(t *testing.T) {
	aks := &session.Node{Stack: []views.Spec{{Kind: "resourcesByType", Type: aksType}}}
	m, err := Restore(oneTab(aks), views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.tabs[0].scope; got != scopeAll {
		t.Errorf("scope = %+v", got)
	}
	if got, want := m.top(0).Title(), "AKS clusters"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
}

func TestRestoredExplorerShowsOnlyTheSubscription(t *testing.T) {
	s := oneTab(leaf("subscriptions"))
	s.Tabs[0].Subscription, s.Tabs[0].SubscriptionName = "id-1", "Dev"
	s.SidebarVisible = true
	m, err := Restore(s, views.NewSubscriptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{m: m}
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.send(data.ResultMsg{Key: keySubs, Data: devProd})
	if got := h.explorer(); !strings.Contains(got, "Dev") || strings.Contains(got, "Prod") {
		t.Errorf("explorer:\n%s", got)
	}
}

func TestNumberKeyAfterTheSubscriptionsFailedShowsNotice(t *testing.T) {
	h := newHarness(100, 30)
	h.send(data.FetchMsg{Key: keySubs})
	h.send(data.ResultMsg{Key: keySubs, Err: errors.New("boom")})
	h.send(key("1"))
	wantIn(t, "status", h.status(), "subscriptions failed to load")
	if strings.Contains(h.status(), "still loading") {
		t.Errorf("status = %q", h.status())
	}
}

func TestScopeReachesViewsBelowTheTop(t *testing.T) {
	h := scopedHarness(140, 30)
	h.send(key(":"))
	h.typ("aks")
	h.send(key("enter"))
	h.send(data.ResultMsg{Key: keyAKS, Data: []azure.Cluster{
		{Resource: azure.Resource{ID: "/a", Name: "aks-dev", SubscriptionID: "id-1", Subscription: "Dev"}},
		{Resource: azure.Resource{ID: "/b", Name: "aks-prod", SubscriptionID: "id-2", Subscription: "Prod"}},
	}})
	h.send(key("enter"))
	h.send(key("2"))
	h.send(key("esc"))
	if got, want := h.m.top(0).Title(), "AKS clusters (Prod)"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
	if f := h.frame(); strings.Contains(f, "aks-dev") {
		t.Errorf("frame shows a row of another subscription:\n%s", f)
	}
}

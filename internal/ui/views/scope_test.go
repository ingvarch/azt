package views

import (
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
)

var (
	scopeDev  = ScopeMsg{ID: "id-1", Name: "Dev"}
	scopeProd = ScopeMsg{ID: "id-2", Name: "Prod"}
)

func scoped(t *testing.T, v View, msg ScopeMsg) View {
	t.Helper()
	v, cmd := v.Update(msg)
	if cmd != nil {
		t.Errorf("ScopeMsg returned a command")
	}
	return v
}

func count(t *testing.T, v View) int {
	t.Helper()
	n, ok := v.(Counter).Count()
	if !ok {
		t.Fatal("no count")
	}
	return n
}

// acrossSubscriptions are the lists over rows of every subscription, with the data that fills them.
var acrossSubscriptions = []struct {
	name   string
	new    func() View
	data   any
	dev    string // a name only in the Dev row
	prod   string
	title  string
	scoped string
}{
	{"resources by type", func() View { return NewResourcesByType("microsoft.web/sites") }, tenantResources, "web-app", "api-app", "Web apps", "Web apps (Dev)"},
	{"all resource groups", NewAllResourceGroups, allGroups, "rg-a", "rg-b", "Resource groups", "Resource groups (Dev)"},
	{"virtual machines", NewVirtualMachines, []azure.VM{
		{Resource: azure.Resource{ID: "/vm-dev", Name: "vm-dev", SubscriptionID: "id-1", Subscription: "Dev"}},
		{Resource: azure.Resource{ID: "/vm-prod", Name: "vm-prod", SubscriptionID: "id-2", Subscription: "Prod"}},
	}, "vm-dev", "vm-prod", "Virtual machines", "Virtual machines (Dev)"},
	{"AKS clusters", NewAKSClusters, []azure.Cluster{
		{Resource: azure.Resource{ID: "/aks-dev", Name: "aks-dev", SubscriptionID: "id-1", Subscription: "Dev"}},
		{Resource: azure.Resource{ID: "/aks-prod", Name: "aks-prod", SubscriptionID: "id-2", Subscription: "Prod"}},
	}, "aks-dev", "aks-prod", "AKS clusters", "AKS clusters (Dev)"},
}

func TestScopeShowsOnlyRowsOfTheSubscription(t *testing.T) {
	for _, c := range acrossSubscriptions {
		t.Run(c.name, func(t *testing.T) {
			v := scoped(t, deliver(t, c.new().Resize(140, 20), c.data, nil), scopeDev)
			got := v.Render()
			if !strings.Contains(got, c.dev) || strings.Contains(got, c.prod) {
				t.Errorf("render:\n%s", got)
			}
			if v.Title() != c.scoped {
				t.Errorf("Title = %q, want %q", v.Title(), c.scoped)
			}
			if n := count(t, v); n != 1 {
				t.Errorf("Count = %d, want 1", n)
			}
		})
	}
}

func TestScopeBeforeDataFiltersTheResult(t *testing.T) {
	for _, c := range acrossSubscriptions {
		t.Run(c.name, func(t *testing.T) {
			v := deliver(t, scoped(t, c.new().Resize(140, 20), scopeProd), c.data, nil)
			if got := v.Render(); !strings.Contains(got, c.prod) || strings.Contains(got, c.dev) {
				t.Errorf("render:\n%s", got)
			}
			if n := count(t, v); n != 1 {
				t.Errorf("Count = %d, want 1", n)
			}
		})
	}
}

func TestScopeSwitchesAndClears(t *testing.T) {
	for _, c := range acrossSubscriptions {
		t.Run(c.name, func(t *testing.T) {
			v := deliver(t, c.new().Resize(140, 20), c.data, nil)
			v = scoped(t, scoped(t, v, scopeDev), scopeProd)
			if got := v.Render(); !strings.Contains(got, c.prod) || strings.Contains(got, c.dev) {
				t.Errorf("render after switching:\n%s", got)
			}
			v = scoped(t, v, ScopeMsg{})
			if got := v.Render(); !strings.Contains(got, c.prod) || !strings.Contains(got, c.dev) {
				t.Errorf("render after clearing:\n%s", got)
			}
			if v.Title() != c.title {
				t.Errorf("Title = %q, want %q", v.Title(), c.title)
			}
			if n := count(t, v); n != 2 {
				t.Errorf("Count = %d, want 2", n)
			}
		})
	}
}

func TestScopeKeepsTheFilterOnTop(t *testing.T) {
	rows := []azure.Resource{
		{ID: "/1", Name: "web-dev", Type: "microsoft.web/sites", SubscriptionID: "id-1", Subscription: "Dev"},
		{ID: "/2", Name: "api-dev", Type: "microsoft.web/sites", SubscriptionID: "id-1", Subscription: "Dev"},
		{ID: "/3", Name: "web-prod", Type: "microsoft.web/sites", SubscriptionID: "id-2", Subscription: "Prod"},
	}
	v := deliver(t, NewResourcesByType("microsoft.web/sites").Resize(140, 20), rows, nil)
	v = filtered(scoped(t, v, scopeDev), "web")
	got := v.Render()
	if !strings.Contains(got, "web-dev") || strings.Contains(got, "api-dev") || strings.Contains(got, "web-prod") {
		t.Errorf("render:\n%s", got)
	}
	if !strings.Contains(got, "1 of 2") {
		t.Errorf("info line lacks %q:\n%s", "1 of 2", got)
	}
	if n := count(t, v); n != 2 {
		t.Errorf("Count = %d, want 2 (after the subscription, before the filter)", n)
	}
}

func TestScopeOpensTheRowOfTheSubscription(t *testing.T) {
	v := deliver(t, NewResourcesByType("microsoft.web/sites").Resize(140, 20), tenantResources, nil)
	next, ok := scoped(t, v, scopeProd).Open()
	if !ok {
		t.Fatal("Open found nothing")
	}
	if got := next.Spec().Subscription; got != "id-2" {
		t.Errorf("opened subscription %q, want id-2", got)
	}
}

func TestScopeIgnoredByListsInsideOneSubscription(t *testing.T) {
	for _, c := range []struct {
		name string
		view View
		data any
	}{
		{"subscriptions", NewSubscriptions(), subs},
		{"resource groups", NewResourceGroups(subs[0]), groups},
		{"resources", NewResources("id-1", groups[1]), resources},
		{"resources of type", NewResourcesOfType("id-1", groups[1], "microsoft.web/sites"), resources},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := deliver(t, c.view.Resize(140, 20), c.data, nil)
			before, title, n := v.Render(), v.Title(), count(t, v)
			v = scoped(t, v, scopeProd)
			if v.Render() != before || v.Title() != title || count(t, v) != n {
				t.Errorf("the view changed:\n%s\nwas\n%s", v.Render(), before)
			}
		})
	}
}

func TestSubscriptionsNumberTheFirstNine(t *testing.T) {
	var many []azure.Subscription
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		many = append(many, azure.Subscription{ID: "id-" + name, Name: "sub-" + name})
	}
	lines := strings.Split(plain(deliver(t, NewSubscriptions().Resize(100, 20), many, nil).Render()), "\n")
	if got := strings.Fields(lines[0])[0]; got != "#" {
		t.Errorf("first header cell = %q, want #:\n%s", got, strings.Join(lines, "\n"))
	}
	for i, name := range []string{"sub-a", "sub-b", "sub-c", "sub-d", "sub-e", "sub-f", "sub-g", "sub-h", "sub-i", "sub-j"} {
		cells := strings.Fields(lines[i+2])
		want := []string{name}
		if i < 9 {
			want = []string{string(rune('1' + i)), name}
		}
		if cells[0] != want[0] || cells[len(want)-1] != name {
			t.Errorf("row %d starts with %q, want %q", i, cells[:len(want)], want)
		}
	}
}

func TestSubscriptionNumbersSurviveTheFilter(t *testing.T) {
	v := filtered(deliver(t, NewSubscriptions().Resize(100, 20), subs, nil), "prod")
	rows := strings.Split(plain(v.Render()), "\n")
	var row string
	for _, l := range rows {
		if strings.Contains(l, "Prod") {
			row = l
		}
	}
	if f := strings.Fields(row); len(f) == 0 || f[0] != "2" {
		t.Errorf("filtered row = %q, want it to start with 2:\n%s", row, v.Render())
	}
}

func TestFilterSkipsTheSubscriptionNumber(t *testing.T) {
	plainSubs := []azure.Subscription{{ID: "aa", Name: "Dev", State: "Enabled"}, {ID: "bb", Name: "Prod", State: "Enabled"}}
	v := filtered(deliver(t, NewSubscriptions().Resize(100, 20), plainSubs, nil), "2")
	if got := plain(v.Render()); !strings.Contains(got, "No matches.") {
		t.Errorf("a filter on a row number matched:\n%s", got)
	}
}

func TestScopeSelectsTheFirstRowOfTheSubscription(t *testing.T) {
	var rows []azure.Resource
	for _, n := range []string{"a-dev", "b-prod", "c-dev", "d-prod", "e-dev"} {
		sub := "id-1"
		if strings.HasSuffix(n, "prod") {
			sub = "id-2"
		}
		rows = append(rows, azure.Resource{ID: "/" + n, Name: n, Type: "microsoft.web/sites", SubscriptionID: sub})
	}
	v := deliver(t, NewResourcesByType("microsoft.web/sites").Resize(140, 20), rows, nil)
	for range 3 {
		v, _ = press(v, key("j"))
	}
	v = scoped(t, v, scopeProd)
	if next, ok := v.Open(); !ok || next.Title() != "b-prod" {
		t.Errorf("selected after the scope = %v, want b-prod", next)
	}
}

func TestScopeMatchesTheSubscriptionIDIgnoringCase(t *testing.T) {
	rows := []azure.Resource{
		{ID: "/1", Name: "web-dev", Type: "microsoft.web/sites", SubscriptionID: "ID-1", Subscription: "Dev"},
		{ID: "/2", Name: "web-prod", Type: "microsoft.web/sites", SubscriptionID: "id-2", Subscription: "Prod"},
	}
	v := deliver(t, NewResourcesByType("microsoft.web/sites").Resize(140, 20), rows, nil)
	v = scoped(t, v, scopeDev)
	if got := v.Render(); !strings.Contains(got, "web-dev") || strings.Contains(got, "web-prod") {
		t.Errorf("render:\n%s", got)
	}
}

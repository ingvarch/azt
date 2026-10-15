package sidebar

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

const aksType = "microsoft.containerservice/managedclusters"

// resAKS has three clusters (one with another type case), unsorted, and one server.
var resAKS = []azure.Resource{
	{ID: "/aks-b", Name: "Beta", Type: aksType},
	{ID: "/aks-a", Name: "alpha", Type: aksType},
	{ID: "/sql-1", Name: "db1", Type: "microsoft.sql/servers"},
	{ID: "/aks-c", Name: "Gamma", Type: "Microsoft.ContainerService/managedClusters"},
}

// aksModel has the selection on the AKS type node, collapsed.
func aksModel() Model { return press(explorer(resAKS), keyDown, keyDown) }

func TestTypeExpandsWithoutFetchAndSortsResources(t *testing.T) {
	m, cmd := send(aksModel(), key('l'))
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("expanding a type asks for %v, want nothing", got)
	}
	assertLines(t, m,
		"▾ Dev",
		"  ▾ rg-a",
		"    ▾ containerservice/managedclusters 3",
		"      ▸ alpha",
		"      ▸ Beta",
		"      ▸ Gamma",
		"    ▸ sql/servers 1",
		"  ▸ rg-b",
		"▸ Prod",
	)
}

func TestResourceExpandsIntoTabs(t *testing.T) {
	m := press(aksModel(), key('l'), keyDown, key('l'))
	assertLines(t, m,
		"▾ Dev",
		"  ▾ rg-a",
		"    ▾ containerservice/managedclusters 3",
		"      ▾ alpha",
		"          Overview",
		"          Node pools",
		"          Networking",
		"          Identity",
		"          Add-ons",
		"          JSON",
		"      ▸ Beta",
		"      ▸ Gamma",
		"    ▸ sql/servers 1",
		"  ▸ rg-b",
		"▸ Prod",
	)

	m = press(aksModel(), keyDown, key('l'), keyDown, key('l'))
	assertLines(t, m,
		"▾ Dev",
		"  ▾ rg-a",
		"    ▸ containerservice/managedclusters 3",
		"    ▾ sql/servers 1",
		"      ▾ db1",
		"          Overview",
		"          JSON",
		"  ▸ rg-b",
		"▸ Prod",
	)
}

func TestStackOfResourceAndTab(t *testing.T) {
	m := press(aksModel(), key('l'), keyDown)
	keyDetails := data.Key{Kind: data.Details, Subscription: "id-1", ID: "/aks-a"}
	head := []string{"Subscriptions", "Dev", "rg-a", "containerservice/managedclusters", "alpha"}
	keys := []data.Key{keySubs, keyGroups, keyRes, keyRes, keyDetails}

	check := func(name string, m Model, titles []string) {
		t.Helper()
		stack, ok := m.Stack()
		if !ok || len(stack) != len(titles) {
			t.Fatalf("%s: Stack() = %d views, %v; want %d", name, len(stack), ok, len(titles))
		}
		for i, v := range stack {
			if got := v.Title(); got != titles[i] {
				t.Errorf("%s: view %d title = %q, want %q", name, i, got, titles[i])
			}
			if got := fetchKey(t, v); got != keys[i] {
				t.Errorf("%s: view %d key = %+v, want %+v", name, i, got, keys[i])
			}
		}
	}
	check("resource", m, head)

	m = press(m, key('l'), keyDown, keyDown, keyDown)
	check("tab", m, head)
	stack, _ := m.Stack()
	top, _ := stack[len(stack)-1].Resize(60, 8).Update(data.ResultMsg{Key: keyDetails, Data: map[string]any{
		"type": aksType, "properties": map[string]any{"networkProfile": map[string]any{"networkPlugin": "azure"}},
	}})
	if out := top.Render(); !strings.Contains(out, "Network plugin") {
		t.Errorf("the Networking tab did not open:\n%s", out)
	}

	stack, _ = press(aksModel(), key('l'), keyDown).Stack()
	top, _ = stack[len(stack)-1].Resize(60, 8).Update(data.ResultMsg{Key: keyDetails, Data: map[string]any{"type": aksType}})
	if out := top.Render(); strings.Contains(out, "Network plugin") {
		t.Errorf("the resource did not open on its first tab:\n%s", out)
	}
}

func TestResourceStateSurvivesNewResult(t *testing.T) {
	m := press(aksModel(), key('l'), keyDown, key('l'), keyDown, keyDown, keyDown) // on alpha / Networking
	before := lines(m)
	if got := selectedTitle(t, m); got != "alpha" {
		t.Fatalf("selected %q, want alpha", got)
	}

	m = result(m, keyRes, slices.Clone(resAKS), nil)
	if got := strings.Join(lines(m), "\n"); got != strings.Join(before, "\n") {
		t.Errorf("rows changed:\n%s\nwant:\n%s", got, strings.Join(before, "\n"))
	}
	if got := lines(m)[6]; got != "          Networking" {
		t.Fatalf("row 6 = %q", got)
	}
	stack, _ := m.Stack()
	if len(stack) != 5 || stack[4].Spec().Tab != "Networking" {
		t.Fatalf("stack has %d views, top tab %q; want 5 and Networking", len(stack), stack[len(stack)-1].Spec().Tab)
	}

	renamed := slices.Clone(resAKS)
	renamed[1].Name = "alpha2"
	m = result(m, keyRes, renamed, nil)
	assertRow(t, m, 3, "      ▾ alpha2")
	if got := selectedTitle(t, m); got != "alpha2" {
		t.Errorf("after a rename selected %q, want alpha2", got)
	}
	if stack, _ := m.Stack(); stack[len(stack)-1].Spec().Tab != "Networking" {
		t.Errorf("after a rename the selection left the Networking tab")
	}

	m = result(m, keyRes, resAKS[2:3], nil)
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Errorf("after the cluster is gone selected %q, want rg-a", got)
	}
}

func assertRow(t *testing.T, m Model, i int, want string) {
	t.Helper()
	if got := lines(m); i >= len(got) || got[i] != want {
		t.Errorf("rows:\n%s\nrow %d want %q", strings.Join(got, "\n"), i, want)
	}
}

func TestKeysOnResourceLevels(t *testing.T) {
	m := press(aksModel(), key('l'), keyDown, key('l'), keyDown) // on alpha / Overview
	m = press(m, keyRight)
	assertRow(t, m, 4, "          Overview")

	m = press(m, keyLeft)
	if got := lines(m); len(got) != 15 {
		t.Errorf("h on a tab changed the rows: %v", got)
	}
	assertRow(t, m, 3, "      ▾ alpha")

	m = press(m, keyLeft)
	assertRow(t, m, 3, "      ▸ alpha")
	m = press(m, keyLeft)
	assertRow(t, m, 2, "    ▾ containerservice/managedclusters 3")
	m = press(m, keyLeft)
	assertRow(t, m, 2, "    ▸ containerservice/managedclusters 3")
}

func TestLeftOnTabMovesToResource(t *testing.T) {
	m := press(aksModel(), key('l'), keyDown, key('l'), keyDown, keyDown, key('h'))
	assertRow(t, m, 3, "      ▾ alpha")
	stack, _ := m.Stack()
	if got := stack[len(stack)-1].Title(); got != "alpha" || len(stack) != 5 {
		t.Errorf("selected %q with %d views, want the alpha resource", got, len(stack))
	}
}

func TestClickMarkersOnResourceLevels(t *testing.T) {
	m, cmd := click(aksModel(), 4, 2) // type marker
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("type marker asks for %v", got)
	}
	assertRow(t, m, 2, "    ▾ containerservice/managedclusters 3")

	m, cmd = click(m, 7, 3) // resource marker, second cell
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("resource marker asks for %v", got)
	}
	assertRow(t, m, 3, "      ▾ alpha")
	assertRow(t, m, 4, "          Overview")

	m, _ = click(m, 8, 5) // a tab: only selects
	assertRow(t, m, 3, "      ▾ alpha")
	m, _ = click(m, 9, 5)
	if got := len(lines(m)); got != 15 {
		t.Errorf("clicking a tab changed the rows (%d)", got)
	}

	m, _ = click(m, 6, 3)
	assertRow(t, m, 3, "      ▸ alpha")
}

func TestRefreshOnResourceLevels(t *testing.T) {
	for name, m := range map[string]Model{
		"type":     aksModel(),
		"resource": press(aksModel(), key('l'), keyDown),
		"tab":      press(aksModel(), key('l'), keyDown, key('l'), keyDown),
	} {
		_, cmd := send(m, key('r'))
		got := fetches(cmd)
		if len(got) != 1 || got[0] != keyRes {
			t.Errorf("%s: r asks for %v, want the group's resources", name, got)
		}
	}
}

func TestTabsOfTwoResourcesHaveOwnNodes(t *testing.T) {
	downs := []tea.KeyPressMsg{keyDown, keyDown, keyDown, keyDown, keyDown, keyDown, keyDown}
	m := press(aksModel(), key('l'), keyDown, key('l')) // alpha expanded
	m = press(m, downs...)                              // on Beta
	m = press(m, key('l'), keyDown)                     // Beta / Overview
	for i, wantTab := range []string{"", "Node pools", "Networking"} {
		if i > 0 {
			m = press(m, keyDown)
		}
		stack, _ := m.Stack()
		top := stack[len(stack)-1]
		if got := fetchKey(t, top); top.Title() != "Beta" || got.ID != "/aks-b" {
			t.Errorf("step %d: selected %q with key %+v, want Beta /aks-b", i, top.Title(), got)
		}
		if got := top.Spec().Tab; got != wantTab {
			t.Errorf("step %d: tab = %q, want %q", i, got, wantTab)
		}
	}
}

func TestTiedResourcesKeepAStableOrder(t *testing.T) {
	parent := &node{id: "g/s/rg"}
	typ := "microsoft.sql/servers/databases"
	base := []azure.Resource{
		{ID: "/srv2/master", Name: "master", Type: typ},
		{ID: "/srv1/master", Name: "master", Type: typ},
		{ID: "/x/beta", Name: "beta", Type: typ},
		{ID: "/x/Beta", Name: "Beta", Type: typ},
	}
	want := []string{"r//x/Beta", "r//x/beta", "r//srv1/master", "r//srv2/master"}
	for i := range base {
		perm := append(slices.Clone(base[i:]), base[:i]...)
		var got []string
		for _, c := range typeNodes(parent, perm)[0].children {
			got = append(got, c.id)
		}
		if !slices.Equal(got, want) {
			t.Errorf("rotation %d: order %v, want %v", i, got, want)
		}
	}
}

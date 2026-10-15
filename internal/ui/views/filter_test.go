package views

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

func filtered(v View, query string) View { return v.(Filterer).SetFilter(query) }

func firstLine(v View) string { return strings.Split(v.Render(), "\n")[0] }

func newResourcesView() View { return NewResources("id-1", groups[1]) }

func TestFilterMatching(t *testing.T) {
	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"web-app", "kv-prod"}},
		{"web", []string{"web-app"}},
		{"WEB", []string{"web-app"}},
		{"Kv-PROD", []string{"kv-prod"}},
		{"northeurope", []string{"kv-prod"}},
		{"keyvault/vaults", []string{"kv-prod"}},
		{"e", []string{"web-app", "kv-prod"}},
		{"zzz", nil},
	}
	for _, tt := range tests {
		got := filtered(deliver(t, newResourcesView().Resize(120, 20), resources, nil), tt.query).Render()
		for _, name := range []string{"web-app", "kv-prod"} {
			want := false
			for _, w := range tt.want {
				want = want || w == name
			}
			if has := strings.Contains(got, name); has != want {
				t.Errorf("filter %q: shows %s = %v, want %v:\n%s", tt.query, name, has, want, got)
			}
		}
	}
}

func TestFilterAppliesAfterKeep(t *testing.T) {
	all := []azure.Resource{
		{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites"},
		{ID: "/r/kv", Name: "kv-app", Type: "microsoft.keyvault/vaults"},
		{ID: "/r/api", Name: "api-app", Type: "microsoft.web/sites"},
	}
	newView := func() View { return NewResourcesOfType("id-1", groups[1], "microsoft.web/sites") }

	v := filtered(deliver(t, newView().Resize(120, 20), all, nil), "app")
	if got := firstLine(v); !strings.Contains(got, "/app  2 of 2") {
		t.Errorf("info line = %q, want 2 of 2 (the kept items only)", got)
	}
	if got := v.Render(); strings.Contains(got, "kv-app") {
		t.Errorf("render shows an item dropped by keep:\n%s", got)
	}

	v = filtered(deliver(t, newView().Resize(120, 20), all, nil), "kv")
	if got := v.Render(); !strings.Contains(got, "0 of 2") || !strings.Contains(got, "No matches.") {
		t.Errorf("render:\n%s", got)
	}
}

func TestFilterInfoLine(t *testing.T) {
	v := deliver(t, newResourcesView().Resize(120, 20), resources, nil)
	if got := v.Render(); strings.Contains(got, " of ") || !strings.Contains(firstLine(v), "Name") {
		t.Errorf("render without a filter has an info line:\n%s", got)
	}

	v = filtered(v, "web")
	if got := firstLine(v); !strings.Contains(got, "/web  1 of 2") {
		t.Errorf("info line = %q, want it to contain %q", got, "/web  1 of 2")
	}
	if raw := firstLine(v); !strings.Contains(raw, "\x1b[2m") {
		t.Errorf("info line is not faint: %q", raw)
	}
	if got := v.Render(); !strings.Contains(got, "Name") || !strings.Contains(got, "web-app") {
		t.Errorf("table is missing below the info line:\n%s", got)
	}

	v = filtered(v, "")
	if got := firstLine(v); !strings.Contains(got, "Name") {
		t.Errorf("info line stays after the filter is cleared: %q", got)
	}
}

func TestFilterNoMatches(t *testing.T) {
	v := filtered(deliver(t, newResourcesView().Resize(120, 20), resources, nil), "zzz")
	got := v.Render()
	if !strings.Contains(got, "No matches.") || strings.Contains(got, "No resources in this resource group.") {
		t.Errorf("render:\n%s", got)
	}
	if !strings.Contains(firstLine(v), "/zzz  0 of 2") {
		t.Errorf("info line = %q", firstLine(v))
	}
	if next, ok := v.Open(); ok || next != nil {
		t.Errorf("Open = %v, %v; want nil, false", next, ok)
	}

	v = filtered(deliver(t, newResourcesView().Resize(120, 20), nil, nil), "zzz")
	got = v.Render()
	if !strings.Contains(got, "No resources in this resource group.") || strings.Contains(got, "No matches.") {
		t.Errorf("empty list with a filter:\n%s", got)
	}
}

func TestFilterChangeMovesCursorToFirstRow(t *testing.T) {
	v := deliver(t, NewSubscriptions().Resize(120, 20), subs, nil)
	v, _ = press(v, keyDown)
	v = filtered(v, "e")
	next, ok := v.Open()
	if !ok || next.Title() != "Dev" {
		t.Errorf("after a filter change opened %v, want Dev", next)
	}

	v, _ = press(v, keyDown)
	v = filtered(v, "e")
	if next, _ := v.Open(); next.Title() != "Prod" {
		t.Errorf("same query moved the cursor: opened %q, want Prod", next.Title())
	}
}

func TestFilterAppliesToLaterResults(t *testing.T) {
	v := filtered(newResourcesView().Resize(120, 20), "kv")
	v = deliver(t, v, resources, nil)
	got := v.Render()
	if strings.Contains(got, "web-app") || !strings.Contains(got, "kv-prod") {
		t.Errorf("render:\n%s", got)
	}
}

func TestOpenOnFilteredList(t *testing.T) {
	v := filtered(deliver(t, newResourcesView().Resize(120, 20), resources, nil), "kv")
	next, ok := v.Open()
	if !ok || next.Title() != "kv-prod" {
		t.Fatalf("Open = %v, %v; want kv-prod", next, ok)
	}
	if got, want := fetchKey(t, next), (data.Key{Kind: data.Details, Subscription: "id-1", ID: "/r/kv"}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}

	v, _ = press(v, keyDown)
	if next, _ := v.Open(); next.Title() != "kv-prod" {
		t.Errorf("cursor left the only matching row: opened %q", next.Title())
	}
}

func TestFilterSurvivesResizeAndCopy(t *testing.T) {
	v := filtered(deliver(t, newResourcesView().Resize(120, 20), resources, nil), "web")
	resized := v.Resize(60, 10)
	if got := resized.(Filterer).Filter(); got != "web" {
		t.Errorf("Filter() after Resize = %q, want web", got)
	}
	if got := resized.Render(); strings.Contains(got, "kv-prod") {
		t.Errorf("resized view lost its filter:\n%s", got)
	}

	other := filtered(v, "kv")
	if got := v.(Filterer).Filter(); got != "web" {
		t.Errorf("SetFilter changed the original: Filter() = %q", got)
	}
	if got := other.(Filterer).Filter(); got != "kv" {
		t.Errorf("Filter() = %q, want kv", got)
	}
}

func TestListsAreFiltererDetailsIsNot(t *testing.T) {
	for _, l := range lists {
		if _, ok := l.new().(Filterer); !ok {
			t.Errorf("%s does not implement Filterer", l.name)
		}
	}
	if _, ok := newDetails().(Filterer); ok {
		t.Error("details implements Filterer")
	}
}

func TestFilterKeepsCursorRowVisible(t *testing.T) {
	many := make([]azure.Subscription, 50)
	for i := range many {
		many[i] = azure.Subscription{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("sub-%d", i), State: "Enabled"}
	}
	v := filtered(deliver(t, NewSubscriptions().Resize(80, 6), many, nil), "sub-")
	for range 10 {
		v, _ = press(v, keyDown)
	}
	if got := v.Render(); !strings.Contains(got, "sub-10") {
		t.Errorf("cursor row is scrolled out of view:\n%s", got)
	}
}

func TestRefreshingShowsUntilResult(t *testing.T) {
	views := map[string]View{
		"list":    deliver(t, newResourcesView().Resize(120, 20), resources, nil),
		"details": deliver(t, newDetails().Resize(80, 10), detail, nil),
	}
	for name, v := range views {
		if strings.Contains(v.Render(), "refreshing…") {
			t.Errorf("%s: refreshing before r", name)
		}
		v, _ = press(v, keyR)
		// The details view shows it under the tab bar and its rule.
		line := map[string]int{"list": 0, "details": 2}[name]
		if got := strings.Split(v.Render(), "\n")[line]; !strings.Contains(got, "refreshing…") {
			t.Errorf("%s: line %d after r = %q, want refreshing…", name, line, got)
		}
		if got := sendForeign(v, nil).Render(); !strings.Contains(got, "refreshing…") {
			t.Errorf("%s: result of another key cleared refreshing", name)
		}
		v = deliver(t, v, map[bool]any{true: resources, false: detail}[name == "list"], nil)
		if got := v.Render(); strings.Contains(got, "refreshing…") {
			t.Errorf("%s: refreshing stays after the result:\n%s", name, got)
		}
	}
}

func TestRefreshingClearsOnError(t *testing.T) {
	v, _ := press(deliver(t, newResourcesView().Resize(120, 20), resources, nil), keyR)
	v = deliver(t, v, nil, fmt.Errorf("boom"))
	v = deliver(t, v, resources, nil)
	if got := v.Render(); strings.Contains(got, "refreshing…") {
		t.Errorf("refreshing stays after a result:\n%s", got)
	}
}

func TestRefreshingJoinsFilterInfo(t *testing.T) {
	v := deliver(t, newResourcesView().Resize(120, 20), resources, nil)
	v, _ = press(filtered(v, "web"), keyR)
	if got := firstLine(v); !strings.Contains(got, "/web  1 of 2  refreshing…") {
		t.Errorf("info line = %q", got)
	}
}

func TestRefreshingTakesOneLine(t *testing.T) {
	big := map[string]any{}
	for i := range 40 {
		big[fmt.Sprintf("k%02d", i)] = i
	}
	v := deliver(t, newDetailsAt("JSON").Resize(80, 6), big, nil)
	v, _ = press(v, keyR)
	if got := strings.Count(v.Render(), "\n") + 1; got != 6 {
		t.Errorf("height = %d, want 6", got)
	}
	for range 50 {
		v, _ = press(v, keyDown)
	}
	lines := strings.Split(v.Render(), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "}" {
		t.Errorf("last line at the end = %q, want }", last)
	}
}

func TestRefreshKeepsSizeOfList(t *testing.T) {
	many := make([]azure.Subscription, 50)
	for i := range many {
		many[i] = azure.Subscription{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("sub-%d", i)}
	}
	v := deliver(t, NewSubscriptions().Resize(80, 6), many, nil)
	v, _ = press(v, keyR)
	for range 10 {
		v, _ = press(v, keyDown)
	}
	if got := v.Render(); !strings.Contains(got, "sub-10") {
		t.Errorf("cursor row is not shown while refreshing:\n%s", got)
	}
	v = deliver(t, v, many, nil)
	if got := strings.Count(v.Render(), "\n") + 1; got != 6 {
		t.Errorf("height after the result = %d, want 6", got)
	}
}

func TestRefreshResultRestoresSizeOfDetails(t *testing.T) {
	big := map[string]any{}
	for i := range 50 {
		big[fmt.Sprintf("k%02d", i)] = i
	}
	v := deliver(t, newDetailsAt("JSON").Resize(80, 6), big, nil)
	v, _ = press(v, keyR)
	v = deliver(t, v, big, nil)
	if got := strings.Count(v.Render(), "\n") + 1; got != 6 {
		t.Errorf("height after the result = %d, want 6", got)
	}
}

func TestAllResourceGroupsOpen(t *testing.T) {
	v := deliver(t, NewAllResourceGroups().Resize(120, 20), allGroups, nil)
	v, _ = press(v, keyDown)
	next, ok := v.Open()
	if !ok {
		t.Fatal("Open is false")
	}
	if got := next.Title(); got != "rg-b" {
		t.Errorf("title = %q, want rg-b", got)
	}
	if got, want := fetchKey(t, next), (data.Key{Kind: data.Resources, Subscription: "id-2", Group: "rg-b"}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
}

func TestResourcesByTypeOpen(t *testing.T) {
	v := deliver(t, NewResourcesByType("microsoft.web/sites").Resize(120, 20), tenantResources, nil)
	v, _ = press(v, keyDown)
	next, ok := v.Open()
	if !ok {
		t.Fatal("Open is false")
	}
	if got := next.Title(); got != "api-app" {
		t.Errorf("title = %q, want api-app", got)
	}
	if got, want := fetchKey(t, next), (data.Key{Kind: data.Details, Subscription: "id-2", ID: "/r/api"}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
}

func TestFilterIgnoresCaseOfCells(t *testing.T) {
	v := filtered(deliver(t, NewSubscriptions().Resize(120, 20), subs, nil), "dEV")
	got := v.Render()
	if !strings.Contains(got, "Dev") || strings.Contains(got, "Prod") {
		t.Errorf("render:\n%s", got)
	}
}

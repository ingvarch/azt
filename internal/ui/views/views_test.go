package views

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

// collect runs cmd, unwrapping batches, and returns the messages it produces.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, collect(c)...)
	}
	return msgs
}

// fetchKey returns the key v asks for in Init.
func fetchKey(t *testing.T, v View) data.Key {
	t.Helper()
	for _, msg := range collect(v.Init()) {
		if f, ok := msg.(data.FetchMsg); ok {
			return f.Key
		}
	}
	t.Fatalf("%s: Init returned no FetchMsg", v.Title())
	return data.Key{}
}

// deliver sends v the result for its own key.
func deliver(t *testing.T, v View, d any, err error) View {
	t.Helper()
	v, _ = v.Update(data.ResultMsg{Key: fetchKey(t, v), Data: d, Err: err})
	return v
}

func press(v View, k tea.KeyPressMsg) (View, tea.Cmd) { return v.Update(k) }

var (
	keyDown = tea.KeyPressMsg{Code: tea.KeyDown}
	keyR    = tea.KeyPressMsg{Code: 'r', Text: "r"}
)

var (
	subs = []azure.Subscription{
		{ID: "id-1", Name: "Dev", State: "Enabled"},
		{ID: "id-2", Name: "Prod", State: "Disabled"},
	}
	groups = []azure.ResourceGroup{
		{Name: "rg-a", Location: "westeurope", State: "Succeeded"},
		{Name: "rg-b", Location: "northeurope", State: "Deleting"},
	}
	resources = []azure.Resource{
		{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites", Location: "westeurope"},
		{ID: "/r/kv", Name: "kv-prod", Type: "microsoft.keyvault/vaults", Location: "northeurope"},
	}
	detail = map[string]any{
		"id": "/r/web", "name": "web-app", "tags": map[string]any{"env": "prod"},
		"url": "https://x/?a=1&b=<2>",
	}
)

var (
	allGroups = []azure.ResourceGroup{
		{Name: "rg-a", Location: "westeurope", State: "Succeeded", SubscriptionID: "id-1", Subscription: "Dev"},
		{Name: "rg-b", Location: "northeurope", State: "Deleting", SubscriptionID: "id-2", Subscription: "Prod"},
	}
	tenantResources = []azure.Resource{
		{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites", Location: "westeurope", ResourceGroup: "rg-a", SubscriptionID: "id-1", Subscription: "Dev"},
		{ID: "/r/api", Name: "api-app", Type: "microsoft.web/sites", Location: "northeurope", ResourceGroup: "rg-b", SubscriptionID: "id-2", Subscription: "Prod"},
	}
)

// lists are the table views with the data that fills them.
var lists = []struct {
	name  string
	new   func() View
	key   data.Key
	title string
	noun  string
	empty string
	data  any
	cells []string
}{
	{
		"subscriptions", NewSubscriptions,
		data.Key{Kind: data.Subscriptions}, "Subscriptions", "subscriptions",
		"No subscriptions found. Check the account you signed in with az login.",
		subs, []string{"Subscription ID", "Dev", "id-1", "Prod", "id-2", "Disabled"},
	},
	{
		"resource groups", func() View { return NewResourceGroups(subs[0]) },
		data.Key{Kind: data.ResourceGroups, Subscription: "id-1"}, "Dev", "resource groups",
		"No resource groups in this subscription.",
		groups, []string{"Location", "rg-a", "westeurope", "rg-b", "Deleting"},
	},
	{
		"resources", func() View { return NewResources("id-1", groups[1]) },
		data.Key{Kind: data.Resources, Subscription: "id-1", Group: "rg-b"}, "rg-b", "resources",
		"No resources in this resource group.",
		resources, []string{"Type", "web-app", "microsoft.web/sites", "kv-prod", "northeurope"},
	},
	{
		"all resource groups", NewAllResourceGroups,
		data.Key{Kind: data.AllResourceGroups}, "Resource groups", "resource groups",
		"No resource groups found.",
		allGroups, []string{"Subscription", "rg-a", "Dev", "westeurope", "rg-b", "Prod", "Deleting"},
	},
	{
		"resources by type", func() View { return NewResourcesByType("microsoft.web/sites") },
		data.Key{Kind: data.ResourcesOfType, Type: "microsoft.web/sites"}, "Web apps", "resources",
		"No resources of this type.",
		tenantResources, []string{"Resource group", "Subscription", "web-app", "rg-a", "Dev", "api-app", "Prod"},
	},
	{
		"virtual machines", NewVirtualMachines,
		data.Key{Kind: data.VirtualMachines}, "Virtual machines", "resources",
		"No resources of this type.",
		vmRows[:2], []string{"Private IP", "vm-web", "10.0.0.4", "Deallocated"},
	},
	{
		"AKS clusters", NewAKSClusters,
		data.Key{Kind: data.AKSClusters}, "AKS clusters", "resources",
		"No resources of this type.",
		clusterRows[:2], []string{"Auto-upgrade", "aks-prod", "1.30.4", "none"},
	},
}

func TestListKeyAndTitle(t *testing.T) {
	for _, l := range lists {
		v := l.new()
		if got := v.Title(); got != l.title {
			t.Errorf("%s: Title() = %q, want %q", l.name, got, l.title)
		}
		if got := fetchKey(t, v); got != l.key {
			t.Errorf("%s: key = %+v, want %+v", l.name, got, l.key)
		}
	}
}

func TestListStates(t *testing.T) {
	for _, l := range lists {
		loading := l.new().Resize(120, 20).Render()
		if want := "Loading " + l.noun; !strings.Contains(loading, want) {
			t.Errorf("%s: loading render lacks %q:\n%s", l.name, want, loading)
		}

		failed := deliver(t, l.new().Resize(120, 20), nil, errors.New("boom")).Render()
		if want := "Failed to load " + l.noun + ": boom"; !strings.Contains(failed, want) {
			t.Errorf("%s: error render lacks %q:\n%s", l.name, want, failed)
		}

		empty := deliver(t, l.new().Resize(120, 20), nil, nil).Render()
		if !strings.Contains(empty, l.empty) {
			t.Errorf("%s: empty render lacks %q:\n%s", l.name, l.empty, empty)
		}

		got := deliver(t, l.new().Resize(120, 20), l.data, nil).Render()
		for _, want := range l.cells {
			if !strings.Contains(got, want) {
				t.Errorf("%s: render lacks %q:\n%s", l.name, want, got)
			}
		}
		if strings.Contains(got, "Loading") {
			t.Errorf("%s: render still shows loading:\n%s", l.name, got)
		}
	}
}

// sendForeign sends v a result and an error for a key that is not its own.
func sendForeign(v View, d any) View {
	other := data.Key{Kind: data.Details, ID: "other"}
	v, _ = v.Update(data.ResultMsg{Key: other, Data: d})
	v, _ = v.Update(data.ResultMsg{Key: other, Err: errors.New("boom")})
	return v
}

func TestIgnoresResultsOfOtherKeys(t *testing.T) {
	for _, l := range lists {
		if got := sendForeign(l.new().Resize(120, 20), l.data).Render(); !strings.Contains(got, "Loading") {
			t.Errorf("%s: result of another key changed the loading view:\n%s", l.name, got)
		}
		loaded := deliver(t, l.new().Resize(120, 20), l.data, nil)
		if got := sendForeign(loaded, nil).Render(); !strings.Contains(got, l.cells[1]) || strings.Contains(got, "Failed") {
			t.Errorf("%s: result of another key changed the loaded view:\n%s", l.name, got)
		}
	}
}

func TestErrorReplacesRowsAndNextResultRecovers(t *testing.T) {
	for _, l := range lists {
		v := deliver(t, l.new().Resize(120, 20), l.data, nil)
		v = deliver(t, v, nil, errors.New("boom"))
		got := v.Render()
		if !strings.Contains(got, "Failed to load") || strings.Contains(got, l.cells[1]) {
			t.Errorf("%s: error did not replace the rows:\n%s", l.name, got)
		}
		if _, ok := v.Open(); ok {
			t.Errorf("%s: Open is true after an error", l.name)
		}
		v = deliver(t, v, l.data, nil)
		if got := v.Render(); !strings.Contains(got, l.cells[1]) || strings.Contains(got, "Failed") {
			t.Errorf("%s: view did not recover:\n%s", l.name, got)
		}
	}
}

func TestOpenWorksAfterErrorOrEmptyResult(t *testing.T) {
	for _, l := range lists {
		for name, first := range map[string]struct {
			d   any
			err error
		}{"error": {nil, errors.New("boom")}, "empty": {nil, nil}} {
			v := deliver(t, l.new().Resize(120, 20), first.d, first.err)
			v = deliver(t, v, l.data, nil)
			next, ok := v.Open()
			if !ok {
				t.Errorf("%s after %s: Open is false with rows", l.name, name)
				continue
			}
			if want := fetchKey(t, next); want == l.key {
				t.Errorf("%s after %s: Open returned the same view", l.name, name)
			}
		}
	}
}

func TestRefresh(t *testing.T) {
	for _, l := range lists {
		v := deliver(t, l.new().Resize(120, 20), l.data, nil)
		v, cmd := press(v, keyR)
		msgs := collect(cmd)
		if len(msgs) != 1 || msgs[0] != (data.FetchMsg{Key: l.key, Force: true}) {
			t.Errorf("%s: r returned %v, want a forced FetchMsg", l.name, msgs)
		}
		if got := v.Render(); !strings.Contains(got, l.cells[1]) {
			t.Errorf("%s: rows vanished while refreshing:\n%s", l.name, got)
		}
	}
}

func TestOpenIsFalseWithoutRows(t *testing.T) {
	for _, l := range lists {
		views := map[string]View{
			"loading":         l.new(),
			"error":           deliver(t, l.new(), nil, errors.New("boom")),
			"error with data": deliver(t, l.new(), l.data, errors.New("boom")),
			"empty":           deliver(t, l.new(), nil, nil),
		}
		for state, v := range views {
			if next, ok := v.Open(); ok || next != nil {
				t.Errorf("%s %s: Open = %v, %v; want nil, false", l.name, state, next, ok)
			}
		}
	}
}

func TestDrillDown(t *testing.T) {
	// Each step selects the second row and opens it.
	v := deliver(t, NewSubscriptions().Resize(120, 20), subs, nil)
	v, _ = press(v, keyDown)
	groupsView, ok := v.Open()
	if !ok {
		t.Fatal("subscriptions: Open is false")
	}
	if got := groupsView.Title(); got != "Prod" {
		t.Errorf("groups title = %q, want Prod", got)
	}
	if got, want := fetchKey(t, groupsView), (data.Key{Kind: data.ResourceGroups, Subscription: "id-2"}); got != want {
		t.Errorf("groups key = %+v, want %+v", got, want)
	}

	v = deliver(t, groupsView.Resize(120, 20), groups, nil)
	v, _ = press(v, keyDown)
	resView, ok := v.Open()
	if !ok {
		t.Fatal("resource groups: Open is false")
	}
	if got := resView.Title(); got != "rg-b" {
		t.Errorf("resources title = %q, want rg-b", got)
	}
	if got, want := fetchKey(t, resView), (data.Key{Kind: data.Resources, Subscription: "id-2", Group: "rg-b"}); got != want {
		t.Errorf("resources key = %+v, want %+v", got, want)
	}

	v = deliver(t, resView.Resize(120, 20), resources, nil)
	v, _ = press(v, keyDown)
	detView, ok := v.Open()
	if !ok {
		t.Fatal("resources: Open is false")
	}
	if got := detView.Title(); got != "kv-prod" {
		t.Errorf("details title = %q, want kv-prod", got)
	}
	if got, want := fetchKey(t, detView), (data.Key{Kind: data.Details, Subscription: "id-2", ID: "/r/kv"}); got != want {
		t.Errorf("details key = %+v, want %+v", got, want)
	}
}

func newDetails() View {
	return NewDetails("id-1", azure.Resource{ID: "/r/web", Name: "web-app"})
}

func newDetailsAt(tab string) View {
	return NewDetailsAt("id-1", azure.Resource{ID: "/r/web", Name: "web-app"}, tab)
}

func TestDetailsKeyAndTitle(t *testing.T) {
	v := newDetails()
	if got := v.Title(); got != "web-app" {
		t.Errorf("Title() = %q, want web-app", got)
	}
	if got, want := fetchKey(t, v), (data.Key{Kind: data.Details, Subscription: "id-1", ID: "/r/web"}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
}

func TestDetailsStates(t *testing.T) {
	if got := newDetails().Resize(80, 10).Render(); !strings.Contains(got, "Loading details") {
		t.Errorf("loading render:\n%s", got)
	}
	failed := deliver(t, newDetails().Resize(80, 10), nil, errors.New("boom")).Render()
	if !strings.Contains(failed, "Failed to load details: boom") {
		t.Errorf("error render:\n%s", failed)
	}
	if _, ok := deliver(t, newDetails().Resize(80, 10), nil, errors.New("boom")).Open(); ok {
		t.Error("Open is true after an error")
	}
	other := map[string]any{"name": "other-app"}
	if got := sendForeign(newDetails().Resize(80, 10), other).Render(); !strings.Contains(got, "Loading details") {
		t.Errorf("result of another key changed the loading view:\n%s", got)
	}
	loaded := deliver(t, newDetails().Resize(80, 30), detail, nil)
	got := sendForeign(loaded, other).Render()
	if !strings.Contains(got, "Essentials") || !strings.Contains(got, "env = prod") {
		t.Errorf("result of another key replaced the details:\n%s", got)
	}
	for _, bad := range []string{"other-app", "null", "Failed"} {
		if strings.Contains(got, bad) {
			t.Errorf("result of another key shows %q:\n%s", bad, got)
		}
	}
}

func TestDetailsShowsIndentedJSON(t *testing.T) {
	v := deliver(t, newDetailsAt("JSON").Resize(80, 10), detail, nil)
	got := strings.Join(plainLines(v.Render()), "\n")
	if !strings.Contains(got, "\n   \"id\": \"/r/web\",\n") {
		t.Errorf("top-level key is not indented by two spaces:\n%s", got)
	}
	if !strings.Contains(got, "\n     \"env\": \"prod\"") {
		t.Errorf("nested value is not indented by two spaces per level:\n%s", got)
	}
	if !strings.Contains(got, `"url": "https://x/?a=1&b=<2>"`) {
		t.Errorf("HTML characters are escaped:\n%s", got)
	}
	if _, ok := v.Open(); ok {
		t.Error("details Open is true")
	}
}

func TestDetailsScrolls(t *testing.T) {
	big := map[string]any{}
	for i := range 40 {
		big[fmt.Sprintf("k%02d", i)] = i
	}
	v := deliver(t, newDetailsAt("JSON").Resize(80, 7), big, nil)
	if got := v.Render(); !strings.Contains(got, "k00") || strings.Contains(got, "k20") {
		t.Fatalf("initial render:\n%s", got)
	}
	for range 3 {
		v, _ = press(v, keyDown)
	}
	got := v.Render()
	if strings.Contains(got, "k00") || strings.Contains(got, "k01") || !strings.Contains(got, "k02") {
		t.Errorf("render after scrolling:\n%s", got)
	}
	for range 50 {
		v, _ = press(v, keyDown)
	}
	lines := strings.Split(v.Render(), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "}" {
		t.Errorf("last line at the end = %q, want }", last)
	}
}

func TestDetailsRefresh(t *testing.T) {
	v := deliver(t, newDetails().Resize(80, 10), detail, nil)
	v, cmd := press(v, keyR)
	msgs := collect(cmd)
	want := data.FetchMsg{Key: data.Key{Kind: data.Details, Subscription: "id-1", ID: "/r/web"}, Force: true}
	if len(msgs) != 1 || msgs[0] != want {
		t.Errorf("r returned %v, want %v", msgs, want)
	}
	if got := v.Render(); !strings.Contains(got, "Essentials") {
		t.Errorf("content vanished while refreshing:\n%s", got)
	}
}

type renderCase struct {
	name string
	view func(w, h int) View
}

func renderCases(t *testing.T) []renderCase {
	manySubs := make([]azure.Subscription, 100)
	for i := range manySubs {
		manySubs[i] = azure.Subscription{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("sub-%d", i), State: "Enabled"}
	}
	bigDetail := map[string]any{"long": strings.Repeat("x", 300)}
	for i := range 100 {
		bigDetail[fmt.Sprintf("k%d", i)] = i
	}
	long := errors.New(strings.Repeat("very long error text without end ", 20))

	return []renderCase{
		{"loading", func(w, h int) View { return NewSubscriptions().Resize(w, h) }},
		{"table", func(w, h int) View { return deliver(t, NewSubscriptions().Resize(w, h), manySubs, nil) }},
		{"long error", func(w, h int) View { return deliver(t, NewSubscriptions().Resize(w, h), nil, long) }},
		{"empty", func(w, h int) View { return deliver(t, NewSubscriptions().Resize(w, h), nil, nil) }},
		{"details loading", func(w, h int) View { return newDetails().Resize(w, h) }},
		{"details", func(w, h int) View { return deliver(t, newDetails().Resize(w, h), bigDetail, nil) }},
		{"details error", func(w, h int) View { return deliver(t, newDetails().Resize(w, h), nil, long) }},
		{"details json", func(w, h int) View { return deliver(t, newDetailsAt("JSON").Resize(w, h), bigDetail, nil) }},
		{"filtered table", func(w, h int) View {
			return filtered(deliver(t, NewSubscriptions().Resize(w, h), manySubs, nil), "sub-1")
		}},
		{"filtered long query", func(w, h int) View {
			return filtered(deliver(t, NewSubscriptions().Resize(w, h), manySubs, nil), strings.Repeat("q", 300))
		}},
		{"filtered empty", func(w, h int) View {
			return filtered(deliver(t, NewSubscriptions().Resize(w, h), nil, nil), "x")
		}},
		{"refreshing table", func(w, h int) View {
			v, _ := press(deliver(t, NewSubscriptions().Resize(w, h), manySubs, nil), keyR)
			return v
		}},
		{"refreshing details", func(w, h int) View {
			v, _ := press(deliver(t, newDetails().Resize(w, h), bigDetail, nil), keyR)
			return v
		}},
	}
}

func aksCases(t *testing.T) []renderCase {
	var cases []renderCase
	for _, name := range []string{"aks", "aks_min", "aks_stopped"} {
		row := fixture(t, name)
		for _, tab := range Tabs("microsoft.containerservice/managedclusters") {
			cases = append(cases, renderCase{name + " " + tab, func(w, h int) View {
				return deliver(t, newDetailsAt(tab).Resize(w, h), row, nil)
			}})
		}
	}
	row := fixture(t, "aks")
	return append(cases,
		renderCase{"aks refreshing", func(w, h int) View {
			v, _ := press(deliver(t, newDetailsAt("Node pools").Resize(w, h), row, nil), keyR)
			return v
		}},
		renderCase{"aks scrolled", func(w, h int) View {
			v, _ := press(deliver(t, newDetailsAt("JSON").Resize(w, h), row, nil), key("G"))
			return v
		}},
	)
}

func TestRenderFitsSize(t *testing.T) {
	tests := append(append(renderCases(t), aksCases(t)...), resourceCases(t)...)
	sizes := [][2]int{{40, 10}, {120, 20}, {20, 4}, {0, 0}, {1, 1}, {0, 10}, {40, 0}}
	for _, tt := range tests {
		for _, s := range sizes {
			w, h := s[0], s[1]
			out := tt.view(w, h).Render()
			if w == 0 || h == 0 {
				if out != "" {
					t.Errorf("%s %dx%d: render = %q, want empty", tt.name, w, h, out)
				}
				continue
			}
			if out == "" {
				t.Errorf("%s %dx%d: render is empty", tt.name, w, h)
			}
			if got := lipgloss.Height(out); got > h {
				t.Errorf("%s %dx%d: height = %d\n%s", tt.name, w, h, got, out)
			}
			if got := lipgloss.Width(out); got > w {
				t.Errorf("%s %dx%d: width = %d\n%s", tt.name, w, h, got, out)
			}
		}
	}
}

func TestResizeSetsTableHeight(t *testing.T) {
	many := make([]azure.Subscription, 50)
	for i := range many {
		many[i] = azure.Subscription{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("sub-%d", i), State: "Enabled"}
	}
	v := deliver(t, NewSubscriptions().Resize(80, 6), many, nil)
	for range 10 {
		v, _ = press(v, keyDown)
	}
	if got := v.Render(); !strings.Contains(got, "sub-10") {
		t.Errorf("cursor row is scrolled out of view:\n%s", got)
	}
}

func TestSpinnerStopsOnceLoaded(t *testing.T) {
	views := map[string]View{"list": NewSubscriptions(), "details": newDetails()}
	for name, v := range views {
		var tick tea.Msg
		for _, msg := range collect(v.Init()) {
			if _, ok := msg.(data.FetchMsg); !ok {
				tick = msg
			}
		}
		if _, cmd := v.Update(tick); cmd == nil {
			t.Errorf("%s: spinner stopped while loading", name)
		}
		v = deliver(t, v, nil, nil)
		if _, cmd := v.Update(tick); cmd != nil {
			t.Errorf("%s: spinner keeps ticking after load", name)
		}
	}
}

func TestDetailsShowsEncodingError(t *testing.T) {
	v := deliver(t, newDetails().Resize(80, 10), map[string]any{"bad": make(chan int)}, nil)
	if got := v.Render(); !strings.Contains(got, "Failed to load details") {
		t.Errorf("render:\n%s", got)
	}
}

func TestShortType(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"microsoft.web/sites", "web/sites"},
		{"Microsoft.Web/Sites", "Web/Sites"},
		{"MICROSOFT.keyvault/vaults", "keyvault/vaults"},
		{"contoso.web/sites", "contoso.web/sites"},
		{"web/microsoft.sites", "web/microsoft.sites"},
		{"microsoft.", ""},
		{"", ""},
	} {
		if got := ShortType(c.in); got != c.want {
			t.Errorf("ShortType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResourcesOfType(t *testing.T) {
	all := []azure.Resource{
		{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites", Location: "westeurope"},
		{ID: "/r/kv", Name: "kv-prod", Type: "microsoft.keyvault/vaults", Location: "northeurope"},
		{ID: "/r/api", Name: "api-app", Type: "Microsoft.Web/Sites", Location: "westeurope"},
	}
	newOfType := func() View { return NewResourcesOfType("id-1", groups[1], "microsoft.web/sites") }

	v := newOfType()
	if got, want := v.Title(), "Web apps"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := fetchKey(t, v), fetchKey(t, NewResources("id-1", groups[1])); got != want {
		t.Errorf("key = %+v, want the key of NewResources %+v", got, want)
	}

	got := deliver(t, newOfType().Resize(120, 20), all, nil).Render()
	for _, want := range []string{"web-app", "api-app", "Location"} {
		if !strings.Contains(got, want) {
			t.Errorf("render lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "kv-prod") {
		t.Errorf("render shows a resource of another type:\n%s", got)
	}

	empty := deliver(t, newOfType().Resize(120, 20), []azure.Resource{all[1]}, nil).Render()
	if want := "No resources of this type in this resource group."; !strings.Contains(empty, want) {
		t.Errorf("empty render lacks %q:\n%s", want, empty)
	}
	if _, ok := deliver(t, newOfType(), []azure.Resource{all[1]}, nil).Open(); ok {
		t.Error("Open() = true with no matching resources")
	}

	// Open uses the kept items: the cursor row is the first web resource, not kv-prod.
	mixed := deliver(t, newOfType(), []azure.Resource{all[1], all[0]}, nil)
	opened, ok := mixed.Open()
	if !ok {
		t.Fatal("Open() = false with a matching resource")
	}
	if got, want := opened.Title(), "web-app"; got != want {
		t.Errorf("opened %q, want %q", got, want)
	}
}

var errTest = errors.New("boom")

func TestListsColorTheState(t *testing.T) {
	for _, c := range []struct {
		name  string
		view  View
		data  any
		state string
		fg    string
	}{
		{"subscription enabled", NewSubscriptions(), subs, "Enabled", fgGood},
		{"subscription disabled", NewSubscriptions(), subs, "Disabled", fgBad},
		{"group succeeded", NewResourceGroups(subs[0]), groups, "Succeeded", fgGood},
		{"group deleting", NewResourceGroups(subs[0]), groups, "Deleting", fgWarn},
		{"all groups deleting", NewAllResourceGroups(), allGroups, "Deleting", fgWarn},
		{"vm running", NewVirtualMachines(), vmRows[:2], "Running", fgGood},
		{"vm deallocated", NewVirtualMachines(), vmRows[:2], "Deallocated", fgBad},
		{"cluster running", NewAKSClusters(), clusterRows[:2], "Running", fgGood},
		{"cluster stopped", NewAKSClusters(), clusterRows[:2], "Stopped", fgBad},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := deliver(t, c.view.Resize(120, 10), c.data, nil)
			first := v.Render()
			moved, _ := press(v, kDown)
			var line string // the state on a row without the cursor
			for _, out := range []string{first, moved.Render()} {
				for _, l := range strings.Split(out, "\n") {
					if strings.Contains(plain(l), c.state) && !attrsOf(t, l, c.state).reverse {
						line = l
					}
				}
			}
			if line == "" {
				t.Fatalf("no unselected line with %q:\n%s", c.state, first)
			}
			wantFg(t, line, "● "+c.state, c.fg)
			wantFg(t, line, c.state, c.fg)
		})
	}
}

func TestListStateColumnKeepsItsPlace(t *testing.T) {
	v := deliver(t, NewSubscriptions().Resize(120, 10), subs, nil)
	header := plain(strings.Split(v.Render(), "\n")[0])
	// natural widths 4, 15 and 10 with the gaps: 87 spare cells, 29 for each column.
	if i := strings.Index(header, "State"); i != 33+2+44+2 {
		t.Errorf("State starts at %d in %q", i, header)
	}
}

func TestListSelectedRowIsOneColor(t *testing.T) {
	v := deliver(t, NewSubscriptions().Resize(120, 10), subs, nil)
	line := strings.Split(v.Render(), "\n")[2]
	for _, sub := range []string{"Dev", "● Enabled"} {
		if a := attrsOf(t, line, sub); !a.reverse || a.fg != "" {
			t.Errorf("%q on the selected row = %+v, want reversed in the row's own colors", sub, a)
		}
	}
}

func TestListCount(t *testing.T) {
	count := func(v View) (int, bool) { return v.(Counter).Count() }
	if _, ok := count(NewSubscriptions()); ok {
		t.Error("Count() ok while loading")
	}

	v := deliver(t, NewSubscriptions(), subs, nil)
	if n, ok := count(v); !ok || n != len(subs) {
		t.Errorf("Count() = %d, %v, want %d, true", n, ok, len(subs))
	}
	if n, _ := count(filtered(v, "zzz")); n != len(subs) {
		t.Errorf("Count() = %d after a filter, want %d", n, len(subs))
	}

	all := []azure.Resource{
		{ID: "/r/web", Name: "web-app", Type: "microsoft.web/sites"},
		{ID: "/r/kv", Name: "kv-prod", Type: "microsoft.keyvault/vaults"},
		{ID: "/r/api", Name: "api-app", Type: "Microsoft.Web/Sites"},
	}
	ofType := deliver(t, NewResourcesOfType("id-1", groups[1], "microsoft.web/sites"), all, nil)
	if n, ok := count(ofType); !ok || n != 2 {
		t.Errorf("Count() = %d, %v after keep, want 2, true", n, ok)
	}

	if _, ok := count(deliver(t, NewSubscriptions(), nil, errTest)); ok {
		t.Error("Count() ok after an error")
	}
}

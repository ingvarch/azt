package sidebar

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

var (
	keySubs   = data.Key{Kind: data.Subscriptions}
	keyGroups = data.Key{Kind: data.ResourceGroups, Subscription: "id-1"}
	keyRes    = data.Key{Kind: data.Resources, Subscription: "id-1", Group: "rg-a"}

	subs   = []azure.Subscription{{ID: "id-1", Name: "Dev"}, {ID: "id-2", Name: "Prod"}}
	groups = []azure.ResourceGroup{{Name: "rg-a"}, {Name: "rg-b"}}
	res    = []azure.Resource{
		{ID: "/1", Name: "site-b", Type: "microsoft.web/sites"},
		{ID: "/2", Name: "sql-b", Type: "microsoft.sql/servers"},
		{ID: "/3", Name: "Site-a", Type: "Microsoft.Web/Sites"},
		{ID: "/4", Name: "vault-1", Type: "microsoft.keyvault/vaults"},
		{ID: "/5", Name: "sql-a", Type: "microsoft.sql/servers"},
		{ID: "/6", Name: "sql-c", Type: "microsoft.sql/servers"},
	}

	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyUp    = tea.KeyPressMsg{Code: tea.KeyUp}
	keyLeft  = tea.KeyPressMsg{Code: tea.KeyLeft}
	keyRight = tea.KeyPressMsg{Code: tea.KeyRight}
	ansiSeq  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

func key(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func send(m Model, msg tea.Msg) (Model, tea.Cmd) {
	v, cmd := m.Update(msg)
	return v.(Model), cmd
}

func press(m Model, keys ...tea.KeyPressMsg) Model {
	for _, k := range keys {
		m, _ = send(m, k)
	}
	return m
}

func result(m Model, k data.Key, d any, err error) Model {
	m, _ = send(m, data.ResultMsg{Key: k, Data: d, Err: err})
	return m
}

func sized(m Model, w, h int) Model { return m.Resize(w, h).(Model) }

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

// fetches returns the keys cmd asks for.
func fetches(cmd tea.Cmd) []data.Key {
	var keys []data.Key
	for _, msg := range collect(cmd) {
		if f, ok := msg.(data.FetchMsg); ok {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

func fetchKey(t *testing.T, v views.View) data.Key {
	t.Helper()
	keys := fetches(v.Init())
	if len(keys) != 1 {
		t.Fatalf("%s: Init asks for %v, want one key", v.Title(), keys)
	}
	return keys[0]
}

func lines(m Model) []string {
	out := m.Render()
	if out == "" {
		return nil
	}
	return strings.Split(ansiSeq.ReplaceAllString(out, ""), "\n")
}

// full is a tree with Dev and Prod, Dev expanded with its groups, rg-a
// expanded with its types; the selection is on Dev.
func full() Model { return explorer(res) }

// explorer is full() with the given resources in rg-a.
func explorer(resources []azure.Resource) Model {
	m := sized(New(), 40, 20)
	m = result(m, keySubs, subs, nil)
	m = press(m, key('l'))
	m = result(m, keyGroups, groups, nil)
	m = press(m, keyDown, key('l'))
	m = result(m, keyRes, resources, nil)
	return press(m, keyUp, keyUp)
}

func assertLines(t *testing.T, m Model, want ...string) {
	t.Helper()
	got := lines(m)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func selectedTitle(t *testing.T, m Model) string {
	t.Helper()
	stack, ok := m.Stack()
	if !ok {
		t.Fatal("Stack() = false, want a node selected")
	}
	return stack[len(stack)-1].Title()
}

func TestBasics(t *testing.T) {
	m := New()
	if got := m.Title(); got != "Explorer" {
		t.Errorf("Title() = %q, want Explorer", got)
	}
	if v, ok := m.Open(); v != nil || ok {
		t.Errorf("Open() = %v, %v; want nil, false", v, ok)
	}
	if got := fetches(m.Init()); len(got) != 1 || got[0] != keySubs {
		t.Errorf("Init asks for %v, want [%v]", got, keySubs)
	}
}

func TestBodyStates(t *testing.T) {
	for _, c := range []struct {
		name string
		m    Model
		want string
	}{
		{"loading", sized(New(), 60, 5), "Loading subscriptions…"},
		{"failed", result(sized(New(), 60, 5), keySubs, nil, errors.New("boom")), "Failed to load subscriptions: boom"},
		{"empty", result(sized(New(), 60, 5), keySubs, []azure.Subscription{}, nil), "No subscriptions found."},
	} {
		if got := lines(c.m); len(got) != 1 || strings.TrimRight(got[0], " ") != c.want {
			t.Errorf("%s: rows = %q, want [%q]", c.name, got, c.want)
		}
	}
}

func TestSubscriptionsRecoverAfterError(t *testing.T) {
	m := result(sized(New(), 60, 5), keySubs, nil, errors.New("boom"))
	m = result(m, keySubs, subs, nil)
	assertLines(t, m, "▸ Dev", "▸ Prod")

	m = press(full(), keyDown, keyDown, keyDown)
	want := lines(m)
	m = result(m, keySubs, nil, errors.New("boom"))
	m = result(m, keySubs, subs, nil)
	if got := lines(m); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows after recovery:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := selectedTitle(t, m); got != "SQL servers" {
		t.Errorf("selected %q after recovery, want SQL servers", got)
	}
}

func TestGroupsRecoverAfterError(t *testing.T) {
	m := full()
	want := lines(m)
	m = result(m, keyGroups, nil, errors.New("boom"))
	assertLines(t, m, "▾ Dev", "    failed: boom", "▸ Prod")
	m = result(m, keyGroups, groups, nil)
	assertLines(t, m, want...)
}

func TestTypesWithSameShortTypeKeepOrder(t *testing.T) {
	m := full()
	for range 50 {
		m = result(m, keyRes, []azure.Resource{
			{ID: "/1", Type: "microsoft.foo/bar"},
			{ID: "/2", Type: "microsoft.foo/bar"},
			{ID: "/3", Type: "foo/bar"},
		}, nil)
		assertLines(t, m, "▾ Dev", "  ▾ rg-a", "    ▸ foo/bar 1", "    ▸ foo/bar 2", "  ▸ rg-b", "▸ Prod")
		if t.Failed() {
			break
		}
	}
}

func TestTypeRowsUseKindNamesAndSortByThem(t *testing.T) {
	m := result(full(), keyRes, []azure.Resource{
		{ID: "/1", Type: "microsoft.compute/virtualmachines"},
		{ID: "/2", Type: "Microsoft.KeyVault/vaults"},
		{ID: "/3", Type: "microsoft.foo/bar"},
		{ID: "/4", Type: "microsoft.compute/virtualmachines"},
	}, nil)
	assertLines(t, m, "▾ Dev", "  ▾ rg-a",
		"    ▸ foo/bar 1", "    ▸ Key vaults 1", "    ▸ Virtual machines 2", "  ▸ rg-b", "▸ Prod")
}

func TestRows(t *testing.T) {
	assertLines(t, full(),
		"▾ Dev",
		"  ▾ rg-a",
		"    ▸ Key vaults 1",
		"    ▸ SQL servers 3",
		"    ▸ Web apps 2",
		"  ▸ rg-b",
		"▸ Prod",
	)
}

func TestStatusRows(t *testing.T) {
	m := sized(New(), 40, 20)
	m = result(m, keySubs, subs, nil)

	m = press(m, key('l'))
	assertLines(t, m, "▾ Dev", "    loading…", "▸ Prod")

	m = result(m, keyGroups, nil, errors.New("boom"))
	assertLines(t, m, "▾ Dev", "    failed: boom", "▸ Prod")

	m = press(m, key('h'), key('l'))
	assertLines(t, m, "▾ Dev", "    loading…", "▸ Prod")
	m = result(m, keyGroups, []azure.ResourceGroup{}, nil)
	assertLines(t, m, "▾ Dev", "    empty", "▸ Prod")
}

func TestSubscriptionsKeepOrderAndState(t *testing.T) {
	m := full()
	reversed := []azure.Subscription{subs[1], subs[0], {ID: "id-3", Name: "Test"}}
	m = result(m, keySubs, reversed, nil)
	assertLines(t, m,
		"▸ Prod",
		"▾ Dev",
		"  ▾ rg-a",
		"    ▸ Key vaults 1",
		"    ▸ SQL servers 3",
		"    ▸ Web apps 2",
		"  ▸ rg-b",
		"▸ Test",
	)
}

func TestSubscriptionRenameKeepsStateAndUpdatesStack(t *testing.T) {
	m := result(full(), keySubs, []azure.Subscription{{ID: "id-1", Name: "Renamed"}, subs[1]}, nil)
	assertLines(t, m,
		"▾ Renamed",
		"  ▾ rg-a",
		"    ▸ Key vaults 1",
		"    ▸ SQL servers 3",
		"    ▸ Web apps 2",
		"  ▸ rg-b",
		"▸ Prod",
	)
	if got := selectedTitle(t, m); got != "Renamed" {
		t.Errorf("Stack title = %q, want Renamed", got)
	}
}

func TestSubscriptionsDropRemovedNodes(t *testing.T) {
	m := result(full(), keySubs, subs[1:], nil)
	assertLines(t, m, "▸ Prod")
}

func TestGroupsKeepExpandedState(t *testing.T) {
	m := full()
	m = result(m, keyGroups, []azure.ResourceGroup{{Name: "rg-c"}, groups[0]}, nil)
	assertLines(t, m,
		"▾ Dev",
		"  ▸ rg-c",
		"  ▾ rg-a",
		"    ▸ Key vaults 1",
		"    ▸ SQL servers 3",
		"    ▸ Web apps 2",
		"▸ Prod",
	)
}

func TestIgnoresOtherKeys(t *testing.T) {
	m := full()
	want := lines(m)
	for _, k := range []data.Key{
		{Kind: data.Details, Subscription: "id-1", ID: "/1"},
		{Kind: data.ResourceGroups, Subscription: "id-9"},
		{Kind: data.Resources, Subscription: "id-1", Group: "rg-z"},
		{Kind: data.Resources, Subscription: "id-9", Group: "rg-a"},
		{Kind: data.Resources, Subscription: "id-1"},
	} {
		if got := lines(result(m, k, []azure.Resource{{Type: "x/y"}}, nil)); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("result for %+v changed the tree:\n%s", k, strings.Join(got, "\n"))
		}
		if got := lines(result(m, k, nil, errors.New("boom"))); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("error for %+v changed the tree:\n%s", k, strings.Join(got, "\n"))
		}
	}
}

func TestResultForCollapsedNodeIsStored(t *testing.T) {
	m := sized(New(), 40, 20)
	m = result(m, keySubs, subs, nil)
	m = result(m, keyGroups, groups, nil)
	assertLines(t, m, "▸ Dev", "▸ Prod")

	m, cmd := send(m, key('l'))
	assertLines(t, m, "▾ Dev", "  ▸ rg-a", "  ▸ rg-b", "▸ Prod")
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("expanding a loaded node asks for %v, want nothing", got)
	}
}

func TestExpandFetchesOnceAndAgainAfterFailure(t *testing.T) {
	m := result(sized(New(), 40, 20), keySubs, subs, nil)

	m, cmd := send(m, key('l'))
	if got := fetches(cmd); len(got) != 1 || got[0] != keyGroups {
		t.Fatalf("first l asks for %v, want [%v]", got, keyGroups)
	}

	m = result(m, keyGroups, nil, errors.New("boom"))
	m = press(m, key('h'))
	m, cmd = send(m, key('l'))
	if got := fetches(cmd); len(got) != 1 || got[0] != keyGroups {
		t.Fatalf("l after a failure asks for %v, want [%v]", got, keyGroups)
	}

	m = result(m, keyGroups, groups, nil)
	m = press(m, key('h'))
	_, cmd = send(m, key('l'))
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("l after a success asks for %v, want nothing", got)
	}
}

func TestExpandGroupFetchesItsResources(t *testing.T) {
	m := result(sized(New(), 40, 20), keySubs, subs, nil)
	m = press(m, key('l'))
	m = result(m, keyGroups, groups, nil)
	m, cmd := send(press(m, keyDown), keyRight)
	if got := fetches(cmd); len(got) != 1 || got[0] != keyRes {
		t.Errorf("expanding a group asks for %v, want [%v]", got, keyRes)
	}
	assertLines(t, m, "▾ Dev", "  ▾ rg-a", "      loading…", "  ▸ rg-b", "▸ Prod")
}

func TestRightOnExpandedMovesToFirstChild(t *testing.T) {
	m, cmd := send(full(), key('l'))
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("l on an expanded node asks for %v", got)
	}
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Errorf("selected %q, want rg-a", got)
	}

	m = press(m, keyRight)
	if got := selectedTitle(t, m); got != "Key vaults" {
		t.Errorf("selected %q, want Key vaults", got)
	}
	m, cmd = send(m, keyRight)
	if got := selectedTitle(t, m); got != "Key vaults" || len(fetches(cmd)) != 0 {
		t.Errorf("l on a collapsed type: selected %q, fetches %v", got, fetches(cmd))
	}
	if got := lines(m)[2]; got != "    ▾ Key vaults 1" {
		t.Errorf("l on a type left it as %q, want it expanded", got)
	}
	m = press(m, keyRight)
	if got := selectedTitle(t, m); got != "vault-1" {
		t.Errorf("l on an expanded type selected %q, want its first resource", got)
	}
}

func TestRightOnExpandedMovesToStatusRow(t *testing.T) {
	m := result(sized(New(), 40, 20), keySubs, subs, nil)
	m = press(m, key('l'), key('l'))
	if _, ok := m.Stack(); ok {
		t.Error("Stack() = true on the loading row")
	}
}

func TestLeft(t *testing.T) {
	m := press(full(), key('l'))
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Fatalf("selected %q, want rg-a", got)
	}

	m = press(m, key('h'))
	assertLines(t, m, "▾ Dev", "  ▸ rg-a", "  ▸ rg-b", "▸ Prod")
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Errorf("collapse moved the selection to %q", got)
	}

	m = press(m, keyLeft)
	if got := selectedTitle(t, m); got != "Dev" {
		t.Errorf("h on a collapsed group selected %q, want Dev", got)
	}
	m = press(m, key('h'))
	assertLines(t, m, "▸ Dev", "▸ Prod")

	m = press(m, key('h'))
	if got := selectedTitle(t, m); got != "Dev" {
		t.Errorf("h at the top level selected %q, want Dev", got)
	}
}

func TestLeftOnTypeAndStatusRowMovesToParent(t *testing.T) {
	m := press(full(), key('l'), key('l'), key('h'))
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Errorf("h on a type selected %q, want rg-a", got)
	}

	m = result(sized(New(), 40, 20), keySubs, subs, nil)
	m = press(m, key('l'), key('l'), key('h'))
	assertLines(t, m, "▾ Dev", "    loading…", "▸ Prod")
	if got := selectedTitle(t, m); got != "Dev" {
		t.Errorf("h on a status row selected %q, want Dev", got)
	}
}

func TestSelectionFollowsNodeWhenRowsChange(t *testing.T) {
	m := press(full(), keyDown, keyDown, keyDown, keyDown, keyDown, keyDown)
	if got := selectedTitle(t, m); got != "Prod" {
		t.Fatalf("selected %q, want Prod", got)
	}

	m = result(m, keyGroups, []azure.ResourceGroup{{Name: "rg-x"}, {Name: "rg-y"}, {Name: "rg-z"}}, nil)
	if got := selectedTitle(t, m); got != "Prod" {
		t.Errorf("after rows appeared above: selected %q, want Prod", got)
	}

	m = result(m, keySubs, []azure.Subscription{{ID: "id-0", Name: "New"}, subs[1], subs[0]}, nil)
	if got := selectedTitle(t, m); got != "Prod" {
		t.Errorf("after a node appeared above: selected %q, want Prod", got)
	}
}

func TestSelectionStaysWhenRowsDisappearAbove(t *testing.T) {
	m := press(full(), keyDown, keyDown, keyDown, keyDown, keyDown, keyDown)
	if got := selectedTitle(t, m); got != "Prod" {
		t.Fatalf("selected %q, want Prod", got)
	}
	m = result(m, keyGroups, []azure.ResourceGroup{}, nil)
	if got := selectedTitle(t, m); got != "Prod" {
		t.Errorf("after rows disappeared above: selected %q, want Prod", got)
	}
}

func TestSelectionFallsBackWhenItsNodeDisappears(t *testing.T) {
	m := press(full(), keyDown, keyDown, keyDown)
	m = result(m, keyGroups, []azure.ResourceGroup{{Name: "rg-b"}}, nil)
	if got := selectedTitle(t, m); got != "Dev" {
		t.Errorf("selected %q after the groups disappeared, want its ancestor Dev", got)
	}

	m = press(full(), keyDown, keyDown, keyDown)
	m = result(m, keyRes, []azure.Resource{}, nil)
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Errorf("selected %q after the types disappeared, want its ancestor rg-a", got)
	}
}

func TestSelectionClampsWhenLastNodeDisappears(t *testing.T) {
	m := press(full(), keyDown, keyDown, keyDown, keyDown, keyDown, keyDown)
	m = result(m, keySubs, subs[:1], nil)
	assertLines(t, m,
		"▾ Dev",
		"  ▾ rg-a",
		"    ▸ Key vaults 1",
		"    ▸ SQL servers 3",
		"    ▸ Web apps 2",
		"  ▸ rg-b",
	)
	if got := selectedTitle(t, m); got != "rg-b" {
		t.Errorf("selected %q after the last node disappeared, want the new last row rg-b", got)
	}
}

func TestSelectionKeepsPositionAfterNoRows(t *testing.T) {
	m := press(full(), keyDown, keyDown, keyDown, keyDown, keyDown, keyDown)
	m = result(m, keySubs, nil, errors.New("boom"))
	m = result(m, keySubs, subs[:1], nil)
	if got := selectedTitle(t, m); got != "rg-b" {
		t.Errorf("selected %q after an error and a result without Prod, want rg-b at the same position", got)
	}
}

func TestMoveStopsAtEnds(t *testing.T) {
	m := full()
	m = press(m, keyUp, key('k'))
	if got := selectedTitle(t, m); got != "Dev" {
		t.Errorf("up at the top selected %q, want Dev", got)
	}
	for range 20 {
		m = press(m, key('j'))
	}
	if got := selectedTitle(t, m); got != "Prod" {
		t.Errorf("down at the bottom selected %q, want Prod", got)
	}
	m = press(m, keyDown)
	if got := selectedTitle(t, m); got != "Prod" {
		t.Errorf("down past the bottom selected %q, want Prod", got)
	}
	m = press(m, key('k'))
	if got := selectedTitle(t, m); got != "rg-b" {
		t.Errorf("k selected %q, want rg-b", got)
	}
}

func TestKeysWithoutRowsDoNothing(t *testing.T) {
	for _, m := range []Model{
		sized(New(), 40, 5),
		result(sized(New(), 40, 5), keySubs, nil, errors.New("boom")),
		result(sized(New(), 40, 5), keySubs, []azure.Subscription{}, nil),
		result(full(), keySubs, nil, errors.New("boom")),
	} {
		m, cmd := send(press(m, key('j'), key('k'), key('h')), key('l'))
		if cmd != nil {
			t.Error("l without rows returned a command")
		}
		if _, ok := m.Stack(); ok {
			t.Error("Stack() = true without rows")
		}
	}
}

func TestSelectionStyle(t *testing.T) {
	for _, c := range []struct {
		name    string
		focused bool
		style   lipgloss.Style
	}{
		{"focused", true, lipgloss.NewStyle().Reverse(true)},
		{"unfocused", false, lipgloss.NewStyle().Bold(true)},
	} {
		m := full().SetFocused(c.focused)
		rows := strings.Split(m.Render(), "\n")
		if want := c.style.Render("▾ Dev"); rows[0] != want {
			t.Errorf("%s: selected row = %q, want %q", c.name, rows[0], want)
		}
		if got := ansiSeq.ReplaceAllString(rows[1], ""); got != rows[1] {
			t.Errorf("%s: unselected row is styled: %q", c.name, rows[1])
		}
	}
}

func TestScrollKeepsSelectionVisible(t *testing.T) {
	m := sized(full(), 40, 3)
	check := func(step string, want string) {
		t.Helper()
		got := lines(m)
		if len(got) != 3 {
			t.Fatalf("%s: %d lines, want 3: %q", step, len(got), got)
		}
		if !slices.Contains(got, want) {
			t.Errorf("%s: rows %q lack the selected row %q", step, got, want)
		}
	}
	check("start", "▾ Dev")
	for _, want := range []string{"  ▾ rg-a", "    ▸ Key vaults 1", "    ▸ SQL servers 3", "    ▸ Web apps 2"} {
		m = press(m, keyDown)
		check("down", want)
	}
	for range 3 {
		m = press(m, keyDown)
	}
	check("end", "▸ Prod")
	if got := lines(m); got[len(got)-1] != "▸ Prod" {
		t.Errorf("at the end the last row is %q, want ▸ Prod", got[len(got)-1])
	}
	for range 8 {
		m = press(m, keyUp)
	}
	check("top", "▾ Dev")
	if got := lines(m); got[0] != "▾ Dev" {
		t.Errorf("at the top the first row is %q, want ▾ Dev", got[0])
	}
}

func TestResizeKeepsSelectionVisible(t *testing.T) {
	m := press(sized(full(), 40, 20), keyDown, keyDown, keyDown, keyDown)
	m = sized(m, 40, 2)
	if got := lines(m); !slices.Contains(got, "    ▸ SQL servers 3") {
		t.Errorf("rows %q lack the selected row after a resize", got)
	}
}

func TestRenderFitsSize(t *testing.T) {
	long := errors.New(strings.Repeat("very long error ", 8) + "\nsecond line")
	failedGroups := result(press(result(sized(New(), 40, 20), keySubs, subs, nil), key('l')), keyGroups, nil, long)
	longName := result(sized(New(), 40, 20), keySubs, []azure.Subscription{{ID: "x", Name: strings.Repeat("name ", 20)}}, nil)

	states := map[string]Model{
		"loading":       New(),
		"failed":        result(New(), keySubs, nil, long),
		"empty":         result(New(), keySubs, []azure.Subscription{}, nil),
		"tree":          full(),
		"loading row":   press(result(New(), keySubs, subs, nil), key('l')),
		"failed row":    failedGroups,
		"long name":     longName,
		"selected last": press(full(), keyUp),
	}
	for name, m := range states {
		for _, size := range [][2]int{{0, 0}, {0, 5}, {5, 0}, {1, 1}, {10, 1}, {10, 3}, {32, 4}, {32, 30}, {80, 3}} {
			w, h := size[0], size[1]
			out := sized(m, w, h).Render()
			if w <= 0 || h <= 0 {
				if out != "" {
					t.Errorf("%s %dx%d: render = %q, want empty", name, w, h, out)
				}
				continue
			}
			rows := strings.Split(out, "\n")
			if len(rows) > h {
				t.Errorf("%s %dx%d: %d lines", name, w, h, len(rows))
			}
			for _, r := range rows {
				if got := lipgloss.Width(r); got > w {
					t.Errorf("%s %dx%d: line %q is %d wide", name, w, h, r, got)
				}
			}
		}
	}
}

func TestStack(t *testing.T) {
	type want struct {
		titles []string
		keys   []data.Key
	}
	cases := []struct {
		name  string
		moves int
		want  want
	}{
		{"subscription", 0, want{[]string{"Subscriptions", "Dev"}, []data.Key{keySubs, keyGroups}}},
		{"group", 1, want{[]string{"Subscriptions", "Dev", "rg-a"}, []data.Key{keySubs, keyGroups, keyRes}}},
		{"type", 3, want{[]string{"Subscriptions", "Dev", "rg-a", "SQL servers"}, []data.Key{keySubs, keyGroups, keyRes, keyRes}}},
	}
	for _, c := range cases {
		m := full()
		for range c.moves {
			m = press(m, keyDown)
		}
		stack, ok := m.Stack()
		if !ok || len(stack) != len(c.want.titles) {
			t.Fatalf("%s: Stack() = %d views, %v; want %d", c.name, len(stack), ok, len(c.want.titles))
		}
		for i, v := range stack {
			if got := v.Title(); got != c.want.titles[i] {
				t.Errorf("%s: view %d title = %q, want %q", c.name, i, got, c.want.titles[i])
			}
			if got := fetchKey(t, v); got != c.want.keys[i] {
				t.Errorf("%s: view %d key = %+v, want %+v", c.name, i, got, c.want.keys[i])
			}
		}
	}
}

func TestStackTypeViewFiltersItsType(t *testing.T) {
	m := press(full(), keyDown, keyDown, keyDown)
	stack, _ := m.Stack()
	top := stack[len(stack)-1].Resize(80, 10)
	top, _ = top.Update(data.ResultMsg{Key: keyRes, Data: res})
	out := top.Render()
	if strings.Contains(out, "microsoft.web") || !strings.Contains(out, "microsoft.sql/servers") {
		t.Errorf("type view shows the wrong resources:\n%s", out)
	}
}

func TestStackFalseForStatusRows(t *testing.T) {
	for name, m := range map[string]Model{
		"loading": press(result(sized(New(), 40, 20), keySubs, subs, nil), key('l'), key('l')),
		"failed": press(result(press(result(sized(New(), 40, 20), keySubs, subs, nil), key('l')),
			keyGroups, nil, errors.New("boom")), key('l')),
		"empty": press(result(press(result(sized(New(), 40, 20), keySubs, subs, nil), key('l')),
			keyGroups, []azure.ResourceGroup{}, nil), key('l')),
	} {
		if stack, ok := m.Stack(); ok || stack != nil {
			t.Errorf("%s: Stack() = %v, %v; want nil, false", name, stack, ok)
		}
	}
}

func TestScrollBackWhenRowsDisappearBelow(t *testing.T) {
	m := sized(full(), 40, 3)
	m = press(m, keyDown, keyDown, keyDown, keyDown, keyDown, keyDown, keyUp, keyUp)
	m = result(m, keyGroups, groups[:1], nil)
	assertLines(t, m, "    ▸ SQL servers 3", "    ▸ Web apps 2", "▸ Prod")
}

func TestUpdateLeavesEarlierModelUnchanged(t *testing.T) {
	before := full()
	want := lines(before)
	press(before, key('j'), key('h'))
	result(before, keyGroups, nil, errors.New("boom"))
	if got := lines(before); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("an update changed the earlier model:\n%s", strings.Join(got, "\n"))
	}
}

func TestRefresh(t *testing.T) {
	loading := press(result(sized(New(), 40, 20), keySubs, subs, nil), key('l'), keyDown) // on Dev's "loading…" row
	tests := []struct {
		name string
		m    Model
		want data.Key
	}{
		{"subscription", full(), keyGroups},
		{"group", press(full(), keyDown), keyRes},
		{"type", press(full(), keyDown, keyDown), keyRes},
		{"status row", loading, keyGroups},
		{"loading subscriptions", sized(New(), 40, 5), keySubs},
		{"failed subscriptions", result(full(), keySubs, nil, errors.New("boom")), keySubs},
	}
	for _, tt := range tests {
		_, cmd := send(tt.m, key('r'))
		msgs := collect(cmd)
		if len(msgs) != 1 {
			t.Fatalf("%s: r produced %v, want one FetchMsg", tt.name, msgs)
		}
		if f, ok := msgs[0].(data.FetchMsg); !ok || !f.Force || f.Key != tt.want {
			t.Errorf("%s: r produced %+v, want forced fetch of %+v", tt.name, msgs[0], tt.want)
		}
	}
}

func TestTypesSortIgnoringCase(t *testing.T) {
	m := full()
	m = result(m, keyRes, []azure.Resource{{ID: "/1", Type: "Microsoft.Web/sites"}, {ID: "/2", Type: "microsoft.alpha/x"}}, nil)
	assertLines(t, m,
		"▾ Dev",
		"  ▾ rg-a",
		"    ▸ alpha/x 1",
		"    ▸ Web apps 1",
		"  ▸ rg-b",
		"▸ Prod",
	)
}

func TestLongTypeNameKeepsCount(t *testing.T) {
	m := sized(explorer([]azure.Resource{{ID: "/1", Type: "Microsoft.Cdn/profiles"}, {ID: "/2", Type: "Microsoft.Cdn/profiles"}}), 30, 20)
	if got, want := lines(m)[2], "    ▸ Front Door and CDN pr… 2"; got != want {
		t.Errorf("row = %q, want %q: the name cut with … and the count kept", got, want)
	}
}

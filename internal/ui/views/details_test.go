package views

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
)

var (
	keyTab      = tea.KeyPressMsg{Code: tea.KeyTab}
	keyShiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func tabOf(v View) string { return v.Spec().Tab }

func TestDetailsTabKeysWrap(t *testing.T) {
	for _, c := range []struct {
		name string
		keys []tea.KeyPressMsg
		want string
	}{
		{"tab", []tea.KeyPressMsg{keyTab}, "Node pools"},
		{"]", []tea.KeyPressMsg{key("]")}, "Node pools"},
		{"tab twice", []tea.KeyPressMsg{keyTab, keyTab}, "Networking"},
		{"tab around", []tea.KeyPressMsg{keyTab, keyTab, keyTab, keyTab, keyTab, keyTab}, ""},
		{"tab to the last", []tea.KeyPressMsg{keyTab, keyTab, keyTab, keyTab, keyTab}, "JSON"},
		{"shift+tab wraps", []tea.KeyPressMsg{keyShiftTab}, "JSON"},
		{"[ wraps", []tea.KeyPressMsg{key("[")}, "JSON"},
		{"shift+tab back", []tea.KeyPressMsg{keyTab, keyTab, keyShiftTab}, "Node pools"},
		{"[ and ]", []tea.KeyPressMsg{key("]"), key("["), key("[")}, "JSON"},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := openedAt(t, "aks", "", 80, 20)
			for _, k := range c.keys {
				var cmd tea.Cmd
				if v, cmd = press(v, k); cmd != nil {
					t.Errorf("%v returned a command", k)
				}
			}
			if got := tabOf(v); got != c.want {
				t.Errorf("tab = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetailsTabKeysOnGenericRow(t *testing.T) {
	v := openedAt(t, "webapp", "", 80, 20)
	v, _ = press(v, keyTab)
	if got := tabOf(v); got != "JSON" {
		t.Errorf("tab = %q, want JSON", got)
	}
	v, _ = press(v, keyTab)
	if got := tabOf(v); got != "" {
		t.Errorf("tab = %q, want the first", got)
	}
}

func TestDetailsTabKeysBeforeData(t *testing.T) {
	v := NewDetails("s1", azure.Resource{ID: "/r/x", Name: "x"}).Resize(80, 10)
	for _, k := range []tea.KeyPressMsg{keyTab, keyShiftTab, key("]"), key("j")} {
		v, _ = press(v, k)
	}
	if got := v.Render(); !strings.Contains(got, "Loading details") {
		t.Errorf("render:\n%s", got)
	}
}

func firstBody(v View) string { return plainLines(v.Render())[1] }

func TestDetailsScrollKeys(t *testing.T) {
	// Networking has 13 lines; a 6-line pane shows 5 of them.
	open := func() View { return openedAt(t, "aks", "Networking", 80, 6) }
	for _, c := range []struct {
		name string
		keys []string
		want string
	}{
		{"start", nil, "Network plugin"},
		{"j", []string{"j"}, "Network policy"},
		{"down", []string{"down"}, "Network policy"},
		{"j j k", []string{"j", "j", "k"}, "Network policy"},
		{"up at the start", []string{"up"}, "Network plugin"},
		{"k at the start", []string{"k"}, "Network plugin"},
		{"pgdown", []string{"pgdown"}, "DNS service IP"},
		{"f", []string{"f"}, "DNS service IP"},
		{"end", []string{"end"}, "IP families"},
		{"G", []string{"G"}, "IP families"},
		{"past the end", []string{"G", "j", "j", "pgdown"}, "IP families"},
		{"1000 downs", append(make([]string, 0), repeat("j", 1000)...), "IP families"},
		{"pgup", []string{"G", "pgup"}, "Pod CIDR"},
		{"b", []string{"G", "b"}, "Pod CIDR"},
		{"home", []string{"G", "home"}, "Network plugin"},
		{"g", []string{"G", "g"}, "Network plugin"},
		{"pgup at the start", []string{"b"}, "Network plugin"},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := open()
			for _, k := range c.keys {
				v, _ = press(v, key(k))
			}
			if got := firstBody(v); !strings.HasPrefix(got, c.want) {
				t.Errorf("first body line = %q, want %q", got, c.want)
			}
			if got := plainLines(v.Render())[0]; !strings.HasPrefix(got, "Overview") {
				t.Errorf("tab bar scrolled away: %q", got)
			}
		})
	}
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func TestDetailsScrollResetsOnTabSwitch(t *testing.T) {
	v := openedAt(t, "aks", "Networking", 80, 6)
	v, _ = press(v, key("G"))
	v, _ = press(v, keyTab)
	if got := firstBody(v); !strings.HasPrefix(got, "Identity ") {
		t.Errorf("after tab: %q", got)
	}
	v, _ = press(v, key("G"))
	v, _ = press(v, keyShiftTab)
	if got := firstBody(v); !strings.HasPrefix(got, "Network plugin") {
		t.Errorf("after shift+tab: %q", got)
	}
}

func TestDetailsScrollFollowsResize(t *testing.T) {
	v := openedAt(t, "aks", "Networking", 80, 6)
	v, _ = press(v, key("G"))
	v = v.Resize(80, 30)
	if got := firstBody(v); !strings.HasPrefix(got, "Network plugin") {
		t.Errorf("after growing: %q", got)
	}
}

func TestDetailsScrollWhileRefreshing(t *testing.T) {
	v := openedAt(t, "aks", "Networking", 80, 6)
	v, _ = press(v, key("G"))
	v, _ = press(v, keyR)
	got := plainLines(v.Render())
	if got[1] != refreshingText || len(got) > 6 {
		t.Errorf("render:\n%s", strings.Join(got, "\n"))
	}
	v, _ = press(v, key("G"))
	got = plainLines(v.Render())
	if !strings.HasPrefix(got[len(got)-1], "Private FQDN") {
		t.Errorf("last line at the end = %q", got[len(got)-1])
	}
}

func TestDetailsKeepsTabOnRefreshResult(t *testing.T) {
	v := openedAt(t, "aks", "Identity", 80, 10)
	v = deliver(t, v, fixture(t, "aks"), nil)
	if got := tabOf(v); got != "Identity" {
		t.Errorf("tab = %q, want Identity", got)
	}
}

func TestDetailsClickSelectsTab(t *testing.T) {
	// Overview 0-7, Node pools 10-19, Networking 22-31, Identity 34-41, Add-ons 44-50, JSON 53-56.
	for _, c := range []struct {
		name string
		x, y int
		want string
		hit  bool
	}{
		{"first tab", 0, 0, "", true},
		{"end of the first tab", 7, 0, "", true},
		{"gap", 8, 0, "Networking", false},
		{"second gap cell", 9, 0, "Networking", false},
		{"second tab", 10, 0, "Node pools", true},
		{"end of the second tab", 19, 0, "Node pools", true},
		{"third tab", 25, 0, "Networking", true},
		{"identity", 34, 0, "Identity", true},
		{"add-ons", 50, 0, "Add-ons", true},
		{"json", 53, 0, "JSON", true},
		{"end of json", 56, 0, "JSON", true},
		{"after the last tab", 57, 0, "Networking", false},
		{"far right", 79, 0, "Networking", false},
		{"body line", 12, 1, "Networking", false},
		{"below", 12, 5, "Networking", false},
		{"negative", -1, 0, "Networking", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := openedAt(t, "aks", "Networking", 80, 6)
			next, cmd, hit := v.(Clicker).Click(c.x, c.y)
			if cmd != nil {
				t.Error("Click returned a command")
			}
			if hit != c.hit {
				t.Errorf("hit = %v, want %v", hit, c.hit)
			}
			if got := tabOf(next); got != c.want {
				t.Errorf("tab = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetailsClickResetsScroll(t *testing.T) {
	v := openedAt(t, "aks", "Networking", 80, 6)
	v, _ = press(v, key("G"))
	v, _, _ = v.(Clicker).Click(34, 0)
	if got := firstBody(v); !strings.HasPrefix(got, "Identity ") {
		t.Errorf("first body line = %q", got)
	}
}

func TestDetailsClickBeforeData(t *testing.T) {
	v := NewDetails("s1", azure.Resource{ID: "/r/x"}).Resize(80, 6)
	if _, _, hit := v.(Clicker).Click(3, 0); hit {
		t.Error("click hit before the data arrived")
	}
	failed := deliver(t, NewDetails("s1", azure.Resource{ID: "/r/x"}).Resize(80, 6), nil, errTest)
	if _, _, hit := failed.(Clicker).Click(3, 0); hit {
		t.Error("click hit after an error")
	}
}

func TestDetailsClickOnGenericTabs(t *testing.T) {
	v := openedAt(t, "webapp", "", 80, 6)
	next, _, hit := v.(Clicker).Click(12, 0)
	if !hit || tabOf(next) != "JSON" {
		t.Errorf("hit = %v, tab = %q", hit, tabOf(next))
	}
	if _, _, hit := v.(Clicker).Click(14, 0); hit {
		t.Error("hit past the last generic tab")
	}
}

func TestNewDetailsAtOpensOnTab(t *testing.T) {
	v := openedAt(t, "aks", "Add-ons", 80, 20)
	if got := tabOf(v); got != "Add-ons" {
		t.Errorf("tab = %q", got)
	}
	if got := firstBody(v); !strings.HasPrefix(got, "azurepolicy") {
		t.Errorf("first body line = %q", got)
	}
	if got := tabOf(openedAt(t, "aks", "json", 80, 20)); got != "JSON" {
		t.Errorf("tab in another case = %q, want JSON", got)
	}
	same := NewDetailsAt("s1", azure.Resource{ID: "/r/x", Name: "x"}, "JSON")
	if fetchKey(t, same) != fetchKey(t, NewDetails("s1", azure.Resource{ID: "/r/x", Name: "x"})) {
		t.Error("NewDetailsAt asks for another key than NewDetails")
	}
}

func TestNewDetailsAtFallsBackToTheFirstTab(t *testing.T) {
	v := NewDetailsAt("s1", azure.Resource{ID: "/r/x", Name: "x"}, "Networking").Resize(80, 20)
	if got := tabOf(v); got != "Networking" {
		t.Errorf("tab before the data = %q, want the requested one", got)
	}
	v = deliver(t, v, fixture(t, "webapp"), nil)
	if got := tabOf(v); got != "" {
		t.Errorf("tab after the data = %q, want the first", got)
	}
	got := plainLines(v.Render())
	if got[0] != "Overview  JSON" || got[1] != "Essentials" {
		t.Errorf("render:\n%s", strings.Join(got, "\n"))
	}
	v, _ = press(v, keyTab)
	if got := tabOf(v); got != "JSON" {
		t.Errorf("tab after tab = %q", got)
	}
}

func TestDetailsSpecTab(t *testing.T) {
	res := azure.Resource{ID: "/r/x", Name: "x"}
	if got := NewDetails("s1", res).Spec(); got.Tab != "" {
		t.Errorf("Tab = %q on a new view", got.Tab)
	}
	if got := NewDetailsAt("s1", res, "Overview").Spec(); got.Tab != "" {
		t.Errorf("Tab = %q on the first tab", got.Tab)
	}
	spec := NewDetailsAt("s1", res, "JSON").Spec()
	if spec.Tab != "JSON" || spec.Kind != "details" {
		t.Fatalf("Spec = %+v", spec)
	}
	raw, err := json.Marshal(spec)
	if err != nil || !strings.Contains(string(raw), `"tab":"JSON"`) {
		t.Errorf("json = %s, %v", raw, err)
	}
	raw, _ = json.Marshal(NewDetails("s1", res).Spec())
	if strings.Contains(string(raw), "tab") {
		t.Errorf("json of the first tab has a tab: %s", raw)
	}
}

func TestBuildDetailsOnTab(t *testing.T) {
	res := azure.Resource{ID: "/r/x", Name: "x"}
	v := openedAt(t, "aks", "Identity", 80, 10)
	built, err := Build(v.Spec())
	if err != nil {
		t.Fatal(err)
	}
	if built.Spec() != v.Spec() || tabOf(built) != "Identity" {
		t.Errorf("Spec = %+v, want %+v", built.Spec(), v.Spec())
	}
	built = deliver(t, built.Resize(80, 10), fixture(t, "aks"), nil)
	if got := firstBody(built); !strings.HasPrefix(got, "Identity ") {
		t.Errorf("first body line = %q", got)
	}

	first, err := Build(Spec{Kind: "details", Subscription: "s1", ResourceID: res.ID, ResourceName: res.Name})
	if err != nil {
		t.Fatal(err)
	}
	first = deliver(t, first.Resize(80, 10), fixture(t, "aks"), nil)
	if got := firstBody(first); got != "Essentials" {
		t.Errorf("a spec without tab opens on %q", got)
	}
}

func TestDetailsRefreshToShorterRowClampsScroll(t *testing.T) {
	v := openedAt(t, "aks", "JSON", 80, 6)
	v, _ = press(v, key("G"))
	v, _ = press(v, keyR)
	v = deliver(t, v, map[string]any{"type": aksType}, nil)
	got := plainLines(v.Render())
	if !strings.HasPrefix(got[0], "Overview  Node pools") || got[1] != "{" || got[2] != `  "type": "`+aksType+`"` {
		t.Errorf("render:\n%s", strings.Join(got, "\n"))
	}
}

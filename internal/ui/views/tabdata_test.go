package views

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

const keyedType = "test/keyed"

func keyedKey(row map[string]any, period string) data.Key {
	return data.Key{Kind: data.AKSMonitoring, Subscription: str(row, "subscriptionId"), ID: str(row, "id"), Period: period}
}

func drawKeyed(_ map[string]any, res tabData, _ int) []string {
	switch {
	case res.loading:
		return []string{"keyed loading"}
	case res.err != nil:
		return []string{"keyed failed " + res.err.Error()}
	}
	return []string{fmt.Sprintf("keyed %v", res.data)}
}

// withKeyedKind adds a resource type whose tabs are Overview, Keyed (with a
// data key) and Other.
func withKeyedKind(t *testing.T) {
	t.Helper()
	saved := kinds
	t.Cleanup(func() { kinds = saved })
	kinds = append(slices.Clone(kinds), resourceKind{types: []string{keyedType}, tabs: []tabDef{
		{name: overviewTab, render: boxesTab(genericEssentials)},
		{name: "Keyed", key: keyedKey, draw: drawKeyed},
		{name: "Other", render: jsonTab},
	}})
}

var keyedRow = map[string]any{"type": keyedType, "id": "/r/x", "subscriptionId": "s1"}

func keyedKeyAt(period string) data.Key { return keyedKey(keyedRow, period) }

// fetches returns the requests cmd makes.
func fetches(cmd tea.Cmd) []data.FetchMsg {
	var out []data.FetchMsg
	for _, msg := range collect(cmd) {
		if f, ok := msg.(data.FetchMsg); ok {
			out = append(out, f)
		}
	}
	return out
}

// wantFetches checks the requests cmd makes.
func wantFetches(t *testing.T, cmd tea.Cmd, want ...data.FetchMsg) {
	t.Helper()
	got := fetches(cmd)
	if len(got) != len(want) {
		t.Fatalf("requests = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// keyedAt returns a details view of keyedRow on the tab and what the row's
// arrival asked for.
func keyedAt(t *testing.T, tab string) (View, tea.Cmd) {
	t.Helper()
	withKeyedKind(t)
	v := NewDetailsAt("s1", azure.Resource{ID: "/r/x", Name: "x"}, tab).Resize(80, 10)
	return v.Update(data.ResultMsg{Key: fetchKey(t, v), Data: keyedRow})
}

func keyedView(t *testing.T, tab string) View {
	t.Helper()
	v, _ := keyedAt(t, tab)
	return v
}

func bodyText(v View) string { return strings.Join(plainLines(v.Render()), "\n") }

func TestTabDataFetchOnOpeningAtTheTab(t *testing.T) {
	_, cmd := keyedAt(t, "Keyed")
	wantFetches(t, cmd, data.FetchMsg{Key: keyedKeyAt("24h")})
}

func TestTabDataNoFetchOnTabWithoutKey(t *testing.T) {
	for _, tab := range []string{"", "Other"} {
		if _, cmd := keyedAt(t, tab); cmd != nil {
			t.Errorf("tab %q: row asked for %+v", tab, fetches(cmd))
		}
	}
}

func TestTabDataFetchOnSwitching(t *testing.T) {
	for _, c := range []struct {
		name string
		from string
		do   func(View) (View, tea.Cmd)
	}{
		{"tab", "", func(v View) (View, tea.Cmd) { return press(v, keyTab) }},
		{"]", "", func(v View) (View, tea.Cmd) { return press(v, key("]")) }},
		{"shift+tab", "Other", func(v View) (View, tea.Cmd) { return press(v, keyShiftTab) }},
		{"[", "Other", func(v View) (View, tea.Cmd) { return press(v, key("[")) }},
		{"click", "", func(v View) (View, tea.Cmd) {
			next, cmd, _ := v.(Clicker).Click(13, 0)
			return next, cmd
		}},
		{"click on the rule", "", func(v View) (View, tea.Cmd) {
			next, cmd, _ := v.(Clicker).Click(13, 1)
			return next, cmd
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			next, cmd := c.do(keyedView(t, c.from))
			if got := tabOf(next); got != "Keyed" {
				t.Fatalf("tab = %q, want Keyed", got)
			}
			wantFetches(t, cmd, data.FetchMsg{Key: keyedKeyAt("24h")})
			if got := bodyText(next); !strings.Contains(got, "keyed loading") {
				t.Errorf("render:\n%s", got)
			}
		})
	}
}

func TestTabDataNoFetchOnSwitchingToTabWithoutKey(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, cmd := press(v, keyTab)
	if got := tabOf(v); got != "Other" || cmd != nil {
		t.Errorf("tab %q, requests %+v", got, fetches(cmd))
	}
	v, cmd = press(v, keyTab)
	if got := tabOf(v); got != "" || cmd != nil {
		t.Errorf("tab %q, requests %+v", got, fetches(cmd))
	}
	next, cmd, _ := v.(Clicker).Click(30, 0)
	if cmd != nil {
		t.Errorf("click on Other asked for %+v (tab %q)", fetches(cmd), tabOf(next))
	}
}

func TestTabDataFetchesAgainOnReturn(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "one"})
	v, _ = press(v, keyTab)
	v, cmd := press(v, keyShiftTab)
	wantFetches(t, cmd, data.FetchMsg{Key: keyedKeyAt("24h")})
	if got := bodyText(v); !strings.Contains(got, "keyed loading") {
		t.Errorf("render after returning:\n%s", got)
	}
}

func TestTabDataLoadingUntilResult(t *testing.T) {
	v := keyedView(t, "Keyed")
	if got := bodyText(v); !strings.Contains(got, "keyed loading") {
		t.Errorf("render:\n%s", got)
	}
	v, cmd := v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "numbers"})
	if cmd != nil {
		t.Errorf("result asked for %+v", fetches(cmd))
	}
	if got := bodyText(v); !strings.Contains(got, "keyed numbers") || strings.Contains(got, "loading") {
		t.Errorf("render:\n%s", got)
	}
}

func TestTabDataError(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Err: errTest})
	if got := bodyText(v); !strings.Contains(got, "keyed failed boom") {
		t.Errorf("render:\n%s", got)
	}
}

func TestTabDataIgnoresResultOfAnotherKey(t *testing.T) {
	v := keyedView(t, "Keyed")
	other := keyedKeyAt("7d")
	v, _ = v.Update(data.ResultMsg{Key: other, Data: "week"})
	if got := bodyText(v); !strings.Contains(got, "keyed loading") {
		t.Errorf("render after another key:\n%s", got)
	}
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "day"})
	v, _ = v.Update(data.ResultMsg{Key: other, Data: "week"})
	if got := bodyText(v); !strings.Contains(got, "keyed day") {
		t.Errorf("render after another key:\n%s", got)
	}
}

func TestTabDataIgnoresResultAfterLeavingTheTab(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = press(v, keyTab)
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "late"})
	v, _ = press(v, keyShiftTab)
	if got := bodyText(v); !strings.Contains(got, "keyed loading") {
		t.Errorf("render:\n%s", got)
	}
}

func TestTabDataRowRefreshKeepsTheData(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "day"})
	v, cmd := v.Update(data.ResultMsg{Key: fetchKey(t, v), Data: keyedRow})
	if cmd != nil {
		t.Errorf("row result asked for %+v", fetches(cmd))
	}
	if got := bodyText(v); !strings.Contains(got, "keyed day") {
		t.Errorf("render:\n%s", got)
	}
}

func TestTabDataRefreshRefreshesBothKeys(t *testing.T) {
	v := keyedView(t, "Keyed")
	rowKey := fetchKey(t, v)
	_, cmd := press(v, keyR)
	wantFetches(t, cmd,
		data.FetchMsg{Key: rowKey, Force: true},
		data.FetchMsg{Key: keyedKeyAt("24h"), Force: true})

	v = keyedView(t, "Other")
	_, cmd = press(v, keyR)
	wantFetches(t, cmd, data.FetchMsg{Key: rowKey, Force: true})
}

func TestTabDataPeriodSteps(t *testing.T) {
	v := keyedView(t, "Keyed")
	for _, want := range []string{"7d", "30d", "24h", "7d"} {
		var cmd tea.Cmd
		v, cmd = press(v, key("p"))
		wantFetches(t, cmd, data.FetchMsg{Key: keyedKeyAt(want)})
		if got := bodyText(v); !strings.Contains(got, "keyed loading") {
			t.Errorf("period %s: render:\n%s", want, got)
		}
		v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt(want), Data: want})
		if got := bodyText(v); !strings.Contains(got, "keyed "+want) {
			t.Errorf("period %s: render:\n%s", want, got)
		}
	}
}

func TestTabDataPeriodIgnoresResultOfThePreviousOne(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = press(v, key("p"))
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "day"})
	if got := bodyText(v); !strings.Contains(got, "keyed loading") {
		t.Errorf("render:\n%s", got)
	}
}

func TestTabDataPeriodKeyElsewhereDoesNothing(t *testing.T) {
	for _, tab := range []string{"", "Other"} {
		v := keyedView(t, tab)
		before := v.Render()
		next, cmd := press(v, key("p"))
		if cmd != nil || next.Render() != before {
			t.Errorf("tab %q: p asked for %+v", tab, fetches(cmd))
		}
	}
	// The period stays the first one.
	v, _ := press(keyedView(t, ""), key("p"))
	_, cmd := press(v, keyTab)
	wantFetches(t, cmd, data.FetchMsg{Key: keyedKeyAt("24h")})
}

func TestTabDataPeriodIsKeptWhenSwitchingTabs(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = press(v, key("p"))
	v, _ = press(v, keyTab)
	_, cmd := press(v, keyShiftTab)
	wantFetches(t, cmd, data.FetchMsg{Key: keyedKeyAt("7d")})
}

func TestTabDataPeriodKeyBeforeData(t *testing.T) {
	withKeyedKind(t)
	v := NewDetailsAt("s1", azure.Resource{ID: "/r/x", Name: "x"}, "Keyed").Resize(80, 10)
	if _, cmd := press(v, key("p")); cmd != nil {
		t.Errorf("p before the row asked for %+v", fetches(cmd))
	}
}

func TestTabDataSelectingTheCurrentTabKeepsTheData(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "day"})
	for _, x := range []int{13, 13} {
		var cmd tea.Cmd
		v, cmd, _ = v.(Clicker).Click(x, 0)
		if cmd != nil {
			t.Errorf("click on the current tab asked for %+v", fetches(cmd))
		}
		if got := bodyText(v); !strings.Contains(got, "keyed day") {
			t.Errorf("render:\n%s", got)
		}
	}
}

func TestTabDataKeysAfterLeavingTheKeyedTab(t *testing.T) {
	v := keyedView(t, "Keyed")
	rowKey := fetchKey(t, v)
	v, _ = press(v, keyTab)
	if _, cmd := press(v, key("p")); cmd != nil {
		t.Errorf("p on Other asked for %+v", fetches(cmd))
	}
	_, cmd := press(v, keyR)
	wantFetches(t, cmd, data.FetchMsg{Key: rowKey, Force: true})
}

func TestTabDataRefreshNoteLastsUntilTheTabResult(t *testing.T) {
	v := keyedView(t, "Keyed")
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "day"})
	v, _ = press(v, keyR)
	v, _ = v.Update(data.ResultMsg{Key: fetchKey(t, v), Data: keyedRow})
	if got := bodyText(v); !strings.Contains(got, refreshingText) {
		t.Errorf("note gone before the tab result:\n%s", got)
	}
	v, _ = v.Update(data.ResultMsg{Key: keyedKeyAt("24h"), Data: "fresh"})
	if got := bodyText(v); strings.Contains(got, refreshingText) || !strings.Contains(got, "keyed fresh") {
		t.Errorf("render:\n%s", got)
	}
}

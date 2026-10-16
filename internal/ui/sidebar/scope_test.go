package sidebar

import (
	"strings"
	"testing"
)

func TestScopeShowsOnlyTheSubscription(t *testing.T) {
	m := full().SetScope("id-2")
	assertLines(t, m, "▸ Prod")
	if got := selectedTitle(t, m); got != "Prod" {
		t.Errorf("selected %q, want Prod", got)
	}
}

func TestScopeKeepsExpansionOfHiddenSubscriptions(t *testing.T) {
	m := full()
	want := lines(m)
	m = m.SetScope("id-2")
	m = press(m, key('l')) // expand Prod while Dev is hidden
	m = m.SetScope("")
	got := lines(m)
	if len(got) <= len(want) || strings.Join(got[:len(want)-1], "\n") != strings.Join(want[:len(want)-1], "\n") {
		t.Errorf("rows after the scope is cleared:\n%s\nwant Dev as it was, then Prod expanded:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	m = m.SetScope("id-1")
	assertLines(t, m, want[:len(want)-1]...)
}

func TestScopeKeepsTheSelectionOnItsSubscription(t *testing.T) {
	m := full().SetScope("id-1")
	m = press(m, keyDown) // rg-a
	m = m.SetScope("")
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Errorf("selected %q, want rg-a", got)
	}
}

func TestScopeOfAnUnknownSubscriptionSelectsNothing(t *testing.T) {
	m := full().SetScope("gone")
	if _, ok := m.Stack(); ok {
		t.Error("Stack() = true with nothing shown")
	}
}

func TestScopeKeysStayInTheSubscription(t *testing.T) {
	m := full().SetScope("id-1")
	m = press(m, keyDown, keyDown, keyDown, keyDown, keyDown, keyDown, keyDown, keyDown)
	if got := selectedTitle(t, m); strings.Contains(got, "Prod") {
		t.Errorf("selected %q, outside the scope", got)
	}
}

func TestScopeOfAnUnknownSubscriptionSaysSo(t *testing.T) {
	m := full().SetScope("gone")
	if got := m.Render(); !strings.Contains(got, "Subscription not found. Press 0 for all.") {
		t.Errorf("render = %q", got)
	}
	if got := full().SetScope("id-1").Render(); strings.Contains(got, "not found") {
		t.Errorf("render = %q", got)
	}
}

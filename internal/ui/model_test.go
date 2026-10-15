package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
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

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// started returns a model that got the initial window size, as Bubble Tea sends it,
// and ran Init with a loader returning subs and err.
func started(subs []azure.Subscription, err error) Model {
	m := New(func() ([]azure.Subscription, error) { return subs, err })
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	for _, msg := range collect(m.Init()) {
		m, _ = update(m, msg)
	}
	return m
}

func TestShowsLoadingBeforeSubscriptionsArrive(t *testing.T) {
	m := New(func() ([]azure.Subscription, error) { return nil, nil })

	if got := m.View().Content; !strings.Contains(got, "Loading subscriptions") {
		t.Errorf("view does not show loading:\n%s", got)
	}
}

func TestShowsSubscriptions(t *testing.T) {
	m := started([]azure.Subscription{
		{ID: "id-1", Name: "Dev", State: "Enabled"},
		{ID: "id-2", Name: "Prod", State: "Disabled"},
	}, nil)

	got := m.View().Content
	for _, want := range []string{"Subscription ID", "Dev", "id-1", "Prod", "id-2", "Disabled"} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Loading subscriptions") {
		t.Errorf("view still shows loading:\n%s", got)
	}
}

func TestShowsLoadError(t *testing.T) {
	m := started(nil, errors.New("az login required"))

	want := "Failed to load subscriptions: az login required"
	if got := m.View().Content; !strings.Contains(got, want) {
		t.Errorf("view does not contain %q:\n%s", want, got)
	}
}

func TestShowsEmptyState(t *testing.T) {
	m := started(nil, nil)

	if got := m.View().Content; !strings.Contains(got, "No subscriptions found") {
		t.Errorf("view does not show empty state:\n%s", got)
	}
}

func TestViewFitsWindowHeight(t *testing.T) {
	subs := make([]azure.Subscription, 100)
	for i := range subs {
		subs[i] = azure.Subscription{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("sub-%d", i), State: "Enabled"}
	}
	m := started(subs, nil)

	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 20})

	if h := lipgloss.Height(m.View().Content); h > 20 {
		t.Errorf("view height = %d, want <= 20", h)
	}
}

func TestQuitKeys(t *testing.T) {
	keys := []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'c', Mod: tea.ModCtrl},
	}
	for _, key := range keys {
		_, cmd := update(started(nil, nil), key)
		if cmd == nil {
			t.Errorf("%s: no command, want quit", key)
			continue
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s: command does not quit", key)
		}
	}
}

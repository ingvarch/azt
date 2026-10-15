package views

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

// started returns a view sized w x h that ran Init with a loader returning subs and err.
func started(w, h int, subs []azure.Subscription, err error) View {
	var v View = NewSubscriptions(func() ([]azure.Subscription, error) { return subs, err })
	v = v.Resize(w, h)
	for _, msg := range collect(v.Init()) {
		v, _ = v.Update(msg)
	}
	return v
}

func TestTitle(t *testing.T) {
	v := NewSubscriptions(nil)
	if got := v.Title(); got != "Subscriptions" {
		t.Errorf("Title() = %q, want Subscriptions", got)
	}
}

func TestShowsLoadingBeforeSubscriptionsArrive(t *testing.T) {
	v := NewSubscriptions(func() ([]azure.Subscription, error) { return nil, nil }).Resize(40, 5)

	if got := v.Render(); !strings.Contains(got, "Loading subscriptions") {
		t.Errorf("render does not show loading:\n%s", got)
	}
}

func TestShowsSubscriptions(t *testing.T) {
	v := started(120, 20, []azure.Subscription{
		{ID: "id-1", Name: "Dev", State: "Enabled"},
		{ID: "id-2", Name: "Prod", State: "Disabled"},
	}, nil)

	got := v.Render()
	for _, want := range []string{"Subscription ID", "Dev", "id-1", "Prod", "id-2", "Disabled"} {
		if !strings.Contains(got, want) {
			t.Errorf("render does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Loading subscriptions") {
		t.Errorf("render still shows loading:\n%s", got)
	}
}

func TestShowsLoadError(t *testing.T) {
	v := started(120, 20, nil, errors.New("az login required"))

	want := "Failed to load subscriptions: az login required"
	if got := v.Render(); !strings.Contains(got, want) {
		t.Errorf("render does not contain %q:\n%s", want, got)
	}
}

func TestShowsEmptyState(t *testing.T) {
	v := started(120, 20, nil, nil)

	if got := v.Render(); !strings.Contains(got, "No subscriptions found") {
		t.Errorf("render does not show empty state:\n%s", got)
	}
}

func TestIgnoresResultsOfOtherViews(t *testing.T) {
	v := started(120, 20, []azure.Subscription{{ID: "id-1", Name: "Dev", State: "Enabled"}}, nil)
	other := NewSubscriptions(func() ([]azure.Subscription, error) { return nil, errors.New("boom") })

	for _, msg := range collect(other.Init()) {
		v, _ = v.Update(msg)
	}

	if got := v.Render(); !strings.Contains(got, "Dev") || strings.Contains(got, "boom") {
		t.Errorf("result of another view changed this one:\n%s", got)
	}
}

func TestRenderFitsSize(t *testing.T) {
	many := make([]azure.Subscription, 100)
	for i := range many {
		many[i] = azure.Subscription{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("sub-%d", i), State: "Enabled"}
	}
	long := errors.New(strings.Repeat("very long error text without end ", 20))

	tests := []struct {
		name string
		view func(w, h int) View
	}{
		{"loading", func(w, h int) View {
			return NewSubscriptions(func() ([]azure.Subscription, error) { return nil, nil }).Resize(w, h)
		}},
		{"table", func(w, h int) View { return started(w, h, many, nil) }},
		{"long error", func(w, h int) View { return started(w, h, nil, long) }},
		{"empty", func(w, h int) View { return started(w, h, nil, nil) }},
	}
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
	v := started(80, 6, many, nil)
	for range 10 {
		v, _ = v.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got := v.Render(); !strings.Contains(got, "sub-10") {
		t.Errorf("cursor row is scrolled out of view:\n%s", got)
	}
}

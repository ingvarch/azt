package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/views"
)

// clipboard makes the model write the system clipboard into the returned slice.
func (h *harness) clipboard(err error) *[]string {
	var got []string
	h.m.clipboard = func(s string) error {
		got = append(got, s)
		return err
	}
	return &got
}

// showDetails puts a details view of a resource named web in pane 0.
func (h *harness) showDetails() {
	h.m.stacks[0] = []views.View{views.NewDetails("sub-1", azure.Resource{ID: "/r/web", Name: "web"})}
	h.m.resizeViews()
}

func TestCopyKeys(t *testing.T) {
	for _, tt := range []struct{ key, want string }{{"y", "/r/web"}, {"Y", "web"}} {
		t.Run(tt.key, func(t *testing.T) {
			h := newHarness(100, 30)
			h.showDetails()
			written := h.clipboard(nil)
			msgs := collect(h.send(key(tt.key)))
			if !slices.Contains(msgs, tea.SetClipboard(tt.want)()) {
				t.Errorf("%s sends no OSC 52 for %q: %v", tt.key, tt.want, msgs)
			}
			if !slices.Equal(*written, []string{tt.want}) {
				t.Errorf("%s wrote %q to the system clipboard, want [%q]", tt.key, *written, tt.want)
			}
			wantIn(t, "status", h.status(), "copied "+tt.want)
		})
	}
}

func TestCopyFromTheExplorer(t *testing.T) {
	h := newSidebarHarness(140, 30)
	h.loadSubs()
	h.prefix("h")
	written := h.clipboard(nil)
	collect(h.send(key("y")))
	if !slices.Equal(*written, []string{"sub-1"}) {
		t.Errorf("wrote %q, want [sub-1]", *written)
	}
}

func TestCopyIsNotAnError(t *testing.T) {
	h := newHarness(100, 30)
	h.showDetails()
	h.clipboard(errors.New("no clipboard utility"))
	collect(h.send(key("y")))
	if h.m.notice != "" {
		t.Errorf("notice = %q, want none", h.m.notice)
	}
	if strings.Contains(h.raw(), errorStyle.Render("copied /r/web")) {
		t.Error("copied is drawn as an error")
	}
	wantIn(t, "status", h.status(), "copied /r/web")
	h.send(key("j"))
	if strings.Contains(h.status(), "copied") {
		t.Errorf("the next key keeps %q", h.status())
	}
}

func TestCopyWithNothingSelected(t *testing.T) {
	h := newHarness(100, 30)
	written := h.clipboard(nil)
	if cmd := h.send(key("y")); cmd != nil {
		t.Errorf("y on a view without a selection returned %v", collect(cmd))
	}
	if len(*written) != 0 {
		t.Errorf("wrote %q", *written)
	}
	if len(h.pane(0).keys) != 0 {
		t.Errorf("the view got %v", h.pane(0).keys)
	}
	if h.m.notice != "nothing to copy" {
		t.Errorf("notice = %q, want nothing to copy", h.m.notice)
	}
}

func TestCopyWithAnEmptyName(t *testing.T) {
	h := newHarness(100, 30)
	h.m.stacks[0] = []views.View{views.NewDetails("sub-1", azure.Resource{ID: "/r/web"})}
	written := h.clipboard(nil)
	if cmd := h.send(key("Y")); cmd != nil {
		t.Errorf("Y returned %v", collect(cmd))
	}
	if len(*written) != 0 || h.m.notice != "nothing to copy" {
		t.Errorf("wrote %q, notice %q; want nothing written and nothing to copy", *written, h.m.notice)
	}
}

func TestModelsWriteTheSystemClipboard(t *testing.T) {
	h := &harness{}
	restored, err := Restore(oneTab(leaf("subscriptions")), h.newFake, h.fetch)
	if err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]Model{"New": New(h.newFake, h.fetch), "Restore": restored} {
		if m.clipboard == nil {
			t.Errorf("%s leaves the system clipboard unset", name)
		}
	}
}

func TestCopyKeysTypeInInputs(t *testing.T) {
	tests := []struct {
		name  string
		open  func(h *harness)
		input func(h *harness) string
	}{
		{"filter", func(h *harness) { h.send(key("/")) }, func(h *harness) string { return h.m.input.Value() }},
		{"palette", func(h *harness) { h.send(key(":")) }, func(h *harness) string { return h.m.palette.text }},
		{"tab dialog", func(h *harness) { h.prefix(",") }, func(h *harness) string { return h.m.input.Value() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newFilterHarness(100, 30)
			written := h.clipboard(nil)
			tt.open(h)
			h.typ("yY")
			if got := tt.input(h); !strings.HasSuffix(got, "yY") {
				t.Errorf("input = %q, want it to end with yY", got)
			}
			if len(*written) != 0 {
				t.Errorf("wrote %q", *written)
			}
		})
	}
}

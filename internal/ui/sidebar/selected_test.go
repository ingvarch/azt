package sidebar

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/ui/views"
)

func TestSelected(t *testing.T) {
	vault := views.Item{ID: "/4", Name: "vault-1"}
	cases := []struct {
		name string
		keys []tea.KeyPressMsg
		want views.Item
		ok   bool
	}{
		{"subscription", nil, views.Item{ID: "id-1", Name: "Dev"}, true},
		{"group", []tea.KeyPressMsg{keyDown}, views.Item{ID: "/subscriptions/id-1/resourceGroups/rg-a", Name: "rg-a"}, true},
		{"type", []tea.KeyPressMsg{keyDown, keyDown}, views.Item{}, false},
		{"resource", []tea.KeyPressMsg{keyDown, keyDown, key('l'), keyDown}, vault, true},
		{"tab", []tea.KeyPressMsg{keyDown, keyDown, key('l'), keyDown, key('l'), keyDown}, vault, true},
		{"status row", []tea.KeyPressMsg{keyDown, keyDown, keyDown, keyDown, keyDown, keyDown, key('l'), keyDown}, views.Item{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var v views.View = press(full(), c.keys...)
			s, ok := v.(views.Selector)
			if !ok {
				t.Fatal("the explorer does not implement views.Selector")
			}
			if got, ok := s.Selected(); got != c.want || ok != c.ok {
				t.Errorf("Selected() = %+v, %v; want %+v, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestSelectedWithoutRows(t *testing.T) {
	if got, ok := New().Selected(); ok {
		t.Errorf("Selected() before the subscriptions load = %+v, true; want false", got)
	}
}

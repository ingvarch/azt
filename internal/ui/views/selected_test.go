package views

import (
	"testing"

	"github.com/ingvarch/azt/internal/azure"
)

func selected(t *testing.T, v View) (Item, bool) {
	t.Helper()
	s, ok := v.(Selector)
	if !ok {
		t.Fatalf("%T does not implement Selector", v)
	}
	return s.Selected()
}

func TestListSelectedIsTheRowUnderTheCursor(t *testing.T) {
	for _, l := range lists {
		t.Run(l.name, func(t *testing.T) {
			v := must(press(deliver(t, l.new().Resize(120, 10), l.data, nil), keyDown))
			got, ok := selected(t, v)
			if !ok || got != l.second {
				t.Errorf("Selected() = %+v, %v; want %+v, true", got, ok, l.second)
			}
		})
	}
}

func TestListSelectedFollowsTheFilter(t *testing.T) {
	v := filtered(deliver(t, NewSubscriptions().Resize(80, 10), subs, nil), "prod")
	if got, ok := selected(t, v); !ok || got.ID != "id-2" {
		t.Errorf("Selected() = %+v, %v; want id-2", got, ok)
	}
}

func TestListSelectedWithoutRows(t *testing.T) {
	tests := []struct {
		name string
		view View
	}{
		{"loading", NewSubscriptions().Resize(80, 10)},
		{"failed", deliver(t, NewSubscriptions().Resize(80, 10), nil, errTest)},
		{"empty", deliver(t, NewSubscriptions().Resize(80, 10), []azure.Subscription{}, nil)},
		{"no matches", filtered(deliver(t, NewSubscriptions().Resize(80, 10), subs, nil), "nothing")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := selected(t, tt.view); ok {
				t.Errorf("Selected() = %+v, true; want false", got)
			}
		})
	}
}

func TestDetailsSelectedIsTheResource(t *testing.T) {
	r := azure.Resource{ID: "/r/web", Name: "web-app"}
	tests := []struct {
		name string
		view View
	}{
		{"loading", NewDetails("id-1", r).Resize(80, 10)},
		{"loaded", deliver(t, NewDetails("id-1", r).Resize(80, 10), detail, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := selected(t, tt.view)
			if want := (Item{ID: "/r/web", Name: "web-app"}); !ok || got != want {
				t.Errorf("Selected() = %+v, %v; want %+v, true", got, ok, want)
			}
		})
	}
}

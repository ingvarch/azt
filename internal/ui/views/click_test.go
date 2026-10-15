package views

import (
	"errors"
	"testing"
)

func click(t *testing.T, v View, x, y int) View {
	t.Helper()
	c, ok := v.(Clicker)
	if !ok {
		t.Fatalf("%T does not implement Clicker", v)
	}
	next, cmd, _ := c.Click(x, y)
	if cmd != nil {
		t.Errorf("Click(%d, %d) returned a command", x, y)
	}
	return next
}

// opened returns the title of the view the selected row opens.
func opened(t *testing.T, v View) string {
	t.Helper()
	next, ok := v.Open()
	if !ok {
		t.Fatal("nothing opens")
	}
	return next.Title()
}

func TestListClickSelectsRow(t *testing.T) {
	loaded := deliver(t, NewSubscriptions().Resize(80, 10), subs, nil)
	tests := []struct {
		name string
		view View
		x, y int
		want string
	}{
		{"second row", loaded, 3, 3, "Prod"},
		{"first row after moving", must(press(loaded, keyDown)), 0, 2, "Dev"},
		{"last column", loaded, 79, 3, "Prod"},
		{"header", must(press(loaded, keyDown)), 3, 0, "Prod"},
		{"rule", must(press(loaded, keyDown)), 3, 1, "Prod"},
		{"below the last row", must(press(loaded, keyDown)), 3, 4, "Prod"},
		{"below the list", must(press(loaded, keyDown)), 3, 50, "Prod"},
		{"negative line", must(press(loaded, keyDown)), 3, -1, "Prod"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := opened(t, click(t, tt.view, tt.x, tt.y)); got != tt.want {
				t.Errorf("opened %q, want %q", got, tt.want)
			}
		})
	}
}

func must(v View, _ any) View { return v }

func TestListClickWithInfoLine(t *testing.T) {
	// The filter "d" keeps Dev (name) and Prod (state "Disabled"): the info line is line 0.
	v := filtered(deliver(t, NewSubscriptions().Resize(80, 10), subs, nil), "d")
	tests := []struct {
		y    int
		want string
	}{
		{0, "Dev"},  // info line: nothing changes
		{1, "Dev"},  // header
		{2, "Dev"},  // rule
		{3, "Dev"},  // first row
		{4, "Prod"}, // second row
	}
	for _, tt := range tests {
		if got := opened(t, click(t, v, 3, tt.y)); got != tt.want {
			t.Errorf("click on line %d: opened %q, want %q", tt.y, got, tt.want)
		}
	}
}

func TestListClickWhileRefreshingCountsInfoLine(t *testing.T) {
	v := deliver(t, NewSubscriptions().Resize(80, 10), subs, nil)
	v, _ = press(v, keyR)
	if got := opened(t, click(t, v, 3, 4)); got != "Prod" {
		t.Errorf("opened %q, want Prod", got)
	}
}

func TestListClickWithoutTable(t *testing.T) {
	tests := []struct {
		name string
		view View
	}{
		{"loading", NewSubscriptions().Resize(80, 10)},
		{"error", deliver(t, NewSubscriptions().Resize(80, 10), nil, errors.New("boom"))},
		{"empty", deliver(t, NewSubscriptions().Resize(80, 10), nil, nil)},
		{"no matches", filtered(deliver(t, NewSubscriptions().Resize(80, 10), subs, nil), "zzz")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.view.Render()
			for y := range 6 {
				if got := click(t, tt.view, 3, y).Render(); got != before {
					t.Errorf("click on line %d changed the view:\n%s\nwant:\n%s", y, got, before)
				}
			}
		})
	}
}

func TestDetailsIsNotAClicker(t *testing.T) {
	if _, ok := newDetails().(Clicker); ok {
		t.Error("details implements Clicker")
	}
}

func TestListClickReportsRowHit(t *testing.T) {
	loaded := deliver(t, NewSubscriptions().Resize(80, 10), subs, nil)
	for _, c := range []struct {
		y   int
		hit bool
	}{{0, false}, {1, false}, {2, true}, {3, true}, {4, false}, {-1, false}} {
		if _, _, hit := loaded.(Clicker).Click(3, c.y); hit != c.hit {
			t.Errorf("line %d: hit = %v, want %v", c.y, hit, c.hit)
		}
	}
}

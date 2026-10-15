package sidebar

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/ui/views"
)

var _ views.Clicker = Model{}

func click(m Model, x, y int) (Model, tea.Cmd) {
	v, cmd, _ := m.Click(x, y)
	return v.(Model), cmd
}

// fullRows are the lines of full().
var fullRows = []string{
	"▾ Dev",
	"  ▾ rg-a",
	"    ▸ keyvault/vaults 1",
	"    ▸ sql/servers 3",
	"    ▸ web/sites 2",
	"  ▸ rg-b",
	"▸ Prod",
}

func TestFullRows(t *testing.T) { assertLines(t, full(), fullRows...) }

func TestClickSelectsRow(t *testing.T) {
	tests := []struct {
		name string
		x, y int
		want string
	}{
		{"subscription label", 4, 0, "Dev"},
		{"group label", 5, 1, "rg-a"},
		{"type label", 10, 3, "sql/servers"},
		{"type indent", 0, 3, "sql/servers"},
		{"far right", 39, 6, "Prod"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cmd := click(full(), tt.x, tt.y)
			if got := selectedTitle(t, m); got != tt.want {
				t.Errorf("selected %q, want %q", got, tt.want)
			}
			if len(collect(cmd)) != 0 {
				t.Errorf("click returned a command")
			}
			assertLines(t, m, fullRows...)
		})
	}
}

func TestClickMarkerTogglesSubscriptionAndGroup(t *testing.T) {
	tests := []struct {
		name string
		x, y int
		want []string
	}{
		{"expanded subscription, first marker cell", 0, 0, []string{"▸ Dev", "▸ Prod"}},
		{"expanded subscription, second marker cell", 1, 0, []string{"▸ Dev", "▸ Prod"}},
		{"expanded group, first marker cell", 2, 1, []string{
			"▾ Dev", "  ▸ rg-a", "  ▸ rg-b", "▸ Prod",
		}},
		{"expanded group, second marker cell", 3, 1, []string{
			"▾ Dev", "  ▸ rg-a", "  ▸ rg-b", "▸ Prod",
		}},
		{"subscription label", 2, 0, fullRows},
		{"group indent", 1, 1, fullRows},
		{"group label", 4, 1, fullRows},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cmd := click(full(), tt.x, tt.y)
			assertLines(t, m, tt.want...)
			if got := fetches(cmd); len(got) != 0 {
				t.Errorf("click asks for %v, want nothing", got)
			}
		})
	}
}

func TestClickMarkerExpandsAndFetches(t *testing.T) {
	m := result(sized(New(), 40, 20), keySubs, subs, nil)

	m, cmd := click(m, 0, 1) // Prod
	if got := fetches(cmd); len(got) != 1 || got[0].Subscription != "id-2" {
		t.Errorf("expanding Prod asks for %v, want its groups", got)
	}
	assertLines(t, m, "▸ Dev", "▾ Prod", "    loading…")
	if got := selectedTitle(t, press(m, keyUp, keyDown)); got != "Prod" {
		t.Errorf("selection = %q, want Prod", got)
	}

	m, cmd = click(m, 1, 0) // Dev
	if got := fetches(cmd); len(got) != 1 || got[0] != keyGroups {
		t.Errorf("expanding Dev asks for %v, want [%v]", got, keyGroups)
	}
	if got := selectedTitle(t, m); got != "Dev" {
		t.Errorf("selection = %q, want Dev", got)
	}
}

func TestClickMarkerExpandsLoadedWithoutFetch(t *testing.T) {
	m := result(sized(New(), 40, 20), keySubs, subs, nil)
	m = result(m, keyGroups, groups, nil)
	m, cmd := click(m, 0, 0)
	assertLines(t, m, "▾ Dev", "  ▸ rg-a", "  ▸ rg-b", "▸ Prod")
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("expanding a loaded node asks for %v, want nothing", got)
	}
	m, _ = click(m, 0, 0)
	m, cmd = click(m, 0, 0)
	assertLines(t, m, "▾ Dev", "  ▸ rg-a", "  ▸ rg-b", "▸ Prod")
	if got := fetches(cmd); len(got) != 0 {
		t.Errorf("re-expanding asks for %v, want nothing", got)
	}
}

func TestClickMarkerRetriesAfterFailure(t *testing.T) {
	m := result(sized(New(), 40, 20), keySubs, subs, nil)
	m = press(m, key('l'))
	m = result(m, keyGroups, nil, errors.New("boom"))
	m, _ = click(m, 0, 0)
	m, cmd := click(m, 0, 0)
	if got := fetches(cmd); len(got) != 1 || got[0] != keyGroups {
		t.Errorf("expanding after a failure asks for %v, want [%v]", got, keyGroups)
	}
	assertLines(t, m, "▾ Dev", "    loading…", "▸ Prod")
}

func TestClickMarkerSelectsTheToggledRow(t *testing.T) {
	m, _ := click(full(), 2, 1) // collapse rg-a while Dev is selected
	if got := selectedTitle(t, m); got != "rg-a" {
		t.Errorf("selected %q, want rg-a", got)
	}
}

func TestClickStatusRowSelectsItWithoutToggle(t *testing.T) {
	m := result(sized(New(), 40, 20), keySubs, subs, nil)
	m = press(m, key('l')) // Dev with a "loading…" row
	m, cmd := click(m, 0, 1)
	assertLines(t, m, "▾ Dev", "    loading…", "▸ Prod")
	if _, ok := m.Stack(); ok {
		t.Error("a status row has a stack")
	}
	if len(collect(cmd)) != 0 {
		t.Error("click returned a command")
	}
}

func TestClickOutsideRowsChangesNothing(t *testing.T) {
	scrolled := press(sized(full(), 40, 3), keyDown, keyDown, keyDown, keyDown) // top = 2
	tests := []struct {
		name string
		m    Model
		x, y int
	}{
		{"below the last row", full(), 0, 7},
		{"far below", full(), 0, 19},
		{"above", full(), 0, -1},
		{"below the height", scrolled, 0, 3},
		{"before the subscriptions load", sized(New(), 40, 20), 0, 0},
		{"after a load error", result(sized(New(), 40, 20), keySubs, nil, errors.New("boom")), 0, 0},
		{"no subscriptions", result(sized(New(), 40, 20), keySubs, nil, nil), 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.m.Render()
			m, cmd := click(tt.m, tt.x, tt.y)
			if got := m.Render(); got != want {
				t.Errorf("click changed the view:\n%s\nwant:\n%s", got, want)
			}
			if len(collect(cmd)) != 0 {
				t.Error("click returned a command")
			}
			if tt.m.sel != m.sel {
				t.Errorf("selection %q, want %q", m.sel, tt.m.sel)
			}
		})
	}
}

func TestClickUsesScrollOffset(t *testing.T) {
	m := press(sized(full(), 40, 3), keyDown, keyDown, keyDown, keyDown) // on sql/servers, rows 2..4 shown
	if got := lines(m); !slices.Equal(got, fullRows[2:5]) {
		t.Fatalf("setup: lines = %q", got)
	}
	m, _ = click(m, 8, 2) // web/sites
	if got := selectedTitle(t, m); got != "web/sites" {
		t.Errorf("selected %q, want web/sites", got)
	}
	m, _ = click(m, 8, 0)
	if got := selectedTitle(t, m); got != "keyvault/vaults" {
		t.Errorf("selected %q, want keyvault/vaults", got)
	}
}

func TestClickLeavesEarlierModelUnchanged(t *testing.T) {
	before := full()
	want := strings.Join(lines(before), "\n")
	click(before, 0, 0)
	click(before, 2, 1)
	if got := strings.Join(lines(before), "\n"); got != want {
		t.Errorf("a click changed the earlier model:\n%s", got)
	}
}

func TestSpecIsExplorer(t *testing.T) {
	if got := New().Spec(); got != (views.Spec{Kind: "explorer"}) {
		t.Errorf("Spec = %+v", got)
	}
}

func TestClickReportsRowHit(t *testing.T) {
	last := len(fullRows) - 1 // ▸ Prod
	for _, c := range []struct {
		name string
		x, y int
		hit  bool
	}{
		{"label", 4, 0, true},
		{"last label", 4, last, true},
		{"marker of an expanded node", 0, 0, true},
		{"marker of a collapsed node", 0, last, true},
		{"below the rows", 4, len(fullRows), false},
		{"above the rows", 4, -1, false},
	} {
		if _, _, hit := full().Click(c.x, c.y); hit != c.hit {
			t.Errorf("%s: hit = %v, want %v", c.name, hit, c.hit)
		}
	}
}

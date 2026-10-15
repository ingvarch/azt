package layout

import (
	"reflect"
	"strconv"
	"testing"
)

func split(o Orientation, ratio float64, a, b *Node) *Node {
	return &Node{Split: o, Ratio: ratio, A: a, B: b}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name string
		tree *Node
		do   func(*Node) *Node
		want *Node
	}{
		{
			name: "single leaf",
			tree: Leaf(1),
			do:   func(n *Node) *Node { return n.SplitPane(1, 2, Horizontal) },
			want: split(Horizontal, 0.5, Leaf(1), Leaf(2)),
		},
		{
			name: "nested leaf",
			tree: split(Horizontal, 0.5, Leaf(1), Leaf(2)),
			do:   func(n *Node) *Node { return n.SplitPane(2, 3, Vertical) },
			want: split(Horizontal, 0.5, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3))),
		},
		{
			name: "unknown target",
			tree: split(Horizontal, 0.5, Leaf(1), Leaf(2)),
			do:   func(n *Node) *Node { return n.SplitPane(9, 3, Vertical) },
			want: split(Horizontal, 0.5, Leaf(1), Leaf(2)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := clone(tt.tree)
			got := tt.do(tt.tree)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %s, want %s", dump(got), dump(tt.want))
			}
			assertUnchanged(t, tt.tree, orig)
		})
	}
}

func TestClose(t *testing.T) {
	tests := []struct {
		name   string
		tree   *Node
		target PaneID
		want   *Node
	}{
		{"only leaf", Leaf(1), 1, nil},
		{"promotes sibling", split(Horizontal, 0.5, Leaf(1), Leaf(2)), 1, Leaf(2)},
		{
			"nested promotes sibling in place",
			split(Horizontal, 0.3, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3))),
			3,
			split(Horizontal, 0.3, Leaf(1), Leaf(2)),
		},
		{
			"closing a leaf next to a subtree promotes the subtree",
			split(Horizontal, 0.3, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3))),
			1,
			split(Vertical, 0.5, Leaf(2), Leaf(3)),
		},
		{"unknown target", split(Horizontal, 0.5, Leaf(1), Leaf(2)), 9, split(Horizontal, 0.5, Leaf(1), Leaf(2))},
		{"unknown target on leaf", Leaf(1), 9, Leaf(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := clone(tt.tree)
			got := tt.tree.Close(tt.target)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %s, want %s", dump(got), dump(tt.want))
			}
			assertUnchanged(t, tt.tree, orig)
		})
	}
}

func TestPanesOrder(t *testing.T) {
	tree := split(Horizontal, 0.5, split(Vertical, 0.5, Leaf(1), Leaf(2)), split(Vertical, 0.5, Leaf(3), Leaf(4)))
	if got, want := tree.Panes(), []PaneID{1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRects(t *testing.T) {
	tests := []struct {
		name string
		tree *Node
		area Rect
		want map[PaneID]Rect
	}{
		{
			name: "leaf fills area",
			tree: Leaf(1),
			area: Rect{2, 3, 10, 5},
			want: map[PaneID]Rect{1: {2, 3, 10, 5}},
		},
		{
			name: "odd width horizontal",
			tree: split(Horizontal, 0.5, Leaf(1), Leaf(2)),
			area: Rect{0, 0, 101, 31},
			want: map[PaneID]Rect{1: {0, 0, 50, 31}, 2: {50, 0, 51, 31}},
		},
		{
			name: "odd height vertical",
			tree: split(Vertical, 0.5, Leaf(1), Leaf(2)),
			area: Rect{0, 0, 101, 31},
			want: map[PaneID]Rect{1: {0, 0, 101, 15}, 2: {0, 15, 101, 16}},
		},
		{
			name: "nested",
			tree: split(Horizontal, 0.5, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3))),
			area: Rect{0, 0, 101, 31},
			want: map[PaneID]Rect{1: {0, 0, 50, 31}, 2: {50, 0, 51, 15}, 3: {50, 15, 51, 16}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := clone(tt.tree)
			got := tt.tree.Rects(tt.area)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			assertTiles(t, got, tt.area)
			assertUnchanged(t, tt.tree, orig)
		})
	}
}

func TestRectsTileExactly(t *testing.T) {
	tree := split(Horizontal, 0.37,
		split(Vertical, 0.61, Leaf(1), split(Horizontal, 0.5, Leaf(2), Leaf(3))),
		split(Vertical, 0.2, Leaf(4), Leaf(5)),
	)
	for _, area := range []Rect{{0, 0, 101, 31}, {5, 7, 97, 29}, {0, 0, 80, 24}} {
		assertTiles(t, tree.Rects(area), area)
	}
}

// assertTiles checks that rects cover area exactly, with no overlaps.
func assertTiles(t *testing.T, rects map[PaneID]Rect, area Rect) {
	t.Helper()
	cells := map[[2]int]PaneID{}
	for id, r := range rects {
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				if other, ok := cells[[2]int{x, y}]; ok {
					t.Fatalf("panes %d and %d overlap at (%d,%d)", id, other, x, y)
				}
				cells[[2]int{x, y}] = id
			}
		}
	}
	for y := area.Y; y < area.Y+area.H; y++ {
		for x := area.X; x < area.X+area.W; x++ {
			if _, ok := cells[[2]int{x, y}]; !ok {
				t.Fatalf("gap at (%d,%d)", x, y)
			}
			delete(cells, [2]int{x, y})
		}
	}
	if len(cells) != 0 {
		t.Fatalf("%d cells outside area", len(cells))
	}
}

func TestNeighbor(t *testing.T) {
	grid := split(Vertical, 0.5,
		split(Horizontal, 0.5, Leaf(1), Leaf(2)),
		split(Horizontal, 0.5, Leaf(3), Leaf(4)),
	)
	// 1 on the left, 2 top right, 3 and 4 bottom right.
	lshape := split(Horizontal, 0.5, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3)))
	area := Rect{0, 0, 100, 40}

	tests := []struct {
		name string
		tree *Node
		from PaneID
		dir  Direction
		want PaneID
		ok   bool
	}{
		{"grid right", grid, 1, Right, 2, true},
		{"grid down", grid, 1, Down, 3, true},
		{"grid left", grid, 4, Left, 3, true},
		{"grid up", grid, 4, Up, 2, true},
		{"grid edge left", grid, 1, Left, 0, false},
		{"grid edge up", grid, 2, Up, 0, false},
		{"grid edge right", grid, 2, Right, 0, false},
		{"grid edge down", grid, 3, Down, 0, false},
		{"l-shape right from tall pane picks top (tie)", lshape, 1, Right, 2, true},
		{"l-shape left from top", lshape, 2, Left, 1, true},
		{"l-shape left from bottom", lshape, 3, Left, 1, true},
		{"l-shape up from bottom", lshape, 3, Up, 2, true},
		{"l-shape down from top", lshape, 2, Down, 3, true},
		{"single leaf", Leaf(1), 1, Right, 0, false},
		{"unknown pane", grid, 9, Right, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := clone(tt.tree)
			got, ok := tt.tree.Neighbor(tt.from, tt.dir, area)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got (%d, %v), want (%d, %v)", got, ok, tt.want, tt.ok)
			}
			assertUnchanged(t, tt.tree, orig)
		})
	}
}

func TestNeighborLargestOverlap(t *testing.T) {
	// Pane 1 on the left; right column has 2 (top, 25%) and 3 (bottom, 75%).
	tree := split(Horizontal, 0.5, Leaf(1), split(Vertical, 0.25, Leaf(2), Leaf(3)))
	got, ok := tree.Neighbor(1, Right, Rect{0, 0, 100, 40})
	if !ok || got != 3 {
		t.Errorf("got (%d, %v), want (3, true)", got, ok)
	}
}

func TestNeighborTie(t *testing.T) {
	tests := []struct {
		name string
		tree *Node
		from PaneID
		dir  Direction
		area Rect
		want PaneID
	}{
		{
			// 1 on top; 2 and 3 below overlap it equally.
			name: "down picks smaller X",
			tree: split(Vertical, 0.5, Leaf(1), split(Horizontal, 0.5, Leaf(2), Leaf(3))),
			from: 1, dir: Down, area: Rect{0, 0, 100, 40},
			want: 2,
		},
		{
			// 3 and 4 get a zero-width column at X=1, 5 the full column there.
			// 4 (Y=20) and 5 (Y=0) overlap 2 equally; 4 comes first in Panes().
			name: "right picks smaller Y over pane order",
			tree: split(Horizontal, 0.5,
				split(Vertical, 0.5, Leaf(1), Leaf(2)),
				split(Horizontal, 0.5, split(Vertical, 0.5, Leaf(3), Leaf(4)), Leaf(5))),
			from: 2, dir: Right, area: Rect{0, 0, 2, 40},
			want: 5,
		},
		{
			// The same tree turned: zero-height row at Y=1.
			name: "down picks smaller X over pane order",
			tree: split(Vertical, 0.5,
				split(Horizontal, 0.5, Leaf(1), Leaf(2)),
				split(Vertical, 0.5, split(Horizontal, 0.5, Leaf(3), Leaf(4)), Leaf(5))),
			from: 2, dir: Down, area: Rect{0, 0, 40, 2},
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.tree.Neighbor(tt.from, tt.dir, tt.area)
			if !ok || got != tt.want {
				t.Errorf("got (%d, %v), want (%d, true)", got, ok, tt.want)
			}
		})
	}
}

func TestResize(t *testing.T) {
	base := split(Horizontal, 0.5, Leaf(1), Leaf(2))
	nested := split(Horizontal, 0.5, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3)))

	tests := []struct {
		name   string
		tree   *Node
		target PaneID
		o      Orientation
		delta  float64
		want   *Node
	}{
		{"grow in A", base, 1, Horizontal, 0.1, split(Horizontal, 0.6, Leaf(1), Leaf(2))},
		{"grow in B", base, 2, Horizontal, 0.1, split(Horizontal, 0.4, Leaf(1), Leaf(2))},
		{"clamp max", base, 1, Horizontal, 0.8, split(Horizontal, MaxRatio, Leaf(1), Leaf(2))},
		{"clamp min", base, 2, Horizontal, 0.8, split(Horizontal, MinRatio, Leaf(1), Leaf(2))},
		{"no matching ancestor", base, 1, Vertical, 0.1, base},
		{"single leaf", Leaf(1), 1, Horizontal, 0.1, Leaf(1)},
		{"unknown target", base, 9, Horizontal, 0.1, base},
		{
			"nearest ancestor with orientation",
			nested, 2, Vertical, 0.2,
			split(Horizontal, 0.5, Leaf(1), split(Vertical, 0.7, Leaf(2), Leaf(3))),
		},
		{
			"skips non-matching nearer ancestor",
			nested, 3, Horizontal, 0.1,
			split(Horizontal, 0.4, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3))),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := clone(tt.tree)
			got := tt.tree.Resize(tt.target, tt.o, tt.delta)
			if !sameTree(got, tt.want) {
				t.Errorf("got %s, want %s", dump(got), dump(tt.want))
			}
			assertUnchanged(t, tt.tree, orig)
		})
	}
}

func TestEqualize(t *testing.T) {
	tests := []struct {
		name string
		tree *Node
		want *Node
	}{
		{
			name: "three columns",
			tree: split(Horizontal, 0.7, Leaf(1), split(Horizontal, 0.2, Leaf(2), Leaf(3))),
			want: split(Horizontal, 1.0/3, Leaf(1), split(Horizontal, 0.5, Leaf(2), Leaf(3))),
		},
		{
			name: "three columns, nested in A",
			tree: split(Horizontal, 0.2, split(Horizontal, 0.9, Leaf(1), Leaf(2)), Leaf(3)),
			want: split(Horizontal, 2.0/3, split(Horizontal, 0.5, Leaf(1), Leaf(2)), Leaf(3)),
		},
		{
			name: "mixed orientation counts child as one",
			tree: split(Horizontal, 0.8, Leaf(1), split(Vertical, 0.1, Leaf(2), Leaf(3))),
			want: split(Horizontal, 0.5, Leaf(1), split(Vertical, 0.5, Leaf(2), Leaf(3))),
		},
		{name: "leaf", tree: Leaf(1), want: Leaf(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := clone(tt.tree)
			got := tt.tree.Equalize()
			if !sameTree(got, tt.want) {
				t.Errorf("got %s, want %s", dump(got), dump(tt.want))
			}
			assertUnchanged(t, tt.tree, orig)
		})
	}
}

func TestEqualizeClampsRatio(t *testing.T) {
	ten := Leaf(10)
	for id := PaneID(9); id >= 1; id-- {
		ten = split(Horizontal, 0.5, Leaf(id), ten)
	}
	tests := []struct {
		name string
		tree *Node
		want float64
	}{
		{"one column before ten", split(Horizontal, 0.5, Leaf(0), ten), MinRatio},
		{"ten columns before one", split(Horizontal, 0.5, ten, Leaf(11)), MaxRatio},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tree.Equalize().Ratio; got != tt.want {
				t.Errorf("root ratio = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLeaf(t *testing.T) {
	n := Leaf(4)
	if !n.IsLeaf() || n.Pane != 4 {
		t.Errorf("Leaf(4) = %+v", n)
	}
	if split(Horizontal, 0.5, Leaf(1), Leaf(2)).IsLeaf() {
		t.Error("split reported as leaf")
	}
}

// sameTree compares trees with a tolerance on ratios.
func sameTree(a, b *Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.IsLeaf() != b.IsLeaf() {
		return false
	}
	if a.IsLeaf() {
		return a.Pane == b.Pane
	}
	d := a.Ratio - b.Ratio
	if a.Split != b.Split || d > 1e-9 || d < -1e-9 {
		return false
	}
	return sameTree(a.A, b.A) && sameTree(a.B, b.B)
}

func clone(n *Node) *Node {
	if n == nil {
		return nil
	}
	c := *n
	c.A, c.B = clone(n.A), clone(n.B)
	return &c
}

func assertUnchanged(t *testing.T, tree, orig *Node) {
	t.Helper()
	if !reflect.DeepEqual(tree, orig) {
		t.Errorf("receiver mutated: got %s, want %s", dump(tree), dump(orig))
	}
}

func dump(n *Node) string {
	if n == nil {
		return "nil"
	}
	if n.IsLeaf() {
		return "L" + strconv.Itoa(int(n.Pane))
	}
	o := "H"
	if n.Split == Vertical {
		o = "V"
	}
	return o + "(" + dump(n.A) + "," + dump(n.B) + ")"
}

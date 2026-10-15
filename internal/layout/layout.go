// Package layout is an immutable binary split tree of panes.
// Every operation returns a new tree and leaves the receiver intact.
package layout

type PaneID int

type Orientation int

const (
	// Horizontal puts children side by side: A left, B right.
	Horizontal Orientation = iota
	// Vertical stacks children: A top, B bottom.
	Vertical
)

type Direction int

const (
	Left Direction = iota
	Right
	Up
	Down
)

const (
	MinRatio = 0.1
	MaxRatio = 0.9
)

type Rect struct{ X, Y, W, H int }

// Branch picks a child of a split.
type Branch int

const (
	A Branch = iota
	B
)

// Path leads from the root to a split node.
type Path []Branch

// Node is a pane (leaf) or a split of two subtrees.
type Node struct {
	Pane  PaneID      // leaves
	Split Orientation // splits
	Ratio float64     // splits: share of A
	A, B  *Node       // nil on leaves
}

func Leaf(id PaneID) *Node { return &Node{Pane: id} }

func (n *Node) IsLeaf() bool { return n.A == nil && n.B == nil }

// Panes returns the leaves in order: left to right, top to bottom.
func (n *Node) Panes() []PaneID {
	if n.IsLeaf() {
		return []PaneID{n.Pane}
	}
	return append(n.A.Panes(), n.B.Panes()...)
}

func (n *Node) contains(id PaneID) bool {
	if n.IsLeaf() {
		return n.Pane == id
	}
	return n.A.contains(id) || n.B.contains(id)
}

// SplitPane turns the target leaf into a split of target and added.
func (n *Node) SplitPane(target, added PaneID, o Orientation) *Node {
	if n.IsLeaf() {
		if n.Pane != target {
			return n
		}
		return &Node{Split: o, Ratio: 0.5, A: Leaf(target), B: Leaf(added)}
	}
	if n.A.contains(target) {
		return n.withChildren(n.A.SplitPane(target, added, o), n.B)
	}
	return n.withChildren(n.A, n.B.SplitPane(target, added, o))
}

func (n *Node) withChildren(a, b *Node) *Node {
	c := *n
	c.A, c.B = a, b
	return &c
}

// Close removes the target leaf; its sibling takes the parent's place.
// Closing the only leaf returns nil.
func (n *Node) Close(target PaneID) *Node {
	if n.IsLeaf() {
		if n.Pane == target {
			return nil
		}
		return n
	}
	switch {
	case n.A.IsLeaf() && n.A.Pane == target:
		return n.B
	case n.B.IsLeaf() && n.B.Pane == target:
		return n.A
	case n.A.contains(target):
		return n.withChildren(n.A.Close(target), n.B)
	case n.B.contains(target):
		return n.withChildren(n.A, n.B.Close(target))
	}
	return n
}

// Rects returns each pane's rectangle; the panes tile area exactly.
func (n *Node) Rects(area Rect) map[PaneID]Rect {
	out := map[PaneID]Rect{}
	n.fill(area, out)
	return out
}

func (n *Node) fill(area Rect, out map[PaneID]Rect) {
	if n.IsLeaf() {
		out[n.Pane] = area
		return
	}
	a, b := n.areas(area)
	n.A.fill(a, out)
	n.B.fill(b, out)
}

// areas splits area between the children of a split.
func (n *Node) areas(area Rect) (a, b Rect) {
	a, b = area, area
	if n.Split == Horizontal {
		a.W = int(float64(area.W) * n.Ratio)
		b.X, b.W = area.X+a.W, area.W-a.W
	} else {
		a.H = int(float64(area.H) * n.Ratio)
		b.Y, b.H = area.Y+a.H, area.H-a.H
	}
	return a, b
}

// Boundary returns the split whose boundary holds cell (x, y): the last
// column of A or the first column of B for a Horizontal split, the last row
// of A or the first row of B for a Vertical one, within the split's area.
// The deepest split wins.
func (n *Node) Boundary(area Rect, x, y int) (Path, bool) {
	if n.IsLeaf() {
		return nil, false
	}
	a, b := n.areas(area)
	if p, ok := n.A.Boundary(a, x, y); ok {
		return append(Path{A}, p...), true
	}
	if p, ok := n.B.Boundary(b, x, y); ok {
		return append(Path{B}, p...), true
	}
	if n.onBoundary(area, a, b, x, y) {
		return Path{}, true
	}
	return nil, false
}

func (n *Node) onBoundary(area, a, b Rect, x, y int) bool {
	if n.Split == Horizontal {
		inRows := y >= area.Y && y < area.Y+area.H
		return inRows && (a.W > 0 && x == a.X+a.W-1 || b.W > 0 && x == b.X)
	}
	inCols := x >= area.X && x < area.X+area.W
	return inCols && (a.H > 0 && y == a.Y+a.H-1 || b.H > 0 && y == b.Y)
}

// SplitArea returns the area the split at p covers; false when p does not
// end at a split.
// SplitAt returns the split node at p; false when p does not end at a split.
func (n *Node) SplitAt(p Path) (*Node, bool) {
	for _, b := range p {
		if n.IsLeaf() {
			return nil, false
		}
		if b == A {
			n = n.A
		} else {
			n = n.B
		}
	}
	if n.IsLeaf() {
		return nil, false
	}
	return n, true
}

func (n *Node) SplitArea(p Path, area Rect) (Rect, bool) {
	if n.IsLeaf() {
		return Rect{}, false
	}
	if len(p) == 0 {
		return area, true
	}
	a, b := n.areas(area)
	if p[0] == A {
		return n.A.SplitArea(p[1:], a)
	}
	return n.B.SplitArea(p[1:], b)
}

// SetRatio sets the ratio of the split at p, clamped to [MinRatio, MaxRatio];
// a path that does not end at a split changes nothing.
func (n *Node) SetRatio(p Path, ratio float64) *Node {
	if n.IsLeaf() {
		return n
	}
	if len(p) == 0 {
		c := *n
		c.Ratio = clamp(ratio)
		return &c
	}
	if p[0] == A {
		return n.withChildren(n.A.SetRatio(p[1:], ratio), n.B)
	}
	return n.withChildren(n.A, n.B.SetRatio(p[1:], ratio))
}

// Neighbor returns the pane touching from's edge on side d, preferring the
// largest overlap, then the smaller Y (Left/Right) or X (Up/Down).
func (n *Node) Neighbor(from PaneID, d Direction, area Rect) (PaneID, bool) {
	rects := n.Rects(area)
	f, ok := rects[from]
	if !ok {
		return 0, false
	}
	var (
		best    PaneID
		bestOv  int
		bestPos int
	)
	for _, id := range n.Panes() {
		c := rects[id]
		ov, pos, touches := edge(f, c, d)
		if !touches || ov <= 0 {
			continue
		}
		if ov > bestOv || (ov == bestOv && pos < bestPos) {
			best, bestOv, bestPos = id, ov, pos
		}
	}
	return best, bestOv > 0
}

// edge reports whether c touches f on side d, its overlap with f along the
// other axis, and its position along that axis.
func edge(f, c Rect, d Direction) (overlap, pos int, touches bool) {
	switch d {
	case Left:
		touches = c.X+c.W == f.X
	case Right:
		touches = c.X == f.X+f.W
	case Up:
		touches = c.Y+c.H == f.Y
	case Down:
		touches = c.Y == f.Y+f.H
	}
	if d == Left || d == Right {
		return span(f.Y, f.H, c.Y, c.H), c.Y, touches
	}
	return span(f.X, f.W, c.X, c.W), c.X, touches
}

func span(a, alen, b, blen int) int {
	return min(a+alen, b+blen) - max(a, b)
}

// Resize moves the divider of the nearest ancestor split of target whose
// orientation is o, growing target by delta.
func (n *Node) Resize(target PaneID, o Orientation, delta float64) *Node {
	out, _ := n.resize(target, o, delta)
	return out
}

func (n *Node) resize(target PaneID, o Orientation, delta float64) (*Node, bool) {
	if n.IsLeaf() {
		return n, false
	}
	inA := n.A.contains(target)
	if !inA && !n.B.contains(target) {
		return n, false
	}
	child := n.B
	if inA {
		child = n.A
	}
	if resized, ok := child.resize(target, o, delta); ok {
		if inA {
			return n.withChildren(resized, n.B), true
		}
		return n.withChildren(n.A, resized), true
	}
	if n.Split != o {
		return n, false
	}
	c := *n
	if inA {
		c.Ratio = clamp(n.Ratio + delta)
	} else {
		c.Ratio = clamp(n.Ratio - delta)
	}
	return &c, true
}

func clamp(r float64) float64 { return min(max(r, MinRatio), MaxRatio) }

// Equalize gives every pane in a run of same-orientation splits the same size.
func (n *Node) Equalize() *Node {
	if n.IsLeaf() {
		return n
	}
	a, b := weight(n.A, n.Split), weight(n.B, n.Split)
	c := *n
	c.Ratio = clamp(a / (a + b))
	c.A, c.B = n.A.Equalize(), n.B.Equalize()
	return &c
}

// weight counts the panes of x along orientation o; a split of another
// orientation counts as one.
func weight(x *Node, o Orientation) float64 {
	if x.IsLeaf() || x.Split != o {
		return 1
	}
	return weight(x.A, o) + weight(x.B, o)
}

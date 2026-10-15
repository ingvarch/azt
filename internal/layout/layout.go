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
	a, b := area, area
	if n.Split == Horizontal {
		a.W = int(float64(area.W) * n.Ratio)
		b.X, b.W = area.X+a.W, area.W-a.W
	} else {
		a.H = int(float64(area.H) * n.Ratio)
		b.Y, b.H = area.Y+a.H, area.H-a.H
	}
	n.A.fill(a, out)
	n.B.fill(b, out)
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

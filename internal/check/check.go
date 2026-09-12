// Package check holds layout invariants shared by the layout tests.
package check

import (
	"testing"

	"github.com/daniel-widrick/diagram"
)

// Abs is |v|.
func Abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// Cross returns a point's coordinate on the cross axis for a direction.
func Cross(d diagram.Direction, p diagram.Point) float64 {
	if d.Transposed() {
		return p.Y
	}
	return p.X
}

// Rank returns a point's coordinate along the rank axis, increasing away
// from roots or sources regardless of direction.
func Rank(d diagram.Direction, size diagram.Size, p diagram.Point) float64 {
	switch d {
	case diagram.BottomUp:
		return size.H - p.Y
	case diagram.LeftRight:
		return p.X
	case diagram.RightLeft:
		return size.W - p.X
	default:
		return p.Y
	}
}

// RankSpan returns a rect's start and end on the rank axis.
func RankSpan(d diagram.Direction, size diagram.Size, r diagram.Rect) (start, end float64) {
	a := Rank(d, size, diagram.Point{X: r.X, Y: r.Y})
	b := Rank(d, size, diagram.Point{X: r.Right(), Y: r.Bottom()})
	if a > b {
		a, b = b, a
	}
	return a, b
}

// Layout verifies what any layout must satisfy: nodes inside the drawing
// and not overlapping, every line inside its node, edge endpoints on node
// borders, labels inside the drawing and clear of nodes.
func Layout(t *testing.T, g *diagram.Graph, l *diagram.Layout) {
	t.Helper()
	for i, a := range l.Nodes {
		if a.Rect.X < -0.01 || a.Rect.Y < -0.01 || a.Rect.Right() > l.Size.W+0.01 || a.Rect.Bottom() > l.Size.H+0.01 {
			t.Errorf("node %s outside drawing: %+v in %+v", a.ID, a.Rect, l.Size)
		}
		for _, b := range l.Nodes[i+1:] {
			if a.Rect.Overlaps(b.Rect) {
				t.Errorf("nodes %s and %s overlap: %+v %+v", a.ID, b.ID, a.Rect, b.Rect)
			}
		}
		for _, ln := range a.Lines {
			if ln.W > a.Rect.W-2*a.Padding+0.01 {
				t.Errorf("node %s line %q wider than node: %v > %v", a.ID, ln.Spans[0].Text, ln.W, a.Rect.W-2*a.Padding)
			}
		}
	}
	for _, e := range l.Edges {
		from, to := l.Node(e.From), l.Node(e.To)
		if len(e.Path) < 2 {
			t.Errorf("edge %s->%s has no path", e.From, e.To)
			continue
		}
		first, last := e.Path[0], e.Path[len(e.Path)-1]
		if !onBorder(from.Rect, first) {
			t.Errorf("edge %s->%s starts off its node border: %+v not on %+v", e.From, e.To, first, from.Rect)
		}
		if !onBorder(to.Rect, last) {
			t.Errorf("edge %s->%s ends off its node border: %+v not on %+v", e.From, e.To, last, to.Rect)
		}
		if e.Label != nil {
			bx := e.Label.Box
			if bx.X < -0.01 || bx.Y < -0.01 || bx.Right() > l.Size.W+0.01 || bx.Bottom() > l.Size.H+0.01 {
				t.Errorf("label %q outside drawing: %+v", e.Label.Text, bx)
			}
			if bx.W <= 0 || bx.H <= 0 {
				t.Errorf("label %q has empty box", e.Label.Text)
			}
			for _, n := range l.Nodes {
				if bx.Overlaps(n.Rect) {
					t.Errorf("label %q overlaps node %s", e.Label.Text, n.ID)
				}
			}
			if Abs(e.Label.Pos.X-bx.Center().X) > 0.01 || Abs(e.Label.Pos.Y-bx.Center().Y) > 0.01 {
				t.Errorf("label %q anchor not at box centre", e.Label.Text)
			}
		}
	}
}

func onBorder(r diagram.Rect, p diagram.Point) bool {
	const eps = 0.01
	inX := p.X >= r.X-eps && p.X <= r.Right()+eps
	inY := p.Y >= r.Y-eps && p.Y <= r.Bottom()+eps
	onV := Abs(p.X-r.X) < eps || Abs(p.X-r.Right()) < eps
	onH := Abs(p.Y-r.Y) < eps || Abs(p.Y-r.Bottom()) < eps
	return (onV && inY) || (onH && inX)
}

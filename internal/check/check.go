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

// Ports verifies that edges leaving or entering one node from one side do
// not share an endpoint, and that their order along the side matches the
// order of the other ends on the cross axis.
func Ports(t *testing.T, g *diagram.Graph, l *diagram.Layout) {
	t.Helper()
	type end struct {
		at    float64 // endpoint on the cross axis
		other float64 // where the edge heads, on the cross axis
		text  string
	}
	sides := map[string][]end{} // node id + side
	for _, e := range l.Edges {
		if e.From == e.To || len(e.Path) < 2 {
			continue
		}
		first, last := e.Path[0], e.Path[len(e.Path)-1]
		second, penult := e.Path[1], e.Path[len(e.Path)-2]
		fromKey := e.From + "/" + sideOf(g.Direction, l.Node(e.From).Rect, first)
		toKey := e.To + "/" + sideOf(g.Direction, l.Node(e.To).Rect, last)
		sides[fromKey] = append(sides[fromKey], end{Cross(g.Direction, first), Cross(g.Direction, second), e.From + "->" + e.To})
		sides[toKey] = append(sides[toKey], end{Cross(g.Direction, last), Cross(g.Direction, penult), e.From + "->" + e.To})
	}
	for key, ends := range sides {
		for i, a := range ends {
			for _, b := range ends[i+1:] {
				if Abs(a.at-b.at) < 0.01 {
					t.Errorf("edges %s and %s share endpoint %v on %s", a.text, b.text, a.at, key)
				}
				if (a.at < b.at) != (a.other <= b.other) && Abs(a.other-b.other) > 0.01 {
					t.Errorf("edges %s and %s leave %s in crossing order", a.text, b.text, key)
				}
			}
		}
	}
}

// EdgesClear verifies that no edge segment passes through a node or through
// another edge's label box, and, when the graph asks for orthogonal routing,
// that every segment is axis-aligned. Self loops are skipped.
func EdgesClear(t *testing.T, g *diagram.Graph, l *diagram.Layout) {
	t.Helper()
	for _, e := range l.Edges {
		if e.From == e.To {
			continue
		}
		for i := 0; i+1 < len(e.Path); i++ {
			a, b := e.Path[i], e.Path[i+1]
			if g.Routing == diagram.RoutingOrthogonal && Abs(a.X-b.X) > 0.01 && Abs(a.Y-b.Y) > 0.01 {
				t.Errorf("edge %s->%s segment %d not axis-aligned: %+v %+v", e.From, e.To, i, a, b)
			}
			for _, n := range l.Nodes {
				if segmentEntersRect(a, b, n.Rect) {
					t.Errorf("edge %s->%s segment %d enters node %s", e.From, e.To, i, n.ID)
				}
			}
			for _, o := range l.Edges {
				if o == e || o.Label == nil {
					continue
				}
				if segmentEntersRect(a, b, o.Label.Box) {
					t.Errorf("edge %s->%s segment %d crosses label %q", e.From, e.To, i, o.Label.Text)
				}
			}
		}
	}
}

// segmentEntersRect reports whether the segment ab has a point strictly
// inside r (touching the border does not count).
func segmentEntersRect(a, b diagram.Point, r diagram.Rect) bool {
	const eps = 0.05
	in := diagram.Rect{X: r.X + eps, Y: r.Y + eps, W: r.W - 2*eps, H: r.H - 2*eps}
	if in.W <= 0 || in.H <= 0 {
		return false
	}
	// Liang-Barsky clipping of the parametric segment against the rect.
	dx, dy := b.X-a.X, b.Y-a.Y
	t0, t1 := 0.0, 1.0
	for _, c := range [][2]float64{{-dx, a.X - in.X}, {dx, in.Right() - a.X}, {-dy, a.Y - in.Y}, {dy, in.Bottom() - a.Y}} {
		p, q := c[0], c[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		tt := q / p
		if p < 0 {
			if tt > t0 {
				t0 = tt
			}
		} else if tt < t1 {
			t1 = tt
		}
	}
	return t0 < t1
}

func sideOf(d diagram.Direction, r diagram.Rect, p diagram.Point) string {
	switch {
	case Abs(p.Y-r.Y) < 0.01:
		return "top"
	case Abs(p.Y-r.Bottom()) < 0.01:
		return "bottom"
	case Abs(p.X-r.X) < 0.01:
		return "left"
	default:
		return "right"
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

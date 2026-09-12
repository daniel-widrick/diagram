package tree

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/text"
)

func opts() diagram.Options {
	return diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
}

// checkInvariants verifies what any tree layout must satisfy.
func checkInvariants(t *testing.T, g *diagram.Graph, l *diagram.Layout) {
	t.Helper()
	// No two nodes overlap, and every node is inside the drawing.
	for i, a := range l.Nodes {
		if a.Rect.X < 0 || a.Rect.Y < 0 || a.Rect.Right() > l.Size.W+0.01 || a.Rect.Bottom() > l.Size.H+0.01 {
			t.Errorf("node %s outside drawing: %+v in %+v", a.ID, a.Rect, l.Size)
		}
		for _, b := range l.Nodes[i+1:] {
			if a.Rect.Overlaps(b.Rect) {
				t.Errorf("nodes %s and %s overlap: %+v %+v", a.ID, b.ID, a.Rect, b.Rect)
			}
		}
		// Every line fits inside its node.
		for _, ln := range a.Lines {
			if ln.W > a.Rect.W-2*a.Padding+0.01 {
				t.Errorf("node %s line %q wider than node: %v > %v", a.ID, ln.Spans[0].Text, ln.W, a.Rect.W-2*a.Padding)
			}
		}
	}
	// Parents are centred over their children; children are below (or above) parents.
	children := map[string][]*diagram.PlacedNode{}
	for _, e := range g.Edges {
		children[e.From] = append(children[e.From], l.Node(e.To))
	}
	for pid, kids := range children {
		p := l.Node(pid)
		lo, hi := kids[0].Rect.Center().X, kids[0].Rect.Center().X
		for _, k := range kids {
			cx := k.Rect.Center().X
			if cx < lo {
				lo = cx
			}
			if cx > hi {
				hi = cx
			}
			if g.Direction == diagram.BottomUp {
				if k.Rect.Bottom() > p.Rect.Y+0.01 {
					t.Errorf("child %s not above parent %s", k.ID, pid)
				}
			} else if k.Rect.Y < p.Rect.Bottom()-0.01 {
				t.Errorf("child %s not below parent %s", k.ID, pid)
			}
		}
		if mid := (lo + hi) / 2; abs(mid-p.Rect.Center().X) > 0.01 {
			t.Errorf("parent %s not centred over children: %v vs %v", pid, p.Rect.Center().X, mid)
		}
	}
	// Edge endpoints touch their nodes' borders; labels stay inside the drawing
	// and do not overlap any node.
	for _, e := range l.Edges {
		p, c := l.Node(e.From), l.Node(e.To)
		first, last := e.Path[0], e.Path[len(e.Path)-1]
		if g.Direction == diagram.BottomUp {
			if abs(first.Y-p.Rect.Y) > 0.01 || abs(last.Y-c.Rect.Bottom()) > 0.01 {
				t.Errorf("edge %s->%s endpoints off border", e.From, e.To)
			}
		} else if abs(first.Y-p.Rect.Bottom()) > 0.01 || abs(last.Y-c.Rect.Y) > 0.01 {
			t.Errorf("edge %s->%s endpoints off border", e.From, e.To)
		}
		if e.Label != nil {
			lr := diagram.Rect{X: e.Label.Pos.X, Y: e.Label.Pos.Y - e.Label.H/2, W: e.Label.W, H: e.Label.H}
			if lr.Right() > l.Size.W+0.01 {
				t.Errorf("label %q past right edge", e.Label.Text)
			}
			for _, n := range l.Nodes {
				if lr.Overlaps(n.Rect) {
					t.Errorf("label %q overlaps node %s", e.Label.Text, n.ID)
				}
			}
		}
	}
}

func planGraph() *diagram.Graph {
	bar := func(f float64) *float64 { return &f }
	return &diagram.Graph{
		Nodes: []*diagram.Node{
			{ID: "agg", Lines: []diagram.Line{diagram.L("title", "Aggregate"), diagram.L("muted", "0.05 ms · 0%")}, Bar: bar(0)},
			{ID: "sort", Lines: []diagram.Line{{{Text: "Sort ", Style: "title"}, {Text: "by p.name", Style: "muted"}}, diagram.L("muted", "0.3 ms · 0%")}, Bar: bar(0.01)},
			{ID: "nl", Lines: []diagram.Line{diagram.L("title", "Nested Loop"), diagram.L("muted", "0.1 ms · 0%")}, Bar: bar(0)},
			{ID: "idx", Lines: []diagram.Line{diagram.L("title", "Index Scan"), diagram.L("muted", "products_pkey · p.id = 42"), diagram.L("muted", "0.07 ms · 0%")}, Bar: bar(0)},
			{ID: "gather", Lines: []diagram.Line{{{Text: "Gather ", Style: "title"}, {Text: "2 workers", Style: "muted"}}, diagram.L("muted", "12.6 ms · 17%")}, Bar: bar(0.17)},
			{ID: "seq", Kind: "hot", Lines: []diagram.Line{{{Text: "Seq Scan ", Style: "title"}, {Text: "order_items", Style: "detail"}}, diagram.L("detail", "filter product_id = 42"), diagram.L("detail", "62.0 ms · 83% of time"), diagram.L("detail", "est 312 · actual 260 rows")}, Bar: bar(0.83)},
		},
		Edges: []*diagram.Edge{
			{From: "agg", To: "sort", Label: "781 rows", Weight: 0.5, Arrow: diagram.ArrowBackward},
			{From: "sort", To: "nl", Label: "781 rows", Weight: 0.5, Arrow: diagram.ArrowBackward},
			{From: "nl", To: "idx", Label: "1 row", Weight: 0.05, Arrow: diagram.ArrowBackward},
			{From: "nl", To: "gather", Label: "781 rows", Weight: 0.5, Arrow: diagram.ArrowBackward},
			{From: "gather", To: "seq", Label: "260 rows × 3 workers", Weight: 0.5, Arrow: diagram.ArrowBackward},
		},
	}
}

func TestPlanTree(t *testing.T) {
	g := planGraph()
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, g, l)
	if len(l.Nodes) != 6 || len(l.Edges) != 5 {
		t.Fatalf("counts: %d nodes %d edges", len(l.Nodes), len(l.Edges))
	}
	// The wide edge label on the left leaf must push the right leaf away.
	idx, gather := l.Node("idx"), l.Node("gather")
	if gather.Rect.X <= idx.Rect.Right() {
		t.Errorf("siblings too close: %+v %+v", idx.Rect, gather.Rect)
	}
	seq := l.Node("seq")
	if seq.BarRect == nil || seq.BarRect.W <= 0 {
		t.Error("bar rect missing on hot node")
	}
	for _, ln := range seq.Lines {
		if ln.Baseline <= 0 || ln.Baseline > seq.Rect.H {
			t.Errorf("baseline outside node: %v", ln.Baseline)
		}
	}
}

func TestBottomUp(t *testing.T) {
	g := planGraph()
	g.Direction = diagram.BottomUp
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, g, l)
	if l.Node("agg").Rect.Y <= l.Node("seq").Rect.Y {
		t.Error("root should be at the bottom in BottomUp")
	}
}

func TestOverflowPolicies(t *testing.T) {
	long := "a_very_long_identifier_that_goes_on_and_on_and_on_and_on_for_a_while"
	g := &diagram.Graph{
		Nodes: []*diagram.Node{
			{ID: "grow", Lines: []diagram.Line{diagram.L("detail", long)}},
			{ID: "ell", Lines: []diagram.Line{diagram.L("detail", long)}, MaxWidth: 160, Overflow: diagram.Ellipsize},
			{ID: "wrap", Lines: []diagram.Line{diagram.L("detail", "several words that should wrap onto more than one line inside the node")}, MaxWidth: 160, Overflow: diagram.Wrap},
		},
		Edges: []*diagram.Edge{{From: "grow", To: "ell"}, {From: "grow", To: "wrap"}},
	}
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, g, l)
	if l.Node("grow").Rect.W <= 160 {
		t.Error("grow node should be wide")
	}
	ell := l.Node("ell")
	if ell.Rect.W > 160.01 || ell.Lines[0].Spans[0].Text == long || ell.Lines[0].Spans[0].Full != long {
		t.Errorf("ellipsize: %+v", ell.Lines[0].Spans[0])
	}
	wrap := l.Node("wrap")
	if wrap.Rect.W > 160.01 || len(wrap.Lines) < 3 {
		t.Errorf("wrap: width %v lines %d", wrap.Rect.W, len(wrap.Lines))
	}
}

func TestForestAndErrors(t *testing.T) {
	g := &diagram.Graph{Nodes: []*diagram.Node{{ID: "a", Lines: []diagram.Line{diagram.L("", "a")}}, {ID: "b", Lines: []diagram.Line{diagram.L("", "b")}}}}
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, g, l)
	if l.Node("a").Rect.Y != l.Node("b").Rect.Y {
		t.Error("two roots should share a level")
	}
	bad := &diagram.Graph{Nodes: g.Nodes, Edges: []*diagram.Edge{{From: "a", To: "b"}, {From: "b", To: "a"}}}
	if _, err := Layout(bad, opts()); err == nil {
		t.Error("expected error for two parents / cycle")
	}
	if _, err := Layout(&diagram.Graph{Nodes: g.Nodes, Edges: []*diagram.Edge{{From: "a", To: "zz"}}}, opts()); err == nil {
		t.Error("expected error for unknown node")
	}
}

// TestRandomTrees hammers the invariants on random shapes and sizes.
func TestRandomTrees(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for iter := 0; iter < 60; iter++ {
		n := 2 + rng.Intn(40)
		g := &diagram.Graph{}
		for i := 0; i < n; i++ {
			lines := []diagram.Line{diagram.L("title", fmt.Sprintf("n%d %s", i, strings.Repeat("x", 1+rng.Intn(12))))}
			for j := rng.Intn(3); j > 0; j-- {
				lines = append(lines, diagram.L("muted", fmt.Sprintf("%0*d", 1+rng.Intn(30), j)))
			}
			g.Nodes = append(g.Nodes, &diagram.Node{ID: fmt.Sprint(i), Lines: lines})
			if i > 0 {
				e := &diagram.Edge{From: fmt.Sprint(rng.Intn(i)), To: fmt.Sprint(i)}
				if rng.Intn(2) == 0 {
					e.Label = fmt.Sprintf("%d rows", rng.Intn(100000))
				}
				g.Edges = append(g.Edges, e)
			}
		}
		if rng.Intn(2) == 0 {
			g.Direction = diagram.BottomUp
		}
		l, err := Layout(g, opts())
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		checkInvariants(t, g, l)
	}
}

package layered

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/internal/check"
	"github.com/daniel-widrick/diagram/text"
)

func opts() diagram.Options {
	return diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
}

func mk(id string, lines ...string) *diagram.Node {
	n := &diagram.Node{ID: id}
	for i, l := range lines {
		style := "detail"
		if i == 0 {
			style = "title"
		}
		n.Lines = append(n.Lines, diagram.L(style, l))
	}
	return n
}

// joinGraph is the shop schema join graph from pginspect.
func joinGraph() *diagram.Graph {
	return &diagram.Graph{
		Nodes: []*diagram.Node{
			mk("customers", "customers", "Index Scan · pkey", "1 row"),
			mk("orders", "orders", "Index Scan · customer_id", "8,109 rows"),
			{ID: "order_items", Kind: "hot", Lines: []diagram.Line{diagram.L("title", "order_items"), diagram.L("detail", "Seq Scan · 750,000 rows"), diagram.L("detail", "83% of time")}},
			mk("products", "products", "Index Scan · pkey", "1 row"),
			mk("categories", "categories", "Index Scan · pkey"),
		},
		Edges: []*diagram.Edge{
			{From: "customers", To: "orders", Label: "o.customer_id = c.id", Arrow: diagram.ArrowNone},
			{From: "orders", To: "order_items", Label: "oi.order_id = o.id", Arrow: diagram.ArrowNone},
			{From: "products", To: "order_items", Label: "oi.product_id = p.id", Kind: "weak", Arrow: diagram.ArrowNone},
			{From: "categories", To: "products", Label: "p.category_id = cat.id", Arrow: diagram.ArrowNone},
		},
	}
}

// checkLayered adds layered-specific invariants: every non-reversed edge
// runs forward along the rank axis, reversed ones backward, and labels do
// not overlap each other.
func checkLayered(t *testing.T, g *diagram.Graph, l *diagram.Layout) {
	t.Helper()
	check.Layout(t, g, l)
	check.Ports(t, g, l)
	for _, e := range l.Edges {
		if e.From == e.To {
			continue
		}
		from, to := l.Node(e.From), l.Node(e.To)
		_, fromEnd := check.RankSpan(g.Direction, l.Size, from.Rect)
		toStart, _ := check.RankSpan(g.Direction, l.Size, to.Rect)
		if !e.Reversed && toStart < fromEnd-0.01 {
			t.Errorf("edge %s->%s does not run forward: from ends %v, to starts %v", e.From, e.To, fromEnd, toStart)
		}
		if e.Reversed {
			toEnd := toStart + 0
			_, toEnd = check.RankSpan(g.Direction, l.Size, to.Rect)
			fromStart, _ := check.RankSpan(g.Direction, l.Size, from.Rect)
			if fromStart < toEnd-0.01 {
				t.Errorf("reversed edge %s->%s should run backward", e.From, e.To)
			}
		}
	}
	for i, a := range l.Edges {
		if a.Label == nil {
			continue
		}
		for _, b := range l.Edges[i+1:] {
			if b.Label != nil && a.Label.Box.Overlaps(b.Label.Box) {
				t.Errorf("labels %q and %q overlap", a.Label.Text, b.Label.Text)
			}
		}
	}
}

func TestJoinGraph(t *testing.T) {
	for _, dir := range []diagram.Direction{diagram.TopDown, diagram.LeftRight, diagram.BottomUp, diagram.RightLeft} {
		g := joinGraph()
		g.Direction = dir
		l, err := Layout(g, opts())
		if err != nil {
			t.Fatal(err)
		}
		checkLayered(t, g, l)
		if len(l.Nodes) != 5 || len(l.Edges) != 4 {
			t.Fatalf("counts: %d nodes %d edges", len(l.Nodes), len(l.Edges))
		}
		for _, e := range l.Edges {
			if e.Label == nil || !e.Label.Background || !e.Curved {
				t.Errorf("edge %s->%s: expected boxed label on a curved edge", e.From, e.To)
			}
		}
		// order_items is two ranks below customers (customers -> orders -> order_items).
		if l.Node("order_items").Depth != l.Node("customers").Depth+2 {
			t.Errorf("depths: %d vs %d", l.Node("order_items").Depth, l.Node("customers").Depth)
		}
		// categories feeds products which feeds order_items, so it sits one
		// rank above products, not at the top.
		if l.Node("categories").Depth != l.Node("products").Depth-1 {
			t.Errorf("source pulled toward target: categories %d products %d", l.Node("categories").Depth, l.Node("products").Depth)
		}
	}
}

func TestCyclesAndLoops(t *testing.T) {
	g := &diagram.Graph{
		Nodes: []*diagram.Node{mk("a", "a"), mk("b", "b"), mk("c", "c"), mk("d", "d")},
		Edges: []*diagram.Edge{
			{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "a", Label: "back"},
			{From: "c", To: "d"}, {From: "d", To: "d", Label: "self"},
			{From: "a", To: "b", Label: "second"}, // multi-edge
		},
	}
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkLayered(t, g, l)
	reversed := 0
	for _, e := range l.Edges {
		if e.Reversed {
			reversed++
		}
	}
	if reversed != 1 {
		t.Errorf("expected exactly one reversed edge, got %d", reversed)
	}
	// The self loop must leave and re-enter its own node.
	for _, e := range l.Edges {
		if e.From == "d" && e.To == "d" && len(e.Path) < 3 {
			t.Error("self loop path too short")
		}
	}
}

func TestLongEdgesAreRouted(t *testing.T) {
	g := &diagram.Graph{
		Nodes: []*diagram.Node{mk("s", "source"), mk("m1", "middle one"), mk("m2", "middle two"), mk("t", "target")},
		Edges: []*diagram.Edge{
			{From: "s", To: "m1"}, {From: "m1", To: "m2"}, {From: "m2", To: "t"},
			{From: "s", To: "t", Label: "skips two ranks"},
		},
	}
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkLayered(t, g, l)
	var long *diagram.PlacedEdge
	for _, e := range l.Edges {
		if e.From == "s" && e.To == "t" {
			long = e
		}
	}
	if long == nil || len(long.Path) < 5 {
		t.Fatalf("long edge should pass through dummies: %+v", long)
	}
	// Brandes-Köpf keeps the dummies of a long edge vertically aligned.
	for _, p := range long.Path[1 : len(long.Path)-1] {
		if check.Abs(p.X-long.Path[1].X) > 0.01 {
			t.Errorf("long edge not straight through dummies: %v", long.Path)
			break
		}
	}
	// The long edge must not pass through the middle nodes.
	for _, p := range long.Path[1 : len(long.Path)-1] {
		for _, id := range []string{"m1", "m2"} {
			r := l.Node(id).Rect
			if p.X > r.X && p.X < r.Right() && p.Y > r.Y && p.Y < r.Bottom() {
				t.Errorf("long edge passes through %s at %+v", id, p)
			}
		}
	}
}

func TestCrossingReduction(t *testing.T) {
	// Two sources each feeding two targets in swapped input order: the
	// initial order has crossings; the sweep should remove them all.
	g := &diagram.Graph{
		Nodes: []*diagram.Node{mk("a", "a"), mk("b", "b"), mk("y", "y"), mk("x", "x")},
		Edges: []*diagram.Edge{{From: "a", To: "x"}, {From: "b", To: "y"}},
	}
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkLayered(t, g, l)
	a, b, x, y := l.Node("a"), l.Node("b"), l.Node("x"), l.Node("y")
	if (a.Rect.X < b.Rect.X) != (x.Rect.X < y.Rect.X) {
		t.Error("crossing not removed")
	}
}

func TestRandomGraphs(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	dirs := []diagram.Direction{diagram.TopDown, diagram.BottomUp, diagram.LeftRight, diagram.RightLeft}
	for iter := 0; iter < 60; iter++ {
		n := 2 + rng.Intn(25)
		g := &diagram.Graph{Direction: dirs[iter%4]}
		for i := 0; i < n; i++ {
			g.Nodes = append(g.Nodes, mk(fmt.Sprint(i), fmt.Sprintf("node %d", i), fmt.Sprintf("%0*d", 1+rng.Intn(20), i)))
		}
		m := n + rng.Intn(n*2)
		for j := 0; j < m; j++ {
			e := &diagram.Edge{From: fmt.Sprint(rng.Intn(n)), To: fmt.Sprint(rng.Intn(n))}
			if rng.Intn(3) == 0 {
				e.Label = fmt.Sprintf("e%d", j)
			}
			g.Edges = append(g.Edges, e)
		}
		l, err := Layout(g, opts())
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		checkLayered(t, g, l)
	}
}

func TestPortsSpread(t *testing.T) {
	g := &diagram.Graph{
		Nodes: []*diagram.Node{mk("hub", "hub node"), mk("a", "a"), mk("b", "b"), mk("c", "c"), mk("d", "d")},
		Edges: []*diagram.Edge{{From: "hub", To: "a"}, {From: "hub", To: "b"}, {From: "hub", To: "c"}, {From: "hub", To: "d"}, {From: "a", To: "d"}, {From: "b", To: "d"}},
	}
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkLayered(t, g, l)
	hub := l.Node("hub")
	starts := map[float64]bool{}
	for _, e := range l.Edges {
		if e.From == "hub" {
			x := e.Path[0].X
			if x <= hub.Rect.X || x >= hub.Rect.Right() {
				t.Errorf("port outside node: %v not in %+v", x, hub.Rect)
			}
			starts[x] = true
		}
	}
	if len(starts) != 4 {
		t.Errorf("expected 4 distinct ports on hub, got %d", len(starts))
	}
}

func TestStraightChain(t *testing.T) {
	// A simple chain must be a straight vertical line.
	g := &diagram.Graph{
		Nodes: []*diagram.Node{mk("a", "aaaa"), mk("b", "b"), mk("c", "cccccccc"), mk("d", "dd")},
		Edges: []*diagram.Edge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "d"}},
	}
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkLayered(t, g, l)
	x := l.Node("a").Rect.Center().X
	for _, id := range []string{"b", "c", "d"} {
		if check.Abs(l.Node(id).Rect.Center().X-x) > 0.01 {
			t.Errorf("chain not straight: %s at %v vs %v", id, l.Node(id).Rect.Center().X, x)
		}
	}
}

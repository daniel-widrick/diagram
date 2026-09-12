package tree

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/internal/check"
	"github.com/daniel-widrick/diagram/text"
)

func opts() diagram.Options {
	return diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
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

// checkTree adds tree-specific invariants to the shared ones: parents are
// centred over their children on the cross axis.
func checkTree(t *testing.T, g *diagram.Graph, l *diagram.Layout) {
	t.Helper()
	check.Layout(t, g, l)
	check.Ports(t, g, l)
	check.EdgesClear(t, g, l)
	check.Groups(t, g, l)
	if len(g.Groups) > 0 {
		// Group boxes may cut through subtrees; centring cannot hold then.
		return
	}
	children := map[string][]*diagram.PlacedNode{}
	for _, e := range g.Edges {
		if c := l.Node(e.To); c != nil && l.Node(e.From) != nil && !l.Node(e.From).Collapsed {
			children[e.From] = append(children[e.From], c)
		}
	}
	for pid, kids := range children {
		p := l.Node(pid)
		lo, hi := check.Cross(g.Direction, kids[0].Rect.Center()), check.Cross(g.Direction, kids[0].Rect.Center())
		for _, k := range kids {
			c := check.Cross(g.Direction, k.Rect.Center())
			if c < lo {
				lo = c
			}
			if c > hi {
				hi = c
			}
		}
		if mid := (lo + hi) / 2; check.Abs(mid-check.Cross(g.Direction, p.Rect.Center())) > 0.01 {
			t.Errorf("parent %s not centred over children: %v vs %v", pid, check.Cross(g.Direction, p.Rect.Center()), mid)
		}
	}
}

func TestPlanTree(t *testing.T) {
	g := planGraph()
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	checkTree(t, g, l)
	if len(l.Nodes) != 6 || len(l.Edges) != 5 {
		t.Fatalf("counts: %d nodes %d edges", len(l.Nodes), len(l.Edges))
	}
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

func TestDirections(t *testing.T) {
	for _, dir := range []diagram.Direction{diagram.TopDown, diagram.BottomUp, diagram.LeftRight, diagram.RightLeft} {
		g := planGraph()
		g.Direction = dir
		l, err := Layout(g, opts())
		if err != nil {
			t.Fatal(err)
		}
		checkTree(t, g, l)
		root, leaf := l.Node("agg"), l.Node("seq")
		switch dir {
		case diagram.TopDown:
			if root.Rect.Y >= leaf.Rect.Y {
				t.Error("TopDown: root should be above")
			}
		case diagram.BottomUp:
			if root.Rect.Y <= leaf.Rect.Y {
				t.Error("BottomUp: root should be below")
			}
		case diagram.LeftRight:
			if root.Rect.X >= leaf.Rect.X {
				t.Error("LeftRight: root should be left")
			}
			if l.Size.W <= l.Size.H {
				t.Errorf("LeftRight plan should be wider than tall: %+v", l.Size)
			}
		case diagram.RightLeft:
			if root.Rect.X <= leaf.Rect.X {
				t.Error("RightLeft: root should be right")
			}
		}
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
	checkTree(t, g, l)
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
	checkTree(t, g, l)
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

func TestRandomTrees(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	dirs := []diagram.Direction{diagram.TopDown, diagram.BottomUp, diagram.LeftRight, diagram.RightLeft}
	for iter := 0; iter < 80; iter++ {
		n := 2 + rng.Intn(40)
		g := &diagram.Graph{Direction: dirs[iter%4]}
		if iter%3 == 0 {
			g.Groups = []diagram.Group{{ID: "g1", Label: "group one"}, {ID: "g2", Label: "two"}}
		}
		for i := 0; i < n; i++ {
			lines := []diagram.Line{diagram.L("title", fmt.Sprintf("n%d %s", i, strings.Repeat("x", 1+rng.Intn(12))))}
			for j := rng.Intn(3); j > 0; j-- {
				lines = append(lines, diagram.L("muted", fmt.Sprintf("%0*d", 1+rng.Intn(30), j)))
			}
			nd := &diagram.Node{ID: fmt.Sprint(i), Lines: lines, Collapsed: rng.Intn(8) == 0}
			if len(g.Groups) > 0 && rng.Intn(3) > 0 {
				nd.Group = g.Groups[rng.Intn(2)].ID
			}
			g.Nodes = append(g.Nodes, nd)
			if i > 0 {
				e := &diagram.Edge{From: fmt.Sprint(rng.Intn(i)), To: fmt.Sprint(i)}
				if rng.Intn(2) == 0 {
					e.Label = fmt.Sprintf("%d rows", rng.Intn(100000))
				}
				g.Edges = append(g.Edges, e)
			}
		}
		l, err := Layout(g, opts())
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		checkTree(t, g, l)
		checkHidden(t, g, l)
	}
}

// checkHidden verifies collapsed nodes hide exactly their subtrees.
func checkHidden(t *testing.T, g *diagram.Graph, l *diagram.Layout) {
	t.Helper()
	hidden := map[string]bool{}
	for _, id := range l.HiddenNodes {
		hidden[id] = true
	}
	for _, n := range l.Nodes {
		if hidden[n.ID] {
			t.Errorf("hidden node %s was placed", n.ID)
		}
	}
	for _, e := range g.Edges {
		if hidden[e.To] && !hidden[e.From] && !l.Node(e.From).Collapsed {
			t.Errorf("node %s hidden though parent %s is visible and not collapsed", e.To, e.From)
		}
		if !hidden[e.To] && l.Node(e.From) == nil {
			t.Errorf("node %s visible though parent %s is hidden", e.To, e.From)
		}
	}
}

func TestParentPorts(t *testing.T) {
	g := planGraph()
	l, err := Layout(g, opts())
	if err != nil {
		t.Fatal(err)
	}
	var toIdx, toGather diagram.Point
	for _, e := range l.Edges {
		if e.From == "nl" && e.To == "idx" {
			toIdx = e.Path[0]
		}
		if e.From == "nl" && e.To == "gather" {
			toGather = e.Path[0]
		}
	}
	if toIdx.X >= toGather.X {
		t.Errorf("ports not ordered like children: %v vs %v", toIdx, toGather)
	}
	nl := l.Node("nl").Rect
	if toIdx.X <= nl.X || toGather.X >= nl.Right() || toGather.X-toIdx.X > diagram.PortSpacing+0.01 {
		t.Errorf("ports misplaced: %v %v in %+v", toIdx, toGather, nl)
	}
}

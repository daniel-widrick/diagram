package diagram

import "testing"

func TestCollapse(t *testing.T) {
	n := func(id string, collapsed bool) *Node { return &Node{ID: id, Collapsed: collapsed} }
	g := &Graph{
		Nodes: []*Node{n("root", false), n("a", true), n("a1", false), n("a2", false), n("shared", false), n("b", false), n("b1", false)},
		Edges: []*Edge{{From: "root", To: "a"}, {From: "root", To: "b"}, {From: "a", To: "a1"}, {From: "a", To: "a2"},
			{From: "a1", To: "shared"}, {From: "b", To: "shared"}, {From: "b", To: "b1"}},
	}
	hidden, counts := Collapse(g)
	if len(hidden) != 2 || hidden[0] != "a1" || hidden[1] != "a2" {
		t.Errorf("hidden: %v", hidden)
	}
	if counts["a"] != 2 {
		t.Errorf("count: %v", counts)
	}
	// "shared" stays: b still reaches it.
	v, _, _ := Visible(g)
	if len(v.Nodes) != 5 || len(v.Edges) != 4 {
		t.Errorf("visible: %d nodes %d edges", len(v.Nodes), len(v.Edges))
	}
	// Collapsing b too hides shared and b1.
	g.Nodes[5].Collapsed = true
	hidden, counts = Collapse(g)
	if len(hidden) != 4 || counts["b"] != 2 || counts["a"] != 3 {
		t.Errorf("hidden %v counts %v", hidden, counts)
	}
}

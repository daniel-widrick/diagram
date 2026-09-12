package diagram

// Collapse computes which nodes a set of collapsed nodes hides. A node is
// hidden when every edge into it comes from a hidden node or a collapsed
// node, so anything still reachable from a visible node stays visible.
// Sources (no incoming edges) are never hidden. The result maps each
// collapsed node id to the number of hidden nodes reachable from it, and
// returns the hidden ids in input order.
func Collapse(g *Graph) (hidden []string, counts map[string]int) {
	collapsed := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Collapsed {
			collapsed[n.ID] = true
		}
	}
	counts = map[string]int{}
	if len(collapsed) == 0 {
		return nil, counts
	}
	preds := map[string][]string{}
	succs := map[string][]string{}
	for _, e := range g.Edges {
		if e.From == e.To {
			continue
		}
		preds[e.To] = append(preds[e.To], e.From)
		succs[e.From] = append(succs[e.From], e.To)
	}
	hiddenSet := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, n := range g.Nodes {
			// A collapsed node can itself be hidden by a collapsed ancestor.
			if hiddenSet[n.ID] || len(preds[n.ID]) == 0 {
				continue
			}
			all := true
			for _, p := range preds[n.ID] {
				if !hiddenSet[p] && !collapsed[p] {
					all = false
					break
				}
			}
			if all {
				hiddenSet[n.ID] = true
				changed = true
			}
		}
	}
	for _, n := range g.Nodes {
		if hiddenSet[n.ID] {
			hidden = append(hidden, n.ID)
		}
	}
	for id := range collapsed {
		seen := map[string]bool{}
		var walk func(string)
		walk = func(v string) {
			for _, s := range succs[v] {
				if hiddenSet[s] && !seen[s] {
					seen[s] = true
					walk(s)
				}
			}
		}
		walk(id)
		counts[id] = len(seen)
	}
	return hidden, counts
}

// Visible returns a copy of the graph without hidden nodes and without
// edges touching them, plus the hidden ids and per-collapsed-node counts.
func Visible(g *Graph) (*Graph, []string, map[string]int) {
	hidden, counts := Collapse(g)
	if len(hidden) == 0 {
		return g, nil, counts
	}
	hiddenSet := map[string]bool{}
	for _, id := range hidden {
		hiddenSet[id] = true
	}
	out := *g
	out.Nodes = nil
	out.Edges = nil
	for _, n := range g.Nodes {
		if !hiddenSet[n.ID] {
			out.Nodes = append(out.Nodes, n)
		}
	}
	for _, e := range g.Edges {
		if !hiddenSet[e.From] && !hiddenSet[e.To] {
			out.Edges = append(out.Edges, e)
		}
	}
	return &out, hidden, counts
}

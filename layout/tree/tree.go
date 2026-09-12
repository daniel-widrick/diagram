// Package tree lays out rooted trees (or forests) with the Buchheim, Jünger
// and Leipert linear-time refinement of Walker's algorithm, extended to nodes
// of different widths. Parents are centred over their children, siblings keep
// the configured separation, and subtrees are packed as tightly as their
// contours allow.
//
// Edges must run parent to child (Edge.From is the parent). The arrow
// direction is independent and comes from Edge.Arrow.
package tree

import (
	"fmt"
	"sort"

	"github.com/daniel-widrick/diagram"
)

type node struct {
	pn       *diagram.PlacedNode
	parent   *node
	children []*node
	number   int // index among siblings
	depth    int

	// Buchheim state
	prelim, mod, shift, change float64
	thread                     *node
	ancestor                   *node

	x float64 // final centre x

	// space needed to the right for an incoming edge label
	labelW float64
}

// Layout positions the graph as a tree. Nodes with no incoming edge are
// roots; several roots are laid out side by side.
func Layout(g *diagram.Graph, o diagram.Options) (*diagram.Layout, error) {
	if len(g.Nodes) == 0 {
		return &diagram.Layout{}, nil
	}
	rankSep, nodeSep, margin := g.RankSep, g.NodeSep, g.Margin
	if rankSep <= 0 {
		rankSep = 40
	}
	if nodeSep <= 0 {
		nodeSep = 24
	}
	if margin <= 0 {
		margin = 16
	}

	// Measure.
	byID := map[string]*node{}
	var all []*node
	for _, n := range g.Nodes {
		if _, dup := byID[n.ID]; dup {
			return nil, fmt.Errorf("tree: duplicate node id %q", n.ID)
		}
		pn, err := diagram.MeasureNode(n, o)
		if err != nil {
			return nil, err
		}
		nd := &node{pn: pn}
		nd.ancestor = nd
		byID[n.ID] = nd
		all = append(all, nd)
	}

	// Build hierarchy in edge order.
	type edgeInfo struct {
		edge           *diagram.Edge
		labelW, labelH float64
		labelAscent    float64
	}
	edges := map[*node]*edgeInfo{} // by child
	for _, e := range g.Edges {
		p, ok := byID[e.From]
		if !ok {
			return nil, fmt.Errorf("tree: edge from unknown node %q", e.From)
		}
		c, ok := byID[e.To]
		if !ok {
			return nil, fmt.Errorf("tree: edge to unknown node %q", e.To)
		}
		if c.parent != nil {
			return nil, fmt.Errorf("tree: node %q has two parents (%q and %q)", e.To, c.parent.pn.ID, e.From)
		}
		c.parent = p
		p.children = append(p.children, c)
		info := &edgeInfo{edge: e}
		if e.Label != "" {
			w, h, a, err := diagram.MeasureLabel(e.Label, e.LabelStyle, o)
			if err != nil {
				return nil, err
			}
			info.labelW, info.labelH, info.labelAscent = w, h, a
			c.labelW = w + labelGap
		}
		edges[c] = info
	}
	var roots []*node
	for _, nd := range all {
		if nd.parent == nil {
			roots = append(roots, nd)
		}
	}
	// Cycle check: every node must reach a root.
	for _, nd := range all {
		seen := 0
		for p := nd; p != nil; p = p.parent {
			seen++
			if seen > len(all) {
				return nil, fmt.Errorf("tree: cycle through node %q", nd.pn.ID)
			}
		}
	}

	// A virtual root makes a forest a tree.
	virtual := &node{pn: &diagram.PlacedNode{Node: &diagram.Node{ID: "\x00root"}}}
	virtual.ancestor = virtual
	virtual.children = roots
	for _, r := range roots {
		r.parent = virtual
	}
	number(virtual, -1)

	sep := nodeSep
	firstWalk(virtual, sep)
	secondWalk(virtual, -virtual.prelim)

	// Levels: depth -> max height.
	levelH := map[int]float64{}
	maxDepth := 0
	for _, nd := range all {
		if h := nd.pn.Rect.H; h > levelH[nd.depth] {
			levelH[nd.depth] = h
		}
		if nd.depth > maxDepth {
			maxDepth = nd.depth
		}
	}
	levelY := make([]float64, maxDepth+1)
	y := margin
	for d := 0; d <= maxDepth; d++ {
		levelY[d] = y
		y += levelH[d] + rankSep
	}
	totalH := y - rankSep + margin

	// Shift so the leftmost extent (nodes or labels) sits at the margin.
	minX, maxX := 0.0, 0.0
	for i, nd := range all {
		l := nd.x - nd.pn.Rect.W/2
		r := nd.x + nd.pn.Rect.W/2
		if info := edges[nd]; info != nil && info.labelW > 0 {
			if lr := nd.x + labelGap + info.labelW; lr > r {
				r = lr
			}
		}
		if i == 0 || l < minX {
			minX = l
		}
		if i == 0 || r > maxX {
			maxX = r
		}
	}
	dx := margin - minX
	totalW := maxX - minX + 2*margin

	out := &diagram.Layout{Size: diagram.Size{W: totalW, H: totalH}}
	for _, nd := range all {
		pn := nd.pn
		pn.Depth = nd.depth
		pn.Rect.X = nd.x + dx - pn.Rect.W/2
		pn.Rect.Y = levelY[nd.depth]
		if g.Direction == diagram.BottomUp {
			pn.Rect.Y = totalH - pn.Rect.Y - pn.Rect.H
		}
		if pn.BarRect != nil {
			pn.BarRect.X += pn.Rect.X
			pn.BarRect.Y += pn.Rect.Y
		}
		out.Nodes = append(out.Nodes, pn)
	}
	// Edges in input order.
	for _, e := range g.Edges {
		c := byID[e.To]
		p := c.parent
		info := edges[c]
		pe := &diagram.PlacedEdge{Edge: e}
		pr, cr := p.pn.Rect, c.pn.Rect
		if g.Direction == diagram.BottomUp {
			// Parent sits below the child.
			mid := (cr.Bottom() + pr.Y) / 2
			pe.Path = []diagram.Point{
				{X: pr.X + pr.W/2, Y: pr.Y},
				{X: pr.X + pr.W/2, Y: mid},
				{X: cr.X + cr.W/2, Y: mid},
				{X: cr.X + cr.W/2, Y: cr.Bottom()},
			}
			if info.labelW > 0 {
				pe.Label = &diagram.PlacedLabel{
					Text: e.Label, Style: labelStyle(e), Anchor: "start",
					Pos:      diagram.Point{X: cr.X + cr.W/2 + labelGap, Y: cr.Bottom() + (mid-cr.Bottom())/2},
					Baseline: info.labelAscent - info.labelH/2, W: info.labelW, H: info.labelH,
				}
			}
		} else {
			mid := (pr.Bottom() + cr.Y) / 2
			pe.Path = []diagram.Point{
				{X: pr.X + pr.W/2, Y: pr.Bottom()},
				{X: pr.X + pr.W/2, Y: mid},
				{X: cr.X + cr.W/2, Y: mid},
				{X: cr.X + cr.W/2, Y: cr.Y},
			}
			if info.labelW > 0 {
				pe.Label = &diagram.PlacedLabel{
					Text: e.Label, Style: labelStyle(e), Anchor: "start",
					Pos:      diagram.Point{X: cr.X + cr.W/2 + labelGap, Y: mid + (cr.Y-mid)/2},
					Baseline: info.labelAscent - info.labelH/2, W: info.labelW, H: info.labelH,
				}
			}
		}
		// Collapse the elbow when parent and child are vertically aligned.
		if abs(pe.Path[0].X-pe.Path[3].X) < 0.5 {
			pe.Path = []diagram.Point{pe.Path[0], pe.Path[3]}
		}
		out.Edges = append(out.Edges, pe)
	}
	sort.SliceStable(out.Nodes, func(i, j int) bool { return out.Nodes[i].Depth < out.Nodes[j].Depth })
	return out, nil
}

// labelGap is the horizontal gap between an edge's vertical segment and its label.
const labelGap = 6

func labelStyle(e *diagram.Edge) string {
	if e.LabelStyle != "" {
		return e.LabelStyle
	}
	return "edge"
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func number(v *node, depth int) {
	v.depth = depth
	for i, c := range v.children {
		c.number = i
		number(c, depth+1)
	}
}

// distance is the required gap between the centres of two adjacent nodes.
// The left node's incoming edge label must clear the right node's edge.
func distance(left, right *node, sep float64) float64 {
	d := left.pn.Rect.W/2 + sep + right.pn.Rect.W/2
	if lw := left.labelW + labelGap; lw > d {
		d = lw
	}
	return d
}

func leftSibling(v *node) *node {
	if v.parent == nil || v.number == 0 {
		return nil
	}
	return v.parent.children[v.number-1]
}

func leftmostSibling(v *node) *node {
	if v.parent == nil {
		return nil
	}
	return v.parent.children[0]
}

func nextLeft(v *node) *node {
	if len(v.children) > 0 {
		return v.children[0]
	}
	return v.thread
}

func nextRight(v *node) *node {
	if len(v.children) > 0 {
		return v.children[len(v.children)-1]
	}
	return v.thread
}

func firstWalk(v *node, sep float64) {
	if len(v.children) == 0 {
		v.prelim = 0
		if w := leftSibling(v); w != nil {
			v.prelim = w.prelim + distance(w, v, sep)
		}
		return
	}
	defaultAncestor := v.children[0]
	for _, w := range v.children {
		firstWalk(w, sep)
		defaultAncestor = apportion(w, defaultAncestor, sep)
	}
	executeShifts(v)
	midpoint := (v.children[0].prelim + v.children[len(v.children)-1].prelim) / 2
	if w := leftSibling(v); w != nil {
		v.prelim = w.prelim + distance(w, v, sep)
		v.mod = v.prelim - midpoint
	} else {
		v.prelim = midpoint
	}
}

func apportion(v, defaultAncestor *node, sep float64) *node {
	w := leftSibling(v)
	if w == nil {
		return defaultAncestor
	}
	vip, vop := v, v
	vim := w
	vom := leftmostSibling(vip)
	sip, sop := vip.mod, vop.mod
	sim, som := vim.mod, vom.mod
	for nextRight(vim) != nil && nextLeft(vip) != nil {
		vim = nextRight(vim)
		vip = nextLeft(vip)
		vom = nextLeft(vom)
		vop = nextRight(vop)
		vop.ancestor = v
		shift := (vim.prelim + sim) - (vip.prelim + sip) + distance(vim, vip, sep)
		if shift > 0 {
			moveSubtree(ancestor(vim, v, defaultAncestor), v, shift)
			sip += shift
			sop += shift
		}
		sim += vim.mod
		sip += vip.mod
		som += vom.mod
		sop += vop.mod
	}
	if nextRight(vim) != nil && nextRight(vop) == nil {
		vop.thread = nextRight(vim)
		vop.mod += sim - sop
	}
	if nextLeft(vip) != nil && nextLeft(vom) == nil {
		vom.thread = nextLeft(vip)
		vom.mod += sip - som
		defaultAncestor = v
	}
	return defaultAncestor
}

func moveSubtree(wm, wp *node, shift float64) {
	subtrees := float64(wp.number - wm.number)
	wp.change -= shift / subtrees
	wp.shift += shift
	wm.change += shift / subtrees
	wp.prelim += shift
	wp.mod += shift
}

func executeShifts(v *node) {
	shift, change := 0.0, 0.0
	for i := len(v.children) - 1; i >= 0; i-- {
		w := v.children[i]
		w.prelim += shift
		w.mod += shift
		change += w.change
		shift += w.shift + change
	}
}

func ancestor(vim, v, defaultAncestor *node) *node {
	if vim.ancestor.parent == v.parent {
		return vim.ancestor
	}
	return defaultAncestor
}

func secondWalk(v *node, m float64) {
	v.x = v.prelim + m
	for _, c := range v.children {
		secondWalk(c, m+v.mod)
	}
}

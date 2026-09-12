// Package tree lays out rooted trees (or forests) with the Buchheim, Jünger
// and Leipert linear-time refinement of Walker's algorithm, extended to nodes
// of different sizes. Parents are centred over their children, siblings keep
// the configured separation, and subtrees are packed as tightly as their
// contours allow.
//
// Edges must run parent to child (Edge.From is the parent). The arrow
// direction is independent and comes from Edge.Arrow. All four directions
// are supported; the algorithm works in an abstract (cross, rank) frame and
// diagram.Frame maps the result to x and y.
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

	cross, rank float64 // extents in the abstract frame

	// Buchheim state
	prelim, mod, shift, change float64
	thread                     *node
	ancestor                   *node

	c float64 // final centre on the cross axis

	// extra separation needed after this node for its incoming edge label
	labelSep float64
}

type edgeInfo struct {
	edge                        *diagram.Edge
	labelW, labelH, labelAscent float64
}

// labelGap is the gap between an edge segment and its label.
const labelGap = 6

// Layout positions the graph as a tree. Nodes with no incoming edge are
// roots; several roots are laid out side by side.
func Layout(g *diagram.Graph, o diagram.Options) (*diagram.Layout, error) {
	if len(g.Nodes) == 0 {
		return &diagram.Layout{}, nil
	}
	g, hiddenIDs, hiddenCounts := diagram.Visible(g)
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
	frame := diagram.Frame{Dir: g.Direction}
	transposed := g.Direction.Transposed()

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
		nd.cross, nd.rank = frame.NodeSize(pn.Rect)
		nd.ancestor = nd
		byID[n.ID] = nd
		all = append(all, nd)
	}

	// Build hierarchy in edge order.
	edges := map[*node]*edgeInfo{} // by child
	maxLabelW := 0.0
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
			if !transposed {
				// Label sits to the right of the child's vertical segment; the
				// next sibling's segment must clear it.
				c.labelSep = w + 2*labelGap
			}
			if w > maxLabelW {
				maxLabelW = w
			}
		}
		edges[c] = info
	}
	var roots []*node
	for _, nd := range all {
		if nd.parent == nil {
			roots = append(roots, nd)
		}
	}
	for _, nd := range all {
		seen := 0
		for p := nd; p != nil; p = p.parent {
			seen++
			if seen > len(all) {
				return nil, fmt.Errorf("tree: cycle through node %q", nd.pn.ID)
			}
		}
	}

	// Sideways layouts put labels above the run into the child, so the gap
	// between levels must hold the widest label.
	elbow := 0.0 // distance from the parent's far edge to the elbow
	if transposed {
		elbow = 14
		if need := maxLabelW + elbow + 2*labelGap + 4; need > rankSep {
			rankSep = need
		}
	}

	virtual := &node{pn: &diagram.PlacedNode{Node: &diagram.Node{ID: "\x00root"}}}
	virtual.ancestor = virtual
	virtual.children = roots
	for _, r := range roots {
		r.parent = virtual
	}
	number(virtual, -1)
	firstWalk(virtual, nodeSep)
	secondWalk(virtual, -virtual.prelim)

	// Levels along the rank axis.
	levelExt := map[int]float64{}
	maxDepth := 0
	for _, nd := range all {
		if nd.rank > levelExt[nd.depth] {
			levelExt[nd.depth] = nd.rank
		}
		if nd.depth > maxDepth {
			maxDepth = nd.depth
		}
	}
	levelStart := make([]float64, maxDepth+1)
	r := margin
	for d := 0; d <= maxDepth; d++ {
		levelStart[d] = r
		r += levelExt[d] + rankSep
	}
	totalRank := r - rankSep + margin

	// Cross extent, including labels hanging to the right of children.
	minC, maxC := 0.0, 0.0
	for i, nd := range all {
		lo := nd.c - nd.cross/2
		hi := nd.c + nd.cross/2
		if info := edges[nd]; info != nil && info.labelW > 0 && !transposed {
			if lr := nd.c + labelGap + info.labelW; lr > hi {
				hi = lr
			}
		}
		if i == 0 || lo < minC {
			minC = lo
		}
		if i == 0 || hi > maxC {
			maxC = hi
		}
	}
	dc := margin - minC
	totalCross := maxC - minC + 2*margin
	frame.Total = diagram.Size{W: totalCross, H: totalRank}

	out := &diagram.Layout{Size: frame.Size(), HiddenNodes: hiddenIDs}
	for _, nd := range all {
		pn := nd.pn
		pn.Depth = nd.depth
		pn.Hidden = hiddenCounts[pn.ID]
		nd.c += dc
		rect := frame.Rect(nd.c-nd.cross/2, levelStart[nd.depth], nd.cross, nd.rank)
		if pn.BarRect != nil {
			pn.BarRect.X += rect.X
			pn.BarRect.Y += rect.Y
		}
		pn.Rect = rect
		out.Nodes = append(out.Nodes, pn)
	}

	// Outgoing ports: each parent spreads its edges along its far side in
	// the children's order, so a fan-out does not leave from one point.
	port := map[*node]float64{} // by child
	for _, nd := range all {
		if len(nd.children) == 0 {
			continue
		}
		cs := make([]float64, len(nd.children))
		for i, ch := range nd.children {
			cs[i] = ch.c
		}
		for i, pc := range diagram.AssignPorts(nd.c, nd.cross, cs) {
			port[nd.children[i]] = pc
		}
	}

	// Edges: parent port -> elbow -> child near edge, in the abstract frame.
	for _, e := range g.Edges {
		c := byID[e.To]
		p := c.parent
		info := edges[c]
		pe := &diagram.PlacedEdge{Edge: e, Curved: g.Routing == diagram.RoutingCurved}
		// Elbows sit past the whole parent level, not just this parent, so
		// labels on the far side never overlap a taller node at that level.
		pEnd := levelStart[p.depth] + p.rank
		levelEnd := levelStart[p.depth] + levelExt[p.depth]
		cStart := levelStart[c.depth]
		mid := (levelEnd + cStart) / 2
		if transposed {
			mid = levelEnd + elbow
		}
		pc := port[c]
		pts := [][2]float64{{pc, pEnd}, {pc, mid}, {c.c, mid}, {c.c, cStart}}
		if abs(pc-c.c) < 0.5 {
			pts = [][2]float64{{pc, pEnd}, {c.c, cStart}}
		}
		for _, q := range pts {
			pe.Path = append(pe.Path, frame.Point(q[0], q[1]))
		}
		if info.labelW > 0 {
			var box diagram.Rect
			if transposed {
				// Above the run from the elbow into the child, centred on it.
				runStart, runEnd := mid+labelGap, cStart-labelGap
				centre := (runStart + runEnd) / 2
				box = frame.Rect(c.c-labelGap-info.labelH, centre-info.labelW/2, info.labelH, info.labelW)
				// frame.Rect swaps extents for transposed directions, so pass
				// (cross extent, rank extent) = (labelH, labelW).
			} else {
				centre := (mid + cStart) / 2
				box = frame.Rect(c.c+labelGap, centre-info.labelH/2, info.labelW, info.labelH)
			}
			pe.Label = &diagram.PlacedLabel{
				Text: e.Label, Style: labelStyle(e), Anchor: "middle",
				Pos:      box.Center(),
				Baseline: info.labelAscent - info.labelH/2,
				Box:      box,
			}
		}
		out.Edges = append(out.Edges, pe)
	}
	sort.SliceStable(out.Nodes, func(i, j int) bool { return out.Nodes[i].Depth < out.Nodes[j].Depth })
	return out, nil
}

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

// distance is the required gap between the centres of two adjacent nodes on
// the cross axis. A label hanging off the left node's edge pushes the right
// node away.
func distance(left, right *node, sep float64) float64 {
	d := left.cross/2 + sep + right.cross/2
	if lw := left.labelSep; lw > d {
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
	v.c = v.prelim + m
	for _, c := range v.children {
		secondWalk(c, m+v.mod)
	}
}

// Package layered lays out general directed graphs with the Sugiyama
// pipeline: cycles are broken by reversing back edges, nodes get ranks by
// longest path, long edges are split with dummy nodes, edge labels take
// their own dummy nodes on interleaved ranks so they never collide with
// nodes, crossings are reduced with barycenter sweeps and adjacent swaps,
// and coordinates are assigned by relaxing each rank toward its neighbours
// while keeping the required separation.
//
// It works in an abstract (cross, rank) frame and supports all four
// directions. Edges are returned as polylines through their dummies with
// Curved set, so renderers may smooth them.
package layered

import (
	"fmt"
	"math"
	"sort"

	"github.com/daniel-widrick/diagram"
)

type node struct {
	id    string
	pn    *diagram.PlacedNode // nil for dummies
	cross float64             // extent on the cross axis
	rank  float64             // extent on the rank axis
	layer int                 // doubled rank: real nodes on even layers
	order int
	c     float64 // centre on the cross axis
	r     float64 // start on the rank axis
	preds []*node
	succs []*node
	dummy *edge // set for dummies
	label bool  // dummy that carries the edge label
	input int   // input position, for stable initial ordering
}

type edge struct {
	e        *diagram.Edge
	from, to *node // real nodes in layout direction (after cycle breaking)
	reversed bool
	selfLoop bool
	chain    []*node // dummies between from and to, in layout direction
	labelW   float64
	labelH   float64
	labelAsc float64
}

// Layout positions the graph.
func Layout(g *diagram.Graph, o diagram.Options) (*diagram.Layout, error) {
	if len(g.Nodes) == 0 {
		return &diagram.Layout{}, nil
	}
	rankSep, nodeSep, margin := g.RankSep, g.NodeSep, g.Margin
	if rankSep <= 0 {
		rankSep = 44
	}
	if nodeSep <= 0 {
		nodeSep = 28
	}
	if margin <= 0 {
		margin = 16
	}
	frame := diagram.Frame{Dir: g.Direction}

	byID := map[string]*node{}
	var nodes []*node
	for i, n := range g.Nodes {
		if _, dup := byID[n.ID]; dup {
			return nil, fmt.Errorf("layered: duplicate node id %q", n.ID)
		}
		pn, err := diagram.MeasureNode(n, o)
		if err != nil {
			return nil, err
		}
		nd := &node{id: n.ID, pn: pn, input: i}
		nd.cross, nd.rank = frame.NodeSize(pn.Rect)
		byID[n.ID] = nd
		nodes = append(nodes, nd)
	}

	var edges []*edge
	for _, e := range g.Edges {
		from, ok := byID[e.From]
		if !ok {
			return nil, fmt.Errorf("layered: edge from unknown node %q", e.From)
		}
		to, ok := byID[e.To]
		if !ok {
			return nil, fmt.Errorf("layered: edge to unknown node %q", e.To)
		}
		ed := &edge{e: e, from: from, to: to, selfLoop: from == to}
		if e.Label != "" {
			w, h, a, err := diagram.MeasureLabel(e.Label, e.LabelStyle, o)
			if err != nil {
				return nil, err
			}
			ed.labelW, ed.labelH, ed.labelAsc = w, h, a
		}
		edges = append(edges, ed)
	}

	breakCycles(nodes, edges)
	assignRanks(nodes, edges)
	layers := buildLayers(nodes, edges, frame.Dir.Transposed())
	order(layers)
	assignCrossBK(layers, nodeSep)

	// Rank axis positions per layer.
	layerExt := make([]float64, len(layers))
	for i, l := range layers {
		for _, nd := range l {
			if nd.rank > layerExt[i] {
				layerExt[i] = nd.rank
			}
		}
	}
	layerStart := make([]float64, len(layers))
	r := margin
	for i := range layers {
		layerStart[i] = r
		gap := rankSep / 2
		if i%2 == 1 && layerExt[i] == 0 {
			gap = rankSep / 2 // empty label layer still splits the rank gap
		}
		r += layerExt[i] + gap
	}
	totalRank := r - rankSep/2 + margin

	// Cross extent.
	minC, maxC := math.Inf(1), math.Inf(-1)
	for _, l := range layers {
		for _, nd := range l {
			if v := nd.c - nd.cross/2; v < minC {
				minC = v
			}
			if v := nd.c + nd.cross/2; v > maxC {
				maxC = v
			}
		}
	}
	// Self loops hang off the right of their node.
	for _, ed := range edges {
		if ed.selfLoop {
			if v := ed.from.c + ed.from.cross/2 + loopSize; v > maxC {
				maxC = v
			}
		}
	}
	dc := margin - minC
	frame.Total = diagram.Size{W: maxC - minC + 2*margin, H: totalRank}

	out := &diagram.Layout{Size: frame.Size()}
	for _, l := range layers {
		for _, nd := range l {
			nd.c += dc
			nd.r = layerStart[nd.layer] + (layerExt[nd.layer]-nd.rank)/2 // centre within the layer
			if nd.pn == nil {
				continue
			}
			rect := frame.Rect(nd.c-nd.cross/2, nd.r, nd.cross, nd.rank)
			if nd.pn.BarRect != nil {
				nd.pn.BarRect.X += rect.X
				nd.pn.BarRect.Y += rect.Y
			}
			nd.pn.Rect = rect
			nd.pn.Depth = nd.layer / 2
			out.Nodes = append(out.Nodes, nd.pn)
		}
	}

	outPort, inPort := ports(nodes, edges)
	layerEnd := make([]float64, len(layers))
	for i := range layers {
		layerEnd[i] = layerStart[i] + layerExt[i]
	}
	routes := routeChannels(edges, outPort, inPort, layerStart, layerEnd)

	for _, ed := range edges {
		pe := &diagram.PlacedEdge{Edge: ed.e, Curved: g.Routing == diagram.RoutingCurved, Reversed: ed.reversed}
		if ed.selfLoop {
			n := ed.from
			right := n.c + n.cross/2
			mid := n.r + n.rank/2
			for _, q := range [][2]float64{{right, mid - 8}, {right + loopSize, mid - 10}, {right + loopSize, mid + 10}, {right, mid + 8}} {
				pe.Path = append(pe.Path, frame.Point(q[0], q[1]))
			}
			out.Edges = append(out.Edges, pe)
			continue
		}
		// Path in layout direction through the channel tracks; labels sit on
		// their dummies, which the route passes straight through.
		pts := routes[ed]
		for _, d := range ed.chain {
			if d.label {
				box := frame.Rect(d.c-d.cross/2, d.r, d.cross, d.rank)
				pe.Label = &diagram.PlacedLabel{
					Text: ed.e.Label, Style: labelStyle(ed.e), Anchor: "middle",
					Pos:        box.Center(),
					Baseline:   ed.labelAsc - ed.labelH/2,
					Box:        box,
					Background: true,
				}
			}
		}
		if ed.reversed {
			for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
		for _, q := range pts {
			pe.Path = append(pe.Path, frame.Point(q[0], q[1]))
		}
		out.Edges = append(out.Edges, pe)
	}
	sort.SliceStable(out.Nodes, func(i, j int) bool { return out.Nodes[i].Depth < out.Nodes[j].Depth })
	return out, nil
}

const loopSize = 18

// hop is one edge segment between adjacent layers.
type hop struct {
	ed         *edge
	cu, cv     float64 // cross positions at the upper and lower ends
	uEnd, vStr float64 // rank positions: leaving the upper, entering the lower
	band       int
	track      float64
}

// routeChannels builds an orthogonal route for every edge. Each hop
// between adjacent layers turns in the band between them, where no node or
// label box can be; hops sharing a band get separate tracks when their
// cross intervals overlap, so segments never lie on top of each other.
func routeChannels(edges []*edge, outPort, inPort map[*edge]float64, layerStart, layerEnd []float64) map[*edge][][2]float64 {
	var hops []*hop
	perEdge := map[*edge][]*hop{}
	for _, ed := range edges {
		if ed.selfLoop {
			continue
		}
		seq := append(append([]*node{ed.from}, ed.chain...), ed.to)
		for i := 0; i+1 < len(seq); i++ {
			u, v := seq[i], seq[i+1]
			h := &hop{ed: ed, cu: u.c, cv: v.c, uEnd: u.r + u.rank, vStr: v.r, band: u.layer}
			if i == 0 {
				h.cu = outPort[ed]
			}
			if i+1 == len(seq)-1 {
				h.cv = inPort[ed]
			}
			hops = append(hops, h)
			perEdge[ed] = append(perEdge[ed], h)
		}
	}
	// Track assignment per band: greedy interval colouring by start.
	byBand := map[int][]*hop{}
	for _, h := range hops {
		if math.Abs(h.cu-h.cv) > 0.01 {
			byBand[h.band] = append(byBand[h.band], h)
		}
	}
	for band, hs := range byBand {
		sort.SliceStable(hs, func(i, j int) bool { return math.Min(hs[i].cu, hs[i].cv) < math.Min(hs[j].cu, hs[j].cv) })
		var trackEnd []float64 // max cross reached on each track
		trackOf := make([]int, len(hs))
		for i, h := range hs {
			lo, hi := math.Min(h.cu, h.cv), math.Max(h.cu, h.cv)
			placed := false
			for t := range trackEnd {
				if trackEnd[t]+trackGap <= lo {
					trackEnd[t] = hi
					trackOf[i] = t
					placed = true
					break
				}
			}
			if !placed {
				trackEnd = append(trackEnd, hi)
				trackOf[i] = len(trackEnd) - 1
			}
		}
		bandStart, bandEnd := layerEnd[band], layerStart[band+1]
		n := float64(len(trackEnd))
		for i, h := range hs {
			h.track = bandStart + (bandEnd-bandStart)*float64(trackOf[i]+1)/(n+1)
		}
	}
	routes := map[*edge][][2]float64{}
	for ed, hs := range perEdge {
		var pts [][2]float64
		add := func(p [2]float64) {
			if n := len(pts); n > 0 && math.Abs(pts[n-1][0]-p[0]) < 0.01 && math.Abs(pts[n-1][1]-p[1]) < 0.01 {
				return
			}
			pts = append(pts, p)
		}
		for _, h := range hs {
			add([2]float64{h.cu, h.uEnd})
			if math.Abs(h.cu-h.cv) > 0.01 {
				add([2]float64{h.cu, h.track})
				add([2]float64{h.cv, h.track})
			}
			add([2]float64{h.cv, h.vStr})
		}
		routes[ed] = pts
	}
	return routes
}

// trackGap is the cross-axis clearance between hops sharing a track.
const trackGap = 6

// ports spreads each node's outgoing edges along its far side and incoming
// edges along its near side, ordered by where the other end goes.
func ports(nodes []*node, edges []*edge) (outPort, inPort map[*edge]float64) {
	outPort = map[*edge]float64{}
	inPort = map[*edge]float64{}
	outs := map[*node][]*edge{}
	ins := map[*node][]*edge{}
	for _, ed := range edges {
		if ed.selfLoop {
			continue
		}
		outs[ed.from] = append(outs[ed.from], ed)
		ins[ed.to] = append(ins[ed.to], ed)
	}
	// The first waypoint after a node decides the port order: the first
	// dummy of the chain, or the other node.
	next := func(ed *edge) *node {
		if len(ed.chain) > 0 {
			return ed.chain[0]
		}
		return ed.to
	}
	prev := func(ed *edge) *node {
		if len(ed.chain) > 0 {
			return ed.chain[len(ed.chain)-1]
		}
		return ed.from
	}
	for _, n := range nodes {
		if es := outs[n]; len(es) > 0 {
			cs := make([]float64, len(es))
			for i, ed := range es {
				cs[i] = next(ed).c
			}
			for i, pc := range diagram.AssignPorts(n.c, n.cross, cs) {
				outPort[es[i]] = pc
			}
		}
		if es := ins[n]; len(es) > 0 {
			cs := make([]float64, len(es))
			for i, ed := range es {
				cs[i] = prev(ed).c
			}
			for i, pc := range diagram.AssignPorts(n.c, n.cross, cs) {
				inPort[es[i]] = pc
			}
		}
	}
	return outPort, inPort
}

func labelStyle(e *diagram.Edge) string {
	if e.LabelStyle != "" {
		return e.LabelStyle
	}
	return "edge"
}

// breakCycles reverses back edges found by depth-first search so the graph
// becomes acyclic. Self loops are left alone; they are drawn as loops.
func breakCycles(nodes []*node, edges []*edge) {
	out := map[*node][]*edge{}
	for _, ed := range edges {
		if !ed.selfLoop {
			out[ed.from] = append(out[ed.from], ed)
		}
	}
	const (
		white = iota
		grey
		black
	)
	color := map[*node]int{}
	var visit func(n *node)
	visit = func(n *node) {
		color[n] = grey
		for _, ed := range out[n] {
			switch color[ed.to] {
			case grey:
				ed.reversed = true
				ed.from, ed.to = ed.to, ed.from
			case white:
				visit(ed.to)
			}
		}
		color[n] = black
	}
	for _, n := range nodes {
		if color[n] == white {
			visit(n)
		}
	}
}

// assignRanks gives every node the longest path length from a source, then
// pulls sources that only feed deep nodes down next to their first target,
// which shortens edges without adding crossings.
func assignRanks(nodes []*node, edges []*edge) {
	preds := map[*node][]*node{}
	succs := map[*node][]*node{}
	for _, ed := range edges {
		if ed.selfLoop {
			continue
		}
		preds[ed.to] = append(preds[ed.to], ed.from)
		succs[ed.from] = append(succs[ed.from], ed.to)
	}
	rank := map[*node]int{}
	var longest func(n *node) int
	longest = func(n *node) int {
		if r, ok := rank[n]; ok {
			return r
		}
		r := 0
		for _, p := range preds[n] {
			if v := longest(p) + 1; v > r {
				r = v
			}
		}
		rank[n] = r
		return r
	}
	for _, n := range nodes {
		longest(n)
	}
	// Sinks with room move as close to their sources as possible? Instead,
	// sources with room move toward their targets: a node with no
	// predecessors sits one rank above its shallowest successor.
	for _, n := range nodes {
		if len(preds[n]) == 0 && len(succs[n]) > 0 {
			min := math.MaxInt
			for _, s := range succs[n] {
				if rank[s] < min {
					min = rank[s]
				}
			}
			rank[n] = min - 1
		}
	}
	for _, n := range nodes {
		n.layer = rank[n] * 2 // doubled: odd layers hold labels
	}
}

// buildLayers creates dummy nodes for long edges and label nodes, wires
// pred/succ links at the segment level, and returns nodes grouped by layer.
func buildLayers(nodes []*node, edges []*edge, transposed bool) [][]*node {
	maxLayer := 0
	for _, n := range nodes {
		if n.layer > maxLayer {
			maxLayer = n.layer
		}
	}
	layers := make([][]*node, maxLayer+1)
	for _, n := range nodes {
		layers[n.layer] = append(layers[n.layer], n)
	}
	link := func(a, b *node) {
		a.succs = append(a.succs, b)
		b.preds = append(b.preds, a)
	}
	for i, ed := range edges {
		if ed.selfLoop {
			continue
		}
		lo, hi := ed.from.layer, ed.to.layer
		// Label goes on the odd layer nearest the middle of the span.
		labelLayer := -1
		if ed.labelW > 0 {
			labelLayer = lo + 1
			if mid := (lo + hi) / 2; mid%2 == 1 {
				labelLayer = mid
			} else if mid+1 < hi {
				labelLayer = mid + 1
			}
		}
		prev := ed.from
		for l := lo + 1; l < hi; l++ {
			d := &node{id: fmt.Sprintf("\x00%d@%d", i, l), dummy: ed, input: ed.from.input, layer: l}
			if l == labelLayer {
				d.label = true
				if transposed {
					d.cross, d.rank = ed.labelH+6, ed.labelW+8
				} else {
					d.cross, d.rank = ed.labelW+8, ed.labelH+6
				}
			}
			layers[l] = append(layers[l], d)
			ed.chain = append(ed.chain, d)
			link(prev, d)
			prev = d
		}
		link(prev, ed.to)
	}
	for _, l := range layers {
		sort.SliceStable(l, func(i, j int) bool { return l[i].input < l[j].input })
		for i, n := range l {
			n.order = i
		}
	}
	return layers
}

// order reduces crossings with barycenter sweeps followed by adjacent
// swaps, keeping the best ordering seen.
func order(layers [][]*node) {
	if len(layers) < 2 {
		return
	}
	best := totalCrossings(layers)
	bestOrder := snapshot(layers)
	for iter := 0; iter < 24; iter++ {
		if iter%2 == 0 {
			for l := 1; l < len(layers); l++ {
				sortByBarycenter(layers[l], func(n *node) []*node { return n.preds })
			}
		} else {
			for l := len(layers) - 2; l >= 0; l-- {
				sortByBarycenter(layers[l], func(n *node) []*node { return n.succs })
			}
		}
		transpose(layers)
		if c := totalCrossings(layers); c < best {
			best = c
			bestOrder = snapshot(layers)
			if c == 0 {
				break
			}
		}
	}
	restore(layers, bestOrder)
}

func sortByBarycenter(layer []*node, nbrs func(*node) []*node) {
	type keyed struct {
		n   *node
		key float64
	}
	ks := make([]keyed, len(layer))
	for i, n := range layer {
		ns := nbrs(n)
		key := float64(n.order)
		if len(ns) > 0 {
			sum := 0.0
			for _, m := range ns {
				sum += float64(m.order)
			}
			key = sum / float64(len(ns))
		}
		ks[i] = keyed{n, key}
	}
	sort.SliceStable(ks, func(i, j int) bool { return ks[i].key < ks[j].key })
	for i, k := range ks {
		layer[i] = k.n
		k.n.order = i
	}
}

// transpose swaps adjacent nodes while that lowers crossings with the
// neighbouring layers.
func transpose(layers [][]*node) {
	improved := true
	for guard := 0; improved && guard < 20; guard++ {
		improved = false
		for l := range layers {
			layer := layers[l]
			for i := 0; i+1 < len(layer); i++ {
				before := layerCrossings(layers, l)
				layer[i], layer[i+1] = layer[i+1], layer[i]
				layer[i].order, layer[i+1].order = i, i+1
				if layerCrossings(layers, l) < before {
					improved = true
				} else {
					layer[i], layer[i+1] = layer[i+1], layer[i]
					layer[i].order, layer[i+1].order = i, i+1
				}
			}
		}
	}
}

func layerCrossings(layers [][]*node, l int) int {
	c := 0
	if l > 0 {
		c += crossings(layers[l-1], layers[l])
	}
	if l+1 < len(layers) {
		c += crossings(layers[l], layers[l+1])
	}
	return c
}

func totalCrossings(layers [][]*node) int {
	c := 0
	for l := 0; l+1 < len(layers); l++ {
		c += crossings(layers[l], layers[l+1])
	}
	return c
}

// crossings counts edge crossings between two adjacent layers.
func crossings(upper, lower []*node) int {
	type pair struct{ u, v int }
	var ps []pair
	for _, n := range upper {
		for _, s := range n.succs {
			ps = append(ps, pair{n.order, s.order})
		}
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].u != ps[j].u {
			return ps[i].u < ps[j].u
		}
		return ps[i].v < ps[j].v
	})
	// Count inversions of v with a Fenwick tree.
	tree := make([]int, len(lower)+2)
	add := func(i int) {
		for i++; i < len(tree); i += i & -i {
			tree[i]++
		}
	}
	sum := func(i int) int { // count of values <= i
		s := 0
		for i++; i > 0; i -= i & -i {
			s += tree[i]
		}
		return s
	}
	c := 0
	for k, p := range ps {
		c += k - sum(p.v)
		add(p.v)
	}
	return c
}

func snapshot(layers [][]*node) [][]*node {
	out := make([][]*node, len(layers))
	for i, l := range layers {
		out[i] = append([]*node(nil), l...)
	}
	return out
}

func restore(layers, saved [][]*node) {
	for i := range layers {
		copy(layers[i], saved[i])
		for j, n := range layers[i] {
			n.order = j
		}
	}
}

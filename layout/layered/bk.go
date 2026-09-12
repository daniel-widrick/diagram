package layered

import (
	"math"
	"sort"
)

// Brandes-Köpf coordinate assignment ("Fast and Simple Horizontal
// Coordinate Assignment", 2002), following the corrected block-graph
// compaction used by dagre. Four alignments (up/down x left/right) are
// computed; each vertically aligns nodes with a median neighbour into
// blocks while avoiding type-1 conflicts (ordinary edges crossing the
// inner segments of long edges), compacts blocks with the required
// separation, and the final coordinate is the median of the four. Long
// edges come out straight wherever the alignment allows.

type pair struct{ a, b *node }

// assignCrossBK sets node.c for every node in every layer.
func assignCrossBK(layers [][]*node, nodeSep float64) {
	sep := func(a, b *node) float64 {
		gap := nodeSep
		if a.pn == nil || b.pn == nil {
			gap = nodeSep / 2
		}
		return a.cross/2 + gap + b.cross/2
	}
	conflicts := type1Conflicts(layers)

	var results []map[*node]float64
	for _, down := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			ls := orient(layers, down, right)
			nbrs := func(n *node) []*node {
				if down {
					return n.succs
				}
				return n.preds
			}
			root, align := verticalAlignment(ls, nbrs, conflicts)
			xs := horizontalCompaction(ls, root, align, sep)
			if right {
				for n := range xs {
					xs[n] = -xs[n]
				}
			}
			results = append(results, xs)
		}
	}
	alignToSmallest(results)
	for _, l := range layers {
		for _, n := range l {
			vals := []float64{results[0][n], results[1][n], results[2][n], results[3][n]}
			sort.Float64s(vals)
			n.c = (vals[1] + vals[2]) / 2
		}
	}
	// Median averaging can bring neighbours closer than their separation;
	// push apart where it happened, symmetrically, without changing order.
	for _, l := range layers {
		for pass := 0; pass < 60; pass++ {
			moved := false
			for i := 0; i+1 < len(l); i++ {
				if d := sep(l[i], l[i+1]) - (l[i+1].c - l[i].c); d > 0.01 {
					l[i].c -= d / 2
					l[i+1].c += d / 2
					moved = true
				}
			}
			if !moved {
				break
			}
		}
	}
}

// orient returns the layers in the traversal order for one of the four
// alignments: reversed layer order for downward sweeps, reversed node
// order within layers for right-aligned sweeps. Positions are re-numbered
// for the copy through the pos map used by the alignment.
func orient(layers [][]*node, down, right bool) [][]*node {
	ls := make([][]*node, len(layers))
	for i, l := range layers {
		c := append([]*node(nil), l...)
		if right {
			for a, b := 0, len(c)-1; a < b; a, b = a+1, b-1 {
				c[a], c[b] = c[b], c[a]
			}
		}
		ls[i] = c
	}
	if down {
		for a, b := 0, len(ls)-1; a < b; a, b = a+1, b-1 {
			ls[a], ls[b] = ls[b], ls[a]
		}
	}
	return ls
}

// type1Conflicts marks ordinary edges that cross an inner segment (an edge
// between two dummies); those edges must not be used for alignment so the
// inner segment can stay straight.
func type1Conflicts(layers [][]*node) map[pair]bool {
	conflicts := map[pair]bool{}
	pos := map[*node]int{}
	for _, l := range layers {
		for i, n := range l {
			pos[n] = i
		}
	}
	innerPred := func(v *node) *node {
		if v.pn != nil {
			return nil
		}
		for _, p := range v.preds {
			if p.pn == nil {
				return p
			}
		}
		return nil
	}
	for li := 1; li < len(layers); li++ {
		prev, layer := layers[li-1], layers[li]
		k0, scanPos := 0, 0
		for i, v := range layer {
			w := innerPred(v)
			k1 := len(prev)
			if w != nil {
				k1 = pos[w]
			}
			if w != nil || i == len(layer)-1 {
				for _, u := range layer[scanPos : i+1] {
					for _, p := range u.preds {
						inner := p.pn == nil && u.pn == nil
						if (pos[p] < k0 || pos[p] > k1) && !inner {
							conflicts[pair{p, u}] = true
						}
					}
				}
				scanPos = i + 1
				k0 = k1
			}
		}
	}
	return conflicts
}

// verticalAlignment groups nodes into blocks: each node tries to align
// with its median neighbour in the previous layer, in sweep order, unless
// that neighbour is already taken or the edge is conflicted.
func verticalAlignment(layers [][]*node, nbrs func(*node) []*node, conflicts map[pair]bool) (root, align map[*node]*node) {
	root = map[*node]*node{}
	align = map[*node]*node{}
	pos := map[*node]int{}
	for _, l := range layers {
		for i, n := range l {
			pos[n] = i
			root[n] = n
			align[n] = n
		}
	}
	conflicted := func(a, b *node) bool { return conflicts[pair{a, b}] || conflicts[pair{b, a}] }
	for li := 1; li < len(layers); li++ {
		prevIdx := -1
		for _, v := range layers[li] {
			ws := append([]*node(nil), nbrs(v)...)
			if len(ws) == 0 {
				continue
			}
			sort.Slice(ws, func(i, j int) bool { return pos[ws[i]] < pos[ws[j]] })
			mp := (len(ws) - 1) / 2
			mq := len(ws) / 2
			for _, m := range []int{mp, mq} {
				w := ws[m]
				if align[v] == v && prevIdx < pos[w] && !conflicted(w, v) {
					align[w] = v
					root[v] = root[w]
					align[v] = root[v]
					prevIdx = pos[w]
				}
			}
		}
	}
	return root, align
}

// horizontalCompaction places blocks as far left as their separation
// allows, using longest paths over a graph of blocks.
func horizontalCompaction(layers [][]*node, root, align map[*node]*node, sep func(a, b *node) float64) map[*node]float64 {
	type bedge struct {
		to *node
		w  float64
	}
	out := map[*node][]bedge{}
	in := map[*node]int{}
	blocks := map[*node]bool{}
	for _, l := range layers {
		for i, v := range l {
			blocks[root[v]] = true
			if i == 0 {
				continue
			}
			u, w := root[l[i-1]], root[v]
			if u == w {
				continue
			}
			d := sep(l[i-1], v)
			found := false
			for k := range out[u] {
				if out[u][k].to == w {
					if d > out[u][k].w {
						out[u][k].w = d
					}
					found = true
				}
			}
			if !found {
				out[u] = append(out[u], bedge{w, d})
				in[w]++
			}
		}
	}
	// Longest path in topological order.
	xs := map[*node]float64{}
	var queue []*node
	for b := range blocks {
		if in[b] == 0 {
			queue = append(queue, b)
			xs[b] = 0
		}
	}
	// Deterministic order: by first appearance in layers.
	first := map[*node]int{}
	idx := 0
	for _, l := range layers {
		for _, v := range l {
			if _, ok := first[root[v]]; !ok {
				first[root[v]] = idx
				idx++
			}
		}
	}
	sort.Slice(queue, func(i, j int) bool { return first[queue[i]] < first[queue[j]] })
	for len(queue) > 0 {
		b := queue[0]
		queue = queue[1:]
		for _, e := range out[b] {
			if v := xs[b] + e.w; v > xs[e.to] {
				xs[e.to] = v
			}
			in[e.to]--
			if in[e.to] == 0 {
				queue = append(queue, e.to)
			}
		}
	}
	res := map[*node]float64{}
	for _, l := range layers {
		for _, v := range l {
			res[v] = xs[root[v]]
		}
	}
	return res
}

// alignToSmallest shifts the four assignments so they share the extent
// of the narrowest one, as the paper prescribes before taking medians.
func alignToSmallest(xss []map[*node]float64) {
	type ext struct{ min, max float64 }
	exts := make([]ext, len(xss))
	smallest := 0
	for i, xs := range xss {
		e := ext{math.Inf(1), math.Inf(-1)}
		for _, x := range xs {
			e.min = math.Min(e.min, x)
			e.max = math.Max(e.max, x)
		}
		exts[i] = e
		if e.max-e.min < exts[smallest].max-exts[smallest].min {
			smallest = i
		}
	}
	for i, xs := range xss {
		// Left-aligned variants (even index) align their minimum, right-aligned
		// ones their maximum.
		var delta float64
		if i%2 == 0 {
			delta = exts[smallest].min - exts[i].min
		} else {
			delta = exts[smallest].max - exts[i].max
		}
		if delta != 0 {
			for n := range xs {
				xs[n] += delta
			}
		}
	}
}

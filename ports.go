package diagram

import "sort"

// PortSpacing is the preferred distance between adjacent edge endpoints on
// one side of a node; endpoints are squeezed closer when the node is narrow.
const PortSpacing = 14

// portMargin keeps endpoints away from a node's corners.
const portMargin = 8

// Ports spreads k endpoints along a side of a node whose centre on the
// cross axis is c and whose extent on that axis is ext. The result is
// ordered left to right (or top to bottom). With one endpoint the centre
// is returned.
func Ports(c, ext float64, k int) []float64 {
	if k <= 1 {
		return []float64{c}
	}
	span := float64(k-1) * PortSpacing
	if room := ext - 2*portMargin; span > room {
		span = max(room, 0)
	}
	out := make([]float64, k)
	for i := range out {
		out[i] = c - span/2 + span*float64(i)/float64(k-1)
	}
	return out
}

// AssignPorts orders the given neighbour positions and returns, for each
// index in the original order, the endpoint it was given along the side.
// Neighbours further left (or up) get ports further left (or up), so edges
// leave a node without crossing each other at the border.
func AssignPorts(c, ext float64, neighbourCross []float64) []float64 {
	k := len(neighbourCross)
	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return neighbourCross[idx[a]] < neighbourCross[idx[b]] })
	ports := Ports(c, ext, k)
	out := make([]float64, k)
	for rank, i := range idx {
		out[i] = ports[rank]
	}
	return out
}

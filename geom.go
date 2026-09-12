package diagram

// Frame maps an abstract layout frame to final coordinates. Layouts work in
// (cross, rank) space where rank grows away from roots or sources; Frame
// turns that into x and y for the requested Direction. Total is the drawing
// size in abstract space (cross extent, rank extent).
type Frame struct {
	Dir   Direction
	Total Size // W = cross extent, H = rank extent
}

// Size of the final drawing.
func (f Frame) Size() Size {
	if f.Dir.Transposed() {
		return Size{W: f.Total.H, H: f.Total.W}
	}
	return f.Total
}

// Point maps an abstract point (cross, rank).
func (f Frame) Point(cross, rank float64) Point {
	switch f.Dir {
	case BottomUp:
		return Point{X: cross, Y: f.Total.H - rank}
	case LeftRight:
		return Point{X: rank, Y: cross}
	case RightLeft:
		return Point{X: f.Total.H - rank, Y: cross}
	default:
		return Point{X: cross, Y: rank}
	}
}

// Rect maps an abstract rect given by its cross/rank origin and extents.
func (f Frame) Rect(cross, rank, crossExt, rankExt float64) Rect {
	a := f.Point(cross, rank)
	b := f.Point(cross+crossExt, rank+rankExt)
	return Rect{X: min(a.X, b.X), Y: min(a.Y, b.Y), W: abs(a.X - b.X), H: abs(a.Y - b.Y)}
}

// NodeSize returns a node's extents in the abstract frame (cross, rank).
func (f Frame) NodeSize(r Rect) (cross, rank float64) {
	if f.Dir.Transposed() {
		return r.H, r.W
	}
	return r.W, r.H
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// Package diagram is a measurement-first layout engine for node and edge
// diagrams. Callers describe a graph of labelled nodes, the engine measures
// every label with real font metrics, sizes nodes from the result, lays them
// out, and returns pure geometry. Rendering (SVG, a web component, a terminal)
// is a separate concern built on that geometry.
//
// The core rule is that layout never runs before measurement: a label can
// never overrun its node because the node was sized from the label.
package diagram

import "github.com/daniel-widrick/diagram/text"

// Point is a position in diagram units (CSS pixels at 1x).
type Point struct{ X, Y float64 }

// Size is a width and height.
type Size struct{ W, H float64 }

// Rect is an axis-aligned box given by its top-left corner and size.
type Rect struct{ X, Y, W, H float64 }

// Right returns the x coordinate of the right edge.
func (r Rect) Right() float64 { return r.X + r.W }

// Bottom returns the y coordinate of the bottom edge.
func (r Rect) Bottom() float64 { return r.Y + r.H }

// Center returns the middle of the rect.
func (r Rect) Center() Point { return Point{r.X + r.W/2, r.Y + r.H/2} }

// Overlaps reports whether two rects share any area.
func (r Rect) Overlaps(o Rect) bool {
	return r.X < o.Right() && o.X < r.Right() && r.Y < o.Bottom() && o.Y < r.Bottom()
}

// Overflow says what to do with a line wider than a node's MaxWidth.
type Overflow int

const (
	// Grow widens the node to fit (the default when MaxWidth is zero).
	Grow Overflow = iota
	// Ellipsize shortens the line with a middle ellipsis; the full text is
	// kept in PlacedSpan.Full.
	Ellipsize
	// Wrap breaks the line into several lines at word boundaries.
	Wrap
)

// Arrow says where an edge draws its arrowhead.
type Arrow int

const (
	ArrowForward  Arrow = iota // at the To end
	ArrowBackward              // at the From end
	ArrowNone
	ArrowBoth
)

// Span is a run of text in one style. Style names are resolved through a
// text.StyleSet for measurement and by the renderer for appearance.
type Span struct {
	Text  string
	Style string
}

// Line is one line of a node label, made of styled spans.
type Line []Span

// Text joins the spans' text.
func (l Line) Text() string {
	s := ""
	for _, sp := range l {
		s += sp.Text
	}
	return s
}

// L is a convenience constructor for a single-span line.
func L(style, text string) Line { return Line{{Text: text, Style: style}} }

// Node is a box with labelled lines.
type Node struct {
	ID    string
	Lines []Line
	// Kind is a free-form class ("hot", "scan", "table") that renderers map
	// to appearance. It has no effect on layout.
	Kind string
	// Bar, when non-nil, asks for a horizontal bar filled to the given
	// fraction (0..1) under the lines. Renderers use it for shares of time or
	// cost. It adds to the node's height.
	Bar *float64
	// MaxWidth caps the node width in diagram units; zero means unlimited.
	MaxWidth float64
	Overflow Overflow
	// Padding overrides the default inner padding when non-zero.
	Padding float64
	// Size, when both dimensions are non-zero, is used as-is and the node is
	// not measured. Use it when the caller measured the label itself (for
	// example in a browser).
	Size Size
	// Data is passed through to PlacedNode untouched for renderers and callers.
	Data any
}

// Edge connects two nodes. For tree layouts From is the parent and To the
// child regardless of which way the arrow points.
type Edge struct {
	From, To string
	Label    string
	// LabelStyle names the text style for the label; empty means "edge".
	LabelStyle string
	// Weight (0..1) is a visual weight renderers turn into stroke width.
	Weight float64
	Kind   string
	Arrow  Arrow
	Data   any
}

// Direction is the main flow of a layout: where children (tree) or edge
// targets (layered) go relative to their parents or sources.
type Direction int

const (
	TopDown Direction = iota
	BottomUp
	LeftRight
	RightLeft
)

// Transposed reports whether the rank axis is horizontal.
func (d Direction) Transposed() bool { return d == LeftRight || d == RightLeft }

// Routing is how edges are drawn between their waypoints.
type Routing int

const (
	// RoutingCurved rounds the corners of the routed path (the default).
	RoutingCurved Routing = iota
	// RoutingOrthogonal keeps sharp right-angle corners.
	RoutingOrthogonal
)

// Graph is the input to a layout.
type Graph struct {
	Nodes     []*Node
	Edges     []*Edge
	Direction Direction
	Routing   Routing
	// RankSep is the gap between levels; NodeSep the gap between siblings.
	// Zero picks the defaults (40 and 24).
	RankSep, NodeSep float64
	// Margin is the blank border around the drawing; zero picks 16.
	Margin float64
}

// PlacedSpan is a measured, possibly shortened span with its x offset from
// the line start.
type PlacedSpan struct {
	Text  string // as drawn
	Full  string // original text, differs from Text when ellipsized
	Style string
	X     float64 // offset from the node's inner left
	W     float64
}

// PlacedLine is a line positioned inside a node.
type PlacedLine struct {
	Spans    []PlacedSpan
	Baseline float64 // y of the text baseline relative to the node's top
	Height   float64
	W        float64
}

// PlacedNode is a node with its final rect.
type PlacedNode struct {
	*Node
	Rect  Rect
	Lines []PlacedLine
	// Bar is the rect of the bar track, if the node asked for one.
	BarRect *Rect
	Padding float64
	// Depth is the level in a tree layout (root 0); -1 when not applicable.
	Depth int
}

// PlacedLabel is an edge label. Box is the space reserved for it; Pos is
// the text anchor (the box centre) and the baseline sits at Pos.Y+Baseline.
type PlacedLabel struct {
	Text     string
	Style    string
	Pos      Point
	Anchor   string // always "middle" for now
	Baseline float64
	Box      Rect
	// Background asks the renderer to paint the box in the background
	// colour, because the edge passes underneath the label.
	Background bool
}

// PlacedEdge is an edge with its routed path.
type PlacedEdge struct {
	*Edge
	Path  []Point
	Label *PlacedLabel
	// Curved asks the renderer to draw a smooth curve through Path rather
	// than straight segments.
	Curved bool
	// Reversed is set by layered layouts when the edge was flipped to break
	// a cycle; Path still runs From to To.
	Reversed bool
}

// Layout is the geometry produced by a layout algorithm.
type Layout struct {
	Nodes []*PlacedNode
	Edges []*PlacedEdge
	Size  Size
	byID  map[string]*PlacedNode
}

// Node finds a placed node by ID.
func (l *Layout) Node(id string) *PlacedNode {
	if l.byID == nil {
		l.byID = map[string]*PlacedNode{}
		for _, n := range l.Nodes {
			l.byID[n.ID] = n
		}
	}
	return l.byID[id]
}

// Options carries what every layout needs beyond the graph.
type Options struct {
	Measurer text.Measurer
	Styles   text.StyleSet
	// DefaultPadding is the inner padding for nodes; zero picks 10.
	DefaultPadding float64
	// BarHeight is the height of node bars including their gap; zero picks 11.
	BarHeight float64
}

func (o Options) padding(n *Node) float64 {
	if n.Padding > 0 {
		return n.Padding
	}
	if o.DefaultPadding > 0 {
		return o.DefaultPadding
	}
	return 10
}

func (o Options) barHeight() float64 {
	if o.BarHeight > 0 {
		return o.BarHeight
	}
	return 11
}

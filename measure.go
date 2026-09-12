package diagram

import (
	"errors"

	"github.com/daniel-widrick/diagram/text"
)

// MeasureNode sizes a node from its lines using the options' measurer and
// style set, applying the node's overflow policy. The returned PlacedNode has
// Rect.W and Rect.H set and X, Y zero; layouts position it afterwards.
func MeasureNode(n *Node, o Options) (*PlacedNode, error) {
	if o.Measurer == nil || o.Styles == nil {
		return nil, errors.New("diagram: Options.Measurer and Options.Styles are required")
	}
	pad := o.padding(n)
	pn := &PlacedNode{Node: n, Padding: pad, Depth: -1}
	if n.Size.W > 0 && n.Size.H > 0 {
		pn.Rect = Rect{W: n.Size.W, H: n.Size.H}
		return pn, nil
	}
	inner := 0.0
	if n.MaxWidth > 0 {
		inner = n.MaxWidth - 2*pad
		if inner < 8 {
			inner = 8
		}
	}
	var lines []PlacedLine
	for _, ln := range n.Lines {
		placed, err := placeLine(ln, inner, n.Overflow, o)
		if err != nil {
			return nil, err
		}
		lines = append(lines, placed...)
	}
	width := 0.0
	y := pad
	for i := range lines {
		lines[i].Baseline += y
		y += lines[i].Height
		if lines[i].W > width {
			width = lines[i].W
		}
	}
	if n.MaxWidth > 0 && n.Overflow != Grow && width > inner {
		width = inner
	}
	h := y + pad
	if n.Bar != nil {
		bh := o.barHeight()
		pn.BarRect = &Rect{X: pad, Y: y + (bh - 5), W: 0, H: 5}
		h += bh
	}
	pn.Rect = Rect{W: width + 2*pad, H: h}
	if pn.BarRect != nil {
		pn.BarRect.W = width
	}
	pn.Lines = lines
	return pn, nil
}

// placeLine measures one input line, producing one or more placed lines
// depending on the overflow policy.
func placeLine(ln Line, maxInner float64, ov Overflow, o Options) ([]PlacedLine, error) {
	if ov == Wrap && maxInner > 0 {
		return wrapLine(ln, maxInner, o)
	}
	pl := PlacedLine{}
	x := 0.0
	height := 0.0
	ascent := 0.0
	for i, sp := range ln {
		st, err := o.Styles.Get(sp.Style)
		if err != nil {
			return nil, err
		}
		m := o.Measurer.Metrics(st)
		if m.LineHeight > height {
			height = m.LineHeight
		}
		if m.Ascent > ascent {
			ascent = m.Ascent
		}
		txt := sp.Text
		w := o.Measurer.Width(txt, st)
		if ov == Ellipsize && maxInner > 0 && x+w > maxInner {
			room := maxInner - x
			txt = text.Ellipsize(o.Measurer, txt, st, room)
			w = o.Measurer.Width(txt, st)
			pl.Spans = append(pl.Spans, PlacedSpan{Text: txt, Full: sp.Text, Style: sp.Style, X: x, W: w})
			x += w
			// Anything after the cut is dropped; keep it in Full of a zero-width span
			// so nothing silently disappears from the data.
			for _, rest := range ln[i+1:] {
				pl.Spans = append(pl.Spans, PlacedSpan{Text: "", Full: rest.Text, Style: rest.Style, X: x, W: 0})
			}
			break
		}
		pl.Spans = append(pl.Spans, PlacedSpan{Text: txt, Full: sp.Text, Style: sp.Style, X: x, W: w})
		x += w
	}
	if len(ln) == 0 {
		st, _ := o.Styles.Get("")
		m := o.Measurer.Metrics(st)
		height, ascent = m.LineHeight, m.Ascent
	}
	pl.W = x
	pl.Height = height
	pl.Baseline = ascent + (height-ascent-heightBelow(o, ln))/2
	return []PlacedLine{pl}, nil
}

// heightBelow returns the largest descent among the line's spans, used to
// centre text vertically within its line box.
func heightBelow(o Options, ln Line) float64 {
	d := 0.0
	for _, sp := range ln {
		st, err := o.Styles.Get(sp.Style)
		if err != nil {
			continue
		}
		if m := o.Measurer.Metrics(st); m.Descent > d {
			d = m.Descent
		}
	}
	return d
}

// wrapLine joins the spans' text, wraps it in the first span's style, and
// emits one placed line per wrapped row. Mixed styles collapse to the first
// style when wrapping; wrapping styled runs across rows is not supported.
func wrapLine(ln Line, maxInner float64, o Options) ([]PlacedLine, error) {
	if len(ln) == 0 {
		return placeLine(ln, 0, Grow, o)
	}
	st, err := o.Styles.Get(ln[0].Style)
	if err != nil {
		return nil, err
	}
	rows := text.Wrap(o.Measurer, ln.Text(), st, maxInner)
	var out []PlacedLine
	for _, r := range rows {
		pl, err := placeLine(Line{{Text: r, Style: ln[0].Style}}, 0, Grow, o)
		if err != nil {
			return nil, err
		}
		out = append(out, pl...)
	}
	return out, nil
}

// MeasureLabel sizes an edge label.
func MeasureLabel(txt, style string, o Options) (w, h, ascent float64, err error) {
	if style == "" {
		style = "edge"
	}
	st, err := o.Styles.Get(style)
	if err != nil {
		return 0, 0, 0, err
	}
	m := o.Measurer.Metrics(st)
	return o.Measurer.Width(txt, st), m.LineHeight, m.Ascent, nil
}

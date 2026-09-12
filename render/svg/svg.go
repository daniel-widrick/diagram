// Package svg renders a diagram.Layout as an SVG document. Colours come from
// a Theme, whose values may be literal colours or CSS custom properties such
// as "var(--accent)" when the SVG is inlined in a page.
package svg

import (
	"fmt"
	"html"
	"math"
	"strings"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/text"
)

// KindStyle is the appearance of nodes or edges of one Kind.
type KindStyle struct {
	Fill, Stroke, Text, TextMuted string
	// StrokeDash, when set, dashes edges of this kind (for example "6 4").
	StrokeDash string
}

// Theme is the colour and font vocabulary of a rendering.
type Theme struct {
	Background string
	Node       KindStyle
	Edge       string
	EdgeLabel  string
	BarTrack   string
	BarFill    string
	// Kinds overrides Node (for nodes) and Edge colours (Stroke) per Kind.
	Kinds map[string]KindStyle
	// FontMono and FontSans are CSS font-family stacks for the "mono" and
	// "sans" text families. They must lead with the font the measurer used.
	FontMono, FontSans string
	// Styles maps style names to colours; unknown styles use Node.Text.
	StyleColors map[string]string
}

// Default is a light theme with literal colours, suitable for standalone files.
func Default() Theme {
	return Theme{
		Background: "#ffffff",
		Node:       KindStyle{Fill: "#eef1f5", Stroke: "#1b2430", Text: "#1b2430", TextMuted: "#4e5a68"},
		Edge:       "#1b2430",
		EdgeLabel:  "#4e5a68",
		BarTrack:   "#d3dae3",
		BarFill:    "#1f7a8c",
		Kinds: map[string]KindStyle{
			"hot":  {Fill: "#c94a34", Stroke: "#c94a34", Text: "#ffffff", TextMuted: "#ffe1db"},
			"warn": {Fill: "#f5ebcb", Stroke: "#b8860b", Text: "#1b2430", TextMuted: "#6b5410"},
			"weak": {Stroke: "#c94a34", StrokeDash: "6 4"},
		},
		FontMono:    `"Go Mono", "IBM Plex Mono", ui-monospace, Menlo, monospace`,
		FontSans:    `"Go", "IBM Plex Sans", system-ui, sans-serif`,
		StyleColors: map[string]string{"muted": "muted"},
	}
}

// Tokens is a theme that references CSS custom properties, for SVG inlined
// into a page that defines --bg, --surface-2, --ink, --ink-2, --line,
// --accent and --hot.
func Tokens() Theme {
	t := Default()
	t.Background = "var(--surface)"
	t.Node = KindStyle{Fill: "var(--surface-2)", Stroke: "currentColor", Text: "currentColor", TextMuted: "var(--ink-2)"}
	t.Edge = "currentColor"
	t.EdgeLabel = "var(--ink-2)"
	t.BarTrack = "var(--line)"
	t.BarFill = "var(--accent)"
	t.Kinds["hot"] = KindStyle{Fill: "var(--hot)", Stroke: "var(--hot)", Text: "#ffffff", TextMuted: "rgba(255,255,255,0.85)"}
	t.Kinds["weak"] = KindStyle{Stroke: "var(--hot)", StrokeDash: "6 4"}
	return t
}

// Options controls rendering details.
type Options struct {
	Theme  Theme
	Styles text.StyleSet
	// Title and Description populate <title> and <desc> for accessibility.
	Title, Description string
	// Inline omits the XML declaration and adds role="img", for embedding.
	Inline bool
	// MinStroke and MaxStroke bound edge stroke widths mapped from Weight.
	MinStroke, MaxStroke float64
}

// Render produces the SVG text for a layout.
func Render(l *diagram.Layout, o Options) string {
	th := o.Theme
	if th.FontMono == "" {
		th = Default()
	}
	styles := o.Styles
	if styles == nil {
		styles = text.DefaultStyles()
	}
	minS, maxS := o.MinStroke, o.MaxStroke
	if minS <= 0 {
		minS = 1
	}
	if maxS <= 0 {
		maxS = 4
	}
	var b strings.Builder
	if !o.Inline {
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	}
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s"`,
		f(l.Size.W), f(l.Size.H), f(l.Size.W), f(l.Size.H))
	if o.Inline {
		b.WriteString(` role="img"`)
		if o.Title != "" {
			fmt.Fprintf(&b, ` aria-label="%s"`, html.EscapeString(o.Title))
		}
	}
	b.WriteString(">\n")
	if o.Title != "" {
		fmt.Fprintf(&b, "  <title>%s</title>\n", html.EscapeString(o.Title))
	}
	if o.Description != "" {
		fmt.Fprintf(&b, "  <desc>%s</desc>\n", html.EscapeString(o.Description))
	}
	b.WriteString("  <defs>\n")
	b.WriteString(`    <marker id="dg-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto-start-reverse" markerUnits="userSpaceOnUse"><path d="M0,0 L8,4 L0,8 z" fill="` + th.Edge + `"/></marker>` + "\n")
	b.WriteString("  </defs>\n")
	if th.Background != "" {
		fmt.Fprintf(&b, `  <rect width="%s" height="%s" fill="%s"/>`+"\n", f(l.Size.W), f(l.Size.H), th.Background)
	}

	// Edges first so nodes draw over them.
	for _, e := range l.Edges {
		stroke := th.Edge
		dash := ""
		if ks, ok := th.Kinds[e.Kind]; ok && e.Kind != "" {
			if ks.Stroke != "" {
				stroke = ks.Stroke
			}
			dash = ks.StrokeDash
		}
		w := minS + clamp(e.Weight)*(maxS-minS)
		if e.Curved && len(e.Path) > 2 {
			fmt.Fprintf(&b, `  <path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"`, smoothPath(e.Path), stroke, f(w))
		} else {
			var pts []string
			for _, p := range e.Path {
				pts = append(pts, f(p.X)+","+f(p.Y))
			}
			fmt.Fprintf(&b, `  <polyline points="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"`, strings.Join(pts, " "), stroke, f(w))
		}
		if dash != "" {
			fmt.Fprintf(&b, ` stroke-dasharray="%s"`, dash)
		}
		switch e.Arrow {
		case diagram.ArrowForward:
			b.WriteString(` marker-end="url(#dg-arrow)"`)
		case diagram.ArrowBackward:
			b.WriteString(` marker-start="url(#dg-arrow)"`)
		case diagram.ArrowBoth:
			b.WriteString(` marker-start="url(#dg-arrow)" marker-end="url(#dg-arrow)"`)
		}
		if e.Kind != "" {
			fmt.Fprintf(&b, ` class="edge %s"`, html.EscapeString(e.Kind))
		} else {
			b.WriteString(` class="edge"`)
		}
		b.WriteString("/>\n")
		if e.Label != nil {
			st, _ := styles.Get(e.Label.Style)
			if e.Label.Background && th.Background != "" {
				bx := e.Label.Box
				fmt.Fprintf(&b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s"/>`+"\n", f(bx.X), f(bx.Y), f(bx.W), f(bx.H), labelBackground(th))
			}
			fmt.Fprintf(&b, `  <text x="%s" y="%s" text-anchor="%s" fill="%s" %s xml:space="preserve">%s</text>`+"\n",
				f(e.Label.Pos.X), f(e.Label.Pos.Y+e.Label.Baseline), e.Label.Anchor, th.EdgeLabel, fontAttrs(th, st), html.EscapeString(e.Label.Text))
		}
	}

	for _, n := range l.Nodes {
		ks := th.Node
		if k, ok := th.Kinds[n.Kind]; ok && n.Kind != "" {
			ks = merge(th.Node, k)
		}
		class := "node"
		if n.Kind != "" {
			class += " " + html.EscapeString(n.Kind)
		}
		fmt.Fprintf(&b, `  <g class="%s" data-id="%s">`+"\n", class, html.EscapeString(n.ID))
		fmt.Fprintf(&b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="4" fill="%s" stroke="%s"/>`+"\n",
			f(n.Rect.X), f(n.Rect.Y), f(n.Rect.W), f(n.Rect.H), ks.Fill, ks.Stroke)
		for _, ln := range n.Lines {
			for _, sp := range ln.Spans {
				if sp.Text == "" {
					continue
				}
				st, _ := styles.Get(sp.Style)
				color := ks.Text
				if th.StyleColors[sp.Style] == "muted" {
					color = ks.TextMuted
				}
				title := ""
				if sp.Full != sp.Text {
					title = "<title>" + html.EscapeString(sp.Full) + "</title>"
				}
				fmt.Fprintf(&b, `    <text x="%s" y="%s" fill="%s" %s xml:space="preserve">%s%s</text>`+"\n",
					f(n.Rect.X+n.Padding+sp.X), f(n.Rect.Y+ln.Baseline), color, fontAttrs(th, st), title, html.EscapeString(sp.Text))
			}
		}
		if n.BarRect != nil && n.Bar != nil {
			r := n.BarRect
			fmt.Fprintf(&b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`+"\n", f(r.X), f(r.Y), f(r.W), f(r.H), th.BarTrack)
			fill := th.BarFill
			if n.Kind == "hot" {
				fill = ks.Text
			}
			if w := r.W * clamp(*n.Bar); w > 0 {
				fmt.Fprintf(&b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`+"\n", f(r.X), f(r.Y), f(w), f(r.H), fill)
			}
		}
		b.WriteString("  </g>\n")
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// labelBackground is the colour painted behind labels that sit on an edge.
// A transparent theme background cannot hide the line, so fall back to the
// node fill, which is opaque in every theme.
func labelBackground(th Theme) string {
	if th.Background == "transparent" || th.Background == "none" {
		return th.Node.Fill
	}
	return th.Background
}

// cornerRadius is the largest rounding applied to a path corner.
const cornerRadius = 10

// smoothPath rounds every corner of a polyline with a quadratic curve
// whose control point is the corner itself. The curve stays inside the
// wedge between the two segments, so a route that avoids obstacles with
// sharp corners still avoids them when rounded.
func smoothPath(pts []diagram.Point) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "M%s,%s", f(pts[0].X), f(pts[0].Y))
	for i := 1; i+1 < len(pts); i++ {
		p, c, n := pts[i-1], pts[i], pts[i+1]
		lin := math.Hypot(c.X-p.X, c.Y-p.Y)
		lout := math.Hypot(n.X-c.X, n.Y-c.Y)
		if lin < 0.01 || lout < 0.01 {
			continue
		}
		r := math.Min(cornerRadius, math.Min(lin/2, lout/2))
		a := diagram.Point{X: c.X - (c.X-p.X)/lin*r, Y: c.Y - (c.Y-p.Y)/lin*r}
		b := diagram.Point{X: c.X + (n.X-c.X)/lout*r, Y: c.Y + (n.Y-c.Y)/lout*r}
		fmt.Fprintf(&sb, " L%s,%s Q%s,%s %s,%s", f(a.X), f(a.Y), f(c.X), f(c.Y), f(b.X), f(b.Y))
	}
	last := pts[len(pts)-1]
	fmt.Fprintf(&sb, " L%s,%s", f(last.X), f(last.Y))
	return sb.String()
}

func merge(base, over KindStyle) KindStyle {
	if over.Fill != "" {
		base.Fill = over.Fill
	}
	if over.Stroke != "" {
		base.Stroke = over.Stroke
	}
	if over.Text != "" {
		base.Text = over.Text
	}
	if over.TextMuted != "" {
		base.TextMuted = over.TextMuted
	}
	return base
}

func fontAttrs(th Theme, st text.Style) string {
	family := th.FontMono
	if st.Family == "sans" {
		family = th.FontSans
	}
	size := st.Size
	if size <= 0 {
		size = 11
	}
	s := fmt.Sprintf(`font-family='%s' font-size="%s"`, family, f(size))
	if st.Weight >= 600 {
		s += fmt.Sprintf(` font-weight="%d"`, st.Weight)
	}
	return s
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// f formats a coordinate with at most two decimals and no trailing zeros.
func f(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		s = "0"
	}
	return s
}

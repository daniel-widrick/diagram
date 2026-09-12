// Package layoutjson encodes a diagram.Layout as render-ready JSON: every
// coordinate is absolute and rounded to two decimals, so a renderer in any
// language can draw it without repeating the layout's arithmetic. The
// browser renderer in web/diagram.js consumes this format and produces the
// same SVG as render/svg.
package layoutjson

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/text"
)

// Doc is the top-level document.
type Doc struct {
	Size   Size             `json:"size"`
	Nodes  []Node           `json:"nodes"`
	Edges  []Edge           `json:"edges"`
	Groups []Group          `json:"groups"`
	Hidden []string         `json:"hidden"`
	Styles map[string]Style `json:"styles"`
}

type Size struct{ W, H float64 }
type Point struct{ X, Y float64 }
type Rect struct{ X, Y, W, H float64 }

type Style struct {
	Family string  `json:"family"`
	Size   float64 `json:"size"`
	Weight int     `json:"weight"`
}

type Span struct {
	X     float64 `json:"x"`
	Text  string  `json:"text"`
	Full  string  `json:"full"`
	Style string  `json:"style"`
}

type Line struct {
	Y     float64 `json:"y"`
	Spans []Span  `json:"spans"`
}

type Bar struct {
	Rect
	Fill float64 `json:"fill"` // width of the filled part
}

type Badge struct {
	X, Y, W float64
	TextY   float64 `json:"textY"`
	Text    string  `json:"text"`
}

type Node struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Group     string `json:"group"`
	Rect      Rect   `json:"rect"`
	Lines     []Line `json:"lines"`
	Bar       *Bar   `json:"bar,omitempty"`
	Badge     *Badge `json:"badge,omitempty"`
	Depth     int    `json:"depth"`
	Hidden    int    `json:"hidden"`
	Collapsed bool   `json:"collapsed"`
}

type Label struct {
	Text       string  `json:"text"`
	Style      string  `json:"style"`
	X, Y       float64 // text anchor and baseline
	Anchor     string  `json:"anchor"`
	Box        Rect    `json:"box"`
	Background bool    `json:"background"`
}

type Edge struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Kind     string  `json:"kind"`
	Weight   float64 `json:"weight"`
	Arrow    string  `json:"arrow"`
	Path     []Point `json:"path"`
	Curved   bool    `json:"curved"`
	Reversed bool    `json:"reversed"`
	Label    *Label  `json:"label,omitempty"`
}

type Group struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Kind    string   `json:"kind"`
	Rect    Rect     `json:"rect"`
	LabelX  float64  `json:"labelX"`
	LabelY  float64  `json:"labelY"`
	Members []string `json:"members"`
}

// R2 rounds to two decimals the way both renderers format numbers.
func R2(v float64) float64 {
	r := math.Round(v*100) / 100
	if r == 0 {
		return 0
	}
	return r
}

func rect(r diagram.Rect) Rect { return Rect{R2(r.X), R2(r.Y), R2(r.W), R2(r.H)} }

// Build converts a layout.
func Build(l *diagram.Layout, styles text.StyleSet) Doc {
	if styles == nil {
		styles = text.DefaultStyles()
	}
	d := Doc{Size: Size{R2(l.Size.W), R2(l.Size.H)}, Nodes: []Node{}, Edges: []Edge{}, Groups: []Group{}, Hidden: []string{}, Styles: map[string]Style{}}
	for name, st := range styles {
		size := st.Size
		if size <= 0 {
			size = 11
		}
		d.Styles[name] = Style{Family: st.Family, Size: size, Weight: st.Weight}
	}
	d.Hidden = append(d.Hidden, l.HiddenNodes...)
	for _, gr := range l.Groups {
		d.Groups = append(d.Groups, Group{ID: gr.ID, Label: gr.Label, Kind: gr.Kind, Rect: rect(gr.Rect),
			LabelX: R2(gr.LabelPos.X), LabelY: R2(gr.LabelPos.Y + gr.LabelBaseline), Members: append([]string{}, gr.Members...)})
	}
	for _, e := range l.Edges {
		je := Edge{From: e.From, To: e.To, Kind: e.Kind, Weight: e.Weight, Curved: e.Curved, Reversed: e.Reversed, Path: []Point{}}
		switch e.Arrow {
		case diagram.ArrowForward:
			je.Arrow = "forward"
		case diagram.ArrowBackward:
			je.Arrow = "backward"
		case diagram.ArrowBoth:
			je.Arrow = "both"
		default:
			je.Arrow = "none"
		}
		for _, p := range e.Path {
			je.Path = append(je.Path, Point{R2(p.X), R2(p.Y)})
		}
		if e.Label != nil {
			je.Label = &Label{Text: e.Label.Text, Style: e.Label.Style, X: R2(e.Label.Pos.X), Y: R2(e.Label.Pos.Y + e.Label.Baseline),
				Anchor: e.Label.Anchor, Box: rect(e.Label.Box), Background: e.Label.Background}
		}
		d.Edges = append(d.Edges, je)
	}
	for _, n := range l.Nodes {
		jn := Node{ID: n.ID, Kind: n.Kind, Group: n.Group, Rect: rect(n.Rect), Lines: []Line{}, Depth: n.Depth, Hidden: n.Hidden, Collapsed: n.Collapsed}
		for _, ln := range n.Lines {
			jl := Line{Y: R2(n.Rect.Y + ln.Baseline), Spans: []Span{}}
			for _, sp := range ln.Spans {
				if sp.Text == "" {
					continue
				}
				jl.Spans = append(jl.Spans, Span{X: R2(n.Rect.X + n.Padding + sp.X), Text: sp.Text, Full: sp.Full, Style: sp.Style})
			}
			jn.Lines = append(jn.Lines, jl)
		}
		if n.BarRect != nil && n.Bar != nil {
			v := *n.Bar
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			jn.Bar = &Bar{Rect: rect(*n.BarRect), Fill: R2(n.BarRect.W * v)}
		}
		if n.Collapsed && n.Hidden > 0 {
			label := fmt.Sprintf("+%d", n.Hidden)
			st, _ := styles.Get("edge")
			size := st.Size
			if size <= 0 {
				size = 11
			}
			bw := float64(len(label))*size*0.62 + 10
			bx, by := n.Rect.Right()-bw+4, n.Rect.Bottom()-8
			jn.Badge = &Badge{X: R2(bx), Y: R2(by), W: R2(bw), TextY: R2(by + 11.5), Text: label}
		}
		d.Nodes = append(d.Nodes, jn)
	}
	return d
}

// Encode converts a layout to JSON.
func Encode(l *diagram.Layout, styles text.StyleSet) ([]byte, error) {
	return json.Marshal(Build(l, styles))
}

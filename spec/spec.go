// Package spec is a JSON description of a graph, shared by the CLI and by
// applications that build graphs in another language and hand them to Go
// for layout.
package spec

import (
	"encoding/json"
	"fmt"

	"github.com/daniel-widrick/diagram"
)

// Graph is the JSON shape.
type Graph struct {
	// Direction is "topdown" (default), "bottomup", "leftright" or "rightleft".
	Direction string  `json:"direction,omitempty"`
	RankSep   float64 `json:"rankSep,omitempty"`
	NodeSep   float64 `json:"nodeSep,omitempty"`
	Margin    float64 `json:"margin,omitempty"`
	Nodes     []Node  `json:"nodes"`
	Edges     []Edge  `json:"edges"`
}

// Node is one box.
type Node struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind,omitempty"`
	Bar      *float64 `json:"bar,omitempty"`
	MaxWidth float64  `json:"maxWidth,omitempty"`
	// Overflow is "grow" (default), "ellipsize" or "wrap".
	Overflow string           `json:"overflow,omitempty"`
	Lines    [][]diagram.Span `json:"lines"`
}

// Edge is one connection.
type Edge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Label  string  `json:"label,omitempty"`
	Kind   string  `json:"kind,omitempty"`
	Weight float64 `json:"weight,omitempty"`
	// Arrow is "forward" (default), "backward", "none" or "both".
	Arrow string `json:"arrow,omitempty"`
}

// Decode parses JSON into a diagram.Graph.
func Decode(data []byte) (*diagram.Graph, error) {
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	return g.Graph()
}

// Graph converts the spec to the engine's input.
func (sp Graph) Graph() (*diagram.Graph, error) {
	g := &diagram.Graph{RankSep: sp.RankSep, NodeSep: sp.NodeSep, Margin: sp.Margin}
	switch sp.Direction {
	case "", "topdown":
	case "bottomup":
		g.Direction = diagram.BottomUp
	case "leftright":
		g.Direction = diagram.LeftRight
	case "rightleft":
		g.Direction = diagram.RightLeft
	default:
		return nil, fmt.Errorf("spec: unknown direction %q", sp.Direction)
	}
	for _, n := range sp.Nodes {
		if n.ID == "" {
			return nil, fmt.Errorf("spec: node without id")
		}
		nd := &diagram.Node{ID: n.ID, Kind: n.Kind, Bar: n.Bar, MaxWidth: n.MaxWidth}
		switch n.Overflow {
		case "", "grow":
		case "ellipsize":
			nd.Overflow = diagram.Ellipsize
		case "wrap":
			nd.Overflow = diagram.Wrap
		default:
			return nil, fmt.Errorf("spec: node %q: unknown overflow %q", n.ID, n.Overflow)
		}
		for _, ln := range n.Lines {
			nd.Lines = append(nd.Lines, diagram.Line(ln))
		}
		g.Nodes = append(g.Nodes, nd)
	}
	for _, e := range sp.Edges {
		ed := &diagram.Edge{From: e.From, To: e.To, Label: e.Label, Kind: e.Kind, Weight: e.Weight}
		switch e.Arrow {
		case "", "forward":
		case "backward":
			ed.Arrow = diagram.ArrowBackward
		case "none":
			ed.Arrow = diagram.ArrowNone
		case "both":
			ed.Arrow = diagram.ArrowBoth
		default:
			return nil, fmt.Errorf("spec: edge %s->%s: unknown arrow %q", e.From, e.To, e.Arrow)
		}
		g.Edges = append(g.Edges, ed)
	}
	return g, nil
}

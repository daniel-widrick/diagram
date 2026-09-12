// Command diagram lays out a graph described in JSON and writes SVG.
//
//	diagram -layout tree -theme tokens < graph.json > out.svg
//
// The JSON shape is:
//
//	{
//	  "direction": "topdown" | "bottomup",
//	  "nodes": [{"id": "a", "kind": "hot", "bar": 0.8, "maxWidth": 200, "overflow": "ellipsize",
//	             "lines": [[{"text": "Seq Scan ", "style": "title"}, {"text": "orders", "style": "detail"}]]}],
//	  "edges": [{"from": "a", "to": "b", "label": "781 rows", "weight": 0.5, "arrow": "backward", "kind": "weak"}]
//	}
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/layout/tree"
	"github.com/daniel-widrick/diagram/render/svg"
	"github.com/daniel-widrick/diagram/text"
)

type spec struct {
	Direction string  `json:"direction"`
	RankSep   float64 `json:"rankSep"`
	NodeSep   float64 `json:"nodeSep"`
	Nodes     []struct {
		ID       string           `json:"id"`
		Kind     string           `json:"kind"`
		Bar      *float64         `json:"bar"`
		MaxWidth float64          `json:"maxWidth"`
		Overflow string           `json:"overflow"`
		Lines    [][]diagram.Span `json:"lines"`
	} `json:"nodes"`
	Edges []struct {
		From, To, Label, Kind, Arrow string
		Weight                       float64
	} `json:"edges"`
}

func main() {
	layoutName := flag.String("layout", "tree", "layout algorithm: tree")
	themeName := flag.String("theme", "default", "theme: default or tokens (CSS variables)")
	title := flag.String("title", "", "accessible title")
	inline := flag.Bool("inline", false, "omit the XML declaration for inlining in HTML")
	flag.Parse()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal(err)
	}
	var sp spec
	if err := json.Unmarshal(data, &sp); err != nil {
		fatal(fmt.Errorf("parse input: %w", err))
	}
	g := &diagram.Graph{RankSep: sp.RankSep, NodeSep: sp.NodeSep}
	if sp.Direction == "bottomup" {
		g.Direction = diagram.BottomUp
	}
	for _, n := range sp.Nodes {
		nd := &diagram.Node{ID: n.ID, Kind: n.Kind, Bar: n.Bar, MaxWidth: n.MaxWidth}
		switch n.Overflow {
		case "ellipsize":
			nd.Overflow = diagram.Ellipsize
		case "wrap":
			nd.Overflow = diagram.Wrap
		}
		for _, ln := range n.Lines {
			nd.Lines = append(nd.Lines, diagram.Line(ln))
		}
		g.Nodes = append(g.Nodes, nd)
	}
	for _, e := range sp.Edges {
		ed := &diagram.Edge{From: e.From, To: e.To, Label: e.Label, Kind: e.Kind, Weight: e.Weight}
		switch e.Arrow {
		case "backward":
			ed.Arrow = diagram.ArrowBackward
		case "none":
			ed.Arrow = diagram.ArrowNone
		case "both":
			ed.Arrow = diagram.ArrowBoth
		}
		g.Edges = append(g.Edges, ed)
	}

	opts := diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
	var l *diagram.Layout
	switch *layoutName {
	case "tree":
		l, err = tree.Layout(g, opts)
	default:
		err = fmt.Errorf("unknown layout %q", *layoutName)
	}
	if err != nil {
		fatal(err)
	}
	th := svg.Default()
	if *themeName == "tokens" {
		th = svg.Tokens()
	}
	os.Stdout.WriteString(svg.Render(l, svg.Options{Theme: th, Styles: opts.Styles, Title: *title, Inline: *inline}))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "diagram:", err)
	os.Exit(1)
}

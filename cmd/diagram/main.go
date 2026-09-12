// Command diagram lays out a graph described in JSON and writes SVG.
//
//	diagram -layout tree -theme tokens < graph.json > out.svg
//	diagram -layout layered < join.json > join.svg
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
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/layout/layered"
	"github.com/daniel-widrick/diagram/layout/tree"
	"github.com/daniel-widrick/diagram/render/svg"
	"github.com/daniel-widrick/diagram/spec"
	"github.com/daniel-widrick/diagram/text"
)

func main() {
	layoutName := flag.String("layout", "tree", "layout algorithm: tree or layered")
	themeName := flag.String("theme", "default", "theme: default or tokens (CSS variables)")
	title := flag.String("title", "", "accessible title")
	inline := flag.Bool("inline", false, "omit the XML declaration for inlining in HTML")
	flag.Parse()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal(err)
	}
	g, err := spec.Decode(data)
	if err != nil {
		fatal(err)
	}

	opts := diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
	var l *diagram.Layout
	switch *layoutName {
	case "tree":
		l, err = tree.Layout(g, opts)
	case "layered":
		l, err = layered.Layout(g, opts)
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

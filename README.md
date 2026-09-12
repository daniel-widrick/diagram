# diagram

A measurement-first layout engine for node and edge diagrams, in Go.

The rule that makes it different from drawing boxes by hand: **layout never
runs before measurement**. Every label is measured with real font metrics,
each node is sized from its label, and only then are nodes positioned. A label
cannot overrun its box because the box was made for it. Text that must not
exceed a width is shortened with a middle ellipsis (identifiers are usually
recognisable from both ends) or wrapped, and the full text is kept in the
output for tooltips.

The engine returns geometry, not pictures. Rendering is a separate step, so
the same layout can become a standalone SVG file, an SVG fragment using a
page's CSS variables, or an interactive component in a web app.

```
diagram         core types: Graph, Node, Edge, Line, Layout
text            font measurement (Go fonts embedded; any TTF/OTF can be added),
                middle ellipsis, wrapping
layout/tree     tidy tree layout (Buchheim, Junger and Leipert) for nodes of
                any size, top-down or bottom-up, with edge labels that never
                collide with sibling edges
render/svg      SVG renderer with themes: literal colours or CSS custom properties
cmd/diagram     CLI: JSON in, SVG out
```

## Example

```go
m := text.NewGoFonts()
opts := diagram.Options{Measurer: m, Styles: text.DefaultStyles()}
share := 0.83
g := &diagram.Graph{
    Nodes: []*diagram.Node{
        {ID: "nl", Lines: []diagram.Line{diagram.L("title", "Nested Loop")}},
        {ID: "seq", Kind: "hot", Bar: &share, MaxWidth: 220, Overflow: diagram.Ellipsize,
            Lines: []diagram.Line{
                {{Text: "Seq Scan ", Style: "title"}, {Text: "order_items", Style: "detail"}},
                diagram.L("detail", "filter product_id = 42"),
            }},
    },
    Edges: []*diagram.Edge{{From: "nl", To: "seq", Label: "781 rows", Weight: 0.5, Arrow: diagram.ArrowBackward}},
}
l, err := tree.Layout(g, opts)
out := svg.Render(l, svg.Options{Theme: svg.Default(), Styles: opts.Styles, Title: "Plan tree"})
```

Or from the command line:

```sh
go run github.com/daniel-widrick/diagram/cmd/diagram@latest -title "Plan tree" < testdata/plan.json > plan.svg
```

![Plan tree example](testdata/plan.svg)

## Fonts

Measurement and rendering must agree on the font. The default measurer
embeds the Go fonts (Go Mono and Go Regular, with bold weights) so nothing
needs installing; the default theme's font stacks lead with them. To use
another font, register its TTF or OTF data with `FontMeasurer.AddFont` and
put the same family first in the theme's font stack. A node whose label was
measured elsewhere, for example in a browser with `measureText`, can pass
its size in `Node.Size` and skips measurement.

## Status

Tree layout is complete and tested with invariants (no overlaps, parents
centred over children, labels inside boxes and clear of sibling edges) on
random trees. Layered layout for general directed graphs (the Sugiyama
pipeline) is next. Left-to-right direction and a web component renderer are
planned.

Built first for [pginspect](https://github.com/daniel-widrick/pginspect),
where it draws query plan trees.

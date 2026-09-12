package svg

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/layout/tree"
	"github.com/daniel-widrick/diagram/text"
)

var update = flag.Bool("update", false, "rewrite golden files")

// loadSpec reads the CLI's JSON shape without importing package main.
func loadSpec(t *testing.T, path string) *diagram.Graph {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sp struct {
		Direction string
		Nodes     []struct {
			ID, Kind, Overflow string
			Bar                *float64
			MaxWidth           float64
			Lines              [][]diagram.Span
		}
		Edges []struct {
			From, To, Label, Kind, Arrow string
			Weight                       float64
		}
	}
	if err := json.Unmarshal(data, &sp); err != nil {
		t.Fatal(err)
	}
	g := &diagram.Graph{}
	if sp.Direction == "bottomup" {
		g.Direction = diagram.BottomUp
	}
	for _, n := range sp.Nodes {
		nd := &diagram.Node{ID: n.ID, Kind: n.Kind, Bar: n.Bar, MaxWidth: n.MaxWidth}
		if n.Overflow == "ellipsize" {
			nd.Overflow = diagram.Ellipsize
		}
		for _, ln := range n.Lines {
			nd.Lines = append(nd.Lines, diagram.Line(ln))
		}
		g.Nodes = append(g.Nodes, nd)
	}
	for _, e := range sp.Edges {
		ed := &diagram.Edge{From: e.From, To: e.To, Label: e.Label, Kind: e.Kind, Weight: e.Weight}
		if e.Arrow == "backward" {
			ed.Arrow = diagram.ArrowBackward
		}
		g.Edges = append(g.Edges, ed)
	}
	return g
}

func TestGoldenPlan(t *testing.T) {
	g := loadSpec(t, filepath.Join("..", "..", "testdata", "plan.json"))
	opts := diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
	l, err := tree.Layout(g, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := Render(l, Options{Theme: Default(), Styles: opts.Styles, Title: "Plan tree"})
	golden := filepath.Join("..", "..", "testdata", "plan.svg")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create)", err)
	}
	if string(want) != got {
		t.Errorf("rendered SVG differs from %s (run go test ./... -update after checking the change)", golden)
	}
	// Structural checks that do not depend on exact numbers.
	for _, want := range []string{`<title>Plan tree</title>`, `class="node hot"`, `marker-start="url(#dg-arrow)"`, `<title>filter product_id = 42 and status = &#39;shipped&#39;</title>`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(got, "<rect") < len(g.Nodes)+1 {
		t.Error("expected a rect per node plus background")
	}
}

func TestTokensTheme(t *testing.T) {
	g := loadSpec(t, filepath.Join("..", "..", "testdata", "plan.json"))
	opts := diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
	l, err := tree.Layout(g, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := Render(l, Options{Theme: Tokens(), Inline: true})
	if strings.Contains(got, "<?xml") || !strings.Contains(got, `role="img"`) || !strings.Contains(got, "var(--hot)") {
		t.Error("inline tokens rendering wrong")
	}
}

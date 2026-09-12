package svg

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/layout/layered"
	"github.com/daniel-widrick/diagram/layout/tree"
	"github.com/daniel-widrick/diagram/spec"
	"github.com/daniel-widrick/diagram/text"
)

var update = flag.Bool("update", false, "rewrite golden files")

func loadSpec(t *testing.T, path string) *diagram.Graph {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	g, err := spec.Decode(data)
	if err != nil {
		t.Fatal(err)
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
	for _, want := range []string{`<title>Plan tree</title>`, `class="node hot"`, `marker-start="url(#dg-arrow)"`, `<title>filter product_id = 42 and status = &#39;shipped&#39;</title>`,
		// Spans are separate text elements; a span ending in a space must keep it.
		`xml:space="preserve">Sort </text>`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(got, "<rect") < len(g.Nodes)+1 {
		t.Error("expected a rect per node plus background")
	}
}

func TestGoldenJoin(t *testing.T) {
	g := loadSpec(t, filepath.Join("..", "..", "testdata", "join.json"))
	opts := diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
	l, err := layered.Layout(g, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := Render(l, Options{Theme: Default(), Styles: opts.Styles, Title: "Join graph"})
	golden := filepath.Join("..", "..", "testdata", "join.svg")
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
	for _, want := range []string{`<path d="M`, `stroke-dasharray="6 4"`, `class="edge weak"`, `rx="3"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
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

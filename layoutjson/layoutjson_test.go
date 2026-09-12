package layoutjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/daniel-widrick/diagram"
	"github.com/daniel-widrick/diagram/layout/tree"
	"github.com/daniel-widrick/diagram/spec"
	"github.com/daniel-widrick/diagram/text"
)

func TestEncode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "collapsed.json"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := spec.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	opts := diagram.Options{Measurer: text.NewGoFonts(), Styles: text.DefaultStyles()}
	l, err := tree.Layout(g, opts)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(l, opts.Styles)
	if err != nil {
		t.Fatal(err)
	}
	var d Doc
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Nodes) != 4 || len(d.Edges) != 3 || len(d.Hidden) != 2 || d.Styles["title"].Weight != 700 {
		t.Errorf("doc: %d nodes %d edges hidden %v styles %v", len(d.Nodes), len(d.Edges), d.Hidden, d.Styles)
	}
	var gather *Node
	for i := range d.Nodes {
		if d.Nodes[i].ID == "gather" {
			gather = &d.Nodes[i]
		}
	}
	if gather == nil || gather.Badge == nil || gather.Badge.Text != "+2" || !gather.Collapsed {
		t.Errorf("gather badge: %+v", gather)
	}
	for _, n := range d.Nodes {
		if n.Rect.X != R2(n.Rect.X) {
			t.Errorf("unrounded coordinate %v", n.Rect.X)
		}
		for _, ln := range n.Lines {
			for _, sp := range ln.Spans {
				if sp.X < n.Rect.X || sp.X > n.Rect.X+n.Rect.W {
					t.Errorf("span x outside node: %v not in %+v", sp.X, n.Rect)
				}
			}
		}
	}
	if R2(10.125) != 10.13 || R2(-0.001) != 0 {
		t.Errorf("R2: %v %v", R2(10.125), R2(-0.001))
	}
}

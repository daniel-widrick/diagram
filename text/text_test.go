package text

import (
	"strings"
	"testing"
)

func TestWidthMonotonic(t *testing.T) {
	m := NewGoFonts()
	st := Style{Family: "mono", Size: 11}
	prev := 0.0
	for i := 1; i <= 40; i++ {
		w := m.Width(strings.Repeat("a", i), st)
		if w <= prev {
			t.Fatalf("width not increasing at %d: %v <= %v", i, w, prev)
		}
		prev = w
	}
	// Monospace: every character has the same advance.
	if a, b := m.Width("iiii", st), m.Width("WWWW", st); a != b {
		t.Errorf("mono widths differ: %v vs %v", a, b)
	}
	if m.Width("x", Style{Family: "mono", Size: 22}) <= m.Width("x", st) {
		t.Error("larger size should be wider")
	}
	if m.Width("word", Style{Family: "sans", Size: 11, Weight: 700}) <= 0 {
		t.Error("bold sans should measure")
	}
}

func TestMetrics(t *testing.T) {
	m := NewGoFonts()
	mt := m.Metrics(Style{Family: "mono", Size: 10})
	if mt.Ascent <= 0 || mt.Descent <= 0 || mt.LineHeight != 14 {
		t.Errorf("metrics: %+v", mt)
	}
}

func TestEllipsize(t *testing.T) {
	m := NewGoFonts()
	st := Style{Family: "mono", Size: 11}
	s := "order_items_product_id_idx"
	full := m.Width(s, st)
	for _, frac := range []float64{0.9, 0.6, 0.3} {
		max := full * frac
		got := Ellipsize(m, s, st, max)
		if w := m.Width(got, st); w > max {
			t.Errorf("frac %v: %q is %v wide, max %v", frac, got, w, max)
		}
		if !strings.Contains(got, "…") {
			t.Errorf("frac %v: expected ellipsis in %q", frac, got)
		}
		if !strings.HasPrefix(got, "or") || !strings.HasSuffix(got, "dx") {
			t.Errorf("frac %v: middle ellipsis should keep both ends: %q", frac, got)
		}
	}
	if got := Ellipsize(m, s, st, full); got != s {
		t.Errorf("fits: got %q", got)
	}
	if got := Ellipsize(m, s, st, 1); got != "…" {
		t.Errorf("tiny: got %q", got)
	}
}

func TestWrap(t *testing.T) {
	m := NewGoFonts()
	st := Style{Family: "mono", Size: 11}
	s := "select customer_id, count(*) from shop.orders group by customer_id order by 2 desc"
	max := m.Width("select customer_id, co", st)
	lines := Wrap(m, s, st, max)
	if len(lines) < 3 {
		t.Fatalf("expected several lines, got %v", lines)
	}
	for _, l := range lines {
		if w := m.Width(l, st); w > max {
			t.Errorf("line %q is %v wide, max %v", l, w, max)
		}
	}
	if strings.Join(lines, " ") != s {
		t.Errorf("words lost or reordered: %v", lines)
	}
	// A single word wider than max is split by characters.
	long := Wrap(m, strings.Repeat("x", 60), st, m.Width("xxxxxxxxxx", st))
	if len(long) != 6 {
		t.Errorf("long word split: %d lines", len(long))
	}
}

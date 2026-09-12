// Package text measures strings with real font metrics so layouts can size
// boxes before placing them. The default measurer embeds the Go fonts, which
// need no files on disk; callers can register any TrueType or OpenType font
// and should ship the same font to whatever renders the result.
package text

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Style is how a run of text is set. Family names a registered font family,
// Size is in pixels, Weight is a CSS weight (400 regular, 700 bold), and
// LineHeight is a multiple of Size (zero means 1.4).
type Style struct {
	Family     string
	Size       float64
	Weight     int
	LineHeight float64
}

func (s Style) lineHeight() float64 {
	if s.LineHeight > 0 {
		return s.LineHeight
	}
	return 1.4
}

// Metrics are vertical metrics for a style, in pixels.
type Metrics struct {
	Ascent, Descent, LineHeight float64
}

// Measurer reports text widths and vertical metrics.
type Measurer interface {
	Width(s string, st Style) float64
	Metrics(st Style) Metrics
}

// StyleSet maps style names to styles. The empty name is the fallback.
type StyleSet map[string]Style

// Get resolves a style name, falling back to "" and then erroring.
func (ss StyleSet) Get(name string) (Style, error) {
	if st, ok := ss[name]; ok {
		return st, nil
	}
	if st, ok := ss[""]; ok {
		return st, nil
	}
	return Style{}, fmt.Errorf("text: unknown style %q and no default", name)
}

// DefaultStyles is a style set for technical diagrams: monospace labels with
// a bold title style and a smaller muted detail style.
func DefaultStyles() StyleSet {
	return StyleSet{
		"":       {Family: "mono", Size: 11},
		"title":  {Family: "mono", Size: 11, Weight: 700},
		"detail": {Family: "mono", Size: 11},
		"muted":  {Family: "mono", Size: 11},
		"edge":   {Family: "mono", Size: 11},
		"sans":   {Family: "sans", Size: 12},
	}
}

// FontMeasurer measures with OpenType fonts.
type FontMeasurer struct {
	mu    sync.Mutex
	fonts map[fontKey]*opentype.Font
	faces map[faceKey]font.Face
}

type fontKey struct {
	family string
	bold   bool
}

type faceKey struct {
	fontKey
	size float64
}

// NewGoFonts returns a measurer with the Go fonts registered as "mono" and
// "sans", each in regular and bold.
func NewGoFonts() *FontMeasurer {
	m := &FontMeasurer{fonts: map[fontKey]*opentype.Font{}, faces: map[faceKey]font.Face{}}
	must := func(err error) {
		if err != nil {
			panic("text: embedded Go font failed to parse: " + err.Error())
		}
	}
	must(m.AddFont("mono", 400, gomono.TTF))
	must(m.AddFont("mono", 700, gomonobold.TTF))
	must(m.AddFont("sans", 400, goregular.TTF))
	must(m.AddFont("sans", 700, gobold.TTF))
	return m
}

// AddFont registers TrueType or OpenType data for a family at a weight.
// Weights of 600 and above are treated as bold; others as regular.
func (m *FontMeasurer) AddFont(family string, weight int, ttf []byte) error {
	f, err := opentype.Parse(ttf)
	if err != nil {
		return fmt.Errorf("text: parse font %s: %w", family, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fonts[fontKey{family, weight >= 600}] = f
	return nil
}

func (m *FontMeasurer) face(st Style) font.Face {
	size := st.Size
	if size <= 0 {
		size = 11
	}
	bold := st.Weight >= 600
	key := faceKey{fontKey{st.Family, bold}, size}
	m.mu.Lock()
	defer m.mu.Unlock()
	if f, ok := m.faces[key]; ok {
		return f
	}
	f, ok := m.fonts[key.fontKey]
	if !ok {
		f, ok = m.fonts[fontKey{st.Family, !bold}]
	}
	if !ok {
		f, ok = m.fonts[fontKey{"mono", bold}]
	}
	if !ok {
		for _, any := range m.fonts {
			f, ok = any, true
			break
		}
	}
	if !ok {
		panic("text: no fonts registered")
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic("text: create face: " + err.Error())
	}
	m.faces[key] = face
	return face
}

// Width returns the advance width of s in pixels.
func (m *FontMeasurer) Width(s string, st Style) float64 {
	if s == "" {
		return 0
	}
	return fixedToFloat(font.MeasureString(m.face(st), s))
}

// Metrics returns ascent, descent and line height for the style.
func (m *FontMeasurer) Metrics(st Style) Metrics {
	fm := m.face(st).Metrics()
	size := st.Size
	if size <= 0 {
		size = 11
	}
	return Metrics{
		Ascent:     fixedToFloat(fm.Ascent),
		Descent:    fixedToFloat(fm.Descent),
		LineHeight: size * st.lineHeight(),
	}
}

func fixedToFloat(v fixed.Int26_6) float64 { return float64(v) / 64 }

// Ellipsize shortens s with a middle ellipsis until it fits in max pixels.
// The ends of an identifier are usually more recognisable than its start
// alone, so the middle is removed. Returns "…" alone if even that is too wide.
func Ellipsize(m Measurer, s string, st Style, max float64) string {
	if m.Width(s, st) <= max {
		return s
	}
	const ell = "…"
	r := []rune(s)
	// Binary search the number of runes to keep.
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if m.Width(cutMiddle(r, mid, ell), st) <= max {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ell
	}
	return cutMiddle(r, lo, ell)
}

// cutMiddle keeps n runes of r, split between the two ends, with ell between.
func cutMiddle(r []rune, n int, ell string) string {
	if n >= len(r) {
		return string(r)
	}
	head := (n + 1) / 2
	tail := n - head
	return strings.TrimRight(string(r[:head]), " ") + ell + strings.TrimLeft(string(r[len(r)-tail:]), " ")
}

// Wrap breaks s into lines no wider than max pixels, at spaces where
// possible and inside words when a single word is too wide.
func Wrap(m Measurer, s string, st Style, max float64) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			candidate := w
			if line != "" {
				candidate = line + " " + w
			}
			if m.Width(candidate, st) <= max {
				line = candidate
				continue
			}
			if line != "" {
				out = append(out, line)
				line = ""
			}
			// Word alone is too wide: split it by runes.
			for m.Width(w, st) > max && len([]rune(w)) > 1 {
				r := []rune(w)
				n := len(r) - 1
				for n > 1 && m.Width(string(r[:n]), st) > max {
					n--
				}
				out = append(out, string(r[:n]))
				w = string(r[n:])
			}
			line = w
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

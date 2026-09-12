package diagram

import (
	"math"
	"sort"
)

// Group is a labelled boundary drawn around a set of nodes. Nodes join a
// group through Node.Group. Groups do not nest.
type Group struct {
	ID    string
	Label string
	Kind  string
}

// PlacedGroup is a group with its final box.
type PlacedGroup struct {
	*Group
	Rect    Rect
	Members []string
	// Label position: the text anchor (start) and baseline inside the box.
	LabelPos      Point
	LabelBaseline float64
}

// GroupPad is the space between a group's border and its members.
const GroupPad = 14

// GroupNode is a layout's view of one placed item for the group passes.
// Order is the item's normalised position within its layer (0..1) before
// coordinates exist; C is its centre on the cross axis afterwards, updated
// in place. Dummies take part with Dummy set so they are kept out of member
// lists; a dummy on an edge between two members of one group carries that
// group so the edge stays inside the box.
type GroupNode struct {
	ID     string
	Group  string
	Dummy  bool
	Layer  int
	Order  float64
	C      *float64
	Cross  float64
	RStart float64
	REnd   float64
}

// GroupBox is a group's extent in the abstract frame.
type GroupBox struct {
	MinC, MaxC, MinR, MaxR float64
	Members                []string
}

// AssignClasses gives every item a class index on the cross axis. Groups
// are ordered left to right by the mean position of their members and take
// the odd classes; items outside any group take the even class between the
// groups they fall between. Within a layer, an item is left of a group when
// it sits left of that group's members there; on layers without members the
// group's mean position decides. The returned groups are in class order.
func AssignClasses(nodes []GroupNode, groups []Group) (map[string]int, []Group) {
	if len(groups) == 0 {
		return nil, nil
	}
	type span struct{ lo, hi float64 }
	memberSpan := map[string]map[int]span{} // group -> layer -> span of Order
	meanPos := map[string]float64{}
	count := map[string]int{}
	for _, n := range nodes {
		if n.Group == "" {
			continue
		}
		if memberSpan[n.Group] == nil {
			memberSpan[n.Group] = map[int]span{}
		}
		sp, ok := memberSpan[n.Group][n.Layer]
		if !ok {
			sp = span{n.Order, n.Order}
		}
		sp.lo = math.Min(sp.lo, n.Order)
		sp.hi = math.Max(sp.hi, n.Order)
		memberSpan[n.Group][n.Layer] = sp
		meanPos[n.Group] += n.Order
		count[n.Group]++
	}
	var ordered []Group
	for _, g := range groups {
		if count[g.ID] > 0 {
			meanPos[g.ID] /= float64(count[g.ID])
			ordered = append(ordered, g)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return meanPos[ordered[i].ID] < meanPos[ordered[j].ID] })
	classOfGroup := map[string]int{}
	for i, g := range ordered {
		classOfGroup[g.ID] = 2*i + 1
	}
	cls := map[string]int{}
	for _, n := range nodes {
		if n.Group != "" {
			cls[n.ID] = classOfGroup[n.Group]
			continue
		}
		left := 0
		for _, g := range ordered {
			if sp, ok := memberSpan[g.ID][n.Layer]; ok {
				switch {
				case n.Order > sp.hi:
					left++
				case n.Order < sp.lo:
				default:
					// Between members on this layer: side with the nearer end.
					if n.Order-sp.lo > sp.hi-n.Order {
						left++
					}
				}
			} else if meanPos[g.ID] < n.Order {
				left++
			}
		}
		cls[n.ID] = 2 * left
	}
	return cls, ordered
}

// SeparateClasses shifts whole classes right, in class order, until every
// class interval is clear of the classes before it whose rank ranges it
// meets. Group classes include their padding and label space. Items are
// never re-ordered or moved relative to others in their class, so what the
// coordinate assignment did inside a class survives. Returns each group's
// box in the abstract frame.
func SeparateClasses(nodes []GroupNode, cls map[string]int, groups []Group, sep, labelExt float64, labelOnRank bool) map[string]GroupBox {
	if len(groups) == 0 {
		return nil
	}
	type interval struct {
		lo, hi, rlo, rhi float64
		items            []int
		group            string
	}
	byClass := map[int]*interval{}
	maxClass := 0
	for i, n := range nodes {
		c := cls[n.ID]
		iv := byClass[c]
		if iv == nil {
			iv = &interval{lo: math.Inf(1), hi: math.Inf(-1), rlo: math.Inf(1), rhi: math.Inf(-1)}
			byClass[c] = iv
		}
		iv.items = append(iv.items, i)
		iv.lo = math.Min(iv.lo, *n.C-n.Cross/2)
		iv.hi = math.Max(iv.hi, *n.C+n.Cross/2)
		iv.rlo = math.Min(iv.rlo, n.RStart)
		iv.rhi = math.Max(iv.rhi, n.REnd)
		if c > maxClass {
			maxClass = c
		}
	}
	for i, g := range groups {
		if iv := byClass[2*i+1]; iv != nil {
			iv.group = g.ID
			iv.lo -= GroupPad
			iv.hi += GroupPad
			iv.rlo -= GroupPad
			iv.rhi += GroupPad
			if labelOnRank {
				iv.rlo -= labelExt
			} else {
				iv.lo -= labelExt
			}
		}
	}
	for c := 1; c <= maxClass; c++ {
		iv := byClass[c]
		if iv == nil {
			continue
		}
		need := 0.0
		for p := 0; p < c; p++ {
			prev := byClass[p]
			if prev == nil || prev.rhi <= iv.rlo || iv.rhi <= prev.rlo {
				continue
			}
			if d := prev.hi + sep - iv.lo; d > need {
				need = d
			}
		}
		if need > 0 {
			for _, i := range iv.items {
				*nodes[i].C += need
			}
			iv.lo += need
			iv.hi += need
		}
	}
	out := map[string]GroupBox{}
	for i, g := range groups {
		iv := byClass[2*i+1]
		if iv == nil {
			continue
		}
		box := GroupBox{MinC: iv.lo, MaxC: iv.hi, MinR: iv.rlo, MaxR: iv.rhi}
		for _, k := range iv.items {
			if !nodes[k].Dummy {
				box.Members = append(box.Members, nodes[k].ID)
			}
		}
		out[g.ID] = box
	}
	return out
}

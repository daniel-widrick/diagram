package diagram

// FinishGroups maps abstract group boxes to placed groups in the final
// frame. shiftC is the cross-axis shift applied to nodes after the pass.
// The label sits at the top-left of the final box.
func FinishGroups(groups []Group, boxes map[string]GroupBox, frame Frame, shiftC float64, o Options) []*PlacedGroup {
	var out []*PlacedGroup
	for i := range groups {
		g := &groups[i]
		box, ok := boxes[g.ID]
		if !ok {
			continue
		}
		rect := frame.Rect(box.MinC+shiftC, box.MinR, box.MaxC-box.MinC, box.MaxR-box.MinR)
		pg := &PlacedGroup{Group: g, Rect: rect, Members: box.Members}
		if g.Label != "" {
			if st, err := o.Styles.Get("title"); err == nil {
				m := o.Measurer.Metrics(st)
				pg.LabelPos = Point{X: rect.X + 10, Y: rect.Y + 6}
				pg.LabelBaseline = m.Ascent
			}
		}
		out = append(out, pg)
	}
	return out
}

// GroupLabelExtent is the space a group reserves for its label.
func GroupLabelExtent(groups []Group, o Options) float64 {
	has := false
	for _, g := range groups {
		if g.Label != "" {
			has = true
		}
	}
	if !has {
		return 0
	}
	st, err := o.Styles.Get("title")
	if err != nil {
		return 20
	}
	return o.Measurer.Metrics(st).LineHeight + 8
}

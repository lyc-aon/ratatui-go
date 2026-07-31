package widgets

import (
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/text"
)

// BarGroup is a labeled set of bars shown together by a BarChart.
type BarGroup struct {
	Label *text.Line
	Bars  []Bar
}

// NewBarGroup creates a group from bars (slice is copied).
func NewBarGroup(bars []Bar) BarGroup {
	return BarGroup{Bars: copyBars(bars)}
}

// BarGroupWithLabel creates a labeled group (label and bars are copied).
func BarGroupWithLabel(label text.Line, bars []Bar) BarGroup {
	return BarGroup{
		Label: copyLinePtr(&label),
		Bars:  copyBars(bars),
	}
}

// WithLabel sets the group label (copied).
func (g BarGroup) WithLabel(label text.Line) BarGroup {
	g.Label = copyLinePtr(&label)
	return g
}

// WithBars replaces the bars (slice is copied).
func (g BarGroup) WithBars(bars []Bar) BarGroup {
	g.Bars = copyBars(bars)
	return g
}

// max returns the maximum bar value in the group, or 0 if empty.
func (g BarGroup) max() uint64 {
	var m uint64
	for i := range g.Bars {
		if g.Bars[i].Value > m {
			m = g.Bars[i].Value
		}
	}
	return m
}

// renderLabel paints the group label inside area using defaultLabelStyle as a
// base, honoring the label's own alignment (left/center/right).
//
// Matches Ratatui BarGroup::render_label: label width is NOT clamped to the
// group area; alignment x uses saturating subtraction so oversized labels
// still start at area.X. Height is taken from the passed area.
func (g BarGroup) renderLabel(buf *buffer.Buffer, area layout.Rect, defaultLabelStyle style.Style) {
	if buf == nil || g.Label == nil || area.IsEmpty() {
		return
	}
	label := *g.Label
	width := label.Width()
	if width < 0 {
		width = 0
	}
	// Do not clamp width to area.Width — upstream keeps the raw label width.
	aligned := layout.Rect{X: area.X, Y: area.Y, Width: width, Height: area.Height}
	if label.Alignment != nil {
		switch *label.Alignment {
		case layout.AlignCenter:
			aligned.X = area.X + saturatingSubInt(area.Width, width)/2
		case layout.AlignRight:
			aligned.X = area.X + saturatingSubInt(area.Width, width)
		}
	}
	buf.SetStyle(aligned, defaultLabelStyle)
	renderLineInArea(buf, aligned, label)
}

func copyBars(bars []Bar) []Bar {
	if len(bars) == 0 {
		if bars == nil {
			return nil
		}
		return []Bar{}
	}
	out := make([]Bar, len(bars))
	for i := range bars {
		out[i] = bars[i]
		out[i].Label = copyLinePtr(bars[i].Label)
		out[i].TextValue = copyStringPtr(bars[i].TextValue)
	}
	return out
}

func copyBarGroups(groups []BarGroup) []BarGroup {
	if len(groups) == 0 {
		if groups == nil {
			return nil
		}
		return []BarGroup{}
	}
	out := make([]BarGroup, len(groups))
	for i := range groups {
		out[i] = BarGroup{
			Label: copyLinePtr(groups[i].Label),
			Bars:  copyBars(groups[i].Bars),
		}
	}
	return out
}

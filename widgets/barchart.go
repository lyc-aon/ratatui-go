package widgets

import (
	"math/bits"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
	"github.com/lyc-aon/ratatui-go/text"
)

// BarChart renders values as vertical or horizontal bars, optionally grouped.
//
// Vertical bars use eighth-block symbols from BarSet (default nine levels).
// Horizontal bars use full/empty cells for length. Zero-size areas and
// bar_width==0 are no-ops (never panic).
type BarChart struct {
	block      *Block
	barWidth   int
	barGap     int
	groupGap   int
	barSet     symbols.BarSet
	barStyle   style.Style
	valueStyle style.Style
	labelStyle style.Style
	style      style.Style
	data       []BarGroup
	max        *uint64
	direction  layout.Direction
}

// NewBarChart creates a vertical BarChart from bars (copied; empty input yields empty data).
func NewBarChart(bars []Bar) BarChart {
	return VerticalBarChart(bars)
}

// VerticalBarChart creates a vertical BarChart from bars.
func VerticalBarChart(bars []Bar) BarChart {
	c := defaultBarChart()
	c.data = nonEmptyGroups([]BarGroup{NewBarGroup(bars)})
	c.direction = layout.VerticalDir
	return c
}

// HorizontalBarChart creates a horizontal BarChart from bars.
func HorizontalBarChart(bars []Bar) BarChart {
	c := defaultBarChart()
	c.data = nonEmptyGroups([]BarGroup{NewBarGroup(bars)})
	c.direction = layout.HorizontalDir
	return c
}

// GroupedBarChart creates a BarChart from groups (empty groups skipped; slices copied).
func GroupedBarChart(groups []BarGroup) BarChart {
	c := defaultBarChart()
	c.data = nonEmptyGroups(copyBarGroups(groups))
	return c
}

func defaultBarChart() BarChart {
	return BarChart{
		barWidth:  1,
		barGap:    1,
		groupGap:  0,
		barSet:    symbols.BarNineLevels,
		direction: layout.VerticalDir,
	}
}

func nonEmptyGroups(groups []BarGroup) []BarGroup {
	out := make([]BarGroup, 0, len(groups))
	for i := range groups {
		if len(groups[i].Bars) == 0 {
			continue
		}
		out = append(out, groups[i])
	}
	return out
}

// Data appends a bar group. Empty groups (no bars) are skipped.
// The group is copied.
func (c BarChart) Data(group BarGroup) BarChart {
	if len(group.Bars) == 0 {
		return c
	}
	g := BarGroup{
		Label: copyLinePtr(group.Label),
		Bars:  copyBars(group.Bars),
	}
	c.data = append(append([]BarGroup(nil), c.data...), g)
	return c
}

// DataPairs appends a group built from label/value pairs (convenience).
func (c BarChart) DataPairs(pairs []struct {
	Label string
	Value uint64
}) BarChart {
	if len(pairs) == 0 {
		return c
	}
	bars := make([]Bar, len(pairs))
	for i := range pairs {
		label := text.RawLine(pairs[i].Label)
		bars[i] = BarWithLabel(label, pairs[i].Value)
	}
	return c.Data(NewBarGroup(bars))
}

// Block surrounds the chart with a block.
func (c BarChart) Block(b Block) BarChart {
	cp := b
	c.block = &cp
	return c
}

// Max sets the value that maps to full bar length. When unset, uses data max.
func (c BarChart) Max(max uint64) BarChart {
	c.max = uint64Ptr(max)
	return c
}

// BarStyle sets the default bar style (patched by each bar's style).
func (c BarChart) BarStyle(s style.Style) BarChart {
	c.barStyle = s
	return c
}

// BarWidth sets bar thickness (width for vertical, height for horizontal). Default 1.
func (c BarChart) BarWidth(width int) BarChart {
	if width < 0 {
		width = 0
	}
	c.barWidth = width
	return c
}

// BarGap sets the gap between bars. Default 1.
func (c BarChart) BarGap(gap int) BarChart {
	if gap < 0 {
		gap = 0
	}
	c.barGap = gap
	return c
}

// BarSet sets the eighth-block symbol set used for vertical bars.
func (c BarChart) BarSet(set symbols.BarSet) BarChart {
	c.barSet = set
	return c
}

// ValueStyle sets the default style for value text on bars.
func (c BarChart) ValueStyle(s style.Style) BarChart {
	c.valueStyle = s
	return c
}

// LabelStyle sets the default style for bar and group labels.
func (c BarChart) LabelStyle(s style.Style) BarChart {
	c.labelStyle = s
	return c
}

// GroupGap sets the gap between groups. Default 0.
func (c BarChart) GroupGap(gap int) BarChart {
	if gap < 0 {
		gap = 0
	}
	c.groupGap = gap
	return c
}

// Style sets the widget base style (applied before the block).
func (c BarChart) Style(s style.Style) BarChart {
	c.style = s
	return c
}

// Direction sets vertical or horizontal bar layout.
func (c BarChart) Direction(d layout.Direction) BarChart {
	c.direction = d
	return c
}

// Render draws the chart into buf within area.
func (c BarChart) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}

	buf.SetStyle(area, c.style)
	inner := InnerIfSome(c.block, area, buf)
	if inner.IsEmpty() || len(c.data) == 0 || c.barWidth == 0 {
		return
	}

	switch c.direction {
	case layout.HorizontalDir:
		c.renderHorizontal(buf, inner)
	default:
		c.renderVertical(buf, inner)
	}
}

type labelInfo struct {
	groupLabelVisible bool
	barLabelVisible   bool
	height            int
}

// groupTicks returns visible bars' lengths in eighth-block ticks.
// availableSpace is the axis along which bars are laid out (width vertical,
// height horizontal). barMaxLength is the max bar size in cells.
func (c BarChart) groupTicks(availableSpace, barMaxLength int) [][]uint64 {
	if availableSpace <= 0 || c.barWidth <= 0 {
		return nil
	}
	maxVal := c.maximumDataValue()
	var out [][]uint64
	space := availableSpace
	for gi := range c.data {
		if space == 0 {
			break
		}
		group := c.data[gi]
		nBars := len(group.Bars)
		if nBars == 0 {
			continue
		}
		groupWidth := nBars*c.barWidth + saturatingSubInt(nBars, 1)*c.barGap

		var take int
		if space > groupWidth {
			space = saturatingSubInt(space, groupWidth+c.groupGap+c.barGap)
			take = nBars
		} else {
			denom := c.barWidth + c.barGap
			if denom <= 0 {
				break
			}
			maxBars := (space + c.barGap) / denom
			if maxBars > 0 {
				space = 0
				take = maxBars
			} else {
				break
			}
		}
		if take > nBars {
			take = nBars
		}
		ticks := make([]uint64, take)
		for i := range take {
			ticks[i] = scaleTicks(group.Bars[i].Value, maxVal, barMaxLength)
		}
		out = append(out, ticks)
	}
	return out
}

func scaleTicks(value, max uint64, maxLength int) uint64 {
	if max == 0 || maxLength <= 0 {
		return 0
	}
	maxTicks := uint64(maxLength) * 8
	ticks := mulDivU64(value, maxTicks, max)
	if ticks > maxTicks {
		return maxTicks
	}
	return ticks
}

// mulDivU64 returns (a*b)/c with 128-bit intermediate (matches Rust u128 path).
func mulDivU64(a, b, c uint64) uint64 {
	if c == 0 {
		return 0
	}
	hi, lo := bits.Mul64(a, b)
	if hi == 0 {
		return lo / c
	}
	// bits.Div64 requires hi < c.
	if hi >= c {
		hi = hi % c
	}
	q, _ := bits.Div64(hi, lo, c)
	return q
}

func (c BarChart) labelInfo(availableHeight int) labelInfo {
	if availableHeight <= 0 {
		return labelInfo{}
	}
	barLabelVisible := false
	for i := range c.data {
		for j := range c.data[i].Bars {
			if c.data[i].Bars[j].Label != nil {
				barLabelVisible = true
				break
			}
		}
		if barLabelVisible {
			break
		}
	}
	if availableHeight == 1 && barLabelVisible {
		return labelInfo{barLabelVisible: true, height: 1}
	}
	groupLabelVisible := false
	for i := range c.data {
		if c.data[i].Label != nil {
			groupLabelVisible = true
			break
		}
	}
	h := 0
	if groupLabelVisible {
		h++
	}
	if barLabelVisible {
		h++
	}
	return labelInfo{
		groupLabelVisible: groupLabelVisible,
		barLabelVisible:   barLabelVisible,
		height:            h,
	}
}

func (c BarChart) renderHorizontal(buf *buffer.Buffer, area layout.Rect) {
	labelSize := 0
	for i := range c.data {
		for j := range c.data[i].Bars {
			if c.data[i].Bars[j].Label == nil {
				continue
			}
			w := c.data[i].Bars[j].Label.Width()
			if w > labelSize {
				labelSize = w
			}
		}
	}

	labelX := area.X
	margin := 0
	if labelSize != 0 {
		margin = 1
	}
	barsArea := layout.NewRect(
		area.X+labelSize+margin,
		area.Y,
		saturatingSubInt(saturatingSubInt(area.Width, labelSize), margin),
		area.Height,
	)

	groupTicks := c.groupTicks(barsArea.Height, barsArea.Width)
	barY := barsArea.Y
	for gi, ticksVec := range groupTicks {
		if gi >= len(c.data) {
			break
		}
		group := c.data[gi]
		for bi, ticks := range ticksVec {
			if bi >= len(group.Bars) {
				break
			}
			bar := group.Bars[bi]
			barLength := int(ticks / 8)
			barStyle := c.barStyle.Patch(bar.Style)

			for y := range c.barWidth {
				yy := barY + y
				for x := range barsArea.Width {
					sym := c.barSet.Empty
					if x < barLength {
						sym = c.barSet.Full
					}
					if cell := buf.GetMut(barsArea.X+x, yy); cell != nil {
						cell.SetSymbol(sym).SetStyle(barStyle)
					}
				}
			}

			barValueArea := layout.NewRect(barsArea.X, barY+(c.barWidth>>1), barsArea.Width, 1)
			if bar.Label != nil {
				buf.SetLine(labelX, barValueArea.Y, *bar.Label, labelSize)
			}
			bar.renderValueWithDifferentStyles(buf, barValueArea, barLength, c.valueStyle, c.barStyle)

			barY += c.barGap + c.barWidth
		}

		// Group label sits in the gap after the group's bars when group_gap > 0.
		// Upstream: Rect { y: label_y, ..bars_area } — keeps bars_area height.
		labelY := barY - c.barGap
		if c.groupGap > 0 && labelY < barsArea.Bottom() {
			labelRect := layout.Rect{X: barsArea.X, Y: labelY, Width: barsArea.Width, Height: barsArea.Height}
			group.renderLabel(buf, labelRect, c.labelStyle)
			barY += c.groupGap
		}
	}
}

func (c BarChart) renderVertical(buf *buffer.Buffer, area layout.Rect) {
	info := c.labelInfo(saturatingSubInt(area.Height, 1))
	barsArea := layout.NewRect(area.X, area.Y, area.Width, saturatingSubInt(area.Height, info.height))
	groupTicks := c.groupTicks(barsArea.Width, barsArea.Height)
	c.renderVerticalBars(barsArea, buf, groupTicks)
	c.renderLabelsAndValues(area, buf, info, groupTicks)
}

func (c BarChart) renderVerticalBars(area layout.Rect, buf *buffer.Buffer, groupTicks [][]uint64) {
	barX := area.X
	for gi, ticksVec := range groupTicks {
		if gi >= len(c.data) {
			break
		}
		group := c.data[gi]
		for bi, ticks := range ticksVec {
			if bi >= len(group.Bars) {
				break
			}
			bar := group.Bars[bi]
			t := ticks
			for j := area.Height - 1; j >= 0; j-- {
				sym := symbolForBarTicks(c.barSet, t)
				barStyle := c.barStyle.Patch(bar.Style)
				for x := range c.barWidth {
					if cell := buf.GetMut(barX+x, area.Y+j); cell != nil {
						cell.SetSymbol(sym).SetStyle(barStyle)
					}
				}
				t = saturatingSubU64(t, 8)
			}
			barX += c.barGap + c.barWidth
		}
		barX += c.groupGap
	}
}

func (c BarChart) maximumDataValue() uint64 {
	var m uint64
	if c.max != nil {
		m = *c.max
	} else {
		for i := range c.data {
			if v := c.data[i].max(); v > m {
				m = v
			}
		}
	}
	if m < 1 {
		return 1
	}
	return m
}

func (c BarChart) renderLabelsAndValues(
	area layout.Rect,
	buf *buffer.Buffer,
	info labelInfo,
	groupTicks [][]uint64,
) {
	barX := area.X
	barY := area.Bottom() - info.height - 1
	for gi := range c.data {
		if gi >= len(groupTicks) {
			break
		}
		group := c.data[gi]
		ticksVec := groupTicks[gi]
		if len(group.Bars) == 0 {
			continue
		}
		if info.groupLabelVisible {
			// label_max_width = n_visible_bars * (bar_width + bar_gap) - bar_gap
			n := len(ticksVec)
			labelMaxWidth := n*(c.barWidth+c.barGap) - c.barGap
			if labelMaxWidth < 0 {
				labelMaxWidth = 0
			}
			groupArea := layout.NewRect(barX, area.Bottom()-1, labelMaxWidth, 1)
			group.renderLabel(buf, groupArea, c.labelStyle)
		}
		for bi := range group.Bars {
			if bi >= len(ticksVec) {
				break
			}
			bar := group.Bars[bi]
			ticks := ticksVec[bi]
			if info.barLabelVisible {
				bar.renderLabel(buf, c.barWidth, barX, barY+1, c.labelStyle)
			}
			bar.renderValue(buf, c.barWidth, barX, barY, c.valueStyle, ticks)
			barX += c.barGap + c.barWidth
		}
		barX += c.groupGap
	}
}

func symbolForBarTicks(set symbols.BarSet, ticks uint64) string {
	switch ticks {
	case 0:
		return set.Empty
	case 1:
		return set.OneEighth
	case 2:
		return set.OneQuarter
	case 3:
		return set.ThreeEighths
	case 4:
		return set.Half
	case 5:
		return set.FiveEighths
	case 6:
		return set.ThreeQuarters
	case 7:
		return set.SevenEighths
	default:
		return set.Full
	}
}

func saturatingSubInt(a, b int) int {
	if a < b {
		return 0
	}
	return a - b
}

func saturatingSubU64(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

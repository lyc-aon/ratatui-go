package widgets

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
	"github.com/lyc-aon/ratatui-go/text"
)

// GraphType selects how a Dataset is drawn on a Chart.
type GraphType int

const (
	// GraphScatter draws each point (default).
	GraphScatter GraphType = iota
	// GraphLine draws lines between consecutive points.
	GraphLine
	// GraphBar draws a vertical bar from y=0 to each point.
	GraphBar
	// GraphArea draws a line and fills to Dataset.FillToY.
	GraphArea
)

// String returns a stable name for the graph type.
func (g GraphType) String() string {
	switch g {
	case GraphScatter:
		return "Scatter"
	case GraphLine:
		return "Line"
	case GraphBar:
		return "Bar"
	case GraphArea:
		return "Area"
	default:
		return ""
	}
}

// LegendPosition places the chart legend inside the graph area.
type LegendPosition int

const (
	// LegendTop centers the legend on the top edge.
	LegendTop LegendPosition = iota
	// LegendTopRight is the default legend position.
	LegendTopRight
	// LegendTopLeft places the legend in the top-left corner.
	LegendTopLeft
	// LegendLeft centers the legend on the left edge.
	LegendLeft
	// LegendRight centers the legend on the right edge.
	LegendRight
	// LegendBottom centers the legend on the bottom edge.
	LegendBottom
	// LegendBottomRight places the legend in the bottom-right corner.
	LegendBottomRight
	// LegendBottomLeft places the legend in the bottom-left corner.
	LegendBottomLeft
)

// String returns a stable name for the legend position.
func (p LegendPosition) String() string {
	switch p {
	case LegendTop:
		return "Top"
	case LegendTopRight:
		return "TopRight"
	case LegendTopLeft:
		return "TopLeft"
	case LegendLeft:
		return "Left"
	case LegendRight:
		return "Right"
	case LegendBottom:
		return "Bottom"
	case LegendBottomRight:
		return "BottomRight"
	case LegendBottomLeft:
		return "BottomLeft"
	default:
		return ""
	}
}

// DefaultLegendPosition is TopRight (matches Rust Default).
const DefaultLegendPosition = LegendTopRight

// Axis is an X or Y axis for a Chart.
type Axis struct {
	title           *text.Line
	bounds          [2]float64
	labels          []text.Line
	style           style.Style
	labelsAlignment layout.Alignment
}

// NewAxis creates an empty axis (line only, zero bounds).
func NewAxis() Axis {
	return Axis{
		labelsAlignment: layout.AlignLeft,
	}
}

// Title sets the axis title (right end for X, top for Y).
func (a Axis) Title(title text.Line) Axis {
	t := copyLine(title)
	a.title = &t
	return a
}

// Bounds sets the [min, max] value range on this axis.
func (a Axis) Bounds(bounds [2]float64) Axis {
	a.bounds = bounds
	return a
}

// Labels sets axis labels (copied). X: left→right; Y: bottom→top.
// At least two labels are required for labels to render.
func (a Axis) Labels(labels ...text.Line) Axis {
	a.labels = copyAxisLabels(labels)
	return a
}

// Style sets the style used to draw the axis line.
func (a Axis) Style(st style.Style) Axis {
	a.style = st
	return a
}

// LabelsAlignment sets label alignment.
//
// Y axis: alignment within the left gutter.
// X axis: only the first label is affected (relative to the Y axis).
func (a Axis) LabelsAlignment(align layout.Alignment) Axis {
	a.labelsAlignment = align
	return a
}

func copyAxisLabels(in []text.Line) []text.Line {
	if len(in) == 0 {
		return nil
	}
	out := make([]text.Line, len(in))
	for i := range in {
		out[i] = copyLine(in[i])
	}
	return out
}

// Dataset is one named series plotted on a Chart.
//
// Data is owned (copied on set). Y grows upward (math orientation).
type Dataset struct {
	name      *text.Line
	data      [][2]float64
	marker    symbols.Marker
	graphType GraphType
	style     style.Style
	fillToY   float64
}

// NewDataset creates an empty scatter dataset with the default Dot marker.
func NewDataset() Dataset {
	return Dataset{
		marker:    symbols.Marker{Kind: symbols.MarkerDot},
		graphType: GraphScatter,
	}
}

// Name sets the legend name. Only named datasets appear in the legend.
func (d Dataset) Name(name text.Line) Dataset {
	n := copyLine(name)
	d.name = &n
	return d
}

// NameString is a convenience for Name(text.RawLine(s)).
func (d Dataset) NameString(s string) Dataset {
	return d.Name(text.RawLine(s))
}

// Data replaces the points. The slice is copied.
func (d Dataset) Data(data [][2]float64) Dataset {
	d.data = copyPoints2(data)
	return d
}

// Marker sets the canvas marker used when plotting this dataset.
func (d Dataset) Marker(m symbols.Marker) Dataset {
	d.marker = m
	return d
}

// GraphType sets scatter / line / bar / area rendering.
func (d Dataset) GraphType(t GraphType) Dataset {
	d.graphType = t
	return d
}

// Style sets the dataset style (legend uses full style; points use FG).
func (d Dataset) Style(st style.Style) Dataset {
	d.style = st
	return d
}

// FillToY sets the y baseline for GraphArea fills (default 0).
func (d Dataset) FillToY(y float64) Dataset {
	d.fillToY = y
	return d
}

// StyleValue returns the dataset style (for legend patching).
func (d Dataset) StyleValue() style.Style {
	return d.style
}

func copyPoints2(in [][2]float64) [][2]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make([][2]float64, len(in))
	copy(out, in)
	return out
}

// chartLayout holds resolved positions for chart elements.
type chartLayout struct {
	titleX     *layout.Position
	titleY     *layout.Position
	labelX     *int // y row for x labels
	labelY     *int // x col for y labels
	axisX      *int // y row of horizontal axis
	axisY      *int // x col of vertical axis
	legendArea *layout.Rect
	graphArea  layout.Rect
}

// Chart plots one or more Datasets on a cartesian plane.
type Chart struct {
	block                   *Block
	xAxis                   Axis
	yAxis                   Axis
	datasets                []Dataset
	style                   style.Style
	hiddenLegendConstraints [2]layout.Constraint
	legendPosition          *LegendPosition // nil = hide always
}

// NewChart creates a chart with the given datasets (copied).
// Default legend position is TopRight; legend hides when larger than 1/4 of the graph.
func NewChart(datasets ...Dataset) Chart {
	pos := DefaultLegendPosition
	return Chart{
		datasets: copyDatasets(datasets),
		hiddenLegendConstraints: [2]layout.Constraint{
			layout.Ratio(1, 4),
			layout.Ratio(1, 4),
		},
		legendPosition: &pos,
	}
}

func copyDatasets(in []Dataset) []Dataset {
	if len(in) == 0 {
		return nil
	}
	out := make([]Dataset, len(in))
	for i := range in {
		out[i] = in[i]
		// Deep-copy owned slices/pointers already held by value; re-copy data/name.
		out[i].data = copyPoints2(in[i].data)
		if in[i].name != nil {
			n := copyLine(*in[i].name)
			out[i].name = &n
		}
	}
	return out
}

// Block surrounds the chart with a block.
func (c Chart) Block(b Block) Chart {
	c.block = &b
	return c
}

// Style sets the base style of the whole chart area.
func (c Chart) Style(st style.Style) Chart {
	c.style = st
	return c
}

// XAxis sets the horizontal axis.
func (c Chart) XAxis(axis Axis) Chart {
	c.xAxis = axis
	return c
}

// YAxis sets the vertical axis.
func (c Chart) YAxis(axis Axis) Chart {
	c.yAxis = axis
	return c
}

// HiddenLegendConstraints sets (width, height) constraints; legend hides when
// it would exceed either resolved max size. Constraint Min always allows show.
func (c Chart) HiddenLegendConstraints(width, height layout.Constraint) Chart {
	c.hiddenLegendConstraints = [2]layout.Constraint{width, height}
	return c
}

// LegendPosition sets where the legend is drawn. Pass nil to hide always.
func (c Chart) LegendPosition(pos *LegendPosition) Chart {
	if pos == nil {
		c.legendPosition = nil
		return c
	}
	p := *pos
	c.legendPosition = &p
	return c
}

// Render draws the chart into buf within area.
//
// Intersects with buf.Area; empty/zero/minimal sizes never panic.
func (c Chart) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, c.style)
	chartArea := InnerIfSome(c.block, area, buf)
	layoutResolved, ok := c.resolveLayout(chartArea)
	if !ok {
		return
	}
	graphArea := layoutResolved.graphArea

	// Sample style under the graph for resetting legend/title overlays.
	originalStyle := c.style
	if cell, ok := buf.Get(area.X, area.Y); ok {
		originalStyle = cell.Style
	}

	c.renderXLabels(buf, &layoutResolved, chartArea, graphArea)
	c.renderYLabels(buf, &layoutResolved, chartArea, graphArea)

	if layoutResolved.axisX != nil {
		y := *layoutResolved.axisX
		for x := graphArea.Left(); x < graphArea.Right(); x++ {
			if cell := buf.GetMut(x, y); cell != nil {
				cell.SetSymbol(symbols.LineHorizontal).SetStyle(c.xAxis.style)
			}
		}
	}
	if layoutResolved.axisY != nil {
		x := *layoutResolved.axisY
		for y := graphArea.Top(); y < graphArea.Bottom(); y++ {
			if cell := buf.GetMut(x, y); cell != nil {
				cell.SetSymbol(symbols.LineVertical).SetStyle(c.yAxis.style)
			}
		}
	}
	if layoutResolved.axisX != nil && layoutResolved.axisY != nil {
		x, y := *layoutResolved.axisY, *layoutResolved.axisX
		if cell := buf.GetMut(x, y); cell != nil {
			cell.SetSymbol(symbols.LineBottomLeft).SetStyle(c.xAxis.style)
		}
	}

	// Plot datasets via Canvas.
	bg := style.Reset
	if c.style.HasBG {
		bg = c.style.BG
	}
	datasets := c.datasets
	NewCanvas().
		BackgroundColor(bg).
		XBounds(c.xAxis.bounds).
		YBounds(c.yAxis.bounds).
		Paint(func(ctx *Context) {
			for i := range datasets {
				ds := &datasets[i]
				ctx.SetMarker(ds.marker)
				color := style.Reset
				if ds.style.HasFG {
					color = ds.style.FG
				}
				ctx.Draw(Points{Coords: ds.data, Color: color})
				switch ds.graphType {
				case GraphLine:
					for j := range len(ds.data) - 1 {
						ctx.Draw(Line{
							X1: ds.data[j][0], Y1: ds.data[j][1],
							X2: ds.data[j+1][0], Y2: ds.data[j+1][1],
							Color: color,
						})
					}
				case GraphBar:
					for j := range ds.data {
						ctx.Draw(Line{
							X1: ds.data[j][0], Y1: 0,
							X2: ds.data[j][0], Y2: ds.data[j][1],
							Color: color,
						})
					}
				case GraphArea:
					for j := range len(ds.data) - 1 {
						ctx.Draw(FilledLine{
							X1: ds.data[j][0], Y1: ds.data[j][1],
							X2: ds.data[j+1][0], Y2: ds.data[j+1][1],
							FillToY: ds.fillToY,
							Color:   color,
						})
					}
				case GraphScatter:
					// points already drawn
				}
			}
		}).
		Render(graphArea, buf)

	if layoutResolved.titleX != nil && c.xAxis.title != nil {
		p := *layoutResolved.titleX
		title := c.xAxis.title
		w := minIntChart(graphArea.Right()-p.X, title.Width())
		if w > 0 {
			titleArea := layout.NewRect(p.X, p.Y, w, 1)
			buf.SetStyle(titleArea, originalStyle)
			buf.SetLine(p.X, p.Y, *title, w)
		}
	}
	if layoutResolved.titleY != nil && c.yAxis.title != nil {
		p := *layoutResolved.titleY
		title := c.yAxis.title
		w := minIntChart(graphArea.Right()-p.X, title.Width())
		if w > 0 {
			titleArea := layout.NewRect(p.X, p.Y, w, 1)
			buf.SetStyle(titleArea, originalStyle)
			buf.SetLine(p.X, p.Y, *title, w)
		}
	}

	if layoutResolved.legendArea != nil {
		la := *layoutResolved.legendArea
		buf.SetStyle(la, originalStyle)
		Bordered().Render(la, buf)
		row := 0
		for i := range c.datasets {
			ds := &c.datasets[i]
			if ds.name == nil {
				continue
			}
			name := ds.name.PatchStyle(ds.style)
			nameArea := layout.NewRect(la.X+1, la.Y+1+row, satSub(la.Width, 2), 1)
			// Line::render fills the whole name row with the patched style.
			buf.SetStyle(nameArea.Intersection(buf.Area), name.Style)
			renderLineInArea(buf, nameArea, name)
			row++
		}
	}
}

// resolveLayout computes element positions. Returns ok=false when area is empty.
func (c Chart) resolveLayout(area layout.Rect) (chartLayout, bool) {
	if area.Height == 0 || area.Width == 0 {
		return chartLayout{}, false
	}
	x := area.Left()
	y := area.Bottom() - 1

	var labelX *int
	if len(c.xAxis.labels) > 0 && y > area.Top() {
		v := y
		labelX = &v
		y--
	}

	var labelY *int
	hasYLabels := len(c.yAxis.labels) > 0
	if hasYLabels {
		v := x
		labelY = &v
	}
	x += c.maxWidthOfLabelsLeftOfYAxis(area, hasYLabels)

	var axisX *int
	if len(c.xAxis.labels) > 0 && y > area.Top() {
		v := y
		axisX = &v
		y--
	}

	var axisY *int
	if hasYLabels && x+1 < area.Right() {
		v := x
		axisY = &v
		x++
	}

	graphWidth := satSub(area.Right(), x)
	graphHeight := satAdd(satSub(y, area.Top()), 1)
	// graphWidth/graphHeight stay ≥1: label gutter capped at width/3 and axisY
	// only claims a column when x+1 < right (upstream debug_assert_ne!).
	graphArea := layout.NewRect(x, area.Top(), graphWidth, graphHeight)

	var titleX *layout.Position
	if c.xAxis.title != nil {
		w := c.xAxis.title.Width()
		if w < graphArea.Width && graphArea.Height > 2 {
			p := layout.NewPosition(x+graphArea.Width-w, y)
			titleX = &p
		}
	}

	var titleY *layout.Position
	if c.yAxis.title != nil {
		w := c.yAxis.title.Width()
		if w+1 < graphArea.Width && graphArea.Height > 2 {
			p := layout.NewPosition(x, area.Top())
			titleY = &p
		}
	}

	var legendArea *layout.Rect
	if c.legendPosition != nil {
		// Collect named dataset widths.
		maxInner := 0
		namedCount := 0
		for i := range c.datasets {
			if c.datasets[i].name == nil {
				continue
			}
			namedCount++
			if w := c.datasets[i].name.Width(); w > maxInner {
				maxInner = w
			}
		}
		if maxInner > 0 && namedCount > 0 {
			legendWidth := maxInner + 2
			legendHeight := namedCount + 2

			maxWParts := layout.Horizontal(c.hiddenLegendConstraints[0]).
				Flex(layout.FlexStart).
				Split(graphArea)
			maxHParts := layout.Vertical(c.hiddenLegendConstraints[1]).
				Flex(layout.FlexStart).
				Split(graphArea)
			maxLegendW, maxLegendH := 0, 0
			if len(maxWParts) > 0 {
				maxLegendW = maxWParts[0].Width
			}
			if len(maxHParts) > 0 {
				maxLegendH = maxHParts[0].Height
			}

			if legendWidth <= maxLegendW && legendHeight <= maxLegendH {
				xTitleW := 0
				if titleX != nil && c.xAxis.title != nil {
					xTitleW = c.xAxis.title.Width()
				}
				yTitleW := 0
				if titleY != nil && c.yAxis.title != nil {
					yTitleW = c.yAxis.title.Width()
				}
				if la, ok := legendPositionLayout(*c.legendPosition, graphArea, legendWidth, legendHeight, xTitleW, yTitleW); ok {
					legendArea = &la
				}
			}
		}
	}

	return chartLayout{
		titleX:     titleX,
		titleY:     titleY,
		labelX:     labelX,
		labelY:     labelY,
		axisX:      axisX,
		axisY:      axisY,
		legendArea: legendArea,
		graphArea:  graphArea,
	}, true
}

func (c Chart) maxWidthOfLabelsLeftOfYAxis(area layout.Rect, hasYAxis bool) int {
	maxWidth := 0
	for i := range c.yAxis.labels {
		if w := c.yAxis.labels[i].Width(); w > maxWidth {
			maxWidth = w
		}
	}
	if len(c.xAxis.labels) > 0 {
		first := c.xAxis.labels[0].Width()
		widthLeft := 0
		switch c.xAxis.labelsAlignment {
		case layout.AlignLeft:
			yOff := 0
			if hasYAxis {
				yOff = 1
			}
			widthLeft = satSub(first, yOff)
		case layout.AlignCenter:
			widthLeft = first / 2
		case layout.AlignRight:
			widthLeft = 0
		}
		if widthLeft > maxWidth {
			maxWidth = widthLeft
		}
	}
	// Cap at 1/3 of total width.
	cap := area.Width / 3
	if maxWidth > cap {
		maxWidth = cap
	}
	return maxWidth
}

func (c Chart) renderXLabels(buf *buffer.Buffer, lay *chartLayout, chartArea, graphArea layout.Rect) {
	if lay.labelX == nil {
		return
	}
	y := *lay.labelX
	labels := c.xAxis.labels
	labelsLen := len(labels)
	if labelsLen < 2 {
		return
	}

	widthBetweenTicks := graphArea.Width / labelsLen

	firstArea := c.firstXLabelArea(
		y,
		labels[0].Width(),
		widthBetweenTicks,
		chartArea,
		graphArea,
	)
	// First label alignment is inverted relative to axis labels_alignment.
	firstAlign := layout.AlignRight
	switch c.xAxis.labelsAlignment {
	case layout.AlignLeft:
		firstAlign = layout.AlignRight
	case layout.AlignCenter:
		firstAlign = layout.AlignCenter
	case layout.AlignRight:
		firstAlign = layout.AlignLeft
	}
	renderChartLabel(buf, labels[0], firstArea, firstAlign)

	for i, label := range labels[1 : labelsLen-1] {
		x := graphArea.Left() + (i+1)*widthBetweenTicks + 1
		labelArea := layout.NewRect(x, y, satSub(widthBetweenTicks, 1), 1)
		renderChartLabel(buf, label, labelArea, layout.AlignCenter)
	}

	x := graphArea.Right() - widthBetweenTicks
	lastArea := layout.NewRect(x, y, widthBetweenTicks, 1)
	renderChartLabel(buf, labels[labelsLen-1], lastArea, layout.AlignRight)
}

func (c Chart) firstXLabelArea(
	y, labelWidth, maxWidthAfterYAxis int,
	chartArea, graphArea layout.Rect,
) layout.Rect {
	var minX, maxX int
	switch c.xAxis.labelsAlignment {
	case layout.AlignLeft:
		minX = chartArea.Left()
		maxX = graphArea.Left()
	case layout.AlignCenter:
		minX = chartArea.Left()
		maxX = graphArea.Left() + minIntChart(maxWidthAfterYAxis, labelWidth)
	case layout.AlignRight:
		minX = satSub(graphArea.Left(), 1)
		maxX = graphArea.Left() + maxWidthAfterYAxis
	default:
		minX = chartArea.Left()
		maxX = graphArea.Left()
	}
	return layout.NewRect(minX, y, satSub(maxX, minX), 1)
}

func (c Chart) renderYLabels(buf *buffer.Buffer, lay *chartLayout, chartArea, graphArea layout.Rect) {
	if lay.labelY == nil {
		return
	}
	x := *lay.labelY
	labels := c.yAxis.labels
	labelsLen := len(labels)
	if labelsLen < 2 {
		return
	}
	for i, label := range labels {
		dy := 0
		if labelsLen > 1 && graphArea.Height > 0 {
			dy = i * (graphArea.Height - 1) / (labelsLen - 1)
		}
		labelY := satSub(graphArea.Bottom()-1, dy)
		if labelY < graphArea.Bottom() {
			width := satSub(graphArea.Left()-chartArea.Left(), 1)
			labelArea := layout.NewRect(x, labelY, width, 1)
			renderChartLabel(buf, label, labelArea, c.yAxis.labelsAlignment)
		}
	}
}

func renderChartLabel(buf *buffer.Buffer, label text.Line, area layout.Rect, align layout.Alignment) {
	if buf == nil || area.IsEmpty() {
		return
	}
	switch align {
	case layout.AlignLeft:
		label = label.LeftAligned()
	case layout.AlignCenter:
		label = label.Centered()
	case layout.AlignRight:
		label = label.RightAligned()
	}
	// Apply line style on the area then place aligned content.
	buf.SetStyle(area.Intersection(buf.Area), label.Style)
	renderLineInArea(buf, area, label)
}

// legendPositionLayout places the legend rect, or returns ok=false when it won't fit.
func legendPositionLayout(
	pos LegendPosition,
	area layout.Rect,
	legendWidth, legendHeight, xTitleWidth, yTitleWidth int,
) (layout.Rect, bool) {
	heightMargin := area.Height - legendHeight
	if xTitleWidth != 0 {
		heightMargin--
	}
	if yTitleWidth != 0 {
		heightMargin--
	}
	if heightMargin < 0 {
		return layout.Rect{}, false
	}

	var x, y int
	switch pos {
	case LegendTopRight:
		if legendWidth+yTitleWidth > area.Width {
			x = area.Right() - legendWidth
			y = area.Top() + 1
		} else {
			x = area.Right() - legendWidth
			y = area.Top()
		}
	case LegendTopLeft:
		if yTitleWidth != 0 {
			x = area.Left()
			y = area.Top() + 1
		} else {
			x = area.Left()
			y = area.Top()
		}
	case LegendTop:
		// Upstream compares absolute left+y_title_width against the RELATIVE
		// centering offset x (not left+x). Replicated for layout parity even
		// when graphArea.Left()>0 (y-label gutters); tests only cover left==0.
		xOff := (area.Width - legendWidth) / 2
		if area.Left()+yTitleWidth > xOff {
			x = area.Left() + xOff
			y = area.Top() + 1
		} else {
			x = area.Left() + xOff
			y = area.Top()
		}
	case LegendLeft:
		yy := (area.Height - legendHeight) / 2
		if yTitleWidth != 0 {
			yy++
		}
		if xTitleWidth != 0 {
			yy = satSub(yy, 1)
		}
		x = area.Left()
		y = area.Top() + yy
	case LegendRight:
		yy := (area.Height - legendHeight) / 2
		if yTitleWidth != 0 {
			yy++
		}
		if xTitleWidth != 0 {
			yy = satSub(yy, 1)
		}
		x = area.Right() - legendWidth
		y = area.Top() + yy
	case LegendBottomLeft:
		if xTitleWidth+legendWidth > area.Width {
			x = area.Left()
			y = area.Bottom() - legendHeight - 1
		} else {
			x = area.Left()
			y = area.Bottom() - legendHeight
		}
	case LegendBottomRight:
		if xTitleWidth != 0 {
			x = area.Right() - legendWidth
			y = area.Bottom() - legendHeight - 1
		} else {
			x = area.Right() - legendWidth
			y = area.Bottom() - legendHeight
		}
	case LegendBottom:
		x = area.Left() + (area.Width-legendWidth)/2
		if x+legendWidth > area.Right()-xTitleWidth {
			y = area.Bottom() - legendHeight - 1
		} else {
			y = area.Bottom() - legendHeight
		}
	default:
		x = area.Right() - legendWidth
		y = area.Top()
	}
	return layout.NewRect(x, y, legendWidth, legendHeight), true
}

func minIntChart(a, b int) int {
	if a < b {
		return a
	}
	return b
}

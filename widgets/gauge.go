package widgets

import (
	"fmt"
	"math"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/symbols"
	"github.com/michaelkelly/ratatui-go/text"
)

// Gauge is a horizontal progress bar with an optional centered label.
//
// Percent/Ratio clamp into range instead of panicking (Go contract).
// With UseUnicode, the fractional cell uses eighth-block symbols.
type Gauge struct {
	block      *Block
	ratio      float64
	label      *text.Span
	useUnicode bool
	style      style.Style
	gaugeStyle style.Style
}

// NewGauge creates an empty gauge at 0%.
func NewGauge() Gauge {
	return Gauge{}
}

// Block surrounds the gauge with a block.
func (g Gauge) Block(b Block) Gauge {
	cp := b
	g.block = &cp
	return g
}

// Percent sets progress from a percentage. Values outside 0..100 are clamped.
func (g Gauge) Percent(percent int) Gauge {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	g.ratio = float64(percent) / 100.0
	return g
}

// Ratio sets progress from a float in 0..1. Values outside are clamped.
// NaN becomes 0.
func (g Gauge) Ratio(ratio float64) Gauge {
	if math.IsNaN(ratio) || ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	g.ratio = ratio
	return g
}

// Label sets the centered label. When unset, the default is "N%".
func (g Gauge) Label(label text.Span) Gauge {
	cp := label
	g.label = &cp
	return g
}

// Style sets the widget base style (area behind the bar / block).
func (g Gauge) Style(s style.Style) Gauge {
	g.style = s
	return g
}

// GaugeStyle sets the style of the filled bar.
func (g Gauge) GaugeStyle(s style.Style) Gauge {
	g.gaugeStyle = s
	return g
}

// UseUnicode enables eighth-block fractional cells for higher precision.
func (g Gauge) UseUnicode(unicode bool) Gauge {
	g.useUnicode = unicode
	return g
}

// Render draws the gauge into buf within area.
func (g Gauge) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, g.style)
	inner := InnerIfSome(g.block, area, buf)
	g.renderGauge(inner, buf)
}

func (g Gauge) renderGauge(gaugeArea layout.Rect, buf *buffer.Buffer) {
	if buf == nil || gaugeArea.IsEmpty() {
		return
	}

	buf.SetStyle(gaugeArea, g.gaugeStyle)

	defaultLabel := text.RawSpan(fmt.Sprintf("%d%%", int(math.Round(g.ratio*100.0))))
	label := defaultLabel
	if g.label != nil {
		label = *g.label
	}
	labelWidth := label.Width()
	clampedLabelWidth := gaugeArea.Width
	if labelWidth < clampedLabelWidth {
		clampedLabelWidth = labelWidth
	}
	labelCol := gaugeArea.X + (gaugeArea.Width-clampedLabelWidth)/2
	labelRow := gaugeArea.Y + gaugeArea.Height/2

	filledWidth := float64(gaugeArea.Width) * g.ratio
	var end int
	if g.useUnicode {
		end = gaugeArea.X + int(math.Floor(filledWidth))
	} else {
		end = gaugeArea.X + int(math.Round(filledWidth))
	}

	fg := style.Reset
	bg := style.Reset
	if g.gaugeStyle.HasFG {
		fg = g.gaugeStyle.FG
	}
	if g.gaugeStyle.HasBG {
		bg = g.gaugeStyle.BG
	}
	filledStyle := style.New().WithFG(fg).WithBG(bg)
	// Label region over the fill swaps fg/bg so the label stays readable.
	// Matches Ratatui: x in [labelCol, labelCol+clampedLabelWidth] inclusive.
	labelFillStyle := style.New().WithFG(bg).WithBG(fg)

	for y := gaugeArea.Y; y < gaugeArea.Bottom(); y++ {
		for x := gaugeArea.X; x < end; x++ {
			cell := buf.GetMut(x, y)
			if cell == nil {
				continue
			}
			inLabelBand := y == labelRow && x >= labelCol && x <= labelCol+clampedLabelWidth
			if !inLabelBand {
				cell.SetSymbol(symbols.BlockFull).SetStyle(filledStyle)
			} else {
				cell.SetSymbol(" ").SetStyle(labelFillStyle)
			}
		}
		if g.useUnicode && g.ratio < 1.0 {
			frac := filledWidth - math.Floor(filledWidth)
			if cell := buf.GetMut(end, y); cell != nil {
				cell.SetSymbol(getUnicodeBlock(frac))
			}
		}
	}

	buf.SetSpan(labelCol, labelRow, label, clampedLabelWidth)
}

func getUnicodeBlock(frac float64) string {
	switch uint16(math.Round(frac * 8.0)) {
	case 1:
		return symbols.BlockOneEighth
	case 2:
		return symbols.BlockOneQuarter
	case 3:
		return symbols.BlockThreeEighths
	case 4:
		return symbols.BlockHalf
	case 5:
		return symbols.BlockFiveEighths
	case 6:
		return symbols.BlockThreeQuarters
	case 7:
		return symbols.BlockSevenEighths
	case 8:
		return symbols.BlockFull
	default:
		return " "
	}
}

// LineGauge is a single-row progress bar with a left label and filled/unfilled symbols.
type LineGauge struct {
	block          *Block
	ratio          float64
	label          *text.Line
	style          style.Style
	filledSymbol   string
	unfilledSymbol string
	filledStyle    style.Style
	unfilledStyle  style.Style
}

// NewLineGauge creates a line gauge at 0% with horizontal line symbols.
func NewLineGauge() LineGauge {
	return LineGauge{
		filledSymbol:   symbols.LineHorizontal,
		unfilledSymbol: symbols.LineHorizontal,
	}
}

// Block surrounds the line gauge with a block.
func (g LineGauge) Block(b Block) LineGauge {
	cp := b
	g.block = &cp
	return g
}

// Ratio sets progress from a float in 0..1. Values outside are clamped. NaN → 0.
func (g LineGauge) Ratio(ratio float64) LineGauge {
	if math.IsNaN(ratio) || ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	g.ratio = ratio
	return g
}

// Label sets the left label. When unset, default is a right-padded "N%".
func (g LineGauge) Label(label text.Line) LineGauge {
	g.label = copyLinePtr(&label)
	return g
}

// FilledSymbol sets the symbol for the filled portion.
func (g LineGauge) FilledSymbol(symbol string) LineGauge {
	g.filledSymbol = symbol
	return g
}

// UnfilledSymbol sets the symbol for the unfilled portion.
func (g LineGauge) UnfilledSymbol(symbol string) LineGauge {
	g.unfilledSymbol = symbol
	return g
}

// FilledStyle sets the style of the filled portion.
func (g LineGauge) FilledStyle(s style.Style) LineGauge {
	g.filledStyle = s
	return g
}

// UnfilledStyle sets the style of the unfilled portion.
func (g LineGauge) UnfilledStyle(s style.Style) LineGauge {
	g.unfilledStyle = s
	return g
}

// Style sets the widget base style.
func (g LineGauge) Style(s style.Style) LineGauge {
	g.style = s
	return g
}

// Render draws the line gauge into buf within area (uses the top row of inner).
func (g LineGauge) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, g.style)
	gaugeArea := InnerIfSome(g.block, area, buf)
	if gaugeArea.IsEmpty() {
		return
	}

	defaultLabel := text.RawLine(fmt.Sprintf("%3.0f%%", g.ratio*100.0))
	label := defaultLabel
	if g.label != nil {
		label = *g.label
	}
	col, row := buf.SetLine(gaugeArea.X, gaugeArea.Y, label, gaugeArea.Width)
	start := col + 1
	if start >= gaugeArea.Right() {
		return
	}

	remain := gaugeArea.Right() - start
	if remain < 0 {
		remain = 0
	}
	filled := int(math.Floor(float64(remain) * g.ratio))
	end := start + filled
	if end > gaugeArea.Right() {
		end = gaugeArea.Right()
	}

	filledSym := g.filledSymbol
	if filledSym == "" {
		filledSym = symbols.LineHorizontal
	}
	unfilledSym := g.unfilledSymbol
	if unfilledSym == "" {
		unfilledSym = symbols.LineHorizontal
	}

	for x := start; x < end; x++ {
		if cell := buf.GetMut(x, row); cell != nil {
			cell.SetSymbol(filledSym).SetStyle(g.filledStyle)
		}
	}
	for x := end; x < gaugeArea.Right(); x++ {
		if cell := buf.GetMut(x, row); cell != nil {
			cell.SetSymbol(unfilledSym).SetStyle(g.unfilledStyle)
		}
	}
}

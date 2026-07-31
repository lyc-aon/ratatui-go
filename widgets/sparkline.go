package widgets

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
)

// RenderDirection controls sparkline column order.
type RenderDirection int

const (
	// RenderLeftToRight places the first data value on the left (default).
	RenderLeftToRight RenderDirection = iota
	// RenderRightToLeft places the first data value on the right.
	RenderRightToLeft
)

// String returns a stable name for the render direction.
func (d RenderDirection) String() string {
	switch d {
	case RenderRightToLeft:
		return "RightToLeft"
	default:
		return "LeftToRight"
	}
}

// SparklineBar is one column in a Sparkline.
//
// Absent is distinct from a zero value: absent columns use the sparkline's
// absent symbol/style across the full height.
type SparklineBar struct {
	Value  uint64
	Absent bool
	Style  *style.Style
}

// NewSparklineBar creates a present bar with the given value.
func NewSparklineBar(value uint64) SparklineBar {
	return SparklineBar{Value: value}
}

// AbsentSparklineBar creates an absent bar.
func AbsentSparklineBar() SparklineBar {
	return SparklineBar{Absent: true}
}

// WithStyle sets an optional per-bar style (copied).
func (b SparklineBar) WithStyle(s style.Style) SparklineBar {
	cp := s
	b.Style = &cp
	return b
}

// Sparkline renders a compact multi-row bar graph over a data series.
//
// Each column is scaled into eighth-block ticks (value * height * 8 / max).
// Data longer than the area width is truncated to the first width values
// (Ratatui behavior). RightToLeft mirrors column order.
type Sparkline struct {
	block             *Block
	style             style.Style
	absentValueStyle  style.Style
	absentValueSymbol string
	data              []SparklineBar
	max               *uint64
	barSet            symbols.BarSet
	direction         RenderDirection
}

// NewSparkline creates an empty sparkline with nine-level bars.
func NewSparkline() Sparkline {
	return Sparkline{
		absentValueSymbol: symbols.ShadeEmpty,
		barSet:            symbols.BarNineLevels,
		direction:         RenderLeftToRight,
	}
}

// Block surrounds the sparkline with a block.
func (s Sparkline) Block(b Block) Sparkline {
	cp := b
	s.block = &cp
	return s
}

// Style sets the default bar style (fg = bars, bg = everything else per cell).
func (s Sparkline) Style(st style.Style) Sparkline {
	s.style = st
	return s
}

// AbsentValueStyle sets the style used for absent data points.
func (s Sparkline) AbsentValueStyle(st style.Style) Sparkline {
	s.absentValueStyle = st
	return s
}

// AbsentValueSymbol sets the symbol used for absent data points. Default is a space.
func (s Sparkline) AbsentValueSymbol(symbol string) Sparkline {
	s.absentValueSymbol = symbol
	return s
}

// Data replaces the dataset (copied).
func (s Sparkline) Data(data ...SparklineBar) Sparkline {
	s.data = copySparklineBars(data)
	return s
}

// DataUint64 sets data from uint64 values (all present).
func (s Sparkline) DataUint64(values ...uint64) Sparkline {
	bars := make([]SparklineBar, len(values))
	for i, v := range values {
		bars[i] = NewSparklineBar(v)
	}
	s.data = bars
	return s
}

// Max sets the value that maps to full height. When unset, uses the data max
// (or 1 when the series is empty/all-absent).
func (s Sparkline) Max(max uint64) Sparkline {
	s.max = uint64Ptr(max)
	return s
}

// BarSet sets the eighth-block symbol set. Default is symbols.BarNineLevels.
func (s Sparkline) BarSet(set symbols.BarSet) Sparkline {
	s.barSet = set
	return s
}

// Direction sets left-to-right or right-to-left rendering.
func (s Sparkline) Direction(d RenderDirection) Sparkline {
	s.direction = d
	return s
}

// Render draws the sparkline into buf within area.
func (s Sparkline) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	// Ratatui sparkline does not paint a base style over the full area; style is
	// applied per bar cell. Block (if any) still renders first via InnerIfSome.
	inner := InnerIfSome(s.block, area, buf)
	s.renderSparkline(inner, buf)
}

func (s Sparkline) renderSparkline(sparkArea layout.Rect, buf *buffer.Buffer) {
	if buf == nil || sparkArea.IsEmpty() {
		return
	}

	maxHeight := uint64(1)
	if s.max != nil {
		maxHeight = *s.max
	} else {
		var found bool
		var m uint64
		for i := range s.data {
			if s.data[i].Absent {
				continue
			}
			if !found || s.data[i].Value > m {
				m = s.data[i].Value
				found = true
			}
		}
		if found {
			maxHeight = m
		}
	}

	maxIndex := sparkArea.Width
	if maxIndex > len(s.data) {
		maxIndex = len(s.data)
	}

	absentSym := s.absentValueSymbol
	if absentSym == "" {
		absentSym = symbols.ShadeEmpty
	}

	for i := range maxIndex {
		item := s.data[i]
		var x int
		switch s.direction {
		case RenderRightToLeft:
			x = sparkArea.Right() - i - 1
		default:
			x = sparkArea.X + i
		}

		var height uint64
		var fixedSymbol string
		var useFixed bool
		var barStyle style.Style

		if !item.Absent {
			height = scaleSparkHeight(item.Value, maxHeight, sparkArea.Height)
			if item.Style != nil {
				barStyle = *item.Style
			}
		} else {
			height = uint64(sparkArea.Height) * 8
			fixedSymbol = absentSym
			useFixed = true
			barStyle = s.absentValueStyle
		}

		for j := sparkArea.Height - 1; j >= 0; j-- {
			sym := fixedSymbol
			if !useFixed {
				sym = s.symbolForHeight(height)
			}
			if height >= 8 {
				height -= 8
			} else {
				height = 0
			}
			st := s.style.Patch(barStyle)
			if cell := buf.GetMut(x, sparkArea.Y+j); cell != nil {
				cell.SetSymbol(sym).SetStyle(st)
			}
		}
	}
}

func (s Sparkline) symbolForHeight(height uint64) string {
	return symbolForBarTicks(s.barSet, height)
}

func scaleSparkHeight(value, max uint64, maxHeight int) uint64 {
	if max == 0 || maxHeight <= 0 {
		return 0
	}
	maxTicks := uint64(maxHeight) * 8
	ticks := mulDivU64(value, maxTicks, max)
	if ticks > maxTicks {
		return maxTicks
	}
	return ticks
}

func copySparklineBars(bars []SparklineBar) []SparklineBar {
	if len(bars) == 0 {
		if bars == nil {
			return nil
		}
		return []SparklineBar{}
	}
	out := make([]SparklineBar, len(bars))
	for i := range bars {
		out[i] = bars[i]
		if bars[i].Style != nil {
			st := *bars[i].Style
			out[i].Style = &st
		}
	}
	return out
}

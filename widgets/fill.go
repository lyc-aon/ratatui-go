package widgets

import (
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
)

// Fill paints every cell in its area with a single repeated symbol and style.
//
// Empty or zero-width symbols still walk every cell so style is applied and
// progress is made. Wide symbols are written into every cell (matching
// upstream Fill); each cell independently receives the full symbol.
type Fill struct {
	Symbol string
	Style  style.Style
}

// NewFill creates a Fill that paints symbol into every cell.
func NewFill(symbol string) Fill {
	return Fill{Symbol: symbol}
}

// WithSymbol replaces the painted symbol.
func (f Fill) WithSymbol(symbol string) Fill {
	f.Symbol = symbol
	return f
}

// WithStyle sets the style used to paint each cell.
func (f Fill) WithStyle(sty style.Style) Fill {
	f.Style = sty
	return f
}

// Render fills area ∩ buf.Area with Symbol and Style.
// Empty areas are a no-op. Empty/zero-width symbols still style every cell.
func (f Fill) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}

	sym := f.Symbol

	for y := area.Y; y < area.Bottom(); y++ {
		for x := area.X; x < area.Right(); x++ {
			cell := buf.GetMut(x, y)
			if cell == nil {
				continue
			}
			cell.SetSymbol(sym)
			cell.SetStyle(f.Style)
		}
	}
}

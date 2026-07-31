package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/symbols"
)

func TestMapLowHighProjectionIntegrity(t *testing.T) {
	area := layout.NewRect(0, 0, 80, 40)

	// Low resolution map with Dot marker
	bufLow := buffer.Empty(area)
	cvLow := NewCanvas().
		Marker(symbols.Marker{Kind: symbols.MarkerDot}).
		XBounds([2]float64{-180, 180}).
		YBounds([2]float64{-90, 90}).
		Paint(func(ctx *Context) {
			ctx.Draw(NewMap(MapLow, style.Reset))
		})
	cvLow.Render(area, bufLow)

	// Count painted cells in low res map
	lowCount := 0
	for y := range area.Height {
		for x := range area.Width {
			if c, ok := bufLow.Get(x, y); ok && c.DisplaySymbol() != " " {
				lowCount++
			}
		}
	}
	if lowCount == 0 {
		t.Errorf("MapLow rendered 0 painted cells")
	}

	// High resolution map with Braille marker
	bufHigh := buffer.Empty(area)
	cvHigh := NewCanvas().
		Marker(symbols.Marker{Kind: symbols.MarkerBraille}).
		XBounds([2]float64{-180, 180}).
		YBounds([2]float64{-90, 90}).
		Paint(func(ctx *Context) {
			ctx.Draw(NewMap(MapHigh, style.Reset))
		})
	cvHigh.Render(area, bufHigh)

	highCount := 0
	for y := range area.Height {
		for x := range area.Width {
			if c, ok := bufHigh.Get(x, y); ok && c.DisplaySymbol() != " " {
				highCount++
			}
		}
	}
	if highCount == 0 {
		t.Errorf("MapHigh rendered 0 painted cells")
	}

	// High resolution map must produce more braille points / detail than low resolution map
	if highCount <= lowCount {
		t.Errorf("high resolution map cell count (%d) should be > low resolution (%d)", highCount, lowCount)
	}
}

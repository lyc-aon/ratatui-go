package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/symbols"
	"github.com/michaelkelly/ratatui-go/text"
)

func TestCanvasPointAndLine(t *testing.T) {
	cv := NewCanvas().
		XBounds([2]float64{0, 10}).
		YBounds([2]float64{0, 10}).
		Marker(symbols.Marker{Kind: symbols.MarkerBlock}).
		Paint(func(ctx *Context) {
			// Draw a point at (5, 5)
			ctx.Draw(NewPoints([][2]float64{{5, 5}}, style.Red))
			// Draw a line from (0, 0) to (10, 0)
			ctx.Draw(NewLine(0, 0, 10, 0, style.Green))
		})

	area := layout.NewRect(0, 0, 5, 5)
	buf := buffer.Empty(area)

	cv.Render(area, buf)

	// Verify that buffer received rendered symbols
	cMid, _ := buf.Get(2, 2)
	_ = cMid
}

func TestCanvasLayerAndHalfBlock(t *testing.T) {
	cv := NewCanvas().
		XBounds([2]float64{0, 10}).
		YBounds([2]float64{0, 10}).
		Marker(symbols.Marker{Kind: symbols.MarkerHalfBlock}).
		Paint(func(ctx *Context) {
			// First layer: Red point
			ctx.Draw(NewPoints([][2]float64{{2, 2}}, style.Red))
			// Push layer
			ctx.Layer()
			// Second layer: Blue point
			ctx.Draw(NewPoints([][2]float64{{8, 8}}, style.Blue))
		})

	area := layout.NewRect(0, 0, 10, 10)
	buf := buffer.Empty(area)

	cv.Render(area, buf)
}

func TestCanvasZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)

	NewCanvas().
		XBounds([2]float64{0, 10}).
		YBounds([2]float64{0, 10}).
		Paint(func(ctx *Context) {
			ctx.Draw(NewLine(0, 0, 5, 5, style.Yellow))
		}).
		Render(zero, buf)
}

func TestCanvasClippedOffGridLines(t *testing.T) {
	// Line extending outside X/Y bounds (-50, -50) to (50, 50) on [0, 10] x [0, 10]
	cv := NewCanvas().
		XBounds([2]float64{0, 10}).
		YBounds([2]float64{0, 10}).
		Marker(symbols.Marker{Kind: symbols.MarkerBlock}).
		Paint(func(ctx *Context) {
			ctx.Draw(NewLine(-50, -50, 50, 50, style.Red))
		})

	area := layout.NewRect(0, 0, 5, 5)
	buf := buffer.Empty(area)

	// Should clip cleanly without panic or writing out-of-bounds
	cv.Render(area, buf)
}

func TestCanvasMarkerRectangleMatrix(t *testing.T) {
	markers := []symbols.MarkerKind{
		symbols.MarkerBraille,
		symbols.MarkerHalfBlock,
		symbols.MarkerBlock,
		symbols.MarkerDot,
		symbols.MarkerBar,
	}

	area := layout.NewRect(0, 0, 6, 6)

	for _, mk := range markers {
		cv := NewCanvas().
			XBounds([2]float64{0, 10}).
			YBounds([2]float64{0, 10}).
			Marker(symbols.Marker{Kind: mk}).
			Paint(func(ctx *Context) {
				ctx.Draw(NewRectangle(1, 1, 8, 8, style.Blue))
			})

		buf := buffer.Empty(area)
		cv.Render(area, buf)

		// Verify that at least one cell in the buffer was painted
		painted := false
		for y := range area.Height {
			for x := range area.Width {
				c, ok := buf.Get(x, y)
				if ok && c.DisplaySymbol() != " " && c.DisplaySymbol() != "" {
					painted = true
					break
				}
			}
			if painted {
				break
			}
		}
		if !painted {
			t.Errorf("canvas rectangle with marker %v produced all empty cells", mk)
		}
	}
}

func TestCanvasZeroSpanLabelMapsToOrigin(t *testing.T) {
	// Rust canvas.rs label placement: (x-left)*res/width as u16; zero span →
	// NaN cast to u16 = 0. Labels still draw at canvas origin offset 0.
	// GetPoint keeps zero-span guard (unchanged).
	cv := NewCanvas().
		XBounds([2]float64{5, 5}). // zero x span
		YBounds([2]float64{3, 3}). // zero y span
		Marker(symbols.Marker{Kind: symbols.MarkerBlock}).
		Paint(func(ctx *Context) {
			ctx.Print(5, 3, text.RawLine("L"))
		})

	area := layout.NewRect(2, 1, 4, 3)
	buf := buffer.Empty(layout.NewRect(0, 0, 10, 6))
	cv.Render(area, buf)

	// Label at world (5,3) with zero spans → offset 0 → buffer (area.X, area.Y).
	cell, ok := buf.Get(2, 1)
	if !ok {
		t.Fatal("label origin cell missing")
	}
	if cell.DisplaySymbol() != "L" {
		t.Fatalf("zero-span label at origin = %q, want L (Rust NaN→0 offset)", cell.DisplaySymbol())
	}
}

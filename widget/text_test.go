package widget

import (
	"testing"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

func TestSpanWidgetPreservesLeadingZeroWidthGrapheme(t *testing.T) {
	buf := buffer.Empty(layout.NewRect(0, 0, 3, 1))
	Span(text.RawSpan("\u200ba")).Render(buf.Area, buf)
	cell, _ := buf.Get(0, 0)
	if cell.Symbol != "\u200ba" {
		t.Fatalf("first cell symbol = %q, want leading ZWSP plus a", cell.Symbol)
	}
}

func TestLineWidgetStylesWholeRowAndAligns(t *testing.T) {
	buf := buffer.Empty(layout.NewRect(0, 0, 5, 1))
	line := text.RawLine("x").Centered().WithStyle(style.New().WithBG(style.Blue))
	Line(line).Render(buf.Area, buf)
	for x := 0; x < 5; x++ {
		cell, _ := buf.Get(x, 0)
		bg, ok := cell.Style.Background()
		if !ok || bg != style.Blue {
			t.Fatalf("cell %d background = %v, %v; want blue", x, bg, ok)
		}
	}
	cell, _ := buf.Get(2, 0)
	if cell.DisplaySymbol() != "x" {
		t.Fatalf("center cell = %q, want x", cell.DisplaySymbol())
	}
}

func TestTextWidgetInheritsTextAlignmentAndPatchesLineStyle(t *testing.T) {
	buf := buffer.Empty(layout.NewRect(0, 0, 4, 2))
	value := text.FromLines(
		text.RawLine("a").WithStyle(style.New().WithBG(style.Red)),
		text.RawLine("b"),
	).Centered().WithStyle(style.New().WithFG(style.Green))
	Text(value).Render(buf.Area, buf)

	for y, want := range []string{"a", "b"} {
		cell, _ := buf.Get(1, y)
		if cell.DisplaySymbol() != want {
			t.Fatalf("row %d aligned symbol = %q, want %q", y, cell.DisplaySymbol(), want)
		}
		fg, ok := cell.Style.Foreground()
		if !ok || fg != style.Green {
			t.Fatalf("row %d foreground = %v, %v; want green", y, fg, ok)
		}
	}
	cell, _ := buf.Get(3, 0)
	bg, ok := cell.Style.Background()
	if !ok || bg != style.Red {
		t.Fatalf("line background = %v, %v; want red across row", bg, ok)
	}
}

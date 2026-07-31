package widget

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

// StringWidget renders one unstyled string on the area's first row.
type StringWidget string

// String adapts a string to Widget without allocating a closure.
func String(value string) StringWidget { return StringWidget(value) }

// Render implements Widget.
func (w StringWidget) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStringN(area.X, area.Y, string(w), area.Width, style.Style{})
}

// SpanWidget adapts text.Span to Widget.
type SpanWidget struct{ Value text.Span }

// Span adapts a span to Widget.
func Span(value text.Span) SpanWidget { return SpanWidget{Value: value} }

// Render implements Widget.
func (w SpanWidget) Render(area layout.Rect, buf *buffer.Buffer) {
	renderSpan(w.Value, area, buf)
}

// LineWidget adapts text.Line to Widget.
type LineWidget struct{ Value text.Line }

// Line adapts a line to Widget.
func Line(value text.Line) LineWidget { return LineWidget{Value: value} }

// Render implements Widget.
func (w LineWidget) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	area.Height = 1
	if w.Value.Width() == 0 {
		return
	}
	buf.SetStyle(area, w.Value.Style)
	spans, _, leftPad := w.Value.RenderData(area.Width)
	renderSpans(spans, layout.Rect{
		X:      area.X + leftPad,
		Y:      area.Y,
		Width:  area.Width - leftPad,
		Height: 1,
	}, buf)
}

// TextWidget adapts text.Text to Widget.
type TextWidget struct{ Value text.Text }

// Text adapts styled text to Widget.
func Text(value text.Text) TextWidget { return TextWidget{Value: value} }

// Render implements Widget.
func (w TextWidget) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, w.Value.Style)
	limit := len(w.Value.Lines)
	if limit > area.Height {
		limit = area.Height
	}
	for i := 0; i < limit; i++ {
		line := w.Value.Lines[i]
		row := layout.Rect{X: area.X, Y: area.Y + i, Width: area.Width, Height: 1}
		if line.Width() == 0 {
			continue
		}
		buf.SetStyle(row, line.Style)
		spans, _, leftPad, _, ok := w.Value.LineRenderData(i, area.Width)
		if !ok {
			continue
		}
		renderSpans(spans, layout.Rect{
			X:      row.X + leftPad,
			Y:      row.Y,
			Width:  row.Width - leftPad,
			Height: 1,
		}, buf)
	}
}

func renderSpans(spans []text.Span, area layout.Rect, buf *buffer.Buffer) {
	for i := range spans {
		if area.IsEmpty() {
			return
		}
		used := renderSpan(spans[i], area, buf)
		if used < 0 {
			used = 0
		}
		if used > area.Width {
			used = area.Width
		}
		area.X += used
		area.Width -= used
	}
}

func renderSpan(span text.Span, area layout.Rect, buf *buffer.Buffer) int {
	if buf == nil {
		return 0
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return 0
	}
	x := area.X
	for i, grapheme := range span.StyledGraphemes(style.Style{}) {
		width := grapheme.Width()
		nextX := x + width
		if nextX > area.Right() {
			break
		}

		switch {
		case i == 0:
			buf.GetMut(x, area.Y).SetSymbol(grapheme.Symbol).SetStyle(grapheme.Style)
		case x == area.X:
			cell := buf.GetMut(x, area.Y)
			cell.SetSymbol(cell.Symbol + grapheme.Symbol).SetStyle(grapheme.Style)
		case width == 0:
			cell := buf.GetMut(x-1, area.Y)
			cell.SetSymbol(cell.Symbol + grapheme.Symbol).SetStyle(grapheme.Style)
		default:
			buf.GetMut(x, area.Y).SetSymbol(grapheme.Symbol).SetStyle(grapheme.Style)
		}

		for hiddenX := x + 1; hiddenX < nextX; hiddenX++ {
			buf.GetMut(hiddenX, area.Y).Reset()
		}
		x = nextX
	}
	return x - area.X
}

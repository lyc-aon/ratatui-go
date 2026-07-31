// Package widget defines the core drawing contracts for ratatui-go.
//
// Widgets write cells into a buffer.Buffer for a given layout.Rect. Core packages
// never import the higher-level widgets package; concrete widgets live there.
package widget

import (
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
)

// Widget is a type that can draw itself into a buffer within a rectangular area.
//
// Implementations must tolerate zero-sized or clipped areas without panicking.
// Rendering writes only into cells that fall inside both area and buf.Area.
type Widget interface {
	Render(area layout.Rect, buf *buffer.Buffer)
}

// WidgetFunc adapts a plain function into a Widget.
type WidgetFunc func(area layout.Rect, buf *buffer.Buffer)

// Render calls f(area, buf). A nil func is a no-op.
func (f WidgetFunc) Render(area layout.Rect, buf *buffer.Buffer) {
	if f == nil {
		return
	}
	f(area, buf)
}

// StatefulWidget is a widget that renders with associated mutable state.
//
// State is owned by the caller and passed on each render so the widget can
// remember things (selection, scroll offset, etc.) across frames.
type StatefulWidget[S any] interface {
	RenderStateful(area layout.Rect, buf *buffer.Buffer, state *S)
}

// StatefulWidgetFunc adapts a plain function into a StatefulWidget.
type StatefulWidgetFunc[S any] func(area layout.Rect, buf *buffer.Buffer, state *S)

// RenderStateful calls f(area, buf, state). A nil func is a no-op.
func (f StatefulWidgetFunc[S]) RenderStateful(area layout.Rect, buf *buffer.Buffer, state *S) {
	if f == nil {
		return
	}
	f(area, buf, state)
}

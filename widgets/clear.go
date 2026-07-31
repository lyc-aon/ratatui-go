package widgets

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
)

// Clear resets every cell in its area so later widgets can overdraw cleanly
// (popups, overlays). It does not clear the terminal on first paint — use
// terminal.Clear for that.
type Clear struct{}

// NewClear returns a Clear widget.
func NewClear() Clear {
	return Clear{}
}

// Render resets every cell in area ∩ buf.Area to the empty default cell.
// Zero-sized or fully clipped areas are a no-op.
func (Clear) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	for y := area.Y; y < area.Bottom(); y++ {
		for x := area.X; x < area.Right(); x++ {
			if cell := buf.GetMut(x, y); cell != nil {
				cell.Reset()
			}
		}
	}
}

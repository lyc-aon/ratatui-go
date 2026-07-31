package widgets

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/symbols"
)

// InnerIfSome renders block into area when non-nil and returns its inner rect.
//
// When block is nil the area is returned unchanged and nothing is drawn.
// Peer widget lanes call this after applying the widget base style:
//
//	buf.SetStyle(area, w.Style)
//	inner := widgets.InnerIfSome(w.Block, area, buf)
//	// draw content in inner
func InnerIfSome(block *Block, area layout.Rect, buf *buffer.Buffer) layout.Rect {
	if block == nil {
		return area
	}
	block.Render(area, buf)
	return block.Inner(area)
}

// mergeCellSymbol merges next into cell using strategy.
func mergeCellSymbol(cell *buffer.Cell, next string, strategy symbols.MergeStrategy) {
	if cell != nil {
		cell.MergeSymbol(next, strategy)
	}
}

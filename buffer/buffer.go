package buffer

import (
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/text"
)

// Buffer is a rectangular grid of cells representing desired terminal content.
//
// Area may have a non-zero origin; coordinates passed to Get/Set* are global
// (origin-relative to the screen), not buffer-local indices.
type Buffer struct {
	Area    layout.Rect
	Content []Cell
}

// Empty returns a buffer filled with empty cells covering area.
func Empty(area layout.Rect) *Buffer {
	return Filled(area, NewCell())
}

// Filled returns a buffer with every cell set to cell.
func Filled(area layout.Rect, cell Cell) *Buffer {
	n := area.Area()
	content := make([]Cell, n)
	for i := range content {
		content[i] = cell
	}
	return &Buffer{Area: area, Content: content}
}

// WithLines returns a buffer containing lines at origin (0, 0). Its width is
// the widest line and its height is len(lines).
func WithLines(lines ...text.Line) *Buffer {
	width := 0
	for i := range lines {
		if lineWidth := lines[i].Width(); lineWidth > width {
			width = lineWidth
		}
	}
	b := Empty(layout.NewRect(0, 0, width, len(lines)))
	for y := range lines {
		b.SetLine(0, y, lines[y], width)
	}
	return b
}

// WithStrings is the string convenience form of WithLines.
func WithStrings(lines ...string) *Buffer {
	converted := make([]text.Line, len(lines))
	for i := range lines {
		converted[i] = text.RawLine(lines[i])
	}
	return WithLines(converted...)
}

// Get returns a copy of the cell at global (x, y).
// ok is false when the position lies outside the buffer area.
// Does not allocate.
func (b *Buffer) Get(x, y int) (Cell, bool) {
	if b == nil {
		return Cell{}, false
	}
	i, ok := b.indexOf(x, y)
	if !ok {
		return Cell{}, false
	}
	return b.Content[i], true
}

// GetMut returns a pointer to the cell at global (x, y), or nil if outside.
// The pointer aliases b.Content; it is invalid after Resize/Merge that reallocate.
// Does not allocate.
func (b *Buffer) GetMut(x, y int) *Cell {
	if b == nil {
		return nil
	}
	i, ok := b.indexOf(x, y)
	if !ok {
		return nil
	}
	return &b.Content[i]
}

// indexOf maps global coordinates to a content index.
func (b *Buffer) indexOf(x, y int) (int, bool) {
	if !b.Area.Contains(layout.Position{X: x, Y: y}) {
		return 0, false
	}
	return (y-b.Area.Y)*b.Area.Width + (x - b.Area.X), true
}

// posOf maps a content index to global coordinates.
func (b *Buffer) posOf(index int) (x, y int) {
	w := b.Area.Width
	if w <= 0 {
		return b.Area.X, b.Area.Y
	}
	return index%w + b.Area.X, index/w + b.Area.Y
}

// SetString prints s at (x, y) with style, clipped to the end of the line.
func (b *Buffer) SetString(x, y int, s string, st style.Style) {
	const maxInt = int(^uint(0) >> 1)
	b.SetStringN(x, y, s, maxInt, st)
}

// SetStringN prints at most maxWidth cells of s at (x, y) with style.
//
// Skips grapheme clusters that contain control characters and zero-width
// clusters. Wide graphemes reserve trailing cells by resetting them.
// Safe when the start position is outside the buffer: writes only in-area cells.
// Returns the coordinates immediately after the last written cell column.
func (b *Buffer) SetStringN(x, y int, s string, maxWidth int, st style.Style) (int, int) {
	if b == nil || b.Area.IsEmpty() {
		return x, y
	}
	if maxWidth < 0 {
		maxWidth = 0
	}
	// Entire row outside vertically — nothing to write.
	if y < b.Area.Y || y >= b.Area.Bottom() {
		return x, y
	}

	right := b.Area.Right()
	remaining := right - x
	if remaining < 0 {
		remaining = 0
	}
	if remaining > maxWidth {
		remaining = maxWidth
	}

	rest := s
	for remaining > 0 {
		cluster, next, ok := firstGrapheme(rest)
		if !ok {
			break
		}
		rest = next
		if containsControl(cluster) {
			continue
		}
		width := StringWidth(cluster)
		if width <= 0 {
			continue
		}
		if width > remaining {
			break
		}

		if cell := b.GetMut(x, y); cell != nil {
			cell.SetSymbol(cluster)
			cell.SetStyle(st)
		}
		nextX := x + width
		x++
		// Reset following cells covered by a multi-width grapheme.
		for x < nextX {
			if cell := b.GetMut(x, y); cell != nil {
				cell.Reset()
			}
			x++
		}
		remaining -= width
	}
	return x, y
}

// SetSpan prints a span at (x, y) with at most maxWidth cells.
func (b *Buffer) SetSpan(x, y int, span text.Span, maxWidth int) (int, int) {
	return b.SetStringN(x, y, span.Content, maxWidth, span.Style)
}

// SetLine prints a line's spans at (x, y) with at most maxWidth cells.
// Each span is styled as line.Style.Patch(span.Style).
func (b *Buffer) SetLine(x, y int, line text.Line, maxWidth int) (int, int) {
	if maxWidth < 0 {
		maxWidth = 0
	}
	remaining := maxWidth
	for i := range line.Spans {
		if remaining == 0 {
			break
		}
		span := line.Spans[i]
		st := line.Style.Patch(span.Style)
		nx, _ := b.SetStringN(x, y, span.Content, remaining, st)
		w := nx - x
		if w < 0 {
			w = 0
		}
		x = nx
		remaining -= w
		if remaining < 0 {
			remaining = 0
		}
	}
	return x, y
}

// SetStyle applies style to every cell in area ∩ buffer.Area.
// Safe when area is empty or outside the buffer.
func (b *Buffer) SetStyle(area layout.Rect, st style.Style) {
	if b == nil || b.Area.IsEmpty() {
		return
	}
	area = b.Area.Intersection(area)
	if area.IsEmpty() {
		return
	}
	for y := area.Y; y < area.Bottom(); y++ {
		for x := area.X; x < area.Right(); x++ {
			if cell := b.GetMut(x, y); cell != nil {
				cell.SetStyle(st)
			}
		}
	}
}

// Reset clears every cell to the empty state without changing Area.
func (b *Buffer) Reset() {
	if b == nil {
		return
	}
	for i := range b.Content {
		b.Content[i].Reset()
	}
}

// Resize changes the buffer area. Content is truncated or extended with empty
// cells. Existing cell values are kept in index order (not spatially remapped).
func (b *Buffer) Resize(area layout.Rect) {
	if b == nil {
		return
	}
	n := area.Area()
	if len(b.Content) > n {
		b.Content = b.Content[:n]
	} else {
		empty := NewCell()
		for len(b.Content) < n {
			b.Content = append(b.Content, empty)
		}
	}
	b.Area = area
}

// Merge unions other into b. Overlapping cells take other's value.
// Handles non-zero origins by remapping content into the union area.
func (b *Buffer) Merge(other *Buffer) {
	if b == nil || other == nil {
		return
	}
	area := b.Area.Union(other.Area)
	n := area.Area()

	// Grow to union size with empty fill (matches Rust resize + EMPTY).
	empty := NewCell()
	if len(b.Content) < n {
		for len(b.Content) < n {
			b.Content = append(b.Content, empty)
		}
	} else if len(b.Content) > n {
		b.Content = b.Content[:n]
	}

	// Move original content into the union layout (reverse walk, like Rust).
	oldArea := b.Area
	oldSize := oldArea.Area()
	if oldSize > len(b.Content) {
		oldSize = len(b.Content)
	}
	oldW := oldArea.Width
	if oldW > 0 && oldArea != area {
		for i := oldSize - 1; i >= 0; i-- {
			x := i%oldW + oldArea.X
			y := i/oldW + oldArea.Y
			k := (y-area.Y)*area.Width + (x - area.X)
			if k < 0 || k >= n {
				continue
			}
			if i != k {
				b.Content[k] = b.Content[i]
				b.Content[i] = empty
			}
		}
	}

	// Overlay other.
	ow := other.Area.Width
	if ow > 0 {
		limit := other.Area.Area()
		if limit > len(other.Content) {
			limit = len(other.Content)
		}
		for i := range limit {
			x := i%ow + other.Area.X
			y := i/ow + other.Area.Y
			k := (y-area.Y)*area.Width + (x - area.X)
			if k >= 0 && k < n {
				b.Content[k] = other.Content[i]
			}
		}
	}
	b.Area = area
}

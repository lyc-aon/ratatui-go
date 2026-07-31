package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/symbols"
)

func TestScrollbarVerticalGeometry(t *testing.T) {
	sb := NewScrollbar(ScrollbarVerticalRight)

	// Height 5 (top arrow + 3 track cells + bot arrow)
	area := layout.NewRect(0, 0, 1, 5)
	buf := buffer.Empty(area)

	// Content length 10, viewport 2, position 0 -> thumb at top
	state := NewScrollbarState(10).ViewportContentLength(2).Position(0)

	sb.RenderStateful(area, buf, &state)

	// Arrow heads ▲ and ▼, double line set
	// Cell 0: ▲ (begin)
	// Cell 1: █ (thumb)
	// Cell 2: ║ (track)
	// Cell 3: ║ (track)
	// Cell 4: ▼ (end)
	c0, _ := buf.Get(0, 0)
	c1, _ := buf.Get(0, 1)
	c4, _ := buf.Get(0, 4)

	if c0.DisplaySymbol() != symbols.ScrollbarDoubleVertical.Begin {
		t.Errorf("cell (0,0) = %q, want %q", c0.DisplaySymbol(), symbols.ScrollbarDoubleVertical.Begin)
	}
	if c1.DisplaySymbol() != symbols.ScrollbarDoubleVertical.Thumb {
		t.Errorf("cell (0,1) thumb = %q, want %q", c1.DisplaySymbol(), symbols.ScrollbarDoubleVertical.Thumb)
	}
	if c4.DisplaySymbol() != symbols.ScrollbarDoubleVertical.End {
		t.Errorf("cell (0,4) = %q, want %q", c4.DisplaySymbol(), symbols.ScrollbarDoubleVertical.End)
	}

	// Move position to max (9) -> thumb moves to bottom track position (cell 3)
	bufEnd := buffer.Empty(area)
	stateEnd := state.Position(9)
	sb.RenderStateful(area, bufEnd, &stateEnd)

	c3End, _ := bufEnd.Get(0, 3)
	if c3End.DisplaySymbol() != symbols.ScrollbarDoubleVertical.Thumb {
		t.Errorf("cell (0,3) thumb at end = %q, want %q", c3End.DisplaySymbol(), symbols.ScrollbarDoubleVertical.Thumb)
	}
}

func TestScrollbarHorizontalGeometry(t *testing.T) {
	sb := NewScrollbar(ScrollbarHorizontalBottom)

	// Width 5, Height 1
	area := layout.NewRect(0, 0, 5, 1)
	buf := buffer.Empty(area)

	state := NewScrollbarState(10).Position(0)
	sb.RenderStateful(area, buf, &state)

	c0, _ := buf.Get(0, 0)
	c4, _ := buf.Get(4, 0)

	if c0.DisplaySymbol() != symbols.ScrollbarDoubleHorizontal.Begin {
		t.Errorf("c0 = %q, want %q", c0.DisplaySymbol(), symbols.ScrollbarDoubleHorizontal.Begin)
	}
	if c4.DisplaySymbol() != symbols.ScrollbarDoubleHorizontal.End {
		t.Errorf("c4 = %q, want %q", c4.DisplaySymbol(), symbols.ScrollbarDoubleHorizontal.End)
	}
}

func TestScrollbarZeroContentSmoke(t *testing.T) {
	area := layout.NewRect(0, 0, 1, 5)
	buf := buffer.Empty(area)
	sb := NewScrollbar(ScrollbarVerticalRight)

	// Zero content length or zero area should not panic or draw
	stateZero := NewScrollbarState(0)
	sb.RenderStateful(area, buf, &stateZero)

	zeroArea := layout.NewRect(0, 0, 0, 0)
	sb.RenderStateful(zeroArea, buf, &stateZero)
}

func TestScrollbarOutOfRangeClamp(t *testing.T) {
	sb := NewScrollbar(ScrollbarVerticalRight)
	area := layout.NewRect(0, 0, 1, 5)
	buf := buffer.Empty(area)

	// Position 100 on content length 10 -> should clamp position to max (9)
	stateExcess := NewScrollbarState(10).Position(100)
	sb.RenderStateful(area, buf, &stateExcess)

	// Thumb should be at cell (0, 3)
	c3, _ := buf.Get(0, 3)
	if c3.DisplaySymbol() != symbols.ScrollbarDoubleVertical.Thumb {
		t.Errorf("out-of-range clamp cell (0,3) thumb = %q, want %q", c3.DisplaySymbol(), symbols.ScrollbarDoubleVertical.Thumb)
	}

	// Negative position -> should clamp to 0
	bufNeg := buffer.Empty(area)
	stateNeg := NewScrollbarState(10).Position(-5)
	sb.RenderStateful(area, bufNeg, &stateNeg)
	c1, _ := bufNeg.Get(0, 1)
	if c1.DisplaySymbol() != symbols.ScrollbarDoubleVertical.Thumb {
		t.Errorf("negative clamp cell (0,1) thumb = %q, want %q", c1.DisplaySymbol(), symbols.ScrollbarDoubleVertical.Thumb)
	}
}

func TestScrollbarNilTrackLeavesCellsUntouched(t *testing.T) {
	sb := NewScrollbar(ScrollbarVerticalRight).TrackSymbol(nil)
	area := layout.NewRect(0, 0, 1, 5)
	buf := buffer.Empty(area)

	// Pre-fill buffer with 'X'
	for y := range 5 {
		if cell := buf.GetMut(0, y); cell != nil {
			cell.SetSymbol("X")
		}
	}

	state := NewScrollbarState(10).Position(0)
	sb.RenderStateful(area, buf, &state)

	// Cell 0 is Begin arrow, Cell 1 is Thumb, Cell 4 is End arrow
	// Track cells (2 and 3) must remain 'X' because TrackSymbol is nil!
	c2, _ := buf.Get(0, 2)
	c3, _ := buf.Get(0, 3)

	if c2.DisplaySymbol() != "X" {
		t.Errorf("nil track cell (0,2) = %q, want %q", c2.DisplaySymbol(), "X")
	}
	if c3.DisplaySymbol() != "X" {
		t.Errorf("nil track cell (0,3) = %q, want %q", c3.DisplaySymbol(), "X")
	}
}

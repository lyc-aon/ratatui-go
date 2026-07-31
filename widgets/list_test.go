package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/text"
)

func TestListSelectionAndOffset(t *testing.T) {
	items := []ListItem{
		NewListItem(text.RawText("Item 0")),
		NewListItem(text.RawText("Item 1")),
		NewListItem(text.RawText("Item 2")),
		NewListItem(text.RawText("Item 3")),
		NewListItem(text.RawText("Item 4")),
	}

	lst := NewList(items...).HighlightSymbol(text.RawLine("> "))

	// Area height is 3 -> visible items window of 3 items
	area := layout.NewRect(0, 0, 10, 3)
	buf := buffer.Empty(area)

	state := NewListState()
	sel := 4
	state.Select(&sel) // Select Item 4, offset 0 initial

	lst.RenderStateful(area, buf, &state)

	// State offset must be repaired so Item 4 is visible (offset should become 2: items 2, 3, 4)
	if state.Offset() != 2 {
		t.Errorf("state.Offset() after stateful render = %d, want 2", state.Offset())
	}

	// Bottom line (row 2) should contain "> Item 4"
	r2 := getBufferRowString(buf, 2)
	wantR2 := "> Item 4  "
	if r2 != wantR2 {
		t.Errorf("row 2 = %q, want %q", r2, wantR2)
	}
}

func TestListScrollPadding(t *testing.T) {
	items := make([]ListItem, 10)
	for i := range 10 {
		items[i] = NewListItem(text.RawText("Item"))
	}

	// Height 5, ScrollPadding 1
	lst := NewList(items...).ScrollPadding(1)

	area := layout.NewRect(0, 0, 10, 5)
	buf := buffer.Empty(area)

	state := NewListState()
	sel := 4
	state.Select(&sel)

	lst.RenderStateful(area, buf, &state)

	// With scroll padding 1, selected item 4 at bottom of initial window shifts window so item 5 is not cut off if possible
	if state.Offset() < 1 {
		t.Errorf("expected offset to adjust for scroll padding, got offset %d", state.Offset())
	}
}

func TestListReverseDirection(t *testing.T) {
	items := []ListItem{
		NewListItem(text.RawText("Alpha")),
		NewListItem(text.RawText("Beta")),
	}

	lst := NewList(items...).Direction(ListBottomToTop)

	area := layout.NewRect(0, 0, 10, 3)
	buf := buffer.Empty(area)

	lst.Render(area, buf)

	// In ListBottomToTop:
	// Alpha is rendered at bottom (row 2), Beta is rendered above it (row 1)
	r1 := getBufferRowString(buf, 1)
	r2 := getBufferRowString(buf, 2)

	if r2 != "Alpha     " {
		t.Errorf("row 2 (bottom) = %q, want %q", r2, "Alpha     ")
	}
	if r1 != "Beta      " {
		t.Errorf("row 1 = %q, want %q", r1, "Beta      ")
	}
}

func TestListZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)

	lst := NewList(NewListItem(text.RawText("A"))).
		Direction(ListBottomToTop).
		ScrollPadding(2)

	state := NewListState()
	sel := 0
	state.Select(&sel)

	lst.RenderStateful(zero, buf, &state)
	lst.Render(zero, buf)
}

func TestListBottomToTopMultiline(t *testing.T) {
	item := NewListItem(text.FromLines(
		text.RawLine("Line1"),
		text.RawLine("Line2"),
	))

	lst := NewList(item).Direction(ListBottomToTop)
	area := layout.NewRect(0, 0, 10, 3)
	buf := buffer.Empty(area)
	lst.Render(area, buf)

	// In BottomToTop, item rendered at bottom (rows 1 and 2)
	r1 := getBufferRowString(buf, 1)
	r2 := getBufferRowString(buf, 2)
	if r1 != "Line1     " {
		t.Errorf("row 1 = %q, want %q", r1, "Line1     ")
	}
	if r2 != "Line2     " {
		t.Errorf("row 2 = %q, want %q", r2, "Line2     ")
	}
}

func TestListRepeatedHighlight(t *testing.T) {
	item := NewListItem(text.FromLines(
		text.RawLine("Alpha"),
		text.RawLine("Beta"),
	))
	symbol := text.RawLine("> ")

	area := layout.NewRect(0, 0, 10, 2)

	// RepeatHighlightSymbol(false) -> default: symbol on line 0 only, line 1 gets spaces
	lstNoRepeat := NewList(item).
		HighlightSymbol(symbol).
		RepeatHighlightSymbol(false)
	bufNoRepeat := buffer.Empty(area)
	state0 := NewListState()
	sel := 0
	state0.Select(&sel)
	lstNoRepeat.RenderStateful(area, bufNoRepeat, &state0)

	r0NoRep := getBufferRowString(bufNoRepeat, 0)
	r1NoRep := getBufferRowString(bufNoRepeat, 1)
	if r0NoRep != "> Alpha   " {
		t.Errorf("no-repeat row 0 = %q, want %q", r0NoRep, "> Alpha   ")
	}
	if r1NoRep != "  Beta    " {
		t.Errorf("no-repeat row 1 = %q, want %q", r1NoRep, "  Beta    ")
	}

	// RepeatHighlightSymbol(true) -> symbol repeated on both lines
	lstRepeat := NewList(item).
		HighlightSymbol(symbol).
		RepeatHighlightSymbol(true)
	bufRepeat := buffer.Empty(area)
	state1 := NewListState()
	state1.Select(&sel)
	lstRepeat.RenderStateful(area, bufRepeat, &state1)

	r0Rep := getBufferRowString(bufRepeat, 0)
	r1Rep := getBufferRowString(bufRepeat, 1)
	if r0Rep != "> Alpha   " {
		t.Errorf("repeat row 0 = %q, want %q", r0Rep, "> Alpha   ")
	}
	if r1Rep != "> Beta    " {
		t.Errorf("repeat row 1 = %q, want %q", r1Rep, "> Beta    ")
	}
}

func TestListScrollPaddingClamp(t *testing.T) {
	items := make([]ListItem, 5)
	for i := range 5 {
		items[i] = NewListItem(text.RawText("Item"))
	}

	// ScrollPadding larger than list length / area height
	lst := NewList(items...).ScrollPadding(10)
	area := layout.NewRect(0, 0, 10, 3)
	buf := buffer.Empty(area)

	state := NewListState()
	sel := 0
	state.Select(&sel)

	// Should not panic or produce invalid offset
	lst.RenderStateful(area, buf, &state)
	if state.Offset() < 0 || state.Offset() >= len(items) {
		t.Errorf("invalid offset %d after scroll padding clamp", state.Offset())
	}
}

func TestListZeroWidthLineKeepsTextStyle(t *testing.T) {
	// Rust Line::render returns before set_style when width==0.
	// Text-level style must remain on the empty line row.
	// Pinned: Text style Blue bg + empty Line with Red line style → row stays Blue.
	emptyRed := text.RawLine("").WithStyle(style.New().WithBG(style.Red))
	content := text.FromLines(emptyRed).WithStyle(style.New().WithBG(style.Blue))
	item := NewListItem(content)
	lst := NewList(item)

	area := layout.NewRect(0, 0, 4, 1)
	buf := buffer.Empty(area)
	lst.Render(area, buf)

	cell, ok := buf.Get(0, 0)
	if !ok {
		t.Fatal("cell missing")
	}
	bg, set := cell.Style.Background()
	if !set || bg != style.Blue {
		t.Fatalf("zero-width line row bg = (%v, %v), want Blue Text style (not Red line style)", bg, set)
	}
}

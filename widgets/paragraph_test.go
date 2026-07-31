package widgets

import (
	"testing"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

func TestParagraphUnicodeWrapping(t *testing.T) {
	// Wrap line containing Unicode graphemes
	tText := text.RawText("Hello 世界 World")
	p := NewParagraph(tText).WithWrap(Wrap{Trim: true})

	area := layout.NewRect(0, 0, 8, 3)
	buf := buffer.Empty(area)
	p.Render(area, buf)

	// "Hello 世界 World" in width 8:
	// "Hello" (5) + " " + "世" (2) = 8
	// Row 0: "Hello 世界" or wrapped depending on word bounds.
	r0 := getBufferRowString(buf, 0)
	r1 := getBufferRowString(buf, 1)

	if r0 == "" {
		t.Errorf("row 0 is empty")
	}
	t.Logf("wrapped row 0: %q, row 1: %q", r0, r1)
}

func TestParagraphAlignment(t *testing.T) {
	txt := text.RawText("Hi")
	area := layout.NewRect(0, 0, 6, 3)

	// Left
	bufL := buffer.Empty(area)
	NewParagraph(txt).LeftAligned().Render(area, bufL)
	if getBufferRowString(bufL, 0) != "Hi    " {
		t.Errorf("left align row 0 = %q, want %q", getBufferRowString(bufL, 0), "Hi    ")
	}

	// Center
	bufC := buffer.Empty(area)
	NewParagraph(txt).Centered().Render(area, bufC)
	// width 6, text width 2 -> offset (6/2 - 2/2) = 2 -> "  Hi  "
	if getBufferRowString(bufC, 0) != "  Hi  " {
		t.Errorf("center align row 0 = %q, want %q", getBufferRowString(bufC, 0), "  Hi  ")
	}

	// Right
	bufR := buffer.Empty(area)
	NewParagraph(txt).RightAligned().Render(area, bufR)
	// width 6, text width 2 -> offset (6 - 2) = 4 -> "    Hi"
	if getBufferRowString(bufR, 0) != "    Hi" {
		t.Errorf("right align row 0 = %q, want %q", getBufferRowString(bufR, 0), "    Hi")
	}
}

func TestParagraphScroll(t *testing.T) {
	txt := text.FromLines(
		text.RawLine("Line 1"),
		text.RawLine("Line 2"),
		text.RawLine("Line 3"),
	)
	p := NewParagraph(txt).Scroll(1, 2) // vertical scroll 1 line, horizontal scroll 2 cols

	area := layout.NewRect(0, 0, 10, 2)
	buf := buffer.Empty(area)
	p.Render(area, buf)

	// ScrollY=1 skips "Line 1", so row 0 shows "Line 2"
	// ScrollX=2 skips "Li", so row 0 shows "ne 2"
	r0 := getBufferRowString(buf, 0)
	wantR0 := "ne 2      "
	if r0 != wantR0 {
		t.Errorf("scrolled row 0 = %q, want %q", r0, wantR0)
	}

	r1 := getBufferRowString(buf, 1)
	wantR1 := "ne 3      "
	if r1 != wantR1 {
		t.Errorf("scrolled row 1 = %q, want %q", r1, wantR1)
	}
}

func TestParagraphLoneWideGraphemeClipping(t *testing.T) {
	// A lone wide grapheme 😀 (width 2) in a width 1 box with wrap should be dropped or clipped without panic/overflow.
	txt := text.RawText("😀")
	p := NewParagraph(txt).WithWrap(Wrap{Trim: false})

	area := layout.NewRect(0, 0, 1, 1)
	buf := buffer.Empty(area)

	p.Render(area, buf)
	r0 := getBufferRowString(buf, 0)
	if r0 == "😀" {
		t.Errorf("wide grapheme overflowed width 1 box")
	}
}

func TestParagraphZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)

	p := NewParagraph(text.RawText("Hello")).
		Block(Bordered()).
		WithWrap(Wrap{Trim: true}).
		Scroll(5, 5)

	p.Render(zero, buf)

	count := p.LineCount(0)
	if count != 0 {
		t.Errorf("LineCount(0) = %d, want 0", count)
	}
}

func TestParagraphZeroWidthAtLineEndMatchesUpstream(t *testing.T) {
	// Ratatui 0.30.2 emits a blank second row for this zero-width wrap edge.
	txt := text.RawText("hello\u200B world")
	p := NewParagraph(txt).WithWrap(Wrap{Trim: true})

	area := layout.NewRect(0, 0, 5, 2)
	buf := buffer.Empty(area)
	p.Render(area, buf)

	r0 := getBufferRowString(buf, 0)
	r1 := getBufferRowString(buf, 1)

	if r0 != "hello" {
		t.Errorf("row 0 = %q, want %q", r0, "hello")
	}
	if r1 != "     " {
		t.Errorf("row 1 = %q, want upstream blank row", r1)
	}
}

func TestParagraphWhitespaceOnlyTrimmedWrap(t *testing.T) {
	// Whitespace-only line with Trim: true
	txt := text.FromLines(
		text.RawLine("     "),
		text.RawLine("  foo  "),
	)
	pTrim := NewParagraph(txt).WithWrap(Wrap{Trim: true})
	area := layout.NewRect(0, 0, 5, 2)
	bufTrim := buffer.Empty(area)
	pTrim.Render(area, bufTrim)

	r0 := getBufferRowString(bufTrim, 0)
	r1 := getBufferRowString(bufTrim, 1)

	if r0 != "     " {
		t.Errorf("trimmed row 0 = %q, want %q", r0, "     ")
	}
	if r1 != "foo  " {
		t.Errorf("trimmed row 1 = %q, want %q", r1, "foo  ")
	}
}

func TestParagraphForcedBreakLongWord(t *testing.T) {
	// Long word exceeding max line width must hard-break across lines
	txt := text.RawText("abcdefghijklmno")
	p := NewParagraph(txt).WithWrap(Wrap{Trim: true})

	area := layout.NewRect(0, 0, 5, 3)
	buf := buffer.Empty(area)
	p.Render(area, buf)

	r0 := getBufferRowString(buf, 0)
	r1 := getBufferRowString(buf, 1)
	r2 := getBufferRowString(buf, 2)

	if r0 != "abcde" {
		t.Errorf("row 0 = %q, want %q", r0, "abcde")
	}
	if r1 != "fghij" {
		t.Errorf("row 1 = %q, want %q", r1, "fghij")
	}
	if r2 != "klmno" {
		t.Errorf("row 2 = %q, want %q", r2, "klmno")
	}
}

func TestParagraphWideGraphemePreservesBaseStyle(t *testing.T) {
	// Rust paragraph.rs 0.30.2: wide graphemes only set_symbol/set_style on the
	// lead cell. Trailing covered cells keep the paragraph base style from the
	// earlier buf.set_style (no Reset).
	// Pinned: "あいう" on Green bg → every cell bg Green, symbols on 0/2/4.
	txt := text.RawText("あいう")
	p := NewParagraph(txt).Style(style.New().WithBG(style.Green))

	area := layout.NewRect(0, 0, 10, 1)
	buf := buffer.Empty(area)
	p.Render(area, buf)

	lead0, ok := buf.Get(0, 0)
	if !ok || lead0.DisplaySymbol() != "あ" {
		t.Fatalf("cell(0,0) symbol = %q, want あ", lead0.DisplaySymbol())
	}
	trail1, ok := buf.Get(1, 0)
	if !ok {
		t.Fatal("cell(1,0) missing")
	}
	// Old bug: trail.Reset() cleared Green bg on covered columns.
	bg, set := trail1.Style.Background()
	if !set || bg != style.Green {
		t.Fatalf("wide trailing cell bg = (%v, %v), want Green (Rust preserves base style)", bg, set)
	}
	for x := range 10 {
		cell, ok := buf.Get(x, 0)
		if !ok {
			t.Fatalf("cell(%d,0) missing", x)
		}
		bg, set := cell.Style.Background()
		if !set || bg != style.Green {
			t.Fatalf("cell(%d,0) bg = (%v, %v), want Green", x, bg, set)
		}
	}
}

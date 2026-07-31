package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/symbols"
	"github.com/michaelkelly/ratatui-go/text"
)

func getBufferRowString(buf *buffer.Buffer, y int) string {
	var res []rune
	for x := buf.Area.X; x < buf.Area.Right(); x++ {
		c, ok := buf.Get(x, y)
		if !ok {
			res = append(res, ' ')
			continue
		}
		sym := c.DisplaySymbol()
		if sym == "" {
			res = append(res, ' ')
		} else {
			res = append(res, []rune(sym)...)
		}
	}
	return string(res)
}

func TestBlockBorders(t *testing.T) {
	area := layout.NewRect(0, 0, 5, 3)
	buf := buffer.Empty(area)

	blk := Bordered()
	blk.Render(area, buf)

	topLine := getBufferRowString(buf, 0)
	midLine := getBufferRowString(buf, 1)
	botLine := getBufferRowString(buf, 2)

	if topLine != "┌───┐" {
		t.Errorf("top row = %q, want %q", topLine, "┌───┐")
	}
	if midLine != "│   │" {
		t.Errorf("mid row = %q, want %q", midLine, "│   │")
	}
	if botLine != "└───┘" {
		t.Errorf("bot row = %q, want %q", botLine, "└───┘")
	}
}

func TestBlockPartialBorders(t *testing.T) {
	area := layout.NewRect(0, 0, 4, 3)
	buf := buffer.Empty(area)

	blk := NewBlock().WithBorders(BorderTop | BorderLeft)
	blk.Render(area, buf)

	topLine := getBufferRowString(buf, 0)
	midLine := getBufferRowString(buf, 1)

	if topLine != "┌───" {
		t.Errorf("top row = %q, want %q", topLine, "┌───")
	}
	if midLine != "│   " {
		t.Errorf("mid row = %q, want %q", midLine, "│   ")
	}
}

func TestBlockTitleAndAlignment(t *testing.T) {
	area := layout.NewRect(0, 0, 10, 3)
	buf := buffer.Empty(area)

	blk := Bordered().
		TitleTop(text.RawLine("Hi")).
		TitleAlignment(layout.AlignCenter).
		TitleBottom(text.RawLine("End"))

	blk.Render(area, buf)

	topLine := getBufferRowString(buf, 0)
	botLine := getBufferRowString(buf, 2)

	if topLine != "┌───Hi───┐" {
		t.Errorf("top row = %q, want %q", topLine, "┌───Hi───┐")
	}
	if botLine != "└──End───┘" {
		t.Errorf("bottom row = %q, want %q", botLine, "└──End───┘")
	}
}

func TestBlockInnerAndPadding(t *testing.T) {
	area := layout.NewRect(0, 0, 10, 10)
	blk := Bordered().WithPadding(NewPadding(1, 2, 1, 2))

	inner := blk.Inner(area)
	wantInner := layout.NewRect(2, 2, 5, 5)

	if inner != wantInner {
		t.Errorf("Inner() = %+v, want %+v", inner, wantInner)
	}

	hLeft, hRight := blk.HorizontalSpace()
	vTop, vBot := blk.VerticalSpace()

	if hLeft != 2 || hRight != 3 {
		t.Errorf("HorizontalSpace() = (%d, %d), want (2, 3)", hLeft, hRight)
	}
	if vTop != 2 || vBot != 3 {
		t.Errorf("VerticalSpace() = (%d, %d), want (2, 3)", vTop, vBot)
	}
}

func TestBlockMergeBorders(t *testing.T) {
	area := layout.NewRect(0, 0, 3, 3)
	buf := buffer.Empty(area)

	// Pre-fill left column with vertical border symbol
	for y := range 3 {
		if c := buf.GetMut(0, y); c != nil {
			c.SetSymbol("│")
		}
	}

	blk := NewBlock().
		WithBorders(BorderTop).
		MergeBorders(symbols.MergeStrategyExact)

	blk.Render(area, buf)

	// A full vertical line merged with a full horizontal line forms a cross.
	c, ok := buf.Get(0, 0)
	if !ok || c.DisplaySymbol() != "┼" {
		t.Errorf("cell (0,0) symbol = %q, want %q", c.DisplaySymbol(), "┼")
	}
}

func TestBlockZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)
	blk := Bordered().Title(text.RawLine("Test")).WithPadding(PaddingUniform(2))
	blk.Render(zero, buf)
	_ = blk.Inner(zero)
}

func TestBlockMultipleSameAlignmentTitles(t *testing.T) {
	area := layout.NewRect(0, 0, 15, 3)
	buf := buffer.Empty(area)

	blk := Bordered().
		TitleTop(text.RawLine("T1")).
		TitleTop(text.RawLine("T2")).
		TitleAlignment(layout.AlignLeft)

	blk.Render(area, buf)

	topLine := getBufferRowString(buf, 0)
	// Upstream advances one cell between titles without overwriting the border.
	const want = "┌T1─T2────────┐"
	if topLine != want {
		t.Errorf("multiple same-alignment titles top row = %q, want %q", topLine, want)
	}
}

func TestBlockCenteredTruncation(t *testing.T) {
	// Area width 10, title length 14 -> centered title must truncate left/right or fit available width
	area := layout.NewRect(0, 0, 10, 3)
	buf := buffer.Empty(area)

	blk := Bordered().
		TitleTop(text.RawLine("VeryLongTitleX")).
		TitleAlignment(layout.AlignCenter)

	blk.Render(area, buf)

	topLine := getBufferRowString(buf, 0)
	// Should not panic, and top corners ┌ and ┐ must be present at (0,0) and (9,0)
	if !([]rune(topLine)[0] == '┌' && []rune(topLine)[9] == '┐') {
		t.Errorf("top row corners missing on truncated centered title: %q", topLine)
	}
}

func TestBlockCorners(t *testing.T) {
	area := layout.NewRect(0, 0, 4, 4)

	// Test Rounded border type corners
	bufRounded := buffer.Empty(area)
	blkRounded := Bordered().BorderType(BorderTypeRounded)
	blkRounded.Render(area, bufRounded)

	r0 := getBufferRowString(bufRounded, 0)
	r3 := getBufferRowString(bufRounded, 3)
	if r0 != "╭──╮" {
		t.Errorf("rounded top row = %q, want %q", r0, "╭──╮")
	}
	if r3 != "╰──╯" {
		t.Errorf("rounded bottom row = %q, want %q", r3, "╰──╯")
	}

	// Test Double border type corners
	bufDouble := buffer.Empty(area)
	blkDouble := Bordered().BorderType(BorderTypeDouble)
	blkDouble.Render(area, bufDouble)

	d0 := getBufferRowString(bufDouble, 0)
	d3 := getBufferRowString(bufDouble, 3)
	if d0 != "╔══╗" {
		t.Errorf("double top row = %q, want %q", d0, "╔══╗")
	}
	if d3 != "╚══╝" {
		t.Errorf("double bottom row = %q, want %q", d3, "╚══╝")
	}
}

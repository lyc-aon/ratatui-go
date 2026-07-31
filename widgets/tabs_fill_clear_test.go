package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/text"
)

func TestTabsRender(t *testing.T) {
	tabs := NewTabs(text.RawLine("Tab1"), text.RawLine("Tab2")).
		SelectIndex(0).
		Divider(text.RawSpan("|")).
		Padding(text.RawLine(" "), text.RawLine(" "))

	area := layout.NewRect(0, 0, 15, 1)
	buf := buffer.Empty(area)
	tabs.Render(area, buf)

	// Format: " Tab1 | Tab2 " -> " Tab1 | Tab2    "
	line := getBufferRowString(buf, 0)
	wantPrefix := " Tab1 | Tab2 "
	if len(line) < len(wantPrefix) || line[:len(wantPrefix)] != wantPrefix {
		t.Errorf("Tabs rendered line = %q, want prefix %q", line, wantPrefix)
	}

	// Verify selected tab has highlight style
	cTab1, _ := buf.Get(1, 0) // 'T' in Tab1
	cTab2, _ := buf.Get(8, 0) // 'T' in Tab2

	// Default highlight style has ModReversed in AddModifier
	if !cTab1.Style.AddModifier.Contains(style.ModReversed) {
		t.Errorf("Tab1 cell style expected reversed modifier, got %+v", cTab1.Style)
	}
	if cTab2.Style.AddModifier.Contains(style.ModReversed) {
		t.Errorf("Tab2 cell style unexpectedly has reversed modifier, got %+v", cTab2.Style)
	}
}

func TestTabsZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)
	tabs := NewTabs(text.RawLine("A"), text.RawLine("B")).SelectIndex(1)
	tabs.Render(zero, buf)
}

func TestFillRender(t *testing.T) {
	area := layout.NewRect(0, 0, 4, 2)
	buf := buffer.Empty(area)

	st := style.New().WithFG(style.Red)
	fill := NewFill("#").WithStyle(st)
	fill.Render(area, buf)

	for y := range 2 {
		row := getBufferRowString(buf, y)
		if row != "####" {
			t.Errorf("Fill row %d = %q, want %q", y, row, "####")
		}
		c, ok := buf.Get(0, y)
		if !ok || c.Style.FG != style.Red {
			t.Errorf("Fill cell (0,%d) FG = %v, want %v", y, c.Style.FG, style.Red)
		}
	}
}

func TestFillZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)
	NewFill("X").Render(zero, buf)
}

func TestClearRender(t *testing.T) {
	area := layout.NewRect(0, 0, 3, 2)
	buf := buffer.Empty(area)

	// Pre-fill buffer with 'Z' and red background
	fill := NewFill("Z").WithStyle(style.New().WithBG(style.Red))
	fill.Render(area, buf)

	// Clear area (0,0,2,1)
	clearArea := layout.NewRect(0, 0, 2, 1)
	NewClear().Render(clearArea, buf)

	// (0,0) and (1,0) should be reset (space / zero cell)
	c0, _ := buf.Get(0, 0)
	c1, _ := buf.Get(1, 0)
	c2, _ := buf.Get(2, 0)

	if c0.DisplaySymbol() != " " || c0.Style.BG != style.Reset {
		t.Errorf("cleared cell (0,0) symbol=%q bg=%v, want ' ' and Reset", c0.DisplaySymbol(), c0.Style.BG)
	}
	if c1.DisplaySymbol() != " " || c1.Style.BG != style.Reset {
		t.Errorf("cleared cell (1,0) symbol=%q bg=%v, want ' ' and Reset", c1.DisplaySymbol(), c1.Style.BG)
	}
	// (2,0) should remain 'Z' and Red
	if c2.DisplaySymbol() != "Z" || c2.Style.BG != style.Red {
		t.Errorf("uncleared cell (2,0) symbol=%q bg=%v, want 'Z' and Red", c2.DisplaySymbol(), c2.Style.BG)
	}
}

func TestClearZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)
	NewClear().Render(zero, buf)
}

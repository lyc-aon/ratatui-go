package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
)

func TestMascotEyeColorAndFullSymbolFrame(t *testing.T) {
	area := layout.NewRect(0, 0, 32, 16)

	// Default mascot eye color
	bufDefault := buffer.Empty(area)
	mascotDefault := NewRatatuiMascot().EyeColor(MascotEyeDefault)
	mascotDefault.Render(area, bufDefault)

	cellEyeDef, okDef := bufDefault.Get(21, 5)
	if !okDef {
		t.Fatalf("failed to get cell (21,5)")
	}
	if cellEyeDef.Style.BG != style.Indexed(236) {
		t.Errorf("default mascot eye BG = %v, want Indexed(236)", cellEyeDef.Style.BG)
	}

	// Red blinking mascot eye color
	bufRed := buffer.Empty(area)
	mascotRed := NewRatatuiMascot().EyeColor(MascotEyeRed)
	mascotRed.Render(area, bufRed)

	cellEyeRed, okRed := bufRed.Get(21, 5)
	if !okRed {
		t.Fatalf("failed to get cell (21,5)")
	}
	if cellEyeRed.Style.BG != style.Indexed(196) {
		t.Errorf("red mascot eye BG = %v, want Indexed(196)", cellEyeRed.Style.BG)
	}

	// Verify non-empty cells across full symbol frame
	nonEmptyCount := 0
	for y := range area.Height {
		for x := range area.Width {
			if c, ok := bufDefault.Get(x, y); ok && c.DisplaySymbol() != " " {
				nonEmptyCount++
			}
		}
	}
	if nonEmptyCount < 50 {
		t.Errorf("mascot rendered too few non-empty cells (%d)", nonEmptyCount)
	}
}

package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
)

func TestLogoTinyAndSmallFrames(t *testing.T) {
	// Tiny logo (2x15)
	areaTiny := layout.NewRect(0, 0, 15, 2)
	bufTiny := buffer.Empty(areaTiny)
	Tiny().Render(areaTiny, bufTiny)

	r0Tiny := getBufferRowString(bufTiny, 0)
	r1Tiny := getBufferRowString(bufTiny, 1)

	wantR0Tiny := "▛▚▗▀▖▜▘▞▚▝▛▐ ▌▌"
	wantR1Tiny := "▛▚▐▀▌▐ ▛▜ ▌▝▄▘▌"

	if r0Tiny != wantR0Tiny {
		t.Errorf("tiny logo row 0 = %q, want %q", r0Tiny, wantR0Tiny)
	}
	if r1Tiny != wantR1Tiny {
		t.Errorf("tiny logo row 1 = %q, want %q", r1Tiny, wantR1Tiny)
	}

	// Small logo (2x27)
	areaSmall := layout.NewRect(0, 0, 27, 2)
	bufSmall := buffer.Empty(areaSmall)
	Small().Render(areaSmall, bufSmall)

	r0Small := getBufferRowString(bufSmall, 0)
	r1Small := getBufferRowString(bufSmall, 1)

	wantR0Small := "█▀▀▄ ▄▀▀▄▝▜▛▘▄▀▀▄▝▜▛▘█  █ █"
	wantR1Small := "█▀▀▄ █▀▀█ ▐▌ █▀▀█ ▐▌ ▀▄▄▀ █"

	if r0Small != wantR0Small {
		t.Errorf("small logo row 0 = %q, want %q", r0Small, wantR0Small)
	}
	if r1Small != wantR1Small {
		t.Errorf("small logo row 1 = %q, want %q", r1Small, wantR1Small)
	}
}

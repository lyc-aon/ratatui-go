package backend

import (
	"bytes"
	"strings"
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
)

func TestANSIColorSequences(t *testing.T) {
	tests := []struct {
		name        string
		style       style.Style
		wantSubseqs []string
	}{
		{
			name:        "named foreground color",
			style:       style.New().WithFG(style.Red),
			wantSubseqs: []string{"\x1b[31m"},
		},
		{
			name:        "named background color",
			style:       style.New().WithBG(style.Blue),
			wantSubseqs: []string{"\x1b[44m"},
		},
		{
			name:        "indexed foreground color",
			style:       style.New().WithFG(style.Indexed(42)),
			wantSubseqs: []string{"\x1b[38;5;42m"},
		},
		{
			name:        "indexed background color",
			style:       style.New().WithBG(style.Indexed(100)),
			wantSubseqs: []string{"\x1b[48;5;100m"},
		},
		{
			name:        "RGB foreground color",
			style:       style.New().WithFG(style.RGB(10, 20, 30)),
			wantSubseqs: []string{"\x1b[38;2;10;20;30m"},
		},
		{
			name:        "RGB background color",
			style:       style.New().WithBG(style.RGB(40, 50, 60)),
			wantSubseqs: []string{"\x1b[48;2;40;50;60m"},
		},
		{
			name:        "reset foreground color",
			style:       style.New().WithFG(style.Reset),
			wantSubseqs: []string{"\x1b[39m"},
		},
		{
			name:        "reset background color",
			style:       style.New().WithBG(style.Reset),
			wantSubseqs: []string{"\x1b[49m"},
		},
		{
			name:        "RGB underline color",
			style:       style.New().WithUnderlineColor(style.RGB(70, 80, 90)),
			wantSubseqs: []string{"\x1b[58;2;70;80;90m"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			be := NewANSIBackend(&buf, 10, 5)

			cell := buffer.NewCell()
			cell.SetSymbol("X")
			cell.SetStyle(tt.style)

			err := be.Draw([]buffer.PositionedCell{
				{Position: layout.Position{X: 0, Y: 0}, Cell: cell},
			})
			if err != nil {
				t.Fatalf("unexpected Draw error: %v", err)
			}

			out := buf.String()
			for _, sub := range tt.wantSubseqs {
				if !strings.Contains(out, sub) {
					t.Errorf("output = %q, want it to contain %q", out, sub)
				}
			}
		})
	}
}

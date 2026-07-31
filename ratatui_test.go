package ratatui_test

import (
	"testing"

	rat "github.com/michaelkelly/ratatui-go"
)

func TestApplicationFacadeDrawsAndSplits(t *testing.T) {
	area := rat.NewRect(0, 0, 8, 2)
	backend := rat.NewTestBackend(area.Width, area.Height)
	terminal, err := rat.NewTerminal(backend, rat.WithFixedArea(area))
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	if _, err := terminal.Draw(func(frame *rat.Frame) {
		frame.RenderWidget(rat.AsStringWidget("Go ✓"), frame.Area())
	}); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	if got := backend.BufferLines()[0]; got != "Go ✓    " {
		t.Fatalf("rendered line = %q, want %q", got, "Go ✓    ")
	}

	parts := rat.HorizontalLayout(rat.Length(2), rat.Fill(1)).Split(area)
	if len(parts) != 2 || parts[0].Width != 2 || parts[1].Width != 6 {
		t.Fatalf("facade layout split = %+v", parts)
	}
	if rat.UpstreamVersion != "0.30.2" || len(rat.UpstreamRevision) != 40 {
		t.Fatalf("invalid pinned source metadata: %s %s", rat.UpstreamVersion, rat.UpstreamRevision)
	}
}

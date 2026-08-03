package runtime

import (
	"os"
	"strings"
	"testing"
)

const mouseResetBytes = "\x1b[?1006l\x1b[?1003l\x1b[?1002l\x1b[?1000l"

func newMouseOutputTerminal(t *testing.T, mode MouseMode) (*Terminal, *os.File) {
	t.Helper()
	out, err := os.CreateTemp("", "ompui-mouse-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = out.Close()
		_ = os.Remove(out.Name())
	})
	return &Terminal{out: out, mouseMode: mode}, out
}

func mouseOutput(t *testing.T, out *os.File) string {
	t.Helper()
	if err := out.Sync(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMousePresetSequencesAreByteExact(t *testing.T) {
	tests := []struct {
		name    string
		initial MouseMode
		mode    MouseMode
		want    string
	}{
		{
			name:    "off",
			initial: MouseAll,
			mode:    MouseOff,
			want:    mouseResetBytes,
		},
		{
			name:    "wheel",
			initial: MouseOff,
			mode:    MouseWheel,
			want:    mouseResetBytes + "\x1b[?1000h\x1b[?1006h",
		},
		{
			name:    "buttons",
			initial: MouseOff,
			mode:    MouseButtons,
			want:    mouseResetBytes + "\x1b[?1000h\x1b[?1002h\x1b[?1006h",
		},
		{
			name:    "all",
			initial: MouseOff,
			mode:    MouseAll,
			want:    mouseResetBytes + "\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1006h",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			term, out := newMouseOutputTerminal(t, test.initial)
			term.setMouseMode(test.mode)
			if got := mouseOutput(t, out); got != test.want {
				t.Fatalf("mouse bytes = %q, want %q", got, test.want)
			}
			if term.mouseMode != test.mode {
				t.Fatalf("mode = %q, want %q", term.mouseMode, test.mode)
			}
		})
	}
}

func TestMouseTransitionsAreIdempotent(t *testing.T) {
	term, out := newMouseOutputTerminal(t, MouseOff)
	term.setMouseMode(MouseWheel)
	first := mouseOutput(t, out)
	term.setMouseMode(MouseWheel)
	if got := mouseOutput(t, out); got != first {
		t.Fatalf("same preset rewrote terminal: %q", got[len(first):])
	}

	term.setMouseMode(MouseButtons)
	want := first + mouseResetBytes + "\x1b[?1000h\x1b[?1002h\x1b[?1006h"
	if got := mouseOutput(t, out); got != want {
		t.Fatalf("wheel-to-buttons bytes = %q, want %q", got, want)
	}
}

func TestMouseCompatibilityWrappersAndValidation(t *testing.T) {
	term := &Terminal{
		started: true,
		cmdCh:   make(chan any, 2),
		stopCh:  newLoopSignal(),
	}
	if err := term.EnableMouse(); err != nil {
		t.Fatal(err)
	}
	if cmd := (<-term.cmdCh).(cmdSetMouseMode); cmd.mode != MouseAll {
		t.Fatalf("EnableMouse command = %q, want %q", cmd.mode, MouseAll)
	}
	if err := term.DisableMouse(); err != nil {
		t.Fatal(err)
	}
	if cmd := (<-term.cmdCh).(cmdSetMouseMode); cmd.mode != MouseOff {
		t.Fatalf("DisableMouse command = %q, want %q", cmd.mode, MouseOff)
	}
	if err := term.SetMouseMode(MouseMode("bogus")); err != errInvalidMouseMode {
		t.Fatalf("invalid mode error = %v, want %v", err, errInvalidMouseMode)
	}
}

func TestTerminalShutdownResetsEveryMouseMode(t *testing.T) {
	term, out := newMouseOutputTerminal(t, MouseAll)
	term.started = true
	term.stopCh = newLoopSignal()
	term.loopDone = make(chan struct{})
	if err := term.Stop(); err != nil {
		t.Fatal(err)
	}

	want := "\x1b[?2026l\x1b[?7h" +
		"\x1b[?2004l" +
		"\x1b[?5522l" +
		mouseResetBytes +
		"\x1b[?2031l" +
		"\x1b[?25h"
	if got := mouseOutput(t, out); got != want {
		t.Fatalf("shutdown bytes = %q, want %q", got, want)
	}
	if term.mouseMode != MouseOff {
		t.Fatalf("shutdown mode = %q, want %q", term.mouseMode, MouseOff)
	}
	if strings.Count(mouseOutput(t, out), mouseResetBytes) != 1 {
		t.Fatalf("shutdown did not issue exactly one full mouse reset: %q", mouseOutput(t, out))
	}
}

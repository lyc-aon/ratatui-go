package backend

import (
	"testing"

	"golang.org/x/term"
)

func TestDisableRawModeRetainsStateAfterRestoreFailure(t *testing.T) {
	state := new(term.State)
	tty := &TTYBackend{rawState: state, rawFD: -1}

	if err := tty.DisableRawMode(); err == nil {
		t.Fatal("DisableRawMode with invalid descriptor returned nil")
	}
	if tty.rawState != state || tty.rawFD != -1 {
		t.Fatalf("failed restore discarded retry state: state=%p fd=%d", tty.rawState, tty.rawFD)
	}
}

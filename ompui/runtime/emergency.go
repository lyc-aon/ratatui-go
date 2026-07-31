package runtime

import "sync"

// Process-wide emergency restore registry. Signal/panic handlers call
// EmergencyRestore without holding a Terminal pointer.

var (
	emergencyMu       sync.Mutex
	activeTerminal    *Terminal
	terminalEverStarted bool
	// altScreenActive tracks process-wide alt-screen ownership for blind restore.
	// Only written under emergencyMu + the owning Terminal's lifecycle.
	altScreenActive bool
)

func registerActive(t *Terminal) {
	emergencyMu.Lock()
	activeTerminal = t
	terminalEverStarted = true
	emergencyMu.Unlock()
}

func unregisterActive(t *Terminal) {
	emergencyMu.Lock()
	if activeTerminal == t {
		activeTerminal = nil
	}
	emergencyMu.Unlock()
}

func setAltScreenActiveLocked(active bool) {
	// Caller holds t.mu; also update process-wide flag under emergencyMu.
	emergencyMu.Lock()
	altScreenActive = active
	emergencyMu.Unlock()
}

// EmergencyRestore resets terminal state for signal/panic callers.
// Idempotent and safe when no terminal was started.
//
// Prefer the live Terminal.Stop path when available. Blind restore never writes
// DECRST 1049 unless alt-screen was tracked active (Windows homes the cursor
// on unconditional leave).
func EmergencyRestore() {
	emergencyMu.Lock()
	t := activeTerminal
	ever := terminalEverStarted
	alt := altScreenActive
	emergencyMu.Unlock()

	defer func() {
		// Swallow panics during crash cleanup.
		_ = recover()
	}()

	if t != nil {
		_ = t.Stop()
		if alt {
			_ = t.writeRaw([]byte(seqLeaveAltScreen))
			setAltScreenActiveLocked(false)
		}
		_ = t.writeRaw([]byte(seqShowCursor))
		return
	}
	if !ever {
		return
	}
	// Blind restore: no live instance. Write safe mode teardown only.
	// We have no output file here — best effort via os.Stdout is intentional
	// for crash paths (matches ProcessTerminal emergencyTerminalRestore).
	blindEmergencyWrite(alt)
}

// SetAltScreenActive records alternate-screen ownership for emergency restore.
// The TUI calls this when it enters/leaves alt screen outside Terminal.Start options.
func SetAltScreenActive(active bool) {
	setAltScreenActiveLocked(active)
}

// AltScreenActive reports the process-wide alt-screen flag.
func AltScreenActive() bool {
	emergencyMu.Lock()
	defer emergencyMu.Unlock()
	return altScreenActive
}

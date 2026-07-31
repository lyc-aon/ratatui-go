package termcaps

import (
	"os"
	"runtime"
	"strings"
	"sync"
)

// ProcessEnv copies the live process environment into an Env map.
// Edge wrapper only — pure helpers take Env explicitly.
func ProcessEnv() Env {
	environ := os.Environ()
	out := make(Env, len(environ))
	for _, kv := range environ {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		out[key] = val
	}
	return out
}

// DefaultPlatform returns runtime.GOOS ("windows", "darwin", "linux", …).
func DefaultPlatform() string {
	return runtime.GOOS
}

// DefaultTerminalID detects the terminal family from the process environment.
func DefaultTerminalID() TerminalID {
	return DetectTerminalID(ProcessEnv())
}

// DefaultSnapshot resolves capabilities from the live process environment.
// isTTY should reflect whether stdout is a terminal.
func DefaultSnapshot(isTTY bool) *Snapshot {
	return Resolve(ResolveOptions{
		Env:   ProcessEnv(),
		IsTTY: isTTY,
	})
}

// DefaultSessionID resolves session identity from a supplied tty path and the
// live process environment. Pass ttyPath="" when unknown; env fallbacks still run.
func DefaultSessionID(stdinIsTTY bool, ttyPath string) string {
	return ResolveSessionID(stdinIsTTY, ttyPath, ProcessEnv())
}

// DefaultIsWindowsTerminalPreviewSixelSupported gates WT Sixel using live env/GOOS.
func DefaultIsWindowsTerminalPreviewSixelSupported() bool {
	return IsWindowsTerminalPreviewSixelSupported(ProcessEnv(), DefaultPlatform())
}

// package-level mutable cell dimensions (matches OMP get/setCellDimensions).
var defaultCells = NewCellSize()

var (
	defaultTermMu sync.Mutex
	defaultTerm   *Snapshot
)

// GetCellDimensions returns the process-default cell size.
func GetCellDimensions() CellDimensions {
	return defaultCells.Get()
}

// SetCellDimensions updates the process-default cell size.
func SetCellDimensions(dims CellDimensions) {
	defaultCells.Set(dims)
}

// InitDefaultTerminal installs the process-wide Snapshot used by the edge
// setters below. Call once at startup; later calls replace it.
func InitDefaultTerminal(isTTY bool) *Snapshot {
	s := DefaultSnapshot(isTTY)
	defaultTermMu.Lock()
	defaultTerm = s
	defaultTermMu.Unlock()
	return s
}

// DefaultTerminal returns the process-wide Snapshot, resolving lazily with
// isTTY=true if InitDefaultTerminal was never called.
func DefaultTerminal() *Snapshot {
	defaultTermMu.Lock()
	defer defaultTermMu.Unlock()
	if defaultTerm == nil {
		defaultTerm = DefaultSnapshot(true)
	}
	return defaultTerm
}

// SetTerminalImageProtocol overrides the process-default image protocol.
func SetTerminalImageProtocol(p ImageProtocol) {
	DefaultTerminal().SetImageProtocol(p)
}

// SetTerminalDECCARA overrides the process-default DECCARA flag.
func SetTerminalDECCARA(enabled bool) {
	DefaultTerminal().SetDECCARA(enabled)
}

// SetTerminalScreenToScrollback overrides the process-default screen-to-scrollback flag.
func SetTerminalScreenToScrollback(enabled bool) {
	DefaultTerminal().SetScreenToScrollback(enabled)
}

// SetTerminalTextSizing overrides the process-default text-sizing flag.
func SetTerminalTextSizing(enabled bool) {
	DefaultTerminal().SetTextSizing(enabled)
}

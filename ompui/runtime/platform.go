package runtime

import (
	"os"
	"time"

	"golang.org/x/term"
)

// platformState holds OS-specific restore handles captured at Start.
type platformState struct {
	// raw is the termios/console state captured before MakeRaw.
	raw *term.State
	// rawFD is the fd raw mode was applied to.
	rawFD int
	// win holds Windows VT-input / codepage restore data (nil on Unix).
	win *windowsConsoleState
}

// windowsConsoleState is defined in platform_windows.go; other builds use a stub.
// (Concrete type lives in the windows file.)

// enableRaw puts in into raw mode, returning prior state.
func enableRaw(in *os.File) (*term.State, error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		return nil, errNotTerminal
	}
	return term.MakeRaw(fd)
}

// restoreRaw restores prior termios/console state.
func restoreRaw(fd int, st *term.State) error {
	if st == nil {
		return nil
	}
	return term.Restore(fd, st)
}

// isTerminalFile reports whether f is a terminal device.
func isTerminalFile(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// termSize returns cell cols/rows for f.
func termSize(f *os.File) (cols, rows int, err error) {
	if f == nil {
		return 0, 0, errNotTerminal
	}
	return term.GetSize(int(f.Fd()))
}

// readWithTimeout reads from in with a deadline-style wait.
// Implemented per-OS.
func readWithTimeout(in *os.File, p []byte, timeout time.Duration) (int, error) {
	return platformReadWithTimeout(in, p, timeout)
}

// ttyDevicePath returns the device path for fd when available.
func ttyDevicePath(fd int) string {
	return platformTTYPath(fd)
}

// signalResizeChannel returns a channel that receives on SIGWINCH (Unix)
// or is never used (Windows uses a poller). close stop to end the watcher.
func startResizeWatcher(out *os.File, notify chan struct{}, stop <-chan struct{}) {
	platformStartResizeWatcher(out, notify, stop)
}

// platformEnableVTInput enables Windows VT input; no-op elsewhere.
func platformEnableVTInput(in *os.File) *windowsConsoleState {
	return enableWindowsVTInput(in)
}

// platformRestoreVTInput restores Windows VT input mode.
func platformRestoreVTInput(st *windowsConsoleState) {
	restoreWindowsVTInput(st)
}

// platformEnsureUTF8 re-asserts UTF-8 console codepage on Windows before write.
func platformEnsureUTF8() {
	ensureWindowsConsoleUTF8()
}

// platformIsConPTY reports ConPTY hosting (win32 or WSL).
func platformIsConPTY(platform string, env map[string]string) bool {
	return isConPTYHosted(platform, env)
}

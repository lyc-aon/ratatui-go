//go:build windows

package runtime

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const (
	cpUTF8                  = 65001
	enableVirtualTerminalIn = windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	stdInputHandle          = windows.STD_INPUT_HANDLE
	stdOutputHandle         = windows.STD_OUTPUT_HANDLE
)

// windowsConsoleState holds console mode / codepage restore data.
type windowsConsoleState struct {
	inHandle     windows.Handle
	originalMode uint32
	modeChanged  bool
	// codepage restore is best-effort; we only force UTF-8 on write.
}

func enableWindowsVTInput(in *os.File) *windowsConsoleState {
	if in == nil {
		return nil
	}
	h := windows.Handle(in.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return nil
	}
	st := &windowsConsoleState{inHandle: h, originalMode: mode}
	vt := mode | enableVirtualTerminalIn
	if vt != mode {
		if err := windows.SetConsoleMode(h, vt); err != nil {
			return nil
		}
		st.modeChanged = true
	}
	return st
}

func restoreWindowsVTInput(st *windowsConsoleState) {
	if st == nil || !st.modeChanged {
		return
	}
	_ = windows.SetConsoleMode(st.inHandle, st.originalMode)
	st.modeChanged = false
}

var (
	codepageMu     sync.Mutex
	codepageReady  bool
	codepageFailed bool
)

func ensureWindowsConsoleUTF8() {
	codepageMu.Lock()
	defer codepageMu.Unlock()
	if codepageFailed {
		return
	}
	// Always re-assert: a child process may have flipped the codepage.
	if err := windows.SetConsoleCP(cpUTF8); err != nil {
		codepageFailed = true
		return
	}
	if err := windows.SetConsoleOutputCP(cpUTF8); err != nil {
		codepageFailed = true
		return
	}
	codepageReady = true
}

func isConPTYHosted(platform string, _ map[string]string) bool {
	// Native win32 always hosts through ConPTY for modern terminals.
	return platform == "windows"
}

func platformReadWithTimeout(in *os.File, p []byte, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		return 0, os.ErrDeadlineExceeded
	}
	millis := timeout.Milliseconds()
	if millis < 1 {
		millis = 1
	}
	if millis > int64(^uint32(0)-1) {
		millis = int64(^uint32(0) - 1)
	}
	status, err := windows.WaitForSingleObject(windows.Handle(in.Fd()), uint32(millis))
	if err != nil {
		return 0, err
	}
	switch status {
	case windows.WAIT_OBJECT_0:
		return in.Read(p)
	case uint32(windows.WAIT_TIMEOUT):
		return 0, os.ErrDeadlineExceeded
	default:
		return 0, fmt.Errorf("runtime: terminal input wait failed: status=%#x", status)
	}
}

func platformTTYPath(_ int) string {
	// No stable /dev path on Windows; session id falls back to WT_SESSION etc.
	return ""
}

func platformWindowPixels(fd int) (widthPx, heightPx int) {
	// Classic consoles expose cell size only.
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return 0, 0
	}
	return 0, 0
}

// Windows has no SIGWINCH. Poll console buffer size on a short timer and
// notify when cell geometry changes. stop ends the goroutine.
func platformStartResizeWatcher(out *os.File, notify chan struct{}, stop <-chan struct{}) {
	if out == nil {
		return
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	lastCols, lastRows := 0, 0
	if c, r, err := termSize(out); err == nil {
		lastCols, lastRows = c, r
	}
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			c, r, err := termSize(out)
			if err != nil {
				continue
			}
			if c != lastCols || r != lastRows {
				lastCols, lastRows = c, r
				select {
				case notify <- struct{}{}:
				default:
				}
			}
		}
	}
}

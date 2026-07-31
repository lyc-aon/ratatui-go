//go:build windows

package backend

import (
	"fmt"
	"os"
	"time"

	"github.com/michaelkelly/ratatui-go/layout"
	"golang.org/x/sys/windows"
)

// windowPixelSize returns pixel metrics when available.
// Classic Windows consoles expose cell size only; pixels are (0,0).
func windowPixelSize(fd int) (layout.Size, error) {
	// Console APIs report character cells, not pixels. Keep the call so a
	// non-console handle still surfaces a real error instead of a silent
	// static fallback for the cell size path (handled by term.GetSize).
	var info windows.ConsoleScreenBufferInfo
	err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info)
	if err != nil {
		return layout.Size{}, fmt.Errorf("backend: GetConsoleScreenBufferInfo: %w", err)
	}
	return layout.Size{}, nil
}

func readTTYWithTimeout(in *os.File, p []byte, timeout time.Duration) (int, error) {
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
		return 0, fmt.Errorf("backend: terminal input wait failed: status=%#x", status)
	}
}

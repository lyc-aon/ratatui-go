//go:build !aix && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris && !windows

package runtime

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// windowsConsoleState is unused on this platform.
type windowsConsoleState struct{}

func enableWindowsVTInput(_ *os.File) *windowsConsoleState { return nil }

func restoreWindowsVTInput(_ *windowsConsoleState) {}

func ensureWindowsConsoleUTF8() {}

func isConPTYHosted(_ string, _ map[string]string) bool { return false }

func platformReadWithTimeout(_ *os.File, _ []byte, _ time.Duration) (int, error) {
	return 0, fmt.Errorf("runtime: timed read not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func platformTTYPath(_ int) string { return "" }

func platformWindowPixels(_ int) (widthPx, heightPx int) { return 0, 0 }

func platformStartResizeWatcher(_ *os.File, _ chan struct{}, stop <-chan struct{}) {
	// No resize signal; block until stop so the lifecycle wait group tracks us.
	<-stop
}

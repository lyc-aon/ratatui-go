//go:build !aix && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris && !windows

package backend

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/lyc-aon/ratatui-go/layout"
)

func windowPixelSize(fd int) (layout.Size, error) {
	return layout.Size{}, fmt.Errorf("backend: window pixel size not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func readTTYWithTimeout(_ *os.File, _ []byte, _ time.Duration) (int, error) {
	return 0, fmt.Errorf("backend: cursor query not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

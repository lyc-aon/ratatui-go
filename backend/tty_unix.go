//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package backend

import (
	"fmt"
	"os"
	"time"

	"github.com/lyc-aon/ratatui-go/layout"
	"golang.org/x/sys/unix"
)

// windowPixelSize returns the terminal pixel size via TIOCGWINSZ.
// Xpixel/Ypixel may be 0 on hosts that leave them unused.
func windowPixelSize(fd int) (layout.Size, error) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return layout.Size{}, fmt.Errorf("backend: TIOCGWINSZ: %w", err)
	}
	return layout.Size{
		Width:  int(ws.Xpixel),
		Height: int(ws.Ypixel),
	}, nil
}

func readTTYWithTimeout(in *os.File, p []byte, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		return 0, os.ErrDeadlineExceeded
	}
	millis := int(timeout.Milliseconds())
	if millis < 1 {
		millis = 1
	}
	fds := []unix.PollFd{{Fd: int32(in.Fd()), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, millis)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, os.ErrDeadlineExceeded
	}
	if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		return 0, fmt.Errorf("backend: terminal input poll failed: revents=%#x", fds[0].Revents)
	}
	return in.Read(p)
}

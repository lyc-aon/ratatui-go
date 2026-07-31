//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package runtime

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// windowsConsoleState is unused on Unix.
type windowsConsoleState struct{}

func enableWindowsVTInput(_ *os.File) *windowsConsoleState { return nil }

func restoreWindowsVTInput(_ *windowsConsoleState) {}

func ensureWindowsConsoleUTF8() {}

func isConPTYHosted(platform string, env map[string]string) bool {
	// WSL: stdout still crosses into ConPTY at the wslhost boundary.
	if platform == "linux" {
		if env != nil {
			if env["WSL_DISTRO_NAME"] != "" || env["WSL_INTEROP"] != "" {
				return true
			}
		}
	}
	return false
}

func platformReadWithTimeout(in *os.File, p []byte, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		return 0, os.ErrDeadlineExceeded
	}
	deadline := time.Now().Add(timeout)
	fds := []unix.PollFd{{Fd: int32(in.Fd()), Events: unix.POLLIN}}
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, os.ErrDeadlineExceeded
		}
		millis := int((remaining + time.Millisecond - 1) / time.Millisecond)
		n, err := unix.Poll(fds, millis)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, os.ErrDeadlineExceeded
		}
		if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 &&
			fds[0].Revents&unix.POLLIN == 0 {
			return 0, fmt.Errorf("runtime: terminal input poll failed: revents=%#x", fds[0].Revents)
		}
		n, err = in.Read(p)
		if err == unix.EINTR {
			continue
		}
		return n, err
	}
}

func platformTTYPath(fd int) string {
	// Linux: /proc/self/fd/N is a symlink to the device.
	if link, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd)); err == nil {
		if len(link) >= 5 && link[:5] == "/dev/" {
			return link
		}
	}
	// POSIX ttyname via ioctl TIOCGPATH is not portable; try unix.IoctlGetIntless path.
	// x/sys does not expose ttyname directly on all GOOS — use /dev/fd readlink on
	// BSDs where available, else empty (session id falls back to env).
	if link, err := os.Readlink(fmt.Sprintf("/dev/fd/%d", fd)); err == nil {
		if len(link) >= 5 && link[:5] == "/dev/" {
			return link
		}
	}
	return ""
}

func platformWindowPixels(fd int) (widthPx, heightPx int) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}
	return int(ws.Xpixel), int(ws.Ypixel)
}

func platformStartResizeWatcher(out *os.File, notify chan struct{}, stop <-chan struct{}) {
	ch := make(chan os.Signal, 1)
	platformNotifyResizeSignal(ch)
	defer platformStopResizeSignal(ch)
	// Kick one SIGWINCH so dimensions refresh after suspend/resume gaps.
	platformKickResize()
	_ = out
	for {
		select {
		case <-stop:
			return
		case <-ch:
			select {
			case notify <- struct{}{}:
			default:
				// Coalesce resize bursts.
			}
		}
	}
}

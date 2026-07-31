//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package runtime

import (
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

func platformNotifyResizeSignal(ch chan<- os.Signal) {
	signal.Notify(ch, unix.SIGWINCH)
}

func platformStopResizeSignal(ch chan<- os.Signal) {
	signal.Stop(ch)
}

func platformKickResize() {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return
	}
	_ = p.Signal(unix.SIGWINCH)
}

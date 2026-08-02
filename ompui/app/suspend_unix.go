//go:build unix

package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func (a *App) handleSuspend() {
	a.logf("suspending application via SIGTSTP")

	// 1. Drain and stop terminal raw mode
	if a.term != nil {
		_ = a.term.Stop()
	}

	// 2. Register SIGCONT handler to resume
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGCONT)

	go func() {
		<-sigCh
		signal.Stop(sigCh)
		a.post(command{kind: cmdForceRender})
	}()

	// 3. Send SIGTSTP to self (pid 0 = process group)
	err := syscall.Kill(0, syscall.SIGTSTP)
	if err != nil {
		a.setLocalError(fmt.Sprintf("suspend failed: %v", err))
		return
	}

	// 4. Re-start terminal after resume from SIGCONT
	if a.term != nil {
		_ = a.term.Start(a.ctx)
	}
	a.resetDisplay()
	a.setLocalNotice("resumed from background")
}

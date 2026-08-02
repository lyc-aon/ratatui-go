//go:build !unix

package app

func (a *App) handleSuspend() {
	a.setLocalNotice("suspend (ctrl+z) is not supported on this platform")
}

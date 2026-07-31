package runtime

import "errors"

var (
	errNilFile          = errors.New("runtime: nil file")
	errNotTerminal      = errors.New("runtime: file is not a terminal")
	errAlreadyStarted   = errors.New("runtime: already started")
	errNotStarted       = errors.New("runtime: not started")
	errTerminalStopped  = errors.New("runtime: terminal stopped")
	errTerminalDead     = errors.New("runtime: terminal write failed")
	errCPRTimeout       = errors.New("runtime: cursor position query timed out")
	errInputClosed      = errors.New("runtime: input closed")
)

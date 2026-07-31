package app

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/term"
)

// ttyFiles holds the input/output files used by runtime.Terminal.
// When openOwn is true, Close releases files we opened (not os.Stdin/Stdout).
type ttyFiles struct {
	In      *os.File
	Out     *os.File
	openOwn bool
}

func (t *ttyFiles) Close() {
	if t == nil || !t.openOwn {
		return
	}
	if t.In != nil {
		_ = t.In.Close()
	}
	if t.Out != nil && t.Out != t.In {
		_ = t.Out.Close()
	}
}

// openTTY selects the controlling terminal for keyboard + drawing.
//
// Preferred path (Bun → Go with inherited TTY): use in/out when both are
// terminals. Fallback: open /dev/tty (Unix) or CONIN$/CONOUT$ (Windows).
// Never uses non-TTY stdio as the runtime surface (those may be pipes).
func openTTY(in, out *os.File) (*ttyFiles, error) {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if isTTY(in) && isTTY(out) {
		return &ttyFiles{In: in, Out: out, openOwn: false}, nil
	}
	if isTTY(in) && in == out {
		return &ttyFiles{In: in, Out: out, openOwn: false}, nil
	}
	return openControllingTTY()
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func openControllingTTY() (*ttyFiles, error) {
	if runtime.GOOS == "windows" {
		return openWindowsConsole()
	}
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil && isTTY(f) {
		return &ttyFiles{In: f, Out: f, openOwn: true}, nil
	}
	if f != nil {
		_ = f.Close()
	}
	in, errIn := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	out, errOut := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if errIn != nil || errOut != nil || !isTTY(in) || !isTTY(out) {
		if in != nil {
			_ = in.Close()
		}
		if out != nil {
			_ = out.Close()
		}
		if err == nil {
			err = errIn
		}
		if err == nil {
			err = errOut
		}
		if err == nil {
			err = fmt.Errorf("/dev/tty is not a terminal")
		}
		return nil, fmt.Errorf("open controlling tty: %w", err)
	}
	return &ttyFiles{In: in, Out: out, openOwn: true}, nil
}

func openWindowsConsole() (*ttyFiles, error) {
	in, errIn := os.OpenFile("CONIN$", os.O_RDWR, 0)
	out, errOut := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if errIn != nil || errOut != nil {
		if in != nil {
			_ = in.Close()
		}
		if out != nil {
			_ = out.Close()
		}
		err := errIn
		if err == nil {
			err = errOut
		}
		return nil, fmt.Errorf("open windows console: %w", err)
	}
	return &ttyFiles{In: in, Out: out, openOwn: true}, nil
}

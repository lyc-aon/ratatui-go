package ratatui

import (
	"fmt"
	"os"
	"sync"

	"github.com/lyc-aon/ratatui-go/backend"
	"github.com/lyc-aon/ratatui-go/terminal"
)

// DefaultTerminal is the default application terminal type returned by Init helpers.
//
// It is the existing *terminal.Terminal used throughout the Go port. The
// default backend behind Init is a TTYBackend on os.Stdout.
type DefaultTerminal = terminal.Terminal

// session tracks process-wide terminal lifecycle owned by Init/Restore.
//
// Protected by sessionMu. rawActive / altActive reflect what this package
// enabled so Restore can undo them in the correct order even across panics.
type session struct {
	rawActive bool
	altActive bool
	// tty is the backend created by the most recent successful Init path.
	// Restore uses it when present; otherwise it falls back to os.Stdout
	// sequences + term.Restore via a transient TTYBackend.
	tty *backend.TTYBackend
}

var (
	sessionMu sync.Mutex
	sess      session
)

// TryInit initializes a fullscreen DefaultTerminal with raw mode and the
// alternate screen enabled. Returns a real error on failure.
//
// Pair direct Init/TryInit use with defer Restore. Go has no process-wide panic
// hook; Run provides automatic restore, including panics on the calling goroutine.
// Ordering: enable raw mode, enter alternate screen, construct Terminal.
func TryInit() (*DefaultTerminal, error) {
	return tryInit(true, terminal.Options{})
}

// Init is TryInit but panics on failure. Callers must defer Restore, or use Run.
func Init() *DefaultTerminal {
	t, err := TryInit()
	if err != nil {
		panic(fmt.Sprintf("ratatui: failed to initialize terminal: %v", err))
	}
	return t
}

// TryInitWithOptions initializes a DefaultTerminal with raw mode enabled and
// the given terminal options. Unlike TryInit it does not enter the alternate
// screen (callers that need it should enter it explicitly). Callers must defer
// Restore, or use Run.
func TryInitWithOptions(opts terminal.Options) (*DefaultTerminal, error) {
	return tryInit(false, opts)
}

// InitWithOptions is TryInitWithOptions but panics on failure. Callers must defer Restore.
func InitWithOptions(opts terminal.Options) *DefaultTerminal {
	t, err := TryInitWithOptions(opts)
	if err != nil {
		panic(fmt.Sprintf("ratatui: failed to initialize terminal: %v", err))
	}
	return t
}

// TryRestore disables raw mode first, then leaves the alternate screen.
// Like Ratatui, a raw-mode failure stops restoration before the leave-screen
// sequence; otherwise the leave-screen sequence is always emitted.
func TryRestore() error {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	return restoreLocked()
}

// Restore is TryRestore but prints errors to stderr instead of returning them.
func Restore() {
	if err := TryRestore(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restore terminal: %v\n", err)
	}
}

// Run initializes a fullscreen terminal, invokes f, restores the terminal, and
// returns f's result. Initialization failures panic, matching Init.
//
// On panic, Restore runs before the panic is re-raised.
func Run[R any](f func(t *DefaultTerminal) R) (result R) {
	t := Init()
	defer func() {
		if r := recover(); r != nil {
			Restore()
			panic(r)
		}
		Restore()
	}()
	return f(t)
}

func tryInit(enterAlt bool, opts terminal.Options) (*DefaultTerminal, error) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if sess.tty != nil {
		return nil, fmt.Errorf("ratatui: terminal is already initialized")
	}

	tty, err := backend.NewTTYBackend(os.Stdout)
	if err != nil {
		return nil, err
	}

	// Enable raw mode first.
	if err := tty.EnableRawMode(); err != nil {
		return nil, err
	}
	sess.rawActive = true
	sess.tty = tty

	if enterAlt {
		if err := tty.EnterAlternateScreen(); err != nil {
			if restoreErr := tty.DisableRawMode(); restoreErr != nil {
				// Keep sess intact so the caller can retry TryRestore.
				return nil, fmt.Errorf("%w (raw-mode restore also failed: %v)", err, restoreErr)
			}
			sess.rawActive = false
			sess.tty = nil
			return nil, err
		}
		sess.altActive = true
	}

	term, err := terminal.NewWithOptions(tty, opts)
	if err != nil {
		_ = restoreLocked()
		return nil, err
	}
	return term, nil
}

// restoreLocked performs restore while sessionMu is held.
// Order: disable raw mode first, then always leave the alternate screen.
func restoreLocked() error {
	tty := sess.tty
	if tty == nil {
		var err error
		tty, err = backend.NewTTYBackend(os.Stdout)
		if err != nil {
			return err
		}
	}

	if sess.rawActive || tty.InRawMode() {
		if err := tty.DisableRawMode(); err != nil {
			return err
		}
		sess.rawActive = false
	}

	if err := tty.LeaveAlternateScreen(); err != nil {
		// Keep the backend so a later TryRestore can retry the write.
		sess.tty = tty
		return err
	}
	sess.altActive = false
	sess.tty = nil
	return nil
}

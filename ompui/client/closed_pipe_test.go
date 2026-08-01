package client

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestClosedPipeAfterReadyIsCleanEOF(t *testing.T) {
	client := &Client{pending: make(map[string]*pendingCall)}
	client.ready.Store(true)
	client.onReadError(io.ErrClosedPipe)
	if err := client.Err(); err != nil {
		t.Fatalf("closed pipe after ready recorded as fatal: %v", err)
	}
}

func TestClosedFileAfterReadyIsCleanEOF(t *testing.T) {
	client := &Client{pending: make(map[string]*pendingCall)}
	client.ready.Store(true)
	client.onReadError(&os.PathError{Op: "read", Path: "|0", Err: os.ErrClosed})
	if err := client.Err(); err != nil {
		t.Fatalf("closed file after ready recorded as fatal: %v", err)
	}
}

func TestEOFBeforeReadyPreservesFirstStartupError(t *testing.T) {
	first := errors.New("startup failed")
	client := &Client{
		readyCh: make(chan struct{}),
		pending: make(map[string]*pendingCall),
	}

	// Models the race window after one startup path records its error but before
	// markReady publishes readiness. EOF used to store a different concrete error
	// type in atomic.Value and panic.
	client.readyErr.Store(first)
	client.onReadError(io.EOF)

	if err := client.Err(); !errors.Is(err, first) {
		t.Fatalf("Err() = %v, want first startup error %v", err, first)
	}
	select {
	case <-client.readyCh:
	default:
		t.Fatal("EOF before ready did not publish the startup result")
	}
}

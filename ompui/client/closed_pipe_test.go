package client

import (
	"io"
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

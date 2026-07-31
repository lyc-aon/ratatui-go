package client

import "errors"

// Sentinel errors returned by the RPC client.
var (
	// ErrClosed means the client is shut down or not started.
	ErrClosed = errors.New("ompui/client: closed")

	// ErrNotReady means a call was attempted before startup readiness completed.
	ErrNotReady = errors.New("ompui/client: core not ready")

	// ErrNoProcess means Options lacked both Command.Path and a ProcessFactory result.
	ErrNoProcess = errors.New("ompui/client: no process")

	// ErrInvalidCommand means Command.Path was empty when using the default spawner.
	ErrInvalidCommand = errors.New("ompui/client: invalid command")

	// ErrReadyTimeout means the core did not emit ready/hello before the deadline.
	ErrReadyTimeout = errors.New("ompui/client: ready timeout")

	// ErrBackpressure means a non-critical event could not be delivered because a
	// subscriber channel was full. Critical session/tool/UI frames are never
	// dropped; they block the dispatcher instead. When a non-critical frame is
	// dropped, an Event with this error is also delivered if possible.
	ErrBackpressure = errors.New("ompui/client: event backpressure")

	// ErrProtocol means a protocol-level failure (major mismatch, malformed
	// peer hello, fatal MsgError) forced the session down.
	ErrProtocol = errors.New("ompui/client: protocol error")

	// ErrChildExit means the child process exited while the client was live.
	// Inspect via [ExitError] for the process status.
	ErrChildExit = errors.New("ompui/client: child exited")

	// ErrRPCFailed is returned by Call helpers when the core responded with
	// success:false. The response body is still available via [RPCError.Response].
	ErrRPCFailed = errors.New("ompui/client: rpc command failed")

	// ErrDuplicateRequestID means two live Calls attempted the same correlation id.
	ErrDuplicateRequestID = errors.New("ompui/client: duplicate request id")
)

// RPCError is returned when the core sends a success:false response.
type RPCError struct {
	Command string
	Message string
	// Response is the full decoded response (Raw preserved).
	Response Response
}

func (e *RPCError) Error() string {
	if e == nil {
		return ErrRPCFailed.Error()
	}
	if e.Message != "" {
		if e.Command != "" {
			return e.Command + ": " + e.Message
		}
		return e.Message
	}
	if e.Command != "" {
		return e.Command + ": " + ErrRPCFailed.Error()
	}
	return ErrRPCFailed.Error()
}

func (e *RPCError) Unwrap() error { return ErrRPCFailed }

// ExitError wraps a child process exit for [Client.Wait] / [Client.Err].
type ExitError struct {
	// Err is the error returned by the process Wait (may be *exec.ExitError).
	Err error
	// Code is the exit code when known; -1 if the process was not waited or
	// was terminated without a status.
	Code int
}

func (e *ExitError) Error() string {
	if e == nil {
		return ErrChildExit.Error()
	}
	if e.Err != nil {
		return ErrChildExit.Error() + ": " + e.Err.Error()
	}
	if e.Code >= 0 {
		return ErrChildExit.Error() + ": exit status " + itoa(e.Code)
	}
	return ErrChildExit.Error()
}

func (e *ExitError) Unwrap() error {
	if e == nil || e.Err == nil {
		return ErrChildExit
	}
	return e.Err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

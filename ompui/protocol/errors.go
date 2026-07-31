package protocol

import "errors"

// Sentinel errors returned by the codec and handshake helpers.
var (
	// ErrFrameTooLarge means a frame length or JSONL line exceeded MaxFrameSize.
	ErrFrameTooLarge = errors.New("protocol: frame exceeds maximum size")

	// ErrMalformedLength means a length-prefixed header was unreadable or zero
	// in a context that requires a positive payload length, or the declared
	// length could not be satisfied by the underlying reader.
	ErrMalformedLength = errors.New("protocol: malformed frame length")

	// ErrZeroLength means a length-prefixed frame declared length 0.
	// Empty envelopes are not meaningful on this wire.
	ErrZeroLength = errors.New("protocol: zero-length frame")

	// ErrMajorMismatch means the peer hello advertised an incompatible major.
	ErrMajorMismatch = errors.New("protocol: major version mismatch")

	// ErrInvalidEnvelope means the JSON decoded but lacked required envelope fields.
	ErrInvalidEnvelope = errors.New("protocol: invalid envelope")

	// ErrInvalidJSON means the frame payload was not valid JSON.
	ErrInvalidJSON = errors.New("protocol: invalid JSON")

	// ErrUnexpectedType means a message had an unexpected type for the operation.
	ErrUnexpectedType = errors.New("protocol: unexpected message type")

	// ErrClosed means the encoder or decoder was used after Close.
	ErrClosed = errors.New("protocol: codec closed")
)

// Package protocol defines the versioned Bun↔Go frontend wire protocol.
//
// The protocol carries semantic application state, not cell diffs. Existing
// TypeScript RPC frames (commands, responses, session events, extension UI,
// host tools, host URIs) travel as lossless JSON payloads inside typed
// envelopes. v1 also adds handshake, editor/status/theme sync, remote
// component sessions, overlay mounting, and host shutdown/error.
//
// # Transports
//
// Two frame codecs share the same envelopes and payloads:
//
//   - JSONL — one JSON envelope per line, terminated by '\n'. This is the
//     initial Bun rpc-ui compatibility transport.
//   - Length-prefixed — 4-byte big-endian uint32 payload length, then that
//     many bytes of JSON. Preferred for binary-safe streaming and large frames.
//
// Both codecs enforce [MaxFrameSize], reject malformed lengths, and share
// write serialization on the encoder side.
//
// # Envelope
//
// Every frame is one [Envelope]:
//
//	{"v":1,"type":"hello","id":"...","payload":{...}}
//
//   - v is the protocol major version. Peers MUST reject a major mismatch
//     after hello (see [ErrMajorMismatch]).
//   - type selects the semantic message family (see Message type constants).
//   - id correlates requests and responses when present.
//   - payload is the typed body. Unknown payload fields are preserved as
//     raw JSON for forward compatibility ([RawPayload], [Envelope.Payload]).
//
// # Handshake
//
// Both sides send [MsgHello] with [HelloPayload] (role, major/minor, caps).
// The peer with the lower major must not continue; either side may close with
// [MsgError] / [MsgShutdown] on mismatch.
//
// # Concurrency
//
// [Encoder] serializes writes with a mutex. [Decoder] is not safe for
// concurrent use on the same stream; use one decoder goroutine per reader.
//
// This package is pure data plus encoding. It imports only the Go standard
// library — no ratatui, no TTY I/O.
package protocol

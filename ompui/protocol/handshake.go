package protocol

import (
	"fmt"
	"io"
)

// Handshake performs a v1 hello exchange on the given framing.
//
// local is sent first; then a hello is read from dec and checked with
// [AcceptHello]. Returns the peer hello on success.
//
// dec must already match the framing used by enc (length-prefixed or JSONL).
// For JSONL, pass a function that reads one envelope (e.g. jsonl.Decode).
type EnvelopeReader interface {
	Decode() (Envelope, error)
}

// Handshake sends local hello via enc (using mode) and reads the peer hello
// from rdr. On major mismatch it best-effort sends an error envelope and
// returns [ErrMajorMismatch].
func Handshake(enc *Encoder, mode Framing, rdr EnvelopeReader, local HelloPayload) (HelloPayload, error) {
	if enc == nil || rdr == nil {
		return HelloPayload{}, fmt.Errorf("%w: nil codec", ErrClosed)
	}
	if local.Protocol == "" {
		local.Protocol = ProtocolName
	}
	if local.Major == 0 {
		local.Major = Major
	}
	env, err := NewEnvelope(MsgHello, "", local)
	if err != nil {
		return HelloPayload{}, err
	}
	switch mode {
	case FramingJSONL:
		err = enc.EncodeJSONL(env)
	default:
		err = enc.Encode(env)
	}
	if err != nil {
		return HelloPayload{}, err
	}
	peerEnv, err := rdr.Decode()
	if err != nil {
		return HelloPayload{}, err
	}
	if peerEnv.Type != MsgHello {
		return HelloPayload{}, fmt.Errorf("%w: expected %q got %q", ErrUnexpectedType, MsgHello, peerEnv.Type)
	}
	var peer HelloPayload
	if err := DecodePayload(peerEnv, &peer); err != nil {
		// Some peers may put hello fields at the top level (bare / Extra).
		if len(peerEnv.Payload) == 0 && len(peerEnv.Extra) > 0 {
			raw := peerEnv.HistoricalPayload()
			if err2 := jsonUnmarshal(raw, &peer); err2 != nil {
				return HelloPayload{}, err
			}
		} else {
			return HelloPayload{}, err
		}
	}
	if err := AcceptHello(peer); err != nil {
		_ = writeError(enc, mode, ErrorPayload{
			Code:    ErrCodeMajorMismatch,
			Message: err.Error(),
			Fatal:   true,
		})
		return peer, err
	}
	return peer, nil
}

// HandshakeConn runs [Handshake] using a [Conn] for writes.
func HandshakeConn(c *Conn, rdr EnvelopeReader, local HelloPayload) (HelloPayload, error) {
	if c == nil {
		return HelloPayload{}, ErrClosed
	}
	return Handshake(c.Enc, c.Mode, rdr, local)
}

func writeError(enc *Encoder, mode Framing, p ErrorPayload) error {
	env, err := NewEnvelope(MsgError, "", p)
	if err != nil {
		return err
	}
	switch mode {
	case FramingJSONL:
		return enc.EncodeJSONL(env)
	default:
		return enc.Encode(env)
	}
}

// ReadHello reads a single hello envelope from rdr without sending one.
func ReadHello(rdr EnvelopeReader) (HelloPayload, error) {
	env, err := rdr.Decode()
	if err != nil {
		return HelloPayload{}, err
	}
	if env.Type != MsgHello {
		return HelloPayload{}, fmt.Errorf("%w: expected %q got %q", ErrUnexpectedType, MsgHello, env.Type)
	}
	var peer HelloPayload
	if err := DecodePayload(env, &peer); err != nil {
		if len(env.Payload) == 0 {
			raw := env.HistoricalPayload()
			if err2 := jsonUnmarshal(raw, &peer); err2 == nil {
				return peer, AcceptHello(peer)
			}
		}
		return HelloPayload{}, err
	}
	if err := AcceptHello(peer); err != nil {
		return peer, err
	}
	return peer, nil
}

// WriteShutdown sends a shutdown envelope.
func WriteShutdown(enc *Encoder, mode Framing, p ShutdownPayload) error {
	env, err := NewEnvelope(MsgShutdown, "", p)
	if err != nil {
		return err
	}
	switch mode {
	case FramingJSONL:
		return enc.EncodeJSONL(env)
	default:
		return enc.Encode(env)
	}
}

// WriteError sends a protocol error envelope.
func WriteError(enc *Encoder, mode Framing, p ErrorPayload) error {
	return writeError(enc, mode, p)
}

// Drain discards envelopes from rdr until EOF or error (not ErrMajorMismatch).
// Useful after shutdown.
func Drain(rdr EnvelopeReader) error {
	for {
		_, err := rdr.Decode()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func jsonUnmarshal(data []byte, dest any) error {
	return DecodePayload(Envelope{Payload: RawPayload(data)}, dest)
}

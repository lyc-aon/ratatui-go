package client

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/michaelkelly/ratatui-go/ompui/protocol"
)

// writer serializes all outbound frames onto the child stdin.
//
// Compatibility mode (peerV1 == false): write bare historical JSON objects
// + newline. Never wrap as v1 {v,type:"rpc_command",payload}.
//
// v1 mode (peerV1 == true): write protocol envelopes using the configured
// framing (JSONL or length-prefix).
//
// Write serialization is owned by [protocol.Encoder]'s mutex — one writer
// goroutine is not required; concurrent Call/Send are safe.
type writer struct {
	enc    *protocol.Encoder
	mode   protocol.Framing
	peerV1 atomic.Bool
	closed atomic.Bool
	onRaw  func([]byte)
}

func newWriter(enc *protocol.Encoder, mode protocol.Framing, onRaw func([]byte)) *writer {
	return &writer{enc: enc, mode: mode, onRaw: onRaw}
}

func (w *writer) setPeerV1(v bool) { w.peerV1.Store(v) }
func (w *writer) isPeerV1() bool   { return w.peerV1.Load() }
func (w *writer) close()           { w.closed.Store(true) }

func (w *writer) dead() bool {
	return w == nil || w.closed.Load() || w.enc == nil
}

func (w *writer) note(body []byte) {
	if w.onRaw != nil {
		w.onRaw(body)
	}
}

// writeRawJSONL writes a pre-marshaled JSON object followed by '\n'.
// Used for bare Bun frames. Does not re-encode.
func (w *writer) writeRawJSONL(body []byte) error {
	if w.dead() {
		return ErrClosed
	}
	if len(body) == 0 {
		return protocol.ErrZeroLength
	}
	w.note(body)
	return w.enc.EncodeRawJSONL(body)
}

// writeRawFramed writes pre-marshaled JSON with the active framing.
func (w *writer) writeRawFramed(body []byte) error {
	if w.dead() {
		return ErrClosed
	}
	if len(body) == 0 {
		return protocol.ErrZeroLength
	}
	w.note(body)
	if w.mode == protocol.FramingJSONL || !w.peerV1.Load() {
		return w.enc.EncodeRawJSONL(body)
	}
	return w.enc.EncodeRaw(body)
}

// writeCommand emits an RPC command.
//
// Compatibility: marshal the bare command object (RPCCommand.MarshalJSON) and
// write JSONL. v1: wrap as MsgRPCCommand envelope.
func (w *writer) writeCommand(cmd protocol.RPCCommand) error {
	if w.dead() {
		return ErrClosed
	}
	if cmd.Type == "" {
		return fmt.Errorf("%w: command missing type", protocol.ErrInvalidEnvelope)
	}
	body, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("%w: %v", protocol.ErrInvalidJSON, err)
	}
	if !w.peerV1.Load() {
		return w.writeRawJSONL(body)
	}
	env, err := protocol.WrapRPCCommand(protocol.RawPayload(body))
	if err != nil {
		return err
	}
	return w.writeEnvelope(env)
}

// writeExtensionUIResponse emits a host→core extension UI reply.
func (w *writer) writeExtensionUIResponse(resp protocol.ExtensionUIResponse) error {
	if w.dead() {
		return ErrClosed
	}
	if resp.Type == "" {
		resp.Type = protocol.MsgExtensionUIResponse
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("%w: %v", protocol.ErrInvalidJSON, err)
	}
	if !w.peerV1.Load() {
		return w.writeRawJSONL(body)
	}
	env, err := protocol.WrapHistorical(protocol.RawPayload(body))
	if err != nil {
		return err
	}
	return w.writeEnvelope(env)
}

func (w *writer) writeHostToolResult(res protocol.HostToolResult) error {
	if w.dead() {
		return ErrClosed
	}
	if res.Type == "" {
		res.Type = protocol.MsgHostToolResult
	}
	body, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("%w: %v", protocol.ErrInvalidJSON, err)
	}
	if !w.peerV1.Load() {
		return w.writeRawJSONL(body)
	}
	env, err := protocol.WrapHistorical(protocol.RawPayload(body))
	if err != nil {
		return err
	}
	return w.writeEnvelope(env)
}

func (w *writer) writeHostToolUpdate(u protocol.HostToolUpdate) error {
	if w.dead() {
		return ErrClosed
	}
	if u.Type == "" {
		u.Type = protocol.MsgHostToolUpdate
	}
	body, err := json.Marshal(u)
	if err != nil {
		return fmt.Errorf("%w: %v", protocol.ErrInvalidJSON, err)
	}
	if !w.peerV1.Load() {
		return w.writeRawJSONL(body)
	}
	env, err := protocol.WrapHistorical(protocol.RawPayload(body))
	if err != nil {
		return err
	}
	return w.writeEnvelope(env)
}

func (w *writer) writeHostURIResult(res protocol.HostURIResult) error {
	if w.dead() {
		return ErrClosed
	}
	if res.Type == "" {
		res.Type = protocol.MsgHostURIResult
	}
	body, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("%w: %v", protocol.ErrInvalidJSON, err)
	}
	if !w.peerV1.Load() {
		return w.writeRawJSONL(body)
	}
	env, err := protocol.WrapHistorical(protocol.RawPayload(body))
	if err != nil {
		return err
	}
	return w.writeEnvelope(env)
}

func (w *writer) writeEnvelope(env protocol.Envelope) error {
	if w.dead() {
		return ErrClosed
	}
	body, err := env.Bytes()
	if err != nil {
		return err
	}
	return w.writeRawFramed(body)
}

// writeHello sends a local hello envelope.
func (w *writer) writeHello(h protocol.HelloPayload) error {
	if w.dead() {
		return ErrClosed
	}
	env, err := protocol.NewEnvelope(protocol.MsgHello, "", h)
	if err != nil {
		return err
	}
	body, err := env.Bytes()
	if err != nil {
		return err
	}
	// Always JSONL for hello unless the session is already length-prefixed v1.
	if w.mode == protocol.FramingJSONL || !w.peerV1.Load() {
		return w.writeRawJSONL(body)
	}
	return w.writeRawFramed(body)
}

// writeShutdown sends a v1 shutdown envelope. Historical Bun has no shutdown
// frame — caller closes stdin instead.
func (w *writer) writeShutdown(p protocol.ShutdownPayload) error {
	if w.dead() {
		return ErrClosed
	}
	if !w.peerV1.Load() {
		return nil
	}
	env, err := protocol.NewEnvelope(protocol.MsgShutdown, "", p)
	if err != nil {
		return err
	}
	return w.writeEnvelope(env)
}

// writePreMarshaled writes caller-owned JSON bytes without re-encoding.
func (w *writer) writePreMarshaled(body []byte) error {
	if w.dead() {
		return ErrClosed
	}
	return w.writeRawFramed(body)
}

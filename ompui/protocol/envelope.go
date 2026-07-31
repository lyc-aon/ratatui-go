package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Message type constants. Values match the wire "type" field exactly.
//
// Existing Bun RPC frames keep their historical type strings where they already
// had one (response, extension_ui_request, host_tool_call, …). New frontend
// messages use snake_case names under the v1 envelope.
const (
	// Handshake / control
	MsgHello    = "hello"
	MsgShutdown = "shutdown"
	MsgError    = "error"

	// Existing RPC command/response (payload is the full historical object).
	// Commands arrive as type=rpc_command with the original command object in
	// payload (including its inner "type" field). Responses keep type=response
	// so JSONL peers that already switch on "type":"response" stay compatible
	// when speaking bare historical frames via [DecodeFlexible].
	MsgRPCCommand  = "rpc_command"
	MsgRPCResponse = "response"

	// Session / stream events from the Bun core.
	MsgSessionEvent            = "session_event"
	MsgAvailableCommandsUpdate = "available_commands_update"
	MsgPromptResult            = "prompt_result"
	MsgCommandOutput           = "command_output"
	MsgSubagentLifecycle       = "subagent_lifecycle"
	MsgSubagentProgress        = "subagent_progress"
	MsgSubagentEvent           = "subagent_event"

	// Extension UI (existing RPC strings).
	MsgExtensionUIRequest  = "extension_ui_request"
	MsgExtensionUIResponse = "extension_ui_response"

	// Host tool frames (existing RPC strings).
	MsgHostToolCall   = "host_tool_call"
	MsgHostToolCancel = "host_tool_cancel"
	MsgHostToolUpdate = "host_tool_update"
	MsgHostToolResult = "host_tool_result"

	// Host URI frames (existing RPC strings).
	MsgHostURIRequest = "host_uri_request"
	MsgHostURICancel  = "host_uri_cancel"
	MsgHostURIResult  = "host_uri_result"

	// Editor / status / theme sync (Category C).
	MsgEditorState    = "editor_state"
	MsgEditorUpdate   = "editor_update"
	MsgEditorQuery    = "editor_query"
	MsgStatusSync     = "status_sync"
	MsgWorkingMessage = "working_message"
	MsgToolsExpanded  = "tools_expanded"
	MsgThemeSync      = "theme_sync"
	MsgThemeQuery     = "theme_query"

	// Remote component sessions (Category B).
	MsgComponentOpen       = "component_open"
	MsgComponentRender     = "component_render"
	MsgComponentResult     = "component_result"
	MsgComponentInput      = "component_input"
	MsgComponentInvalidate = "component_invalidate"
	MsgComponentDispose    = "component_dispose"
	MsgComponentFocus      = "component_focus"

	// Bun→Go: request focus for a host-owned remote component (advisory).
	MsgComponentFocusRequest = "component_focus_request"
	// Bun→Go: remote component input handling outcome.
	MsgComponentInputResult = "component_input_result"

	// Terminal input bridge (extension onTerminalInput).
	// Host forwards raw key/paste/mouse only while subscription is active.
	MsgTerminalInputSubscription = "terminal_input_subscription"
	MsgTerminalInput             = "terminal_input"
	MsgTerminalInputResult       = "terminal_input_result"

	// Overlay mounting.
	MsgOverlayMount   = "overlay_mount"
	MsgOverlayUnmount = "overlay_unmount"
	MsgOverlayUpdate  = "overlay_update"
)

// RawPayload holds an opaque JSON object or array for forward-compatible fields.
// It round-trips unknown keys without loss.
type RawPayload = json.RawMessage

// Envelope is the outer wire frame for every protocol message.
//
// Unknown top-level fields beyond v/type/id/payload are preserved in Extra when
// decoding with [ParseEnvelope] and re-emitted on [Envelope.MarshalJSON].
type Envelope struct {
	// V is the protocol major version. Must equal [Major] for v1 peers.
	V int `json:"v"`

	// Type is the semantic message kind (see Msg* constants).
	Type string `json:"type"`

	// ID correlates request/response pairs. Empty when not applicable.
	// Serialized as "id" only when non-empty.
	ID string `json:"id,omitempty"`

	// Payload is the typed body as raw JSON. Nil means JSON null / omitted.
	// Prefer the typed helpers (e.g. [EncodePayload], [DecodePayload]) rather
	// than mutating this field by hand.
	Payload RawPayload `json:"payload,omitempty"`

	// Extra preserves unknown top-level JSON object keys for forward compat.
	// Not itself serialized as "extra"; merged into the object on marshal.
	Extra map[string]RawPayload `json:"-"`
}

// NewEnvelope builds an envelope with the current major version.
func NewEnvelope(typ, id string, payload any) (Envelope, error) {
	env := Envelope{
		V:    Major,
		Type: typ,
		ID:   id,
	}
	if payload == nil {
		return env, nil
	}
	raw, err := EncodePayload(payload)
	if err != nil {
		return Envelope{}, err
	}
	env.Payload = raw
	return env, nil
}

// MustEnvelope is like [NewEnvelope] but panics on marshal error.
// Intended for static test fixtures and package-level defaults.
func MustEnvelope(typ, id string, payload any) Envelope {
	env, err := NewEnvelope(typ, id, payload)
	if err != nil {
		panic(err)
	}
	return env
}

// EncodePayload marshals v to raw JSON. Nil returns nil.
func EncodePayload(v any) (RawPayload, error) {
	if v == nil {
		return nil, nil
	}
	if raw, ok := v.(RawPayload); ok {
		if len(raw) == 0 {
			return nil, nil
		}
		return append(RawPayload(nil), raw...), nil
	}
	if raw, ok := v.(json.RawMessage); ok {
		if len(raw) == 0 {
			return nil, nil
		}
		return append(RawPayload(nil), raw...), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	return RawPayload(b), nil
}

// DecodePayload unmarshals env.Payload into dest.
// Unknown fields in the payload are ignored by encoding/json (use
// [DecodePayloadStrict] or keep RawPayload when loss is unacceptable).
func DecodePayload(env Envelope, dest any) error {
	if len(env.Payload) == 0 || string(env.Payload) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(env.Payload))
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	return nil
}

// DecodePayloadStrict unmarshals env.Payload into dest and rejects unknown fields.
func DecodePayloadStrict(env Envelope, dest any) error {
	if len(env.Payload) == 0 || string(env.Payload) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(env.Payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	return nil
}

// ParseEnvelope decodes a single JSON object into an Envelope, preserving
// unknown top-level keys in Extra.
func ParseEnvelope(data []byte) (Envelope, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Envelope{}, fmt.Errorf("%w: empty", ErrInvalidEnvelope)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Envelope{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	var env Envelope
	if vRaw, ok := raw["v"]; ok {
		if err := json.Unmarshal(vRaw, &env.V); err != nil {
			return Envelope{}, fmt.Errorf("%w: bad v: %v", ErrInvalidEnvelope, err)
		}
		delete(raw, "v")
	} else {
		// Bare historical JSONL frames (no "v") are treated as major=0 until
		// a hello upgrades the session. Callers using enveloped v1 MUST set v.
		env.V = 0
	}
	if tRaw, ok := raw["type"]; ok {
		if err := json.Unmarshal(tRaw, &env.Type); err != nil {
			return Envelope{}, fmt.Errorf("%w: bad type: %v", ErrInvalidEnvelope, err)
		}
		delete(raw, "type")
	}
	if env.Type == "" {
		return Envelope{}, fmt.Errorf("%w: missing type", ErrInvalidEnvelope)
	}
	if idRaw, ok := raw["id"]; ok {
		if err := json.Unmarshal(idRaw, &env.ID); err != nil {
			return Envelope{}, fmt.Errorf("%w: bad id: %v", ErrInvalidEnvelope, err)
		}
		delete(raw, "id")
	}
	if pRaw, ok := raw["payload"]; ok {
		env.Payload = RawPayload(pRaw)
		delete(raw, "payload")
	}
	if len(raw) > 0 {
		env.Extra = make(map[string]RawPayload, len(raw))
		for k, v := range raw {
			env.Extra[k] = RawPayload(v)
		}
	}
	return env, nil
}

// MarshalJSON emits the envelope including Extra keys.
func (e Envelope) MarshalJSON() ([]byte, error) {
	// Fast path when no extra keys: structured marshal.
	if len(e.Extra) == 0 {
		type wire struct {
			V       int        `json:"v"`
			Type    string     `json:"type"`
			ID      string     `json:"id,omitempty"`
			Payload RawPayload `json:"payload,omitempty"`
		}
		return json.Marshal(wire{V: e.V, Type: e.Type, ID: e.ID, Payload: e.Payload})
	}
	m := make(map[string]json.RawMessage, 4+len(e.Extra))
	vb, err := json.Marshal(e.V)
	if err != nil {
		return nil, err
	}
	m["v"] = vb
	tb, err := json.Marshal(e.Type)
	if err != nil {
		return nil, err
	}
	m["type"] = tb
	if e.ID != "" {
		ib, err := json.Marshal(e.ID)
		if err != nil {
			return nil, err
		}
		m["id"] = ib
	}
	if len(e.Payload) > 0 {
		m["payload"] = json.RawMessage(e.Payload)
	}
	for k, v := range e.Extra {
		if k == "v" || k == "type" || k == "id" || k == "payload" {
			continue
		}
		m[k] = json.RawMessage(v)
	}
	return json.Marshal(m)
}

// Bytes returns the JSON encoding of the envelope.
func (e Envelope) Bytes() ([]byte, error) {
	return json.Marshal(e)
}

// ValidateBasic checks required envelope fields without inspecting payload.
func (e Envelope) ValidateBasic() error {
	if e.Type == "" {
		return fmt.Errorf("%w: missing type", ErrInvalidEnvelope)
	}
	return nil
}

// CheckMajor returns [ErrMajorMismatch] when e.V is non-zero and differs from
// [Major]. Historical bare frames (V==0) are allowed until hello.
func (e Envelope) CheckMajor() error {
	if e.V == 0 {
		return nil
	}
	if e.V != Major {
		return fmt.Errorf("%w: got %d want %d", ErrMajorMismatch, e.V, Major)
	}
	return nil
}

// WrapRPCCommand wraps a historical RpcCommand-shaped object (which already
// contains "type", optional "id", and command fields) as an enveloped message.
// The original object is placed in payload verbatim; the envelope id is copied
// from the command's id field when present.
func WrapRPCCommand(commandJSON RawPayload) (Envelope, error) {
	var head struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(commandJSON, &head); err != nil {
		return Envelope{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	if head.Type == "" {
		return Envelope{}, fmt.Errorf("%w: rpc command missing type", ErrInvalidEnvelope)
	}
	return Envelope{
		V:       Major,
		Type:    MsgRPCCommand,
		ID:      head.ID,
		Payload: append(RawPayload(nil), commandJSON...),
	}, nil
}

// WrapHistorical places a historical stdout frame (response, session event,
// extension_ui_request, host_tool_*, …) into an envelope.
//
// The frame's own "type" becomes the envelope type. The full original object
// is kept in payload so no field is lost. Optional envelope id is taken from
// the object's id when present.
func WrapHistorical(frameJSON RawPayload) (Envelope, error) {
	var head struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(frameJSON, &head); err != nil {
		return Envelope{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	if head.Type == "" {
		return Envelope{}, fmt.Errorf("%w: historical frame missing type", ErrInvalidEnvelope)
	}
	return Envelope{
		V:       Major,
		Type:    head.Type,
		ID:      head.ID,
		Payload: append(RawPayload(nil), frameJSON...),
	}, nil
}

// HistoricalPayload returns the raw historical object for envelopes whose
// payload IS the original Bun frame (rpc_command, response, session events,
// extension UI, host tool/uri). When payload is empty, falls back to rebuilding
// from Extra + type/id for bare JSONL passthrough decodes.
func (e Envelope) HistoricalPayload() RawPayload {
	if len(e.Payload) > 0 {
		return e.Payload
	}
	// Bare frame was parsed as envelope with fields in Extra.
	if len(e.Extra) == 0 {
		// Reconstruct minimal {"type":...} maybe with id.
		type mini struct {
			Type string `json:"type"`
			ID   string `json:"id,omitempty"`
		}
		b, err := json.Marshal(mini{Type: e.Type, ID: e.ID})
		if err != nil {
			return nil
		}
		return RawPayload(b)
	}
	m := make(map[string]json.RawMessage, len(e.Extra)+2)
	for k, v := range e.Extra {
		m[k] = json.RawMessage(v)
	}
	tb, _ := json.Marshal(e.Type)
	m["type"] = tb
	if e.ID != "" {
		ib, _ := json.Marshal(e.ID)
		m["id"] = ib
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return RawPayload(b)
}

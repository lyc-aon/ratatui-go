package protocol

import "encoding/json"

// ---------------------------------------------------------------------------
// Handshake / control
// ---------------------------------------------------------------------------

// Role identifies which side of the Bun↔Go link is speaking.
type Role string

const (
	// RoleFrontend is the Go TUI process (owns the TTY).
	RoleFrontend Role = "frontend"
	// RoleCore is the Bun agent/core process.
	RoleCore Role = "core"
)

// Capability flags advertised in hello. Unknown caps MUST be ignored.
const (
	CapJSONL            = "jsonl"             // speaks JSONL framing
	CapLengthPrefix     = "length_prefix"     // speaks length-prefixed framing
	CapRPC              = "rpc"               // full RpcCommand/Response surface
	CapSessionEvents    = "session_events"    // AgentSessionEvent stream
	CapExtensionUI      = "extension_ui"      // extension_ui_request/response
	CapHostTools        = "host_tools"        // host tool bridge
	CapHostURI          = "host_uri"          // host URI bridge
	CapRemoteComponents = "remote_components" // Category B component sessions
	CapEditorSync       = "editor_sync"       // Category C editor/status/theme
	CapOverlays         = "overlays"          // overlay mount/unmount
)

// HelloPayload is the body of [MsgHello].
// Both sides send hello after the stream opens; either may refuse on major mismatch.
type HelloPayload struct {
	// Protocol is a stable name, normally [ProtocolName].
	Protocol string `json:"protocol"`
	// Role is who is speaking.
	Role Role `json:"role"`
	// Major must match peer major ([Major]).
	Major int `json:"major"`
	// Minor is informative; peers may accept lower minors.
	Minor int `json:"minor"`
	// Caps lists supported optional features (see Cap* constants).
	Caps []string `json:"caps,omitempty"`
	// Process is an optional diagnostic label (argv0, version string).
	Process string `json:"process,omitempty"`
	// Extra preserves unknown hello fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON merges Extra into the hello object.
func (h HelloPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Protocol string   `json:"protocol"`
		Role     Role     `json:"role"`
		Major    int      `json:"major"`
		Minor    int      `json:"minor"`
		Caps     []string `json:"caps,omitempty"`
		Process  string   `json:"process,omitempty"`
	}
	return mergeExtra(wire{
		Protocol: h.Protocol,
		Role:     h.Role,
		Major:    h.Major,
		Minor:    h.Minor,
		Caps:     h.Caps,
		Process:  h.Process,
	}, h.Extra)
}

// UnmarshalJSON captures unknown fields into Extra.
func (h *HelloPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Protocol string   `json:"protocol"`
		Role     Role     `json:"role"`
		Major    int      `json:"major"`
		Minor    int      `json:"minor"`
		Caps     []string `json:"caps,omitempty"`
		Process  string   `json:"process,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "protocol", "role", "major", "minor", "caps", "process")
	if err != nil {
		return err
	}
	h.Protocol = w.Protocol
	h.Role = w.Role
	h.Major = w.Major
	h.Minor = w.Minor
	h.Caps = w.Caps
	h.Process = w.Process
	h.Extra = extra
	return nil
}

// NewHello builds a standard v1 hello for the given role and caps.
func NewHello(role Role, caps ...string) HelloPayload {
	if caps == nil {
		caps = DefaultCaps()
	}
	return HelloPayload{
		Protocol: ProtocolName,
		Role:     role,
		Major:    Major,
		Minor:    Minor,
		Caps:     append([]string(nil), caps...),
	}
}

// DefaultCaps is the full v1 capability set advertised by a complete peer.
func DefaultCaps() []string {
	return []string{
		CapJSONL,
		CapLengthPrefix,
		CapRPC,
		CapSessionEvents,
		CapExtensionUI,
		CapHostTools,
		CapHostURI,
		CapRemoteComponents,
		CapEditorSync,
		CapOverlays,
	}
}

// AcceptHello checks a peer hello against this process's major version.
// Returns [ErrMajorMismatch] when majors differ. Minor may differ.
func AcceptHello(peer HelloPayload) error {
	if peer.Major != Major {
		return errf("%w: peer major %d != %d", ErrMajorMismatch, peer.Major, Major)
	}
	return nil
}

// HasCap reports whether caps contains name.
func HasCap(caps []string, name string) bool {
	for _, c := range caps {
		if c == name {
			return true
		}
	}
	return false
}

// ShutdownPayload is the body of [MsgShutdown].
type ShutdownPayload struct {
	// Reason is a short machine-readable token (e.g. "user_quit", "peer_error").
	Reason string `json:"reason,omitempty"`
	// Message is optional human-readable detail.
	Message string `json:"message,omitempty"`
	// ExitCode is a suggested process exit code (0 = clean).
	ExitCode int `json:"exitCode,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (s ShutdownPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Reason   string `json:"reason,omitempty"`
		Message  string `json:"message,omitempty"`
		ExitCode int    `json:"exitCode,omitempty"`
	}
	return mergeExtra(wire{Reason: s.Reason, Message: s.Message, ExitCode: s.ExitCode}, s.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *ShutdownPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Reason   string `json:"reason,omitempty"`
		Message  string `json:"message,omitempty"`
		ExitCode int    `json:"exitCode,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "reason", "message", "exitCode")
	if err != nil {
		return err
	}
	*s = ShutdownPayload{Reason: w.Reason, Message: w.Message, ExitCode: w.ExitCode, Extra: extra}
	return nil
}

// ErrorPayload is the body of [MsgError] (protocol-level, not RpcResponse errors).
type ErrorPayload struct {
	// Code is a stable machine-readable error token.
	Code string `json:"code,omitempty"`
	// Message is human-readable detail.
	Message string `json:"message"`
	// Fatal hints that the connection should close.
	Fatal bool `json:"fatal,omitempty"`
	// RefID references a prior request id when applicable.
	RefID string `json:"refId,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (e ErrorPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message"`
		Fatal   bool   `json:"fatal,omitempty"`
		RefID   string `json:"refId,omitempty"`
	}
	return mergeExtra(wire{Code: e.Code, Message: e.Message, Fatal: e.Fatal, RefID: e.RefID}, e.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *ErrorPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message"`
		Fatal   bool   `json:"fatal,omitempty"`
		RefID   string `json:"refId,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "code", "message", "fatal", "refId")
	if err != nil {
		return err
	}
	*e = ErrorPayload{Code: w.Code, Message: w.Message, Fatal: w.Fatal, RefID: w.RefID, Extra: extra}
	return nil
}

// Common error codes for [ErrorPayload.Code].
const (
	ErrCodeMajorMismatch  = "major_mismatch"
	ErrCodeMalformedFrame = "malformed_frame"
	ErrCodeFrameTooLarge  = "frame_too_large"
	ErrCodeUnauthorized   = "unauthorized"
	ErrCodeInternal       = "internal"
	ErrCodeUnsupported    = "unsupported"
)

// ---------------------------------------------------------------------------
// Existing RPC surfaces — carried as raw JSON for lossless round-trip.
//
// Typed views below mirror rpc-types.ts field names so Go callers can work
// without depending on the Bun source. Unknown / future fields stay in the
// raw payload when using RawMessage helpers.
// ---------------------------------------------------------------------------

// RPCCommand is a host→core command. The wire object matches RpcCommand:
// it always has "type", optional "id", and command-specific fields.
//
// Prefer keeping the original bytes via [RawPayload] when proxying. This
// struct exposes the common discriminator fields; full command bodies are
// available through [RPCCommand.Raw] or by decoding into a map.
type RPCCommand struct {
	// ID correlates with RpcResponse.id.
	ID string `json:"id,omitempty"`
	// Type is the command name (prompt, abort, get_state, …).
	Type string `json:"type"`
	// Raw holds the full original JSON object including type/id and all fields.
	// When set, MarshalJSON emits Raw verbatim.
	Raw RawPayload `json:"-"`
	// Fields holds decoded command-specific keys (excluding type/id) when
	// unmarshaled from JSON without a Raw source. Used for construction.
	Fields map[string]any `json:"-"`
}

// MarshalJSON emits Raw when present; otherwise builds from Type/ID/Fields.
func (c RPCCommand) MarshalJSON() ([]byte, error) {
	if len(c.Raw) > 0 {
		return append([]byte(nil), c.Raw...), nil
	}
	m := make(map[string]any, 2+len(c.Fields))
	for k, v := range c.Fields {
		m[k] = v
	}
	m["type"] = c.Type
	if c.ID != "" {
		m["id"] = c.ID
	}
	return json.Marshal(m)
}

// UnmarshalJSON stores the full object in Raw and extracts type/id.
func (c *RPCCommand) UnmarshalJSON(data []byte) error {
	c.Raw = append(c.Raw[:0], data...)
	var head struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	c.ID = head.ID
	c.Type = head.Type
	var all map[string]any
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	delete(all, "id")
	delete(all, "type")
	if len(all) > 0 {
		c.Fields = all
	}
	return nil
}

// RPCResponse is a core→host response. Matches RpcResponse:
// {id?, type:"response", command, success, data?, error?}.
type RPCResponse struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"` // always "response"
	Command string          `json:"command"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
	// Raw is the full original object when unmarshaled.
	Raw RawPayload `json:"-"`
	// Extra unknown top-level keys.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw when set.
func (r RPCResponse) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type wire struct {
		ID      string          `json:"id,omitempty"`
		Type    string          `json:"type"`
		Command string          `json:"command"`
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data,omitempty"`
		Error   string          `json:"error,omitempty"`
	}
	t := r.Type
	if t == "" {
		t = MsgRPCResponse
	}
	return mergeExtra(wire{
		ID: r.ID, Type: t, Command: r.Command, Success: r.Success, Data: r.Data, Error: r.Error,
	}, r.Extra)
}

// UnmarshalJSON captures Raw and known fields.
func (r *RPCResponse) UnmarshalJSON(data []byte) error {
	r.Raw = append(r.Raw[:0], data...)
	type wire struct {
		ID      string          `json:"id,omitempty"`
		Type    string          `json:"type"`
		Command string          `json:"command"`
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data,omitempty"`
		Error   string          `json:"error,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "id", "type", "command", "success", "data", "error")
	if err != nil {
		return err
	}
	r.ID = w.ID
	r.Type = w.Type
	r.Command = w.Command
	r.Success = w.Success
	r.Data = w.Data
	r.Error = w.Error
	r.Extra = extra
	return nil
}

// SessionEvent is a core→host streamed event.
// Payload shape matches AgentSessionEvent | RpcSubagentFrame | prompt_result |
// available_commands_update. Kept as raw JSON for lossless transport; Type is
// extracted for routing.
type SessionEvent struct {
	// Type is the event discriminator (agent_start, message_update, …).
	Type string `json:"type"`
	// Raw is the full original event object.
	Raw RawPayload `json:"-"`
}

// MarshalJSON emits Raw when present.
func (e SessionEvent) MarshalJSON() ([]byte, error) {
	if len(e.Raw) > 0 {
		return append([]byte(nil), e.Raw...), nil
	}
	return json.Marshal(map[string]string{"type": e.Type})
}

// UnmarshalJSON stores Raw and extracts type.
func (e *SessionEvent) UnmarshalJSON(data []byte) error {
	e.Raw = append(e.Raw[:0], data...)
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	e.Type = head.Type
	return nil
}

// Known session / agent event type strings (routing aids; not exhaustive —
// unknown types MUST still be forwarded via Raw).
const (
	EventAgentStart             = "agent_start"
	EventAgentEnd               = "agent_end"
	EventTurnStart              = "turn_start"
	EventTurnEnd                = "turn_end"
	EventMessageStart           = "message_start"
	EventMessageUpdate          = "message_update"
	EventMessageEnd             = "message_end"
	EventToolExecutionStart     = "tool_execution_start"
	EventToolExecutionUpdate    = "tool_execution_update"
	EventToolExecutionEnd       = "tool_execution_end"
	EventAutoCompactionStart    = "auto_compaction_start"
	EventAutoCompactionEnd      = "auto_compaction_end"
	EventAutoRetryStart         = "auto_retry_start"
	EventAutoRetryEnd           = "auto_retry_end"
	EventRetryFallbackApplied   = "retry_fallback_applied"
	EventRetryFallbackSucceeded = "retry_fallback_succeeded"
	EventTtsrTriggered          = "ttsr_triggered"
	EventTodoReminder           = "todo_reminder"
	EventTodoAutoClear          = "todo_auto_clear"
	EventIRCMessage             = "irc_message"
	EventNotice                 = "notice"
	EventThinkingLevelChanged   = "thinking_level_changed"
	EventGoalUpdated            = "goal_updated"
)

// ExtensionUIRequest is core→host (extension needs UI).
// Matches RpcExtensionUIRequest; full object in Raw.
type ExtensionUIRequest struct {
	ID     string     `json:"id"`
	Type   string     `json:"type"` // "extension_ui_request"
	Method string     `json:"method"`
	Raw    RawPayload `json:"-"`
	// Common optional fields flattened for convenience (also in Raw).
	Title           string                `json:"title,omitempty"`
	Message         string                `json:"message,omitempty"`
	Options         json.RawMessage       `json:"options,omitempty"`
	Timeout         *float64              `json:"timeout,omitempty"`
	Placeholder     string                `json:"placeholder,omitempty"`
	Prefill         string                `json:"prefill,omitempty"`
	PromptStyle     *bool                 `json:"promptStyle,omitempty"`
	TargetID        string                `json:"targetId,omitempty"`
	NotifyType      string                `json:"notifyType,omitempty"`
	StatusKey       string                `json:"statusKey,omitempty"`
	StatusText      *string               `json:"statusText,omitempty"`
	WidgetKey       string                `json:"widgetKey,omitempty"`
	WidgetLines     json.RawMessage       `json:"widgetLines,omitempty"`
	WidgetPlacement string                `json:"widgetPlacement,omitempty"`
	Text            string                `json:"text,omitempty"`
	URL             string                `json:"url,omitempty"`
	Instructions    string                `json:"instructions,omitempty"`
	Extra           map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (r ExtensionUIRequest) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type wire struct {
		ID              string          `json:"id"`
		Type            string          `json:"type"`
		Method          string          `json:"method"`
		Title           string          `json:"title,omitempty"`
		Message         string          `json:"message,omitempty"`
		Options         json.RawMessage `json:"options,omitempty"`
		Timeout         *float64        `json:"timeout,omitempty"`
		Placeholder     string          `json:"placeholder,omitempty"`
		Prefill         string          `json:"prefill,omitempty"`
		PromptStyle     *bool           `json:"promptStyle,omitempty"`
		TargetID        string          `json:"targetId,omitempty"`
		NotifyType      string          `json:"notifyType,omitempty"`
		StatusKey       string          `json:"statusKey,omitempty"`
		StatusText      *string         `json:"statusText,omitempty"`
		WidgetKey       string          `json:"widgetKey,omitempty"`
		WidgetLines     json.RawMessage `json:"widgetLines,omitempty"`
		WidgetPlacement string          `json:"widgetPlacement,omitempty"`
		Text            string          `json:"text,omitempty"`
		URL             string          `json:"url,omitempty"`
		Instructions    string          `json:"instructions,omitempty"`
	}
	t := r.Type
	if t == "" {
		t = MsgExtensionUIRequest
	}
	return mergeExtra(wire{
		ID: r.ID, Type: t, Method: r.Method, Title: r.Title, Message: r.Message,
		Options: r.Options, Timeout: r.Timeout, Placeholder: r.Placeholder,
		Prefill: r.Prefill, PromptStyle: r.PromptStyle, TargetID: r.TargetID,
		NotifyType: r.NotifyType, StatusKey: r.StatusKey, StatusText: r.StatusText,
		WidgetKey: r.WidgetKey, WidgetLines: r.WidgetLines, WidgetPlacement: r.WidgetPlacement,
		Text: r.Text, URL: r.URL, Instructions: r.Instructions,
	}, r.Extra)
}

// UnmarshalJSON captures Raw.
func (r *ExtensionUIRequest) UnmarshalJSON(data []byte) error {
	r.Raw = append(r.Raw[:0], data...)
	type wire struct {
		ID              string          `json:"id"`
		Type            string          `json:"type"`
		Method          string          `json:"method"`
		Title           string          `json:"title,omitempty"`
		Message         string          `json:"message,omitempty"`
		Options         json.RawMessage `json:"options,omitempty"`
		Timeout         *float64        `json:"timeout,omitempty"`
		Placeholder     string          `json:"placeholder,omitempty"`
		Prefill         string          `json:"prefill,omitempty"`
		PromptStyle     *bool           `json:"promptStyle,omitempty"`
		TargetID        string          `json:"targetId,omitempty"`
		NotifyType      string          `json:"notifyType,omitempty"`
		StatusKey       string          `json:"statusKey,omitempty"`
		StatusText      *string         `json:"statusText,omitempty"`
		WidgetKey       string          `json:"widgetKey,omitempty"`
		WidgetLines     json.RawMessage `json:"widgetLines,omitempty"`
		WidgetPlacement string          `json:"widgetPlacement,omitempty"`
		Text            string          `json:"text,omitempty"`
		URL             string          `json:"url,omitempty"`
		Instructions    string          `json:"instructions,omitempty"`
	}
	var w wire
	known := []string{
		"id", "type", "method", "title", "message", "options", "timeout",
		"placeholder", "prefill", "promptStyle", "targetId", "notifyType",
		"statusKey", "statusText", "widgetKey", "widgetLines", "widgetPlacement",
		"text", "url", "instructions",
	}
	extra, err := splitExtra(data, &w, known...)
	if err != nil {
		return err
	}
	*r = ExtensionUIRequest{
		ID: w.ID, Type: w.Type, Method: w.Method, Raw: r.Raw,
		Title: w.Title, Message: w.Message, Options: w.Options, Timeout: w.Timeout,
		Placeholder: w.Placeholder, Prefill: w.Prefill, PromptStyle: w.PromptStyle,
		TargetID: w.TargetID, NotifyType: w.NotifyType, StatusKey: w.StatusKey,
		StatusText: w.StatusText, WidgetKey: w.WidgetKey, WidgetLines: w.WidgetLines,
		WidgetPlacement: w.WidgetPlacement, Text: w.Text, URL: w.URL,
		Instructions: w.Instructions, Extra: extra,
	}
	return nil
}

// Extension UI method names.
const (
	ExtUISelect        = "select"
	ExtUIConfirm       = "confirm"
	ExtUIInput         = "input"
	ExtUIEditor        = "editor"
	ExtUICancel        = "cancel"
	ExtUINotify        = "notify"
	ExtUISetStatus     = "setStatus"
	ExtUISetWidget     = "setWidget"
	ExtUISetTitle      = "setTitle"
	ExtUISetEditorText = "set_editor_text"
	ExtUIOpenURL       = "open_url"
)

// ExtensionUIResponse is host→core reply to an extension UI request.
type ExtensionUIResponse struct {
	Type      string                `json:"type"` // "extension_ui_response"
	ID        string                `json:"id"`
	Value     *string               `json:"value,omitempty"`
	Confirmed *bool                 `json:"confirmed,omitempty"`
	Cancelled *bool                 `json:"cancelled,omitempty"`
	TimedOut  *bool                 `json:"timedOut,omitempty"`
	Raw       RawPayload            `json:"-"`
	Extra     map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (r ExtensionUIResponse) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type wire struct {
		Type      string  `json:"type"`
		ID        string  `json:"id"`
		Value     *string `json:"value,omitempty"`
		Confirmed *bool   `json:"confirmed,omitempty"`
		Cancelled *bool   `json:"cancelled,omitempty"`
		TimedOut  *bool   `json:"timedOut,omitempty"`
	}
	t := r.Type
	if t == "" {
		t = MsgExtensionUIResponse
	}
	return mergeExtra(wire{
		Type: t, ID: r.ID, Value: r.Value, Confirmed: r.Confirmed,
		Cancelled: r.Cancelled, TimedOut: r.TimedOut,
	}, r.Extra)
}

// UnmarshalJSON captures Raw.
func (r *ExtensionUIResponse) UnmarshalJSON(data []byte) error {
	r.Raw = append(r.Raw[:0], data...)
	type wire struct {
		Type      string  `json:"type"`
		ID        string  `json:"id"`
		Value     *string `json:"value,omitempty"`
		Confirmed *bool   `json:"confirmed,omitempty"`
		Cancelled *bool   `json:"cancelled,omitempty"`
		TimedOut  *bool   `json:"timedOut,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "value", "confirmed", "cancelled", "timedOut")
	if err != nil {
		return err
	}
	*r = ExtensionUIResponse{
		Type: w.Type, ID: w.ID, Value: w.Value, Confirmed: w.Confirmed,
		Cancelled: w.Cancelled, TimedOut: w.TimedOut, Raw: r.Raw, Extra: extra,
	}
	return nil
}

// HostToolCall is core→host: execute a registered host tool.
type HostToolCall struct {
	Type       string                `json:"type"` // host_tool_call
	ID         string                `json:"id"`
	ToolCallID string                `json:"toolCallId"`
	ToolName   string                `json:"toolName"`
	Arguments  json.RawMessage       `json:"arguments"`
	Raw        RawPayload            `json:"-"`
	Extra      map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (h HostToolCall) MarshalJSON() ([]byte, error) {
	if len(h.Raw) > 0 {
		return append([]byte(nil), h.Raw...), nil
	}
	type wire struct {
		Type       string          `json:"type"`
		ID         string          `json:"id"`
		ToolCallID string          `json:"toolCallId"`
		ToolName   string          `json:"toolName"`
		Arguments  json.RawMessage `json:"arguments"`
	}
	t := h.Type
	if t == "" {
		t = MsgHostToolCall
	}
	return mergeExtra(wire{Type: t, ID: h.ID, ToolCallID: h.ToolCallID, ToolName: h.ToolName, Arguments: h.Arguments}, h.Extra)
}

// UnmarshalJSON captures Raw.
func (h *HostToolCall) UnmarshalJSON(data []byte) error {
	h.Raw = append(h.Raw[:0], data...)
	type wire struct {
		Type       string          `json:"type"`
		ID         string          `json:"id"`
		ToolCallID string          `json:"toolCallId"`
		ToolName   string          `json:"toolName"`
		Arguments  json.RawMessage `json:"arguments"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "toolCallId", "toolName", "arguments")
	if err != nil {
		return err
	}
	*h = HostToolCall{Type: w.Type, ID: w.ID, ToolCallID: w.ToolCallID, ToolName: w.ToolName, Arguments: w.Arguments, Raw: h.Raw, Extra: extra}
	return nil
}

// HostToolCancel is core→host: abort a pending host tool call.
type HostToolCancel struct {
	Type     string                `json:"type"` // host_tool_cancel
	ID       string                `json:"id"`
	TargetID string                `json:"targetId"`
	Raw      RawPayload            `json:"-"`
	Extra    map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (h HostToolCancel) MarshalJSON() ([]byte, error) {
	if len(h.Raw) > 0 {
		return append([]byte(nil), h.Raw...), nil
	}
	type wire struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		TargetID string `json:"targetId"`
	}
	t := h.Type
	if t == "" {
		t = MsgHostToolCancel
	}
	return mergeExtra(wire{Type: t, ID: h.ID, TargetID: h.TargetID}, h.Extra)
}

// UnmarshalJSON captures Raw.
func (h *HostToolCancel) UnmarshalJSON(data []byte) error {
	h.Raw = append(h.Raw[:0], data...)
	type wire struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		TargetID string `json:"targetId"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "targetId")
	if err != nil {
		return err
	}
	*h = HostToolCancel{Type: w.Type, ID: w.ID, TargetID: w.TargetID, Raw: h.Raw, Extra: extra}
	return nil
}

// HostToolUpdate is host→core: partial tool result stream.
type HostToolUpdate struct {
	Type          string                `json:"type"` // host_tool_update
	ID            string                `json:"id"`
	PartialResult json.RawMessage       `json:"partialResult"`
	Raw           RawPayload            `json:"-"`
	Extra         map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (h HostToolUpdate) MarshalJSON() ([]byte, error) {
	if len(h.Raw) > 0 {
		return append([]byte(nil), h.Raw...), nil
	}
	type wire struct {
		Type          string          `json:"type"`
		ID            string          `json:"id"`
		PartialResult json.RawMessage `json:"partialResult"`
	}
	t := h.Type
	if t == "" {
		t = MsgHostToolUpdate
	}
	return mergeExtra(wire{Type: t, ID: h.ID, PartialResult: h.PartialResult}, h.Extra)
}

// UnmarshalJSON captures Raw.
func (h *HostToolUpdate) UnmarshalJSON(data []byte) error {
	h.Raw = append(h.Raw[:0], data...)
	type wire struct {
		Type          string          `json:"type"`
		ID            string          `json:"id"`
		PartialResult json.RawMessage `json:"partialResult"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "partialResult")
	if err != nil {
		return err
	}
	*h = HostToolUpdate{Type: w.Type, ID: w.ID, PartialResult: w.PartialResult, Raw: h.Raw, Extra: extra}
	return nil
}

// HostToolResult is host→core: completed tool call.
type HostToolResult struct {
	Type    string                `json:"type"` // host_tool_result
	ID      string                `json:"id"`
	Result  json.RawMessage       `json:"result"`
	IsError bool                  `json:"isError,omitempty"`
	Raw     RawPayload            `json:"-"`
	Extra   map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (h HostToolResult) MarshalJSON() ([]byte, error) {
	if len(h.Raw) > 0 {
		return append([]byte(nil), h.Raw...), nil
	}
	type wire struct {
		Type    string          `json:"type"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		IsError bool            `json:"isError,omitempty"`
	}
	t := h.Type
	if t == "" {
		t = MsgHostToolResult
	}
	return mergeExtra(wire{Type: t, ID: h.ID, Result: h.Result, IsError: h.IsError}, h.Extra)
}

// UnmarshalJSON captures Raw.
func (h *HostToolResult) UnmarshalJSON(data []byte) error {
	h.Raw = append(h.Raw[:0], data...)
	type wire struct {
		Type    string          `json:"type"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		IsError bool            `json:"isError,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "result", "isError")
	if err != nil {
		return err
	}
	*h = HostToolResult{Type: w.Type, ID: w.ID, Result: w.Result, IsError: w.IsError, Raw: h.Raw, Extra: extra}
	return nil
}

// HostURIRequest is core→host: satisfy a URI read/write.
type HostURIRequest struct {
	Type      string                `json:"type"` // host_uri_request
	ID        string                `json:"id"`
	Operation string                `json:"operation"` // "read" | "write"
	URL       string                `json:"url"`
	Content   string                `json:"content,omitempty"`
	Raw       RawPayload            `json:"-"`
	Extra     map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (h HostURIRequest) MarshalJSON() ([]byte, error) {
	if len(h.Raw) > 0 {
		return append([]byte(nil), h.Raw...), nil
	}
	type wire struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Operation string `json:"operation"`
		URL       string `json:"url"`
		Content   string `json:"content,omitempty"`
	}
	t := h.Type
	if t == "" {
		t = MsgHostURIRequest
	}
	return mergeExtra(wire{Type: t, ID: h.ID, Operation: h.Operation, URL: h.URL, Content: h.Content}, h.Extra)
}

// UnmarshalJSON captures Raw.
func (h *HostURIRequest) UnmarshalJSON(data []byte) error {
	h.Raw = append(h.Raw[:0], data...)
	type wire struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Operation string `json:"operation"`
		URL       string `json:"url"`
		Content   string `json:"content,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "operation", "url", "content")
	if err != nil {
		return err
	}
	*h = HostURIRequest{Type: w.Type, ID: w.ID, Operation: w.Operation, URL: w.URL, Content: w.Content, Raw: h.Raw, Extra: extra}
	return nil
}

// HostURICancel is core→host: abort a pending URI request.
type HostURICancel struct {
	Type     string                `json:"type"` // host_uri_cancel
	ID       string                `json:"id"`
	TargetID string                `json:"targetId"`
	Raw      RawPayload            `json:"-"`
	Extra    map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (h HostURICancel) MarshalJSON() ([]byte, error) {
	if len(h.Raw) > 0 {
		return append([]byte(nil), h.Raw...), nil
	}
	type wire struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		TargetID string `json:"targetId"`
	}
	t := h.Type
	if t == "" {
		t = MsgHostURICancel
	}
	return mergeExtra(wire{Type: t, ID: h.ID, TargetID: h.TargetID}, h.Extra)
}

// UnmarshalJSON captures Raw.
func (h *HostURICancel) UnmarshalJSON(data []byte) error {
	h.Raw = append(h.Raw[:0], data...)
	type wire struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		TargetID string `json:"targetId"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "targetId")
	if err != nil {
		return err
	}
	*h = HostURICancel{Type: w.Type, ID: w.ID, TargetID: w.TargetID, Raw: h.Raw, Extra: extra}
	return nil
}

// HostURIResult is host→core: completed URI operation.
type HostURIResult struct {
	Type        string                `json:"type"` // host_uri_result
	ID          string                `json:"id"`
	Content     string                `json:"content,omitempty"`
	ContentType string                `json:"contentType,omitempty"`
	Notes       []string              `json:"notes,omitempty"`
	Immutable   *bool                 `json:"immutable,omitempty"`
	IsError     bool                  `json:"isError,omitempty"`
	Error       string                `json:"error,omitempty"`
	Raw         RawPayload            `json:"-"`
	Extra       map[string]RawPayload `json:"-"`
}

// MarshalJSON prefers Raw.
func (h HostURIResult) MarshalJSON() ([]byte, error) {
	if len(h.Raw) > 0 {
		return append([]byte(nil), h.Raw...), nil
	}
	type wire struct {
		Type        string   `json:"type"`
		ID          string   `json:"id"`
		Content     string   `json:"content,omitempty"`
		ContentType string   `json:"contentType,omitempty"`
		Notes       []string `json:"notes,omitempty"`
		Immutable   *bool    `json:"immutable,omitempty"`
		IsError     bool     `json:"isError,omitempty"`
		Error       string   `json:"error,omitempty"`
	}
	t := h.Type
	if t == "" {
		t = MsgHostURIResult
	}
	return mergeExtra(wire{
		Type: t, ID: h.ID, Content: h.Content, ContentType: h.ContentType,
		Notes: h.Notes, Immutable: h.Immutable, IsError: h.IsError, Error: h.Error,
	}, h.Extra)
}

// UnmarshalJSON captures Raw.
func (h *HostURIResult) UnmarshalJSON(data []byte) error {
	h.Raw = append(h.Raw[:0], data...)
	type wire struct {
		Type        string   `json:"type"`
		ID          string   `json:"id"`
		Content     string   `json:"content,omitempty"`
		ContentType string   `json:"contentType,omitempty"`
		Notes       []string `json:"notes,omitempty"`
		Immutable   *bool    `json:"immutable,omitempty"`
		IsError     bool     `json:"isError,omitempty"`
		Error       string   `json:"error,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "type", "id", "content", "contentType", "notes", "immutable", "isError", "error")
	if err != nil {
		return err
	}
	*h = HostURIResult{
		Type: w.Type, ID: w.ID, Content: w.Content, ContentType: w.ContentType,
		Notes: w.Notes, Immutable: w.Immutable, IsError: w.IsError, Error: w.Error,
		Raw: h.Raw, Extra: extra,
	}
	return nil
}

// ---------------------------------------------------------------------------
// Editor / status / theme sync (Category C)
// ---------------------------------------------------------------------------

// EditorStatePayload is a full editor snapshot (either direction).
type EditorStatePayload struct {
	// Text is the current editor buffer contents.
	Text string `json:"text"`
	// Cursor is an absolute UTF-16 code-unit offset, matching JavaScript string indices.
	Cursor int `json:"cursor,omitempty"`
	// SelectionEnd when != cursor indicates a selection range.
	SelectionEnd *int `json:"selectionEnd,omitempty"`
	// Placeholder shown when text is empty.
	Placeholder string `json:"placeholder,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (e EditorStatePayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Text         string `json:"text"`
		Cursor       int    `json:"cursor,omitempty"`
		SelectionEnd *int   `json:"selectionEnd,omitempty"`
		Placeholder  string `json:"placeholder,omitempty"`
	}
	return mergeExtra(wire{Text: e.Text, Cursor: e.Cursor, SelectionEnd: e.SelectionEnd, Placeholder: e.Placeholder}, e.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *EditorStatePayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Text         string `json:"text"`
		Cursor       int    `json:"cursor,omitempty"`
		SelectionEnd *int   `json:"selectionEnd,omitempty"`
		Placeholder  string `json:"placeholder,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "text", "cursor", "selectionEnd", "placeholder")
	if err != nil {
		return err
	}
	*e = EditorStatePayload{Text: w.Text, Cursor: w.Cursor, SelectionEnd: w.SelectionEnd, Placeholder: w.Placeholder, Extra: extra}
	return nil
}

// EditorUpdatePayload is a partial editor mutation request.
type EditorUpdatePayload struct {
	// Op selects the mutation kind.
	//   "set_text"   — replace buffer with Text
	//   "paste"      — insert Text at cursor / selection
	//   "clear"      — empty the buffer
	//   "set_cursor" — move cursor (and optional selection)
	Op string `json:"op"`
	// Text is used by set_text and paste.
	Text string `json:"text,omitempty"`
	// Cursor is used by set_cursor (and optional for set_text).
	Cursor *int `json:"cursor,omitempty"`
	// SelectionEnd optional selection anchor.
	SelectionEnd *int `json:"selectionEnd,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// Editor update op constants.
const (
	EditorOpSetText   = "set_text"
	EditorOpPaste     = "paste"
	EditorOpClear     = "clear"
	EditorOpSetCursor = "set_cursor"
)

// MarshalJSON implements json.Marshaler.
func (e EditorUpdatePayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Op           string `json:"op"`
		Text         string `json:"text,omitempty"`
		Cursor       *int   `json:"cursor,omitempty"`
		SelectionEnd *int   `json:"selectionEnd,omitempty"`
	}
	return mergeExtra(wire{Op: e.Op, Text: e.Text, Cursor: e.Cursor, SelectionEnd: e.SelectionEnd}, e.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *EditorUpdatePayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Op           string `json:"op"`
		Text         string `json:"text,omitempty"`
		Cursor       *int   `json:"cursor,omitempty"`
		SelectionEnd *int   `json:"selectionEnd,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "op", "text", "cursor", "selectionEnd")
	if err != nil {
		return err
	}
	*e = EditorUpdatePayload{Op: w.Op, Text: w.Text, Cursor: w.Cursor, SelectionEnd: w.SelectionEnd, Extra: extra}
	return nil
}

// EditorQueryPayload requests the current editor state (reply with editor_state).
type EditorQueryPayload struct {
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (e EditorQueryPayload) MarshalJSON() ([]byte, error) {
	if len(e.Extra) == 0 {
		return []byte("{}"), nil
	}
	return mergeExtra(struct{}{}, e.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *EditorQueryPayload) UnmarshalJSON(data []byte) error {
	extra, err := splitExtra(data, &struct{}{})
	if err != nil {
		return err
	}
	e.Extra = extra
	return nil
}

// StatusSyncPayload carries keyed status-line entries (setStatus parity).
type StatusSyncPayload struct {
	// Entries maps statusKey → status text. Null/omitted value clears the key
	// when Clear is true or when the value is explicitly JSON null in Raw form.
	Entries map[string]string `json:"entries,omitempty"`
	// ClearKeys lists keys to remove.
	ClearKeys []string `json:"clearKeys,omitempty"`
	// Replace when true means Entries is the full new map (drop unlisted keys).
	Replace bool `json:"replace,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (s StatusSyncPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Entries   map[string]string `json:"entries,omitempty"`
		ClearKeys []string          `json:"clearKeys,omitempty"`
		Replace   bool              `json:"replace,omitempty"`
	}
	return mergeExtra(wire{Entries: s.Entries, ClearKeys: s.ClearKeys, Replace: s.Replace}, s.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *StatusSyncPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Entries   map[string]string `json:"entries,omitempty"`
		ClearKeys []string          `json:"clearKeys,omitempty"`
		Replace   bool              `json:"replace,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "entries", "clearKeys", "replace")
	if err != nil {
		return err
	}
	*s = StatusSyncPayload{Entries: w.Entries, ClearKeys: w.ClearKeys, Replace: w.Replace, Extra: extra}
	return nil
}

// WorkingMessagePayload sets or clears the working/spinner message.
type WorkingMessagePayload struct {
	// Message is the text to show; empty + Clear clears it.
	Message string `json:"message,omitempty"`
	// Clear explicitly clears the working message.
	Clear bool `json:"clear,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (w WorkingMessagePayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Message string `json:"message,omitempty"`
		Clear   bool   `json:"clear,omitempty"`
	}
	return mergeExtra(wire{Message: w.Message, Clear: w.Clear}, w.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (w *WorkingMessagePayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Message string `json:"message,omitempty"`
		Clear   bool   `json:"clear,omitempty"`
	}
	var v wire
	extra, err := splitExtra(data, &v, "message", "clear")
	if err != nil {
		return err
	}
	*w = WorkingMessagePayload{Message: v.Message, Clear: v.Clear, Extra: extra}
	return nil
}

// ToolsExpandedPayload gets/sets whether tool cards are expanded.
type ToolsExpandedPayload struct {
	// Expanded is the desired or current state.
	Expanded bool `json:"expanded"`
	// Query when true is a get (reply with tools_expanded carrying Expanded).
	Query bool `json:"query,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (t ToolsExpandedPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Expanded bool `json:"expanded"`
		Query    bool `json:"query,omitempty"`
	}
	return mergeExtra(wire{Expanded: t.Expanded, Query: t.Query}, t.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *ToolsExpandedPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Expanded bool `json:"expanded"`
		Query    bool `json:"query,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "expanded", "query")
	if err != nil {
		return err
	}
	*t = ToolsExpandedPayload{Expanded: w.Expanded, Query: w.Query, Extra: extra}
	return nil
}

// ThemeSyncPalette is the optional semantic hex map on theme_sync.
// Fields match view.Palette; apply validates strict #RRGGBB.
type ThemeSyncPalette struct {
	Text     string `json:"text,omitempty"`
	Muted    string `json:"muted,omitempty"`
	Dim      string `json:"dim,omitempty"`
	Accent   string `json:"accent,omitempty"`
	Success  string `json:"success,omitempty"`
	Error    string `json:"error,omitempty"`
	Warning  string `json:"warning,omitempty"`
	Thinking string `json:"thinking,omitempty"`
	Code     string `json:"code,omitempty"`
	Border   string `json:"border,omitempty"`
	User     string `json:"user,omitempty"`
}

// ThemeSyncPayload carries the active theme name plus optional appearance/palette.
// Go owns rendering; Bun emits name + resolved semantic colors after setTheme/theme_query.
// Palette is a pointer so name-only peers omit it cleanly.
type ThemeSyncPayload struct {
	// Name is the theme identifier.
	Name string `json:"name"`
	// Appearance is "dark" or "light" when known.
	Appearance string `json:"appearance,omitempty"`
	// Palette is optional semantic hex colors (strict #RRGGBB on apply).
	Palette *ThemeSyncPalette `json:"palette,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (t ThemeSyncPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Name       string            `json:"name"`
		Appearance string            `json:"appearance,omitempty"`
		Palette    *ThemeSyncPalette `json:"palette,omitempty"`
	}
	return mergeExtra(wire{Name: t.Name, Appearance: t.Appearance, Palette: t.Palette}, t.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *ThemeSyncPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Name       string            `json:"name"`
		Appearance string            `json:"appearance,omitempty"`
		Palette    *ThemeSyncPalette `json:"palette,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "name", "appearance", "palette")
	if err != nil {
		return err
	}
	*t = ThemeSyncPayload{Name: w.Name, Appearance: w.Appearance, Palette: w.Palette, Extra: extra}
	return nil
}

// ThemeQueryPayload requests the current theme name (reply with theme_sync).
type ThemeQueryPayload struct {
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (t ThemeQueryPayload) MarshalJSON() ([]byte, error) {
	if len(t.Extra) == 0 {
		return []byte("{}"), nil
	}
	return mergeExtra(struct{}{}, t.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *ThemeQueryPayload) UnmarshalJSON(data []byte) error {
	extra, err := splitExtra(data, &struct{}{})
	if err != nil {
		return err
	}
	t.Extra = extra
	return nil
}

// ---------------------------------------------------------------------------
// Remote component sessions (Category B)
// ---------------------------------------------------------------------------

// ComponentSlot identifies where a remote component is mounted in the Go UI.
type ComponentSlot string

const (
	SlotOverlay     ComponentSlot = "overlay"
	SlotFooter      ComponentSlot = "footer"
	SlotHeader      ComponentSlot = "header"
	SlotEditor      ComponentSlot = "editor"
	SlotWidgetAbove ComponentSlot = "widget_above"
	SlotWidgetBelow ComponentSlot = "widget_below"
	SlotToolCall    ComponentSlot = "tool_call"
	SlotToolResult  ComponentSlot = "tool_result"
	SlotCustom      ComponentSlot = "custom"
)

// ComponentOpenPayload opens a remote component session on the Bun side.
// Bun retains the TS Component factory; Go routes width/input and paints rows.
type ComponentOpenPayload struct {
	// ComponentID is the session id for subsequent render/input/dispose.
	ComponentID string `json:"componentId"`
	// Slot is where Go intends to mount the component.
	Slot ComponentSlot `json:"slot,omitempty"`
	// Kind is an optional factory hint (e.g. "extension_custom", "tool_renderer").
	Kind string `json:"kind,omitempty"`
	// Key is an extension widget key or tool name when applicable.
	Key string `json:"key,omitempty"`
	// Props is an opaque JSON object passed to the factory.
	Props json.RawMessage `json:"props,omitempty"`
	// WantsKeyRelease hints that the component wants key-release events.
	WantsKeyRelease bool `json:"wantsKeyRelease,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentOpenPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID     string          `json:"componentId"`
		Slot            ComponentSlot   `json:"slot,omitempty"`
		Kind            string          `json:"kind,omitempty"`
		Key             string          `json:"key,omitempty"`
		Props           json.RawMessage `json:"props,omitempty"`
		WantsKeyRelease bool            `json:"wantsKeyRelease,omitempty"`
	}
	return mergeExtra(wire{
		ComponentID: c.ComponentID, Slot: c.Slot, Kind: c.Kind, Key: c.Key,
		Props: c.Props, WantsKeyRelease: c.WantsKeyRelease,
	}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentOpenPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID     string          `json:"componentId"`
		Slot            ComponentSlot   `json:"slot,omitempty"`
		Kind            string          `json:"kind,omitempty"`
		Key             string          `json:"key,omitempty"`
		Props           json.RawMessage `json:"props,omitempty"`
		WantsKeyRelease bool            `json:"wantsKeyRelease,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "componentId", "slot", "kind", "key", "props", "wantsKeyRelease")
	if err != nil {
		return err
	}
	*c = ComponentOpenPayload{
		ComponentID: w.ComponentID, Slot: w.Slot, Kind: w.Kind, Key: w.Key,
		Props: w.Props, WantsKeyRelease: w.WantsKeyRelease, Extra: extra,
	}
	return nil
}

// ComponentRenderPayload asks Bun to render a component at a given width.
type ComponentRenderPayload struct {
	ComponentID string `json:"componentId"`
	// Width is the cell width available for render(width).
	Width int `json:"width"`
	// Height is an optional height hint (not all components use it).
	Height int `json:"height,omitempty"`
	// TerminalWidth/Height are full terminal geometry for overlay visibility.
	TerminalWidth  int `json:"terminalWidth,omitempty"`
	TerminalHeight int `json:"terminalHeight,omitempty"`
	// Generation is a monotonic request id so late results can be dropped.
	Generation uint64 `json:"generation,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentRenderPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID    string `json:"componentId"`
		Width          int    `json:"width"`
		Height         int    `json:"height,omitempty"`
		TerminalWidth  int    `json:"terminalWidth,omitempty"`
		TerminalHeight int    `json:"terminalHeight,omitempty"`
		Generation     uint64 `json:"generation,omitempty"`
	}
	return mergeExtra(wire{
		ComponentID: c.ComponentID, Width: c.Width, Height: c.Height,
		TerminalWidth: c.TerminalWidth, TerminalHeight: c.TerminalHeight,
		Generation: c.Generation,
	}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentRenderPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID    string `json:"componentId"`
		Width          int    `json:"width"`
		Height         int    `json:"height,omitempty"`
		TerminalWidth  int    `json:"terminalWidth,omitempty"`
		TerminalHeight int    `json:"terminalHeight,omitempty"`
		Generation     uint64 `json:"generation,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w,
		"componentId", "width", "height", "terminalWidth", "terminalHeight", "generation",
	)
	if err != nil {
		return err
	}
	*c = ComponentRenderPayload{
		ComponentID: w.ComponentID,
		Width:       w.Width, Height: w.Height,
		TerminalWidth: w.TerminalWidth, TerminalHeight: w.TerminalHeight,
		Generation: w.Generation, Extra: extra,
	}
	return nil
}

// ComponentResultPayload is Bun→Go render output (ANSI rows).
type ComponentResultPayload struct {
	ComponentID string `json:"componentId"`
	// Generation echoes the render request generation.
	Generation uint64 `json:"generation,omitempty"`
	// Lines are the rendered ANSI rows (Component.render output).
	Lines []string `json:"lines"`
	// CursorCol/CursorRow optional cursor position within the component.
	CursorCol *int `json:"cursorCol,omitempty"`
	CursorRow *int `json:"cursorRow,omitempty"`
	// LiveRegionStart/CommitSafeEnd/SnapshotSafeEnd mirror native-scrollback
	// seams; -1 / omitted means unset.
	LiveRegionStart *int `json:"liveRegionStart,omitempty"`
	CommitSafeEnd   *int `json:"commitSafeEnd,omitempty"`
	SnapshotSafeEnd *int `json:"snapshotSafeEnd,omitempty"`
	// Error is set when render failed; Lines may be empty.
	Error string `json:"error,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentResultPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID     string   `json:"componentId"`
		Generation      uint64   `json:"generation,omitempty"`
		Lines           []string `json:"lines"`
		CursorCol       *int     `json:"cursorCol,omitempty"`
		CursorRow       *int     `json:"cursorRow,omitempty"`
		LiveRegionStart *int     `json:"liveRegionStart,omitempty"`
		CommitSafeEnd   *int     `json:"commitSafeEnd,omitempty"`
		SnapshotSafeEnd *int     `json:"snapshotSafeEnd,omitempty"`
		Error           string   `json:"error,omitempty"`
	}
	return mergeExtra(wire{
		ComponentID: c.ComponentID, Generation: c.Generation, Lines: c.Lines,
		CursorCol: c.CursorCol, CursorRow: c.CursorRow,
		LiveRegionStart: c.LiveRegionStart, CommitSafeEnd: c.CommitSafeEnd,
		SnapshotSafeEnd: c.SnapshotSafeEnd, Error: c.Error,
	}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentResultPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID     string   `json:"componentId"`
		Generation      uint64   `json:"generation,omitempty"`
		Lines           []string `json:"lines"`
		CursorCol       *int     `json:"cursorCol,omitempty"`
		CursorRow       *int     `json:"cursorRow,omitempty"`
		LiveRegionStart *int     `json:"liveRegionStart,omitempty"`
		CommitSafeEnd   *int     `json:"commitSafeEnd,omitempty"`
		SnapshotSafeEnd *int     `json:"snapshotSafeEnd,omitempty"`
		Error           string   `json:"error,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w,
		"componentId", "generation", "lines", "cursorCol", "cursorRow",
		"liveRegionStart", "commitSafeEnd", "snapshotSafeEnd", "error",
	)
	if err != nil {
		return err
	}
	*c = ComponentResultPayload{
		ComponentID: w.ComponentID, Generation: w.Generation, Lines: w.Lines,
		CursorCol: w.CursorCol, CursorRow: w.CursorRow,
		LiveRegionStart: w.LiveRegionStart, CommitSafeEnd: w.CommitSafeEnd,
		SnapshotSafeEnd: w.SnapshotSafeEnd, Error: w.Error, Extra: extra,
	}
	return nil
}

// ComponentInputPayload forwards raw terminal input bytes to a focused component.
type ComponentInputPayload struct {
	ComponentID string `json:"componentId"`
	// Data is the raw input bytes (key sequences, paste, …), base64 on the wire
	// when binary; for text-only peers UTF-8 string form is also accepted via Text.
	Data []byte `json:"data,omitempty"`
	// Text is an optional UTF-8 convenience form; when set and Data is empty,
	// receivers should treat Text as the input bytes.
	Text string `json:"text,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentInputPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID string `json:"componentId"`
		Data        []byte `json:"data,omitempty"`
		Text        string `json:"text,omitempty"`
	}
	return mergeExtra(wire{ComponentID: c.ComponentID, Data: c.Data, Text: c.Text}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentInputPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID string `json:"componentId"`
		Data        []byte `json:"data,omitempty"`
		Text        string `json:"text,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "componentId", "data", "text")
	if err != nil {
		return err
	}
	*c = ComponentInputPayload{ComponentID: w.ComponentID, Data: w.Data, Text: w.Text, Extra: extra}
	return nil
}

// InputBytes returns the effective input bytes (Data, or Text if Data empty).
func (c ComponentInputPayload) InputBytes() []byte {
	if len(c.Data) > 0 {
		return c.Data
	}
	if c.Text != "" {
		return []byte(c.Text)
	}
	return nil
}

// ComponentInvalidatePayload tells Go (or Bun) that a component's pixels are stale.
type ComponentInvalidatePayload struct {
	ComponentID string `json:"componentId"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentInvalidatePayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID string `json:"componentId"`
	}
	return mergeExtra(wire{ComponentID: c.ComponentID}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentInvalidatePayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID string `json:"componentId"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "componentId")
	if err != nil {
		return err
	}
	*c = ComponentInvalidatePayload{ComponentID: w.ComponentID, Extra: extra}
	return nil
}

// ComponentDisposePayload tears down a remote component session.
type ComponentDisposePayload struct {
	ComponentID string `json:"componentId"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentDisposePayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID string `json:"componentId"`
	}
	return mergeExtra(wire{ComponentID: c.ComponentID}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentDisposePayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID string `json:"componentId"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "componentId")
	if err != nil {
		return err
	}
	*c = ComponentDisposePayload{ComponentID: w.ComponentID, Extra: extra}
	return nil
}

// ComponentFocusPayload notifies focus gain/loss for a remote component.
type ComponentFocusPayload struct {
	ComponentID string `json:"componentId"`
	// Focused is true on focus-in, false on focus-out.
	Focused bool `json:"focused"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentFocusPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID string `json:"componentId"`
		Focused     bool   `json:"focused"`
	}
	return mergeExtra(wire{ComponentID: c.ComponentID, Focused: c.Focused}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentFocusPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID string `json:"componentId"`
		Focused     bool   `json:"focused"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "componentId", "focused")
	if err != nil {
		return err
	}
	*c = ComponentFocusPayload{ComponentID: w.ComponentID, Focused: w.Focused, Extra: extra}
	return nil
}

// ComponentInputResultPayload is Bun→Go input handling outcome for a remote component.
type ComponentInputResultPayload struct {
	// ID is an optional correlation id from the component_input request.
	ID string `json:"id,omitempty"`
	// ComponentID identifies the remote component session.
	ComponentID string `json:"componentId"`
	// Handled is true when the component consumed the input.
	Handled bool `json:"handled"`
	// Dirty is true when the component needs a fresh render.
	Dirty bool `json:"dirty"`
	// Error is an optional handler failure string.
	Error string `json:"error,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (c ComponentInputResultPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID          string `json:"id,omitempty"`
		ComponentID string `json:"componentId"`
		Handled     bool   `json:"handled"`
		Dirty       bool   `json:"dirty"`
		Error       string `json:"error,omitempty"`
	}
	return mergeExtra(wire{
		ID: c.ID, ComponentID: c.ComponentID, Handled: c.Handled, Dirty: c.Dirty, Error: c.Error,
	}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentInputResultPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ID          string `json:"id,omitempty"`
		ComponentID string `json:"componentId"`
		Handled     bool   `json:"handled"`
		Dirty       bool   `json:"dirty"`
		Error       string `json:"error,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "id", "componentId", "handled", "dirty", "error")
	if err != nil {
		return err
	}
	*c = ComponentInputResultPayload{
		ID: w.ID, ComponentID: w.ComponentID, Handled: w.Handled, Dirty: w.Dirty, Error: w.Error, Extra: extra,
	}
	return nil
}

// ComponentFocusRequestPayload is Bun→Go advisory focus for a host-owned remote.
// Host applies focus only when ComponentID is a currently owned Remote; it never
// replies or echoes this frame.
type ComponentFocusRequestPayload struct {
	// ComponentID is the remote session to focus or unfocus.
	ComponentID string `json:"componentId"`
	// Focused is the desired focus state. Nil means true (focus-in).
	Focused *bool `json:"focused,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// WantFocused returns the desired focus state (default true when omitted).
func (c ComponentFocusRequestPayload) WantFocused() bool {
	if c.Focused == nil {
		return true
	}
	return *c.Focused
}

// MarshalJSON implements json.Marshaler.
func (c ComponentFocusRequestPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ComponentID string `json:"componentId"`
		Focused     *bool  `json:"focused,omitempty"`
	}
	return mergeExtra(wire{ComponentID: c.ComponentID, Focused: c.Focused}, c.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *ComponentFocusRequestPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ComponentID string `json:"componentId"`
		Focused     *bool  `json:"focused,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "componentId", "focused")
	if err != nil {
		return err
	}
	*c = ComponentFocusRequestPayload{ComponentID: w.ComponentID, Focused: w.Focused, Extra: extra}
	return nil
}

// ---------------------------------------------------------------------------
// Terminal input bridge (extension onTerminalInput)
// ---------------------------------------------------------------------------

// Wire limits for the terminal-input bridge (host-side queue + payload cap).
const (
	// MaxTerminalInputBytes caps forwarded / result payload size (64 KiB).
	MaxTerminalInputBytes = 64 * 1024
	// MaxTerminalInputQueue is the host FIFO bound for inputs waiting on an in-flight result.
	MaxTerminalInputQueue = 256
)

// TerminalInputSubscriptionPayload is Bun→Go listener presence (0↔1 only).
type TerminalInputSubscriptionPayload struct {
	// Active is true when at least one onTerminalInput listener is registered.
	Active bool `json:"active"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (t TerminalInputSubscriptionPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		Active bool `json:"active"`
	}
	return mergeExtra(wire{Active: t.Active}, t.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *TerminalInputSubscriptionPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		Active bool `json:"active"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "active")
	if err != nil {
		return err
	}
	*t = TerminalInputSubscriptionPayload{Active: w.Active, Extra: extra}
	return nil
}

// TerminalInputPayload is Go→Bun raw terminal input for onTerminalInput listeners.
// Sent as bare additive JSON (not an RpcCommand response waiter). Data is
// base64-encoded on the wire via encoding/json []byte rules.
type TerminalInputPayload struct {
	// ID is the monotonic correlation id echoed on terminal_input_result.
	ID string `json:"id"`
	// Data is the raw input bytes (key / paste / mouse sequences).
	Data []byte `json:"data,omitempty"`
	// Text is an optional UTF-8 convenience form when Data is empty.
	Text string `json:"text,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (t TerminalInputPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID   string `json:"id"`
		Data []byte `json:"data,omitempty"`
		Text string `json:"text,omitempty"`
	}
	return mergeExtra(wire{ID: t.ID, Data: t.Data, Text: t.Text}, t.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *TerminalInputPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		ID   string `json:"id"`
		Data []byte `json:"data,omitempty"`
		Text string `json:"text,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "id", "data", "text")
	if err != nil {
		return err
	}
	*t = TerminalInputPayload{ID: w.ID, Data: w.Data, Text: w.Text, Extra: extra}
	return nil
}

// InputBytes returns Data, or Text bytes when Data is empty.
func (t TerminalInputPayload) InputBytes() []byte {
	if len(t.Data) > 0 {
		return t.Data
	}
	if t.Text != "" {
		return []byte(t.Text)
	}
	return nil
}

// TerminalInputResultPayload is Bun→Go outcome after running terminal-input listeners.
//
// Consume true drops the original host event. When Consume is false:
//   - omitted data → host routes the original event
//   - present data equal to string(original Raw) → original event
//   - present changed data → host re-decodes with a fresh input.Decoder Write+Flush
//
// Data is a RAW UTF-8 string on the Bun wire (rpc-types.ts), NOT base64.
// HasData distinguishes omitted from empty. Host rebuilds via []byte(Data).
type TerminalInputResultPayload struct {
	// ID echoes the terminal_input correlation id.
	ID string `json:"id"`
	// Consume stops the original event from reaching local routing.
	Consume bool `json:"consume"`
	// Data is the (possibly transformed) UTF-8 payload when not consumed.
	Data string `json:"data,omitempty"`
	// HasData is true when the peer included a "data" field (even if empty).
	HasData bool `json:"-"`
	// Error is the first isolated listener failure; consume/data remain authoritative.
	Error string `json:"error,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// DataBytes returns the result payload as bytes for local re-decode.
func (t TerminalInputResultPayload) DataBytes() []byte {
	if !t.HasData {
		return nil
	}
	return []byte(t.Data)
}

// MarshalJSON implements json.Marshaler.
func (t TerminalInputResultPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID      string `json:"id"`
		Consume bool   `json:"consume"`
		Data    string `json:"data,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	w := wire{ID: t.ID, Consume: t.Consume, Error: t.Error}
	if t.HasData {
		if t.Data != "" {
			w.Data = t.Data
			return mergeExtra(w, t.Extra)
		}
		// Empty string must still appear when HasData so tests can round-trip.
		base, err := mergeExtra(w, t.Extra)
		if err != nil {
			return nil, err
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(base, &m); err != nil {
			return nil, err
		}
		if m == nil {
			m = make(map[string]json.RawMessage)
		}
		m["data"] = json.RawMessage(`""`)
		return json.Marshal(m)
	}
	return mergeExtra(w, t.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
// Bun emits data as a raw UTF-8 JSON string (not base64).
func (t *TerminalInputResultPayload) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var w struct {
		ID      string `json:"id"`
		Consume bool   `json:"consume"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	out := TerminalInputResultPayload{ID: w.ID, Consume: w.Consume, Error: w.Error}
	if v, ok := raw["data"]; ok {
		out.HasData = true
		if len(v) > 0 && string(v) != "null" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			out.Data = s
		}
	}
	known := map[string]struct{}{"id": {}, "consume": {}, "data": {}, "error": {}, "type": {}, "v": {}}
	var extra map[string]RawPayload
	for k, v := range raw {
		if _, ok := known[k]; ok {
			continue
		}
		if extra == nil {
			extra = make(map[string]RawPayload)
		}
		extra[k] = RawPayload(v)
	}
	out.Extra = extra
	*t = out
	return nil
}

// ---------------------------------------------------------------------------
// Overlay mounting
// ---------------------------------------------------------------------------

// OverlayMountPayload mounts an overlay surface in the Go frontend.
// Optional sizing/position fields mirror TS OverlayOptions (number or "N%" string).
// Malformed/omitted values are ignored by the host and fall back to defaults.
type OverlayMountPayload struct {
	// OverlayID uniquely identifies the overlay instance.
	OverlayID string `json:"overlayId"`
	// ComponentID optional remote component backing the overlay.
	ComponentID string `json:"componentId,omitempty"`
	// Mode selects presentation: "modal", "alt_screen", "inline".
	Mode string `json:"mode,omitempty"`
	// Title optional chrome title.
	Title string `json:"title,omitempty"`
	// ZIndex stacking order (higher = front).
	ZIndex int `json:"zIndex,omitempty"`
	// Width is absolute cells (number) or percent string ("50%").
	Width json.RawMessage `json:"width,omitempty"`
	// MinWidth floors resolved width in cells.
	MinWidth *int `json:"minWidth,omitempty"`
	// MaxHeight is absolute rows (number) or percent string.
	MaxHeight json.RawMessage `json:"maxHeight,omitempty"`
	// Anchor is one of center/top-left/... (invalid → center).
	Anchor string `json:"anchor,omitempty"`
	// OffsetX/Y are cell offsets from the anchor.
	OffsetX *int `json:"offsetX,omitempty"`
	OffsetY *int `json:"offsetY,omitempty"`
	// Row/Col absolute or percent position overrides.
	Row json.RawMessage `json:"row,omitempty"`
	Col json.RawMessage `json:"col,omitempty"`
	// Margin is a uniform number or {top,right,bottom,left} object.
	Margin json.RawMessage `json:"margin,omitempty"`
	// Fullscreen borrows alt screen (also mode=alt_screen).
	Fullscreen *bool `json:"fullscreen,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// Overlay mode constants.
const (
	OverlayModeModal     = "modal"
	OverlayModeAltScreen = "alt_screen"
	OverlayModeInline    = "inline"
	// OverlayModeHidden is overlay_update-only: host SetHidden(true) without dispose.
	OverlayModeHidden = "hidden"
)

// MarshalJSON implements json.Marshaler.
func (o OverlayMountPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		OverlayID   string          `json:"overlayId"`
		ComponentID string          `json:"componentId,omitempty"`
		Mode        string          `json:"mode,omitempty"`
		Title       string          `json:"title,omitempty"`
		ZIndex      int             `json:"zIndex,omitempty"`
		Width       json.RawMessage `json:"width,omitempty"`
		MinWidth    *int            `json:"minWidth,omitempty"`
		MaxHeight   json.RawMessage `json:"maxHeight,omitempty"`
		Anchor      string          `json:"anchor,omitempty"`
		OffsetX     *int            `json:"offsetX,omitempty"`
		OffsetY     *int            `json:"offsetY,omitempty"`
		Row         json.RawMessage `json:"row,omitempty"`
		Col         json.RawMessage `json:"col,omitempty"`
		Margin      json.RawMessage `json:"margin,omitempty"`
		Fullscreen  *bool           `json:"fullscreen,omitempty"`
	}
	return mergeExtra(wire{
		OverlayID: o.OverlayID, ComponentID: o.ComponentID, Mode: o.Mode,
		Title: o.Title, ZIndex: o.ZIndex,
		Width: o.Width, MinWidth: o.MinWidth, MaxHeight: o.MaxHeight,
		Anchor: o.Anchor, OffsetX: o.OffsetX, OffsetY: o.OffsetY,
		Row: o.Row, Col: o.Col, Margin: o.Margin, Fullscreen: o.Fullscreen,
	}, o.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *OverlayMountPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		OverlayID   string          `json:"overlayId"`
		ComponentID string          `json:"componentId,omitempty"`
		Mode        string          `json:"mode,omitempty"`
		Title       string          `json:"title,omitempty"`
		ZIndex      int             `json:"zIndex,omitempty"`
		Width       json.RawMessage `json:"width,omitempty"`
		MinWidth    *int            `json:"minWidth,omitempty"`
		MaxHeight   json.RawMessage `json:"maxHeight,omitempty"`
		Anchor      string          `json:"anchor,omitempty"`
		OffsetX     *int            `json:"offsetX,omitempty"`
		OffsetY     *int            `json:"offsetY,omitempty"`
		Row         json.RawMessage `json:"row,omitempty"`
		Col         json.RawMessage `json:"col,omitempty"`
		Margin      json.RawMessage `json:"margin,omitempty"`
		Fullscreen  *bool           `json:"fullscreen,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w,
		"overlayId", "componentId", "mode", "title", "zIndex",
		"width", "minWidth", "maxHeight", "anchor", "offsetX", "offsetY",
		"row", "col", "margin", "fullscreen",
	)
	if err != nil {
		return err
	}
	*o = OverlayMountPayload{
		OverlayID: w.OverlayID, ComponentID: w.ComponentID, Mode: w.Mode,
		Title: w.Title, ZIndex: w.ZIndex,
		Width: w.Width, MinWidth: w.MinWidth, MaxHeight: w.MaxHeight,
		Anchor: w.Anchor, OffsetX: w.OffsetX, OffsetY: w.OffsetY,
		Row: w.Row, Col: w.Col, Margin: w.Margin, Fullscreen: w.Fullscreen,
		Extra: extra,
	}
	return nil
}

// OverlayUnmountPayload removes an overlay.
type OverlayUnmountPayload struct {
	OverlayID string `json:"overlayId"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (o OverlayUnmountPayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		OverlayID string `json:"overlayId"`
	}
	return mergeExtra(wire{OverlayID: o.OverlayID}, o.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *OverlayUnmountPayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		OverlayID string `json:"overlayId"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "overlayId")
	if err != nil {
		return err
	}
	*o = OverlayUnmountPayload{OverlayID: w.OverlayID, Extra: extra}
	return nil
}

// OverlayUpdatePayload patches overlay chrome / z-order without remounting.
type OverlayUpdatePayload struct {
	OverlayID string `json:"overlayId"`
	// Mode optionally changes overlay presentation (modal|alt_screen|inline|hidden).
	// mode=hidden maps to OverlayHandle.SetHidden(true); modal/alt_screen/inline unhide.
	Mode *string `json:"mode,omitempty"`
	// Title optionally patches overlay chrome title.
	Title *string `json:"title,omitempty"`
	// ZIndex optionally patches stacking order.
	ZIndex *int `json:"zIndex,omitempty"`
	// Extra preserves unknown fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (o OverlayUpdatePayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		OverlayID string  `json:"overlayId"`
		Mode      *string `json:"mode,omitempty"`
		Title     *string `json:"title,omitempty"`
		ZIndex    *int    `json:"zIndex,omitempty"`
	}
	return mergeExtra(wire{OverlayID: o.OverlayID, Mode: o.Mode, Title: o.Title, ZIndex: o.ZIndex}, o.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *OverlayUpdatePayload) UnmarshalJSON(data []byte) error {
	type wire struct {
		OverlayID string  `json:"overlayId"`
		Mode      *string `json:"mode,omitempty"`
		Title     *string `json:"title,omitempty"`
		ZIndex    *int    `json:"zIndex,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "overlayId", "mode", "title", "zIndex")
	if err != nil {
		return err
	}
	*o = OverlayUpdatePayload{OverlayID: w.OverlayID, Mode: w.Mode, Title: w.Title, ZIndex: w.ZIndex, Extra: extra}
	return nil
}

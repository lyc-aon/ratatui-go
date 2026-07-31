package protocol

import "encoding/json"

// SessionState mirrors RpcSessionState from rpc-types.ts.
// Used when decoding get_state response data. Unknown fields are preserved
// only when the caller keeps the original RawPayload; this struct is a typed view.
type SessionState struct {
	Model                 json.RawMessage `json:"model,omitempty"`
	ThinkingLevel         json.RawMessage `json:"thinkingLevel"` // may be null
	IsStreaming           bool            `json:"isStreaming"`
	IsCompacting          bool            `json:"isCompacting"`
	SteeringMode          string          `json:"steeringMode"`  // "all" | "one-at-a-time"
	FollowUpMode          string          `json:"followUpMode"`  // "all" | "one-at-a-time"
	InterruptMode         string          `json:"interruptMode"` // "immediate" | "wait"
	SessionFile           string          `json:"sessionFile,omitempty"`
	SessionID             string          `json:"sessionId"`
	SessionName           string          `json:"sessionName,omitempty"`
	AutoCompactionEnabled bool            `json:"autoCompactionEnabled"`
	MessageCount          int             `json:"messageCount"`
	QueuedMessageCount    int             `json:"queuedMessageCount"`
	TodoPhases            json.RawMessage `json:"todoPhases"`
	SystemPrompt          json.RawMessage `json:"systemPrompt,omitempty"`
	DumpTools             json.RawMessage `json:"dumpTools,omitempty"`
	ContextUsage          json.RawMessage `json:"contextUsage,omitempty"`
	// Extra preserves unknown session state fields.
	Extra map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (s SessionState) MarshalJSON() ([]byte, error) {
	type wire struct {
		Model                 json.RawMessage `json:"model,omitempty"`
		ThinkingLevel         json.RawMessage `json:"thinkingLevel"`
		IsStreaming           bool            `json:"isStreaming"`
		IsCompacting          bool            `json:"isCompacting"`
		SteeringMode          string          `json:"steeringMode"`
		FollowUpMode          string          `json:"followUpMode"`
		InterruptMode         string          `json:"interruptMode"`
		SessionFile           string          `json:"sessionFile,omitempty"`
		SessionID             string          `json:"sessionId"`
		SessionName           string          `json:"sessionName,omitempty"`
		AutoCompactionEnabled bool            `json:"autoCompactionEnabled"`
		MessageCount          int             `json:"messageCount"`
		QueuedMessageCount    int             `json:"queuedMessageCount"`
		TodoPhases            json.RawMessage `json:"todoPhases"`
		SystemPrompt          json.RawMessage `json:"systemPrompt,omitempty"`
		DumpTools             json.RawMessage `json:"dumpTools,omitempty"`
		ContextUsage          json.RawMessage `json:"contextUsage,omitempty"`
	}
	return mergeExtra(wire{
		Model: s.Model, ThinkingLevel: s.ThinkingLevel, IsStreaming: s.IsStreaming,
		IsCompacting: s.IsCompacting, SteeringMode: s.SteeringMode, FollowUpMode: s.FollowUpMode,
		InterruptMode: s.InterruptMode, SessionFile: s.SessionFile, SessionID: s.SessionID,
		SessionName: s.SessionName, AutoCompactionEnabled: s.AutoCompactionEnabled,
		MessageCount: s.MessageCount, QueuedMessageCount: s.QueuedMessageCount,
		TodoPhases: s.TodoPhases, SystemPrompt: s.SystemPrompt, DumpTools: s.DumpTools,
		ContextUsage: s.ContextUsage,
	}, s.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *SessionState) UnmarshalJSON(data []byte) error {
	type wire struct {
		Model                 json.RawMessage `json:"model,omitempty"`
		ThinkingLevel         json.RawMessage `json:"thinkingLevel"`
		IsStreaming           bool            `json:"isStreaming"`
		IsCompacting          bool            `json:"isCompacting"`
		SteeringMode          string          `json:"steeringMode"`
		FollowUpMode          string          `json:"followUpMode"`
		InterruptMode         string          `json:"interruptMode"`
		SessionFile           string          `json:"sessionFile,omitempty"`
		SessionID             string          `json:"sessionId"`
		SessionName           string          `json:"sessionName,omitempty"`
		AutoCompactionEnabled bool            `json:"autoCompactionEnabled"`
		MessageCount          int             `json:"messageCount"`
		QueuedMessageCount    int             `json:"queuedMessageCount"`
		TodoPhases            json.RawMessage `json:"todoPhases"`
		SystemPrompt          json.RawMessage `json:"systemPrompt,omitempty"`
		DumpTools             json.RawMessage `json:"dumpTools,omitempty"`
		ContextUsage          json.RawMessage `json:"contextUsage,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w,
		"model", "thinkingLevel", "isStreaming", "isCompacting", "steeringMode",
		"followUpMode", "interruptMode", "sessionFile", "sessionId", "sessionName",
		"autoCompactionEnabled", "messageCount", "queuedMessageCount", "todoPhases",
		"systemPrompt", "dumpTools", "contextUsage",
	)
	if err != nil {
		return err
	}
	*s = SessionState{
		Model: w.Model, ThinkingLevel: w.ThinkingLevel, IsStreaming: w.IsStreaming,
		IsCompacting: w.IsCompacting, SteeringMode: w.SteeringMode, FollowUpMode: w.FollowUpMode,
		InterruptMode: w.InterruptMode, SessionFile: w.SessionFile, SessionID: w.SessionID,
		SessionName: w.SessionName, AutoCompactionEnabled: w.AutoCompactionEnabled,
		MessageCount: w.MessageCount, QueuedMessageCount: w.QueuedMessageCount,
		TodoPhases: w.TodoPhases, SystemPrompt: w.SystemPrompt, DumpTools: w.DumpTools,
		ContextUsage: w.ContextUsage, Extra: extra,
	}
	return nil
}

// HostToolDefinition mirrors RpcHostToolDefinition.
type HostToolDefinition struct {
	Name        string          `json:"name"`
	Label       string          `json:"label,omitempty"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Hidden      bool            `json:"hidden,omitempty"`
	Extra       map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (h HostToolDefinition) MarshalJSON() ([]byte, error) {
	type wire struct {
		Name        string          `json:"name"`
		Label       string          `json:"label,omitempty"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
		Hidden      bool            `json:"hidden,omitempty"`
	}
	return mergeExtra(wire{
		Name: h.Name, Label: h.Label, Description: h.Description,
		Parameters: h.Parameters, Hidden: h.Hidden,
	}, h.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (h *HostToolDefinition) UnmarshalJSON(data []byte) error {
	type wire struct {
		Name        string          `json:"name"`
		Label       string          `json:"label,omitempty"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
		Hidden      bool            `json:"hidden,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "name", "label", "description", "parameters", "hidden")
	if err != nil {
		return err
	}
	*h = HostToolDefinition{
		Name: w.Name, Label: w.Label, Description: w.Description,
		Parameters: w.Parameters, Hidden: w.Hidden, Extra: extra,
	}
	return nil
}

// HostURISchemeDefinition mirrors RpcHostUriSchemeDefinition.
type HostURISchemeDefinition struct {
	Scheme      string `json:"scheme"`
	Description string `json:"description,omitempty"`
	Writable    bool   `json:"writable,omitempty"`
	Immutable   bool   `json:"immutable,omitempty"`
	Extra       map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (h HostURISchemeDefinition) MarshalJSON() ([]byte, error) {
	type wire struct {
		Scheme      string `json:"scheme"`
		Description string `json:"description,omitempty"`
		Writable    bool   `json:"writable,omitempty"`
		Immutable   bool   `json:"immutable,omitempty"`
	}
	return mergeExtra(wire{
		Scheme: h.Scheme, Description: h.Description, Writable: h.Writable, Immutable: h.Immutable,
	}, h.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (h *HostURISchemeDefinition) UnmarshalJSON(data []byte) error {
	type wire struct {
		Scheme      string `json:"scheme"`
		Description string `json:"description,omitempty"`
		Writable    bool   `json:"writable,omitempty"`
		Immutable   bool   `json:"immutable,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w, "scheme", "description", "writable", "immutable")
	if err != nil {
		return err
	}
	*h = HostURISchemeDefinition{
		Scheme: w.Scheme, Description: w.Description, Writable: w.Writable, Immutable: w.Immutable, Extra: extra,
	}
	return nil
}

// SubagentSnapshot mirrors RpcSubagentSnapshot; progress kept raw.
type SubagentSnapshot struct {
	ID               string          `json:"id"`
	Index            int             `json:"index"`
	Agent            string          `json:"agent"`
	AgentSource      json.RawMessage `json:"agentSource"`
	Description      string          `json:"description,omitempty"`
	Status           json.RawMessage `json:"status"`
	Task             string          `json:"task,omitempty"`
	Assignment       string          `json:"assignment,omitempty"`
	SessionFile      string          `json:"sessionFile,omitempty"`
	LastUpdate       float64         `json:"lastUpdate"`
	Progress         json.RawMessage `json:"progress,omitempty"`
	ParentToolCallID string          `json:"parentToolCallId,omitempty"`
	Extra            map[string]RawPayload `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (s SubagentSnapshot) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID               string          `json:"id"`
		Index            int             `json:"index"`
		Agent            string          `json:"agent"`
		AgentSource      json.RawMessage `json:"agentSource"`
		Description      string          `json:"description,omitempty"`
		Status           json.RawMessage `json:"status"`
		Task             string          `json:"task,omitempty"`
		Assignment       string          `json:"assignment,omitempty"`
		SessionFile      string          `json:"sessionFile,omitempty"`
		LastUpdate       float64         `json:"lastUpdate"`
		Progress         json.RawMessage `json:"progress,omitempty"`
		ParentToolCallID string          `json:"parentToolCallId,omitempty"`
	}
	return mergeExtra(wire{
		ID: s.ID, Index: s.Index, Agent: s.Agent, AgentSource: s.AgentSource,
		Description: s.Description, Status: s.Status, Task: s.Task, Assignment: s.Assignment,
		SessionFile: s.SessionFile, LastUpdate: s.LastUpdate, Progress: s.Progress,
		ParentToolCallID: s.ParentToolCallID,
	}, s.Extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *SubagentSnapshot) UnmarshalJSON(data []byte) error {
	type wire struct {
		ID               string          `json:"id"`
		Index            int             `json:"index"`
		Agent            string          `json:"agent"`
		AgentSource      json.RawMessage `json:"agentSource"`
		Description      string          `json:"description,omitempty"`
		Status           json.RawMessage `json:"status"`
		Task             string          `json:"task,omitempty"`
		Assignment       string          `json:"assignment,omitempty"`
		SessionFile      string          `json:"sessionFile,omitempty"`
		LastUpdate       float64         `json:"lastUpdate"`
		Progress         json.RawMessage `json:"progress,omitempty"`
		ParentToolCallID string          `json:"parentToolCallId,omitempty"`
	}
	var w wire
	extra, err := splitExtra(data, &w,
		"id", "index", "agent", "agentSource", "description", "status", "task",
		"assignment", "sessionFile", "lastUpdate", "progress", "parentToolCallId",
	)
	if err != nil {
		return err
	}
	*s = SubagentSnapshot{
		ID: w.ID, Index: w.Index, Agent: w.Agent, AgentSource: w.AgentSource,
		Description: w.Description, Status: w.Status, Task: w.Task, Assignment: w.Assignment,
		SessionFile: w.SessionFile, LastUpdate: w.LastUpdate, Progress: w.Progress,
		ParentToolCallID: w.ParentToolCallID, Extra: extra,
	}
	return nil
}

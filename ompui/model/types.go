// Package model owns the Go frontend's semantic view of the OMP RPC stream.
package model

import (
	"encoding/json"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/protocol"
)

// ContentKind identifies one message content block.
type ContentKind uint8

const (
	ContentUnknown ContentKind = iota
	ContentText
	ContentThinking
	ContentRedactedThinking
	ContentImage
	ContentToolCall
)

// ToolCall is an assistant-requested tool invocation.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
	Intent    string
	Raw       json.RawMessage
}

// ContentBlock is one lossless, renderable message block.
type ContentBlock struct {
	Kind     ContentKind
	Text     string
	Data     string
	MIMEType string
	ToolCall *ToolCall
	Raw      json.RawMessage
}

// Message is the frontend-safe view of an AgentMessage. Raw preserves custom
// message fields that the Go frontend does not understand yet.
type Message struct {
	Role       string
	Content    []ContentBlock
	Timestamp  int64
	Synthetic  bool
	ToolCallID string
	ToolName   string
	IsError    bool
	Provider   string
	Model      string
	StopReason string
	Error      string
	Streaming  bool
	Raw        json.RawMessage
}

// ToolExecution is live tool state independent of the eventual tool-result
// message. PartialResult and Result stay raw because tool details are extensible.
type ToolExecution struct {
	ID            string
	Name          string
	Arguments     json.RawMessage
	Intent        string
	PartialResult json.RawMessage
	Result        json.RawMessage
	IsError       bool
	Running       bool
	StartedAt     time.Time
	EndedAt       time.Time
}

// Notice is a user-visible core notice.
type Notice struct {
	Level   string
	Message string
	Source  string
}

// RetryState describes an active automatic retry.
type RetryState struct {
	Active       bool
	Attempt      int
	MaxAttempts  int
	Delay        time.Duration
	ErrorMessage string
}

// CompactionState describes automatic context compaction.
type CompactionState struct {
	Active bool
	Reason string
	Action string
}

// Status is transient frontend status derived from lifecycle events.
type Status struct {
	AgentRunning    bool
	TurnRunning     bool
	Streaming       bool
	Retry           RetryState
	Compaction      CompactionState
	ThinkingLevel   json.RawMessage
	ConfiguredThink json.RawMessage
	ResolvedThink   json.RawMessage
	Goal            json.RawMessage
	GoalState       json.RawMessage
	WorkingMessage  string
	StatusEntries   map[string]string
	ToolsExpanded   bool
	LastNotice      *Notice
	LastError       string
}

// AvailableCommand is a slash command advertised by the core.
type AvailableCommand struct {
	Name        string          `json:"name"`
	Aliases     []string        `json:"aliases,omitempty"`
	Description string          `json:"description,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	Subcommands json.RawMessage `json:"subcommands,omitempty"`
	Source      string          `json:"source,omitempty"`
}

// UnknownFrame preserves forward-compatible frames not yet consumed by State.
type UnknownFrame struct {
	Type string
	Raw  json.RawMessage
}

// Snapshot is an immutable caller-owned copy of State.
type Snapshot struct {
	Generation        uint64
	MessageGeneration uint64
	Messages          []Message
	Tools             []ToolExecution
	Session           protocol.SessionState
	Status            Status
	AvailableCommands []AvailableCommand
	TodoPhases        json.RawMessage
	Subagents         []json.RawMessage
	Unknown           []UnknownFrame
}

// ApplyResult describes what one frame changed. FirstChangedMessage is -1 when
// transcript rows are untouched.
type ApplyResult struct {
	Generation          uint64
	FirstChangedMessage int
	MessagesChanged     bool
	StatusChanged       bool
	SessionChanged      bool
	CommandsChanged     bool
	SubagentsChanged    bool
	Unknown             bool
}

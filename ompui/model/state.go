package model

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/michaelkelly/ratatui-go/ompui/protocol"
)

const maxUnknownFrames = 256

// State is the concurrency-safe semantic frontend state. Apply calls may come
// from the RPC reader while Snapshot is taken by the render loop.
type State struct {
	mu  sync.RWMutex
	now func() time.Time

	generation        uint64
	messageGeneration uint64
	messages          []Message
	activeMessage     int

	tools     map[string]*ToolExecution
	toolOrder []string

	session           protocol.SessionState
	status            Status
	availableCommands []AvailableCommand
	todoPhases        json.RawMessage
	subagents         []json.RawMessage
	unknown           []UnknownFrame
}

// NewState returns an empty model. The active-message sentinel is initialized.
func NewState() *State {
	return NewStateWithClock(time.Now)
}

// NewStateWithClock returns an empty model with an injected timestamp source.
// A nil clock falls back to time.Now.
func NewStateWithClock(now func() time.Time) *State {
	if now == nil {
		now = time.Now
	}
	return &State{
		now:           now,
		activeMessage: -1,
		tools:         make(map[string]*ToolExecution),
	}
}

func (s *State) currentTime() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

// Generation returns the latest semantic state generation.
func (s *State) Generation() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generation
}

// Snapshot returns a deep caller-owned copy safe after the next Apply.
func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := Snapshot{
		Generation:        s.generation,
		MessageGeneration: s.messageGeneration,
		Messages:          cloneMessages(s.messages),
		Session:           cloneSessionState(s.session),
		Status:            cloneStatus(s.status),
		AvailableCommands: cloneCommands(s.availableCommands),
		TodoPhases:        cloneRaw(s.todoPhases),
		Subagents:         cloneRawSlice(s.subagents),
		Unknown:           cloneUnknown(s.unknown),
	}
	if len(s.toolOrder) > 0 {
		out.Tools = make([]ToolExecution, 0, len(s.toolOrder))
		for _, id := range s.toolOrder {
			if tool := s.tools[id]; tool != nil {
				out.Tools = append(out.Tools, cloneTool(*tool))
			}
		}
	}
	return out
}

// ReplaceMessages atomically installs a full transcript snapshot.
func (s *State) ReplaceMessages(messages []Message) ApplyResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = cloneMessages(messages)
	s.activeMessage = -1
	s.messageGeneration++
	s.generation++
	return ApplyResult{
		Generation:          s.generation,
		FirstChangedMessage: 0,
		MessagesChanged:     true,
	}
}

func cloneMessages(in []Message) []Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]Message, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Raw = cloneRaw(in[i].Raw)
		if len(in[i].Content) > 0 {
			out[i].Content = make([]ContentBlock, len(in[i].Content))
			for j := range in[i].Content {
				out[i].Content[j] = in[i].Content[j]
				out[i].Content[j].Raw = cloneRaw(in[i].Content[j].Raw)
				if in[i].Content[j].ToolCall != nil {
					call := *in[i].Content[j].ToolCall
					call.Arguments = cloneRaw(call.Arguments)
					call.Raw = cloneRaw(call.Raw)
					out[i].Content[j].ToolCall = &call
				}
			}
		}
	}
	return out
}

func cloneSessionState(in protocol.SessionState) protocol.SessionState {
	out := in
	out.Model = cloneRaw(in.Model)
	out.ThinkingLevel = cloneRaw(in.ThinkingLevel)
	out.TodoPhases = cloneRaw(in.TodoPhases)
	out.SystemPrompt = cloneRaw(in.SystemPrompt)
	out.DumpTools = cloneRaw(in.DumpTools)
	out.ContextUsage = cloneRaw(in.ContextUsage)
	if len(in.Extra) > 0 {
		out.Extra = make(map[string]protocol.RawPayload, len(in.Extra))
		for key, raw := range in.Extra {
			out.Extra[key] = cloneRaw(raw)
		}
	}
	return out
}

func cloneStatus(in Status) Status {
	out := in
	out.ThinkingLevel = cloneRaw(in.ThinkingLevel)
	out.ConfiguredThink = cloneRaw(in.ConfiguredThink)
	out.ResolvedThink = cloneRaw(in.ResolvedThink)
	out.Goal = cloneRaw(in.Goal)
	out.GoalState = cloneRaw(in.GoalState)
	if len(in.StatusEntries) > 0 {
		out.StatusEntries = make(map[string]string, len(in.StatusEntries))
		for key, value := range in.StatusEntries {
			out.StatusEntries[key] = value
		}
	}
	if in.LastNotice != nil {
		notice := *in.LastNotice
		out.LastNotice = &notice
	}
	return out
}

func cloneCommands(in []AvailableCommand) []AvailableCommand {
	if len(in) == 0 {
		return nil
	}
	out := make([]AvailableCommand, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Aliases = append([]string(nil), in[i].Aliases...)
		out[i].Input = cloneRaw(in[i].Input)
		out[i].Subcommands = cloneRaw(in[i].Subcommands)
	}
	return out
}

func cloneTool(in ToolExecution) ToolExecution {
	out := in
	out.Arguments = cloneRaw(in.Arguments)
	out.PartialResult = cloneRaw(in.PartialResult)
	out.Result = cloneRaw(in.Result)
	return out
}

func cloneRawSlice(in []json.RawMessage) []json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]json.RawMessage, len(in))
	for i := range in {
		out[i] = cloneRaw(in[i])
	}
	return out
}

func cloneUnknown(in []UnknownFrame) []UnknownFrame {
	if len(in) == 0 {
		return nil
	}
	out := make([]UnknownFrame, len(in))
	for i := range in {
		out[i] = UnknownFrame{Type: in[i].Type, Raw: cloneRaw(in[i].Raw)}
	}
	return out
}

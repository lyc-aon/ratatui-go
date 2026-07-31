package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/protocol"
)

// Apply reduces one protocol envelope into State. Unknown frames are retained
// losslessly and reported, never rejected merely for being new.
func (s *State) Apply(env protocol.Envelope) (ApplyResult, error) {
	raw := cloneRaw(env.HistoricalPayload())
	typ := env.Type
	if env.V != 0 && typ == protocol.MsgSessionEvent {
		if len(env.Payload) == 0 {
			return ApplyResult{FirstChangedMessage: -1}, fmt.Errorf("model: empty session_event payload")
		}
		raw = cloneRaw(env.Payload)
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			return ApplyResult{FirstChangedMessage: -1}, fmt.Errorf("model: decode session event: %w", err)
		}
		typ = head.Type
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	result := ApplyResult{FirstChangedMessage: -1}
	changed, err := s.applyLocked(typ, raw, &result)
	if err != nil {
		return result, err
	}
	if changed {
		s.generation++
		result.Generation = s.generation
	} else {
		result.Generation = s.generation
	}
	return result, nil
}

func (s *State) applyLocked(typ string, raw json.RawMessage, result *ApplyResult) (bool, error) {
	switch typ {
	case protocol.MsgRPCResponse:
		return s.applyResponse(raw, result)
	case protocol.MsgAvailableCommandsUpdate:
		var frame struct {
			Commands []AvailableCommand `json:"commands"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: available commands: %w", err)
		}
		s.availableCommands = cloneCommands(frame.Commands)
		result.CommandsChanged = true
		return true, nil
	case protocol.MsgCommandOutput:
		var frame struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: command output: %w", err)
		}
		if frame.Text == "" {
			return false, nil
		}
		index := len(s.messages)
		s.messages = append(s.messages, Message{
			Role:      "assistant",
			Content:   []ContentBlock{{Kind: ContentText, Text: frame.Text}},
			Timestamp: s.currentTime().UnixMilli(),
			Synthetic: true,
			Raw:       cloneRaw(raw),
		})
		s.messageGeneration++
		result.FirstChangedMessage = index
		result.MessagesChanged = true
		return true, nil
	case protocol.EventMessageStart, protocol.EventMessageUpdate, protocol.EventMessageEnd:
		return s.applyMessageEvent(typ, raw, result)
	case protocol.EventToolExecutionStart, protocol.EventToolExecutionUpdate, protocol.EventToolExecutionEnd:
		return s.applyToolEvent(typ, raw, result)
	case protocol.EventAgentStart:
		s.status.AgentRunning = true
		result.StatusChanged = true
		return true, nil
	case protocol.EventAgentEnd:
		s.status.AgentRunning = false
		s.status.Streaming = false
		s.activeMessage = -1
		result.StatusChanged = true
		return true, nil
	case protocol.EventTurnStart:
		s.status.TurnRunning = true
		result.StatusChanged = true
		return true, nil
	case protocol.EventTurnEnd:
		s.status.TurnRunning = false
		result.StatusChanged = true
		return true, nil
	case protocol.EventAutoRetryStart:
		var frame struct {
			Attempt      int    `json:"attempt"`
			MaxAttempts  int    `json:"maxAttempts"`
			DelayMS      int64  `json:"delayMs"`
			ErrorMessage string `json:"errorMessage"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: retry start: %w", err)
		}
		s.status.Retry = RetryState{Active: true, Attempt: frame.Attempt, MaxAttempts: frame.MaxAttempts, Delay: time.Duration(frame.DelayMS) * time.Millisecond, ErrorMessage: frame.ErrorMessage}
		result.StatusChanged = true
		return true, nil
	case protocol.EventAutoRetryEnd:
		var frame struct {
			Success    bool   `json:"success"`
			FinalError string `json:"finalError"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: retry end: %w", err)
		}
		s.status.Retry.Active = false
		if !frame.Success {
			s.status.LastError = frame.FinalError
		}
		result.StatusChanged = true
		return true, nil
	case protocol.EventAutoCompactionStart:
		var frame struct {
			Reason string `json:"reason"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: compaction start: %w", err)
		}
		s.status.Compaction = CompactionState{Active: true, Reason: frame.Reason, Action: frame.Action}
		result.StatusChanged = true
		return true, nil
	case protocol.EventAutoCompactionEnd:
		var frame struct {
			ErrorMessage string `json:"errorMessage"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: compaction end: %w", err)
		}
		s.status.Compaction.Active = false
		if frame.ErrorMessage != "" {
			s.status.LastError = frame.ErrorMessage
		}
		result.StatusChanged = true
		return true, nil
	case protocol.EventNotice:
		var frame struct {
			Level   string `json:"level"`
			Message string `json:"message"`
			Source  string `json:"source"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: notice: %w", err)
		}
		notice := Notice{Level: frame.Level, Message: frame.Message, Source: frame.Source}
		s.status.LastNotice = &notice
		if frame.Level == "error" {
			s.status.LastError = frame.Message
		}
		result.StatusChanged = true
		return true, nil
	case protocol.EventThinkingLevelChanged:
		var frame struct {
			ThinkingLevel json.RawMessage `json:"thinkingLevel"`
			Configured    json.RawMessage `json:"configured"`
			Resolved      json.RawMessage `json:"resolved"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: thinking level: %w", err)
		}
		s.status.ThinkingLevel = cloneRaw(frame.ThinkingLevel)
		s.status.ConfiguredThink = cloneRaw(frame.Configured)
		s.status.ResolvedThink = cloneRaw(frame.Resolved)
		result.StatusChanged = true
		return true, nil
	case protocol.EventGoalUpdated:
		var frame struct {
			Goal  json.RawMessage `json:"goal"`
			State json.RawMessage `json:"state"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: goal: %w", err)
		}
		s.status.Goal = cloneRaw(frame.Goal)
		s.status.GoalState = cloneRaw(frame.State)
		result.StatusChanged = true
		return true, nil
	case protocol.EventTodoReminder:
		var frame struct {
			Todos json.RawMessage `json:"todos"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: todo reminder: %w", err)
		}
		s.todoPhases = cloneRaw(frame.Todos)
		result.StatusChanged = true
		return true, nil
	case protocol.EventTodoAutoClear:
		s.todoPhases = nil
		result.StatusChanged = true
		return true, nil
	case protocol.MsgSubagentLifecycle, protocol.MsgSubagentProgress, protocol.MsgSubagentEvent:
		s.upsertSubagent(raw)
		result.SubagentsChanged = true
		return true, nil
	case protocol.MsgStatusSync:
		var frame protocol.StatusSyncPayload
		if err := json.Unmarshal(raw, &frame); err != nil {
			return false, fmt.Errorf("model: status sync: %w", err)
		}
		if s.status.StatusEntries == nil {
			s.status.StatusEntries = make(map[string]string)
		}
		if frame.Replace {
			clear(s.status.StatusEntries)
		}
		for key, value := range frame.Entries {
			s.status.StatusEntries[key] = value
		}
		for _, key := range frame.ClearKeys {
			delete(s.status.StatusEntries, key)
		}
		result.StatusChanged = true
		return true, nil
	case protocol.MsgWorkingMessage:
		var frame struct {
			Message string `json:"message"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &frame)
		}
		s.status.WorkingMessage = frame.Message
		result.StatusChanged = true
		return true, nil
	case protocol.MsgToolsExpanded:
		var frame struct {
			Expanded bool `json:"expanded"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &frame)
		}
		s.status.ToolsExpanded = frame.Expanded
		result.StatusChanged = true
		return true, nil
	case protocol.MsgPromptResult, protocol.EventRetryFallbackApplied, protocol.EventRetryFallbackSucceeded, protocol.EventTtsrTriggered, protocol.EventIRCMessage:
		// These are user-visible event kinds but do not alter the core transcript
		// shape. Preserve them for a frontend feature component.
		s.appendUnknown(typ, raw)
		result.Unknown = true
		return true, nil
	default:
		s.appendUnknown(typ, raw)
		result.Unknown = true
		return true, nil
	}
}

func (s *State) applyResponse(raw json.RawMessage, result *ApplyResult) (bool, error) {
	var response protocol.RPCResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return false, fmt.Errorf("model: response: %w", err)
	}
	if !response.Success {
		s.status.LastError = response.Error
		result.StatusChanged = true
		return true, nil
	}
	switch response.Command {
	case protocol.CmdGetState:
		var state protocol.SessionState
		if err := json.Unmarshal(response.Data, &state); err != nil {
			return false, fmt.Errorf("model: get_state data: %w", err)
		}
		s.session = state
		s.status.Streaming = state.IsStreaming
		s.status.Compaction.Active = state.IsCompacting
		s.todoPhases = cloneRaw(state.TodoPhases)
		result.SessionChanged = true
		result.StatusChanged = true
		return true, nil
	case protocol.CmdGetMessages:
		var data struct {
			Messages json.RawMessage `json:"messages"`
		}
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return false, fmt.Errorf("model: get_messages data: %w", err)
		}
		messages, err := decodeMessageList(data.Messages)
		if err != nil {
			return false, err
		}
		s.messages = messages
		s.activeMessage = -1
		s.messageGeneration++
		result.FirstChangedMessage = 0
		result.MessagesChanged = true
		return true, nil
	case protocol.CmdGetAvailableCommands:
		var data struct {
			Commands []AvailableCommand `json:"commands"`
		}
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return false, fmt.Errorf("model: get_available_commands data: %w", err)
		}
		s.availableCommands = cloneCommands(data.Commands)
		result.CommandsChanged = true
		return true, nil
	default:
		return false, nil
	}
}

func (s *State) applyMessageEvent(typ string, raw json.RawMessage, result *ApplyResult) (bool, error) {
	var frame struct {
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		return false, fmt.Errorf("model: %s: %w", typ, err)
	}
	message, err := DecodeMessage(frame.Message)
	if err != nil {
		return false, err
	}
	message.Streaming = typ != protocol.EventMessageEnd && message.Role == "assistant"

	index := s.activeMessage
	if index < 0 || index >= len(s.messages) || !sameMessageIdentity(s.messages[index], message) {
		index = findMessage(s.messages, message)
	}

	switch typ {
	case protocol.EventMessageStart:
		if index >= 0 {
			s.messages[index] = message
		} else {
			index = len(s.messages)
			s.messages = append(s.messages, message)
		}
		s.activeMessage = index
	case protocol.EventMessageUpdate:
		if index < 0 {
			index = len(s.messages)
			s.messages = append(s.messages, message)
		} else if bytes.Equal(s.messages[index].Raw, message.Raw) && s.messages[index].Streaming == message.Streaming {
			return false, nil
		} else {
			s.messages[index] = message
		}
		s.activeMessage = index
		s.status.Streaming = true
		result.StatusChanged = true
	case protocol.EventMessageEnd:
		message.Streaming = false
		if index < 0 {
			index = len(s.messages)
			s.messages = append(s.messages, message)
		} else {
			s.messages[index] = message
		}
		s.activeMessage = -1
		s.status.Streaming = false
		result.StatusChanged = true
	}
	s.messageGeneration++
	result.FirstChangedMessage = index
	result.MessagesChanged = true
	return true, nil
}

func (s *State) applyToolEvent(typ string, raw json.RawMessage, result *ApplyResult) (bool, error) {
	var frame struct {
		ToolCallID    string          `json:"toolCallId"`
		ToolName      string          `json:"toolName"`
		Args          json.RawMessage `json:"args"`
		Intent        string          `json:"intent"`
		PartialResult json.RawMessage `json:"partialResult"`
		Result        json.RawMessage `json:"result"`
		IsError       bool            `json:"isError"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		return false, fmt.Errorf("model: %s: %w", typ, err)
	}
	if frame.ToolCallID == "" {
		return false, fmt.Errorf("model: %s missing toolCallId", typ)
	}
	tool := s.tools[frame.ToolCallID]
	if tool == nil {
		tool = &ToolExecution{ID: frame.ToolCallID}
		s.tools[frame.ToolCallID] = tool
		s.toolOrder = append(s.toolOrder, frame.ToolCallID)
	}
	if frame.ToolName != "" {
		tool.Name = frame.ToolName
	}
	switch typ {
	case protocol.EventToolExecutionStart:
		tool.Arguments = cloneRaw(frame.Args)
		tool.Intent = frame.Intent
		tool.Running = true
		tool.StartedAt = s.currentTime()
	case protocol.EventToolExecutionUpdate:
		tool.Arguments = cloneRaw(frame.Args)
		tool.PartialResult = cloneRaw(frame.PartialResult)
		tool.Running = true
	case protocol.EventToolExecutionEnd:
		tool.Result = cloneRaw(frame.Result)
		tool.IsError = frame.IsError
		tool.Running = false
		tool.EndedAt = s.currentTime()
	}
	result.StatusChanged = true
	return true, nil
}

func sameMessageIdentity(a, b Message) bool {
	if a.Role != b.Role {
		return false
	}
	if a.Timestamp != 0 && b.Timestamp != 0 {
		return a.Timestamp == b.Timestamp
	}
	if a.ToolCallID != "" || b.ToolCallID != "" {
		return a.ToolCallID == b.ToolCallID && a.ToolCallID != ""
	}
	return false
}

func findMessage(messages []Message, target Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if sameMessageIdentity(messages[i], target) {
			return i
		}
	}
	return -1
}

func (s *State) upsertSubagent(raw json.RawMessage) {
	var frame struct {
		Payload struct {
			ID         string `json:"id"`
			SubagentID string `json:"subagentId"`
		} `json:"payload"`
	}
	_ = json.Unmarshal(raw, &frame)
	key := frame.Payload.ID
	if key == "" {
		key = frame.Payload.SubagentID
	}
	if key != "" {
		for i := range s.subagents {
			var prior struct {
				Payload struct {
					ID         string `json:"id"`
					SubagentID string `json:"subagentId"`
				} `json:"payload"`
			}
			if json.Unmarshal(s.subagents[i], &prior) == nil && (prior.Payload.ID == key || prior.Payload.SubagentID == key) {
				s.subagents[i] = cloneRaw(raw)
				return
			}
		}
	}
	if len(s.subagents) >= maxUnknownFrames {
		copy(s.subagents, s.subagents[1:])
		s.subagents[len(s.subagents)-1] = cloneRaw(raw)
		return
	}
	s.subagents = append(s.subagents, cloneRaw(raw))
}

func (s *State) appendUnknown(typ string, raw json.RawMessage) {
	entry := UnknownFrame{Type: typ, Raw: cloneRaw(raw)}
	if len(s.unknown) >= maxUnknownFrames {
		copy(s.unknown, s.unknown[1:])
		s.unknown[len(s.unknown)-1] = entry
		return
	}
	s.unknown = append(s.unknown, entry)
}

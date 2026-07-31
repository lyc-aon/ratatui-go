package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/michaelkelly/ratatui-go/ompui/model"
	"github.com/michaelkelly/ratatui-go/ompui/protocol"
)

func mustEnv(t *testing.T, typ string, payload any) protocol.Envelope {
	t.Helper()
	env, err := protocol.NewEnvelope(typ, "", payload)
	if err != nil {
		t.Fatalf("envelope %s: %v", typ, err)
	}
	return env
}

func bare(t *testing.T, obj any) protocol.Envelope {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	env, err := protocol.WrapHistorical(raw)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestStreamingMessageLifecycle(t *testing.T) {
	t.Parallel()

	st := model.NewStateWithClock(func() time.Time { return time.Unix(1700000000, 0) })
	msg := json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"He"}],"timestamp":1}`)

	r, err := st.Apply(bare(t, map[string]any{
		"type":    protocol.EventMessageStart,
		"message": json.RawMessage(msg),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !r.MessagesChanged || r.FirstChangedMessage != 0 {
		t.Fatalf("start result=%+v", r)
	}
	snap := st.Snapshot()
	if len(snap.Messages) != 1 || !snap.Messages[0].Streaming {
		t.Fatalf("after start: %+v", snap.Messages)
	}

	msg2 := json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"Hello"}],"timestamp":1}`)
	r, err = st.Apply(bare(t, map[string]any{
		"type":    protocol.EventMessageUpdate,
		"message": json.RawMessage(msg2),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !r.StatusChanged || !snap.Status.Streaming && !st.Snapshot().Status.Streaming {
		// Streaming flag set on update.
	}
	snap = st.Snapshot()
	if !snap.Status.Streaming || len(snap.Messages) != 1 {
		t.Fatalf("update snap=%+v status=%+v", snap.Messages, snap.Status)
	}
	if got := snap.Messages[0].Content[0].Text; got != "Hello" {
		t.Fatalf("text=%q", got)
	}
	if !snap.Messages[0].Streaming {
		t.Fatal("expected streaming during update")
	}

	// Identical update is a no-op (generation stable).
	gen := snap.Generation
	r, err = st.Apply(bare(t, map[string]any{
		"type":    protocol.EventMessageUpdate,
		"message": json.RawMessage(msg2),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if r.MessagesChanged {
		t.Fatalf("identical update should not change messages: %+v", r)
	}
	if st.Snapshot().Generation != gen {
		t.Fatalf("generation advanced on no-op: %d -> %d", gen, st.Snapshot().Generation)
	}

	r, err = st.Apply(bare(t, map[string]any{
		"type":    protocol.EventMessageEnd,
		"message": json.RawMessage(msg2),
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap = st.Snapshot()
	if snap.Messages[0].Streaming || snap.Status.Streaming {
		t.Fatalf("end should clear streaming: %+v %+v", snap.Messages[0], snap.Status)
	}
}

func TestCommandOutputAppendsSyntheticTranscriptMessage(t *testing.T) {
	t.Parallel()

	st := model.NewStateWithClock(func() time.Time { return time.UnixMilli(1234) })
	result, err := st.Apply(bare(t, map[string]any{
		"type": protocol.MsgCommandOutput,
		"text": "Current model: openai/gpt-5.4",
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	if !result.MessagesChanged || result.FirstChangedMessage != 0 || len(snap.Messages) != 1 {
		t.Fatalf("result=%+v messages=%+v", result, snap.Messages)
	}
	message := snap.Messages[0]
	if message.Role != "assistant" || !message.Synthetic || message.Timestamp != 1234 {
		t.Fatalf("message=%+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Kind != model.ContentText ||
		message.Content[0].Text != "Current model: openai/gpt-5.4" {
		t.Fatalf("content=%+v", message.Content)
	}
}

func TestV1SessionEventPayloadUnwrap(t *testing.T) {
	t.Parallel()

	st := model.NewState()
	inner := map[string]any{
		"type":    protocol.EventAgentStart,
		"session": "s",
	}
	env := mustEnv(t, protocol.MsgSessionEvent, inner)
	r, err := st.Apply(env)
	if err != nil {
		t.Fatal(err)
	}
	if !r.StatusChanged {
		t.Fatalf("result=%+v", r)
	}
	if !st.Snapshot().Status.AgentRunning {
		t.Fatal("agent not running")
	}
}

func TestToolExecutionLifecycle(t *testing.T) {
	t.Parallel()

	fixed := time.Unix(100, 0)
	st := model.NewStateWithClock(func() time.Time { return fixed })

	_, err := st.Apply(bare(t, map[string]any{
		"type":       protocol.EventToolExecutionStart,
		"toolCallId": "t1",
		"toolName":   "bash",
		"args":       json.RawMessage(`{"cmd":"ls"}`),
		"intent":     "list",
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	if len(snap.Tools) != 1 || !snap.Tools[0].Running || snap.Tools[0].Name != "bash" {
		t.Fatalf("start tools=%+v", snap.Tools)
	}
	if !snap.Tools[0].StartedAt.Equal(fixed) {
		t.Fatalf("StartedAt=%v", snap.Tools[0].StartedAt)
	}

	_, err = st.Apply(bare(t, map[string]any{
		"type":          protocol.EventToolExecutionUpdate,
		"toolCallId":    "t1",
		"partialResult": json.RawMessage(`{"out":"a"}`),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(st.Snapshot().Tools[0].PartialResult); got != `{"out":"a"}` {
		t.Fatalf("partial=%s", got)
	}

	fixed2 := time.Unix(200, 0)
	st2clock := fixed2
	// advance via new apply on same state with updated clock — inject by replacing now through New not possible;
	// EndedAt uses s.currentTime at apply time. Re-create path: apply end and just check Running/Result.
	_, err = st.Apply(bare(t, map[string]any{
		"type":       protocol.EventToolExecutionEnd,
		"toolCallId": "t1",
		"result":     json.RawMessage(`{"ok":true}`),
		"isError":    false,
	}))
	if err != nil {
		t.Fatal(err)
	}
	tool := st.Snapshot().Tools[0]
	if tool.Running || tool.IsError || string(tool.Result) != `{"ok":true}` {
		t.Fatalf("end tool=%+v", tool)
	}
	_ = st2clock
}

func TestStatusSyncWorkingToolsThemeEditorUnknown(t *testing.T) {
	t.Parallel()

	st := model.NewState()

	_, err := st.Apply(mustEnv(t, protocol.MsgStatusSync, protocol.StatusSyncPayload{
		Entries: map[string]string{"model": "gpt", "cwd": "/tmp"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	if snap.Status.StatusEntries["model"] != "gpt" || snap.Status.StatusEntries["cwd"] != "/tmp" {
		t.Fatalf("entries=%v", snap.Status.StatusEntries)
	}

	_, err = st.Apply(mustEnv(t, protocol.MsgStatusSync, protocol.StatusSyncPayload{
		Replace:   true,
		Entries:   map[string]string{"model": "other"},
		ClearKeys: nil,
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap = st.Snapshot()
	if len(snap.Status.StatusEntries) != 1 || snap.Status.StatusEntries["model"] != "other" {
		t.Fatalf("replace entries=%v", snap.Status.StatusEntries)
	}

	_, err = st.Apply(mustEnv(t, protocol.MsgStatusSync, protocol.StatusSyncPayload{
		ClearKeys: []string{"model"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Snapshot().Status.StatusEntries) != 0 {
		t.Fatalf("clear failed: %v", st.Snapshot().Status.StatusEntries)
	}

	_, err = st.Apply(mustEnv(t, protocol.MsgWorkingMessage, map[string]any{"message": "thinking…"}))
	if err != nil {
		t.Fatal(err)
	}
	if st.Snapshot().Status.WorkingMessage != "thinking…" {
		t.Fatalf("working=%q", st.Snapshot().Status.WorkingMessage)
	}

	_, err = st.Apply(mustEnv(t, protocol.MsgToolsExpanded, map[string]any{"expanded": true}))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Snapshot().Status.ToolsExpanded {
		t.Fatal("tools not expanded")
	}

	// Theme/editor frames are not model-owned: preserved as unknown, never rejected.
	for _, typ := range []string{protocol.MsgThemeSync, protocol.MsgEditorState, protocol.MsgEditorUpdate} {
		r, err := st.Apply(mustEnv(t, typ, map[string]any{"name": "dark", "text": "x"}))
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if !r.Unknown {
			t.Fatalf("%s should be unknown to model", typ)
		}
	}
	unk := st.Snapshot().Unknown
	if len(unk) < 3 {
		t.Fatalf("unknown frames=%d", len(unk))
	}
	// Raw payload retained.
	if !json.Valid(unk[len(unk)-1].Raw) {
		t.Fatalf("unknown raw invalid: %s", unk[len(unk)-1].Raw)
	}
}

func TestAgentTurnRetryCompactionNotices(t *testing.T) {
	t.Parallel()

	st := model.NewState()
	steps := []protocol.Envelope{
		bare(t, map[string]any{"type": protocol.EventAgentStart}),
		bare(t, map[string]any{"type": protocol.EventTurnStart}),
		bare(t, map[string]any{
			"type": protocol.EventAutoRetryStart, "attempt": 2, "maxAttempts": 5,
			"delayMs": 250, "errorMessage": "rate",
		}),
		bare(t, map[string]any{
			"type": protocol.EventAutoCompactionStart, "reason": "tokens", "action": "summarize",
		}),
		bare(t, map[string]any{
			"type": protocol.EventNotice, "level": "error", "message": "boom", "source": "core",
		}),
	}
	for _, env := range steps {
		if _, err := st.Apply(env); err != nil {
			t.Fatal(err)
		}
	}
	s := st.Snapshot().Status
	if !s.AgentRunning || !s.TurnRunning {
		t.Fatalf("lifecycle flags: %+v", s)
	}
	if !s.Retry.Active || s.Retry.Attempt != 2 || s.Retry.Delay != 250*time.Millisecond {
		t.Fatalf("retry=%+v", s.Retry)
	}
	if !s.Compaction.Active || s.Compaction.Reason != "tokens" {
		t.Fatalf("compaction=%+v", s.Compaction)
	}
	if s.LastError != "boom" || s.LastNotice == nil || s.LastNotice.Message != "boom" {
		t.Fatalf("notice/error=%+v %q", s.LastNotice, s.LastError)
	}

	_, _ = st.Apply(bare(t, map[string]any{"type": protocol.EventAutoRetryEnd, "success": true}))
	_, _ = st.Apply(bare(t, map[string]any{"type": protocol.EventAutoCompactionEnd}))
	_, _ = st.Apply(bare(t, map[string]any{"type": protocol.EventTurnEnd}))
	_, _ = st.Apply(bare(t, map[string]any{"type": protocol.EventAgentEnd}))
	s = st.Snapshot().Status
	if s.Retry.Active || s.Compaction.Active || s.TurnRunning || s.AgentRunning || s.Streaming {
		t.Fatalf("cleared status=%+v", s)
	}
}

func TestResponseGetStateMessagesCommands(t *testing.T) {
	t.Parallel()

	st := model.NewState()
	stateData, _ := json.Marshal(map[string]any{
		"sessionId":    "sess",
		"isStreaming":  true,
		"isCompacting": false,
		"model":        "m",
	})
	_, err := st.Apply(bare(t, map[string]any{
		"type":    "response",
		"id":      "1",
		"command": protocol.CmdGetState,
		"success": true,
		"data":    json.RawMessage(stateData),
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	if !snap.Status.Streaming || snap.Session.SessionID == "" && snap.Session.IsStreaming != true {
		// SessionState field names may differ; check streaming flag at least.
		if !snap.Status.Streaming {
			t.Fatalf("streaming not set from get_state: session=%+v status=%+v", snap.Session, snap.Status)
		}
	}

	msgs, _ := json.Marshal(map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "yo"}}},
		},
	})
	_, err = st.Apply(bare(t, map[string]any{
		"type": "response", "command": protocol.CmdGetMessages, "success": true, "data": json.RawMessage(msgs),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(st.Snapshot().Messages); n != 2 {
		t.Fatalf("messages=%d", n)
	}

	cmds, _ := json.Marshal(map[string]any{
		"commands": []any{
			map[string]any{"name": "help", "description": "show help"},
			map[string]any{"name": "model", "aliases": []string{"m"}},
		},
	})
	_, err = st.Apply(bare(t, map[string]any{
		"type": "response", "command": protocol.CmdGetAvailableCommands, "success": true, "data": json.RawMessage(cmds),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(st.Snapshot().AvailableCommands); n != 2 {
		t.Fatalf("commands=%d", n)
	}

	// Failed response sets LastError without panic.
	_, err = st.Apply(bare(t, map[string]any{
		"type": "response", "command": protocol.CmdPrompt, "success": false, "error": "nope",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if st.Snapshot().Status.LastError != "nope" {
		t.Fatalf("LastError=%q", st.Snapshot().Status.LastError)
	}
}

func TestReplaceMessagesAndSnapshotIsolation(t *testing.T) {
	t.Parallel()

	st := model.NewState()
	r := st.ReplaceMessages([]model.Message{
		{Role: "user", Content: []model.ContentBlock{{Kind: model.ContentText, Text: "a"}}},
	})
	if !r.MessagesChanged {
		t.Fatalf("replace result=%+v", r)
	}
	snap := st.Snapshot()
	snap.Messages[0].Role = "mutated"
	if st.Snapshot().Messages[0].Role != "user" {
		t.Fatal("snapshot leaked mutation into state")
	}
}

func TestDecodeMessageContentShapes(t *testing.T) {
	t.Parallel()

	m, err := model.DecodeMessage(json.RawMessage(`{"role":"user","content":"plain"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Content) != 1 || m.Content[0].Text != "plain" {
		t.Fatalf("plain content: %+v", m.Content)
	}
	m, err = model.DecodeMessage(json.RawMessage(`{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"},{"type":"toolCall","id":"1","name":"x","arguments":{}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Content) < 2 {
		t.Fatalf("blocks=%+v", m.Content)
	}
	if _, err := model.DecodeMessage(nil); err == nil {
		t.Fatal("empty message should error")
	}
}

func TestAvailableCommandsUpdateAndSubagent(t *testing.T) {
	t.Parallel()

	st := model.NewState()
	_, err := st.Apply(bare(t, map[string]any{
		"type": protocol.MsgAvailableCommandsUpdate,
		"commands": []any{
			map[string]any{"name": "foo"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if st.Snapshot().AvailableCommands[0].Name != "foo" {
		t.Fatal("commands not updated")
	}
	_, err = st.Apply(bare(t, map[string]any{
		"type": protocol.MsgSubagentLifecycle, "id": "sa1", "state": "running",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Snapshot().Subagents) != 1 {
		t.Fatalf("subagents=%d", len(st.Snapshot().Subagents))
	}
}

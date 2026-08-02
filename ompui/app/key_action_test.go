package app

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
)

func TestActionKeyRegistryDefaultsAndMatching(t *testing.T) {
	t.Parallel()
	a := New(Config{})

	reg := a.KeyRegistry()
	if reg == nil {
		t.Fatal("KeyRegistry returned nil")
	}

	tests := []struct {
		key    event.Key
		action string
	}{
		{event.Key{ID: "ctrl+p"}, "app.model.cycleForward"},
		{event.Key{ID: "shift+ctrl+p"}, "app.model.cycleBackward"},
		{event.Key{ID: "ctrl+shift+p"}, "app.model.cycleBackward"},
		{event.Key{ID: "alt+m"}, "app.model.select"},
		{event.Key{ID: "alt+p"}, "app.model.selectTemporary"},
		{event.Key{ID: "ctrl+t"}, "app.thinking.toggle"},
		{event.Key{ID: "shift+tab"}, "app.thinking.cycle"},
		{event.Key{ID: "ctrl+r"}, "app.history.search"},
		{event.Key{ID: "alt+r"}, "app.retry"},
		{event.Key{ID: "alt+up"}, "app.message.dequeue"},
		{event.Key{ID: "ctrl+g"}, "app.editor.external"},
		{event.Key{ID: "ctrl+z"}, "app.suspend"},
		{event.Key{ID: "alt+a"}, "app.agents.hub"},
		{event.Key{ID: "ctrl+s"}, "app.session.observe"},
		{event.Key{ID: "alt+shift+p"}, "app.plan.toggle"},
		{event.Key{ID: "ctrl+v"}, "app.clipboard.pasteImage"},
		{event.Key{ID: "ctrl+shift+v"}, "app.clipboard.pasteTextRaw"},
		{event.Key{ID: "alt+shift+l"}, "app.clipboard.copyLine"},
		{event.Key{ID: "alt+shift+c"}, "app.clipboard.copyPrompt"},
	}

	for _, tc := range tests {
		if !reg.Matches(tc.key, tc.action) {
			t.Errorf("expected key %v to match action %s", tc.key, tc.action)
		}
	}
}

func TestCycleModelShortcutCommandsPreserveDirection(t *testing.T) {
	t.Parallel()
	forward := cycleModelRPCCommand("")
	if forward.Type != protocol.CmdCycleModel || len(forward.Fields) != 0 {
		t.Fatalf("forward command=%+v", forward)
	}
	backward := cycleModelRPCCommand("backward")
	if backward.Type != protocol.CmdCycleModel || backward.Fields["direction"] != "backward" {
		t.Fatalf("backward command=%+v", backward)
	}
}

func TestCustomUserKeyBindingsOverride(t *testing.T) {
	t.Parallel()
	cfg := Config{
		UserKeyBindings: map[string][]string{
			"app.suspend":        {"ctrl+x"},
			"app.history.search": {"ctrl+h"},
		},
	}
	a := New(cfg)
	reg := a.KeyRegistry()

	if !reg.Matches(event.Key{ID: "ctrl+x"}, "app.suspend") {
		t.Error("expected ctrl+x to match custom app.suspend")
	}
	if reg.Matches(event.Key{ID: "ctrl+z"}, "app.suspend") {
		t.Error("expected ctrl+z not to match overridden app.suspend")
	}
	if !reg.Matches(event.Key{ID: "ctrl+h"}, "app.history.search") {
		t.Error("expected ctrl+h to match custom app.history.search")
	}
}

func TestHandleGlobalKeyRouting(t *testing.T) {
	t.Parallel()
	a := New(Config{})
	a.width = 80
	a.height = 24
	a.buildUI()

	// Test history search overlay opens on ctrl+r
	a.ed.AddToHistory("hello world")
	ev := event.KeyEvent(event.Key{ID: "ctrl+r"}, []byte("\x12"))
	if !a.handleGlobalKey(ev) {
		t.Error("handleGlobalKey returned false for ctrl+r")
	}
	if !a.overlays.HasVisible(80, 24) {
		t.Error("expected history search overlay to be visible")
	}

	// Esc cancels overlay cleanly
	a.cancelTopOverlay(false)
	if a.overlays.HasVisible(80, 24) {
		t.Error("expected overlay to be hidden after cancel")
	}
}
func TestRestoreDequeuedMessagesPrependsTextAndRestoresImages(t *testing.T) {
	t.Parallel()
	a := New(Config{})
	a.width = 80
	a.height = 24
	a.buildUI()
	a.ed.SetText("current draft")

	a.restoreDequeuedMessages([]dequeuedMessage{
		{
			Text: "first queued",
			Images: []pendingPromptImage{{
				Type: "image", Data: "ZmFrZQ==", MIMEType: "image/png",
			}},
		},
		{Text: "second queued"},
	})

	if got, want := a.ed.Text(), "first queued\nsecond queued\ncurrent draft"; got != want {
		t.Fatalf("editor text=%q want %q", got, want)
	}
	if len(a.pendingPromptImages) != 1 {
		t.Fatalf("restored images=%+v", a.pendingPromptImages)
	}
	image, ok := a.pendingPromptImages[0].(pendingPromptImage)
	if !ok || image.MIMEType != "image/png" {
		t.Fatalf("restored image=%+v", a.pendingPromptImages[0])
	}
}

func TestDetectLeftDoubleTapRejectsBursts(t *testing.T) {
	t.Parallel()
	a := &App{}
	base := time.Unix(1_700_000_000, 0)
	if a.detectLeftDoubleTap(base) {
		t.Fatal("first tap must not trigger")
	}
	if a.detectLeftDoubleTap(base.Add(10 * time.Millisecond)) {
		t.Fatal("terminal-speed burst must not trigger")
	}
	if a.detectLeftDoubleTap(base.Add(100 * time.Millisecond)) {
		t.Fatal("third burst tap must not trigger")
	}
	if a.detectLeftDoubleTap(base.Add(time.Second)) {
		t.Fatal("first tap after a quiet gap must reset the sequence")
	}
	if !a.detectLeftDoubleTap(base.Add(100 * time.Millisecond).Add(time.Second)) {
		t.Fatal("human-speed second tap must trigger")
	}
}

func TestDecodeSubagentViewItemsKeepsSessionTarget(t *testing.T) {
	t.Parallel()
	items, err := decodeSubagentViewItems(client.Response{
		Command: protocol.CmdGetSubagents,
		Data: json.RawMessage(`{"subagents":[{
			"id":"sub-1",
			"agent":"worker",
			"status":"running",
			"task":"inspect",
			"sessionFile":"/tmp/sub-1.jsonl"
		}]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].sessionFile != "/tmp/sub-1.jsonl" {
		t.Fatalf("decoded items=%+v", items)
	}
}

func TestRPCDoneServerErrorRemainsVisible(t *testing.T) {
	t.Parallel()
	a := New(Config{})
	a.width = 80
	a.height = 24
	a.buildUI()

	// Simulated RPC error
	a.handleRPCDone(rpcDone{
		op:  "set_model",
		err: errors.New("model provider not reachable"),
	})

	if a.errBanner == nil {
		t.Fatal("errBanner is nil")
	}
	snap := a.state.Snapshot()
	if snap.Status.LastError == "" {
		t.Error("expected status error to be populated after RPC error")
	}
}

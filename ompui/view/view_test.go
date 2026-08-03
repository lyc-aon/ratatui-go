package view_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/model"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/view"
)

func strip(s string) string {
	var b strings.Builder
	for _, seg := range ansitext.ParseSegments(s) {
		if seg.Kind == "text" {
			b.WriteString(seg.Text)
		}
	}
	return b.String()
}

func assertNoNewlineOrOverflow(t *testing.T, lines []string, width int) {
	t.Helper()
	for i, line := range lines {
		if strings.Contains(line, "\n") {
			t.Fatalf("line %d contains newline: %q", i, line)
		}
		if w := ansitext.VisibleWidth(line); w > width {
			t.Fatalf("line %d overflow vw=%d width=%d: %q", i, w, width, line)
		}
	}
}

func textMsg(role, text string) model.Message {
	return model.Message{
		Role: role,
		Content: []model.ContentBlock{{
			Kind: model.ContentText,
			Text: text,
		}},
	}
}

func linesEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type staticComp struct {
	lines []string
	gen   component.Gen
}

func staticLines(lines []string) *staticComp { return &staticComp{lines: lines} }

func (s *staticComp) Render(width int) component.Frame {
	return component.NewFrame(s.lines, s.gen.Next())
}

func TestClassifyMessage(t *testing.T) {
	cases := map[string]view.MessageKind{
		"user":              view.KindUser,
		"developer":         view.KindUser,
		"assistant":         view.KindAssistant,
		"toolResult":        view.KindToolResult,
		"compactionSummary": view.KindSummary,
		"branchSummary":     view.KindSummary,
		"extension.foo":     view.KindCustom,
	}
	for role, want := range cases {
		got := view.ClassifyMessage(model.Message{Role: role})
		if got != want {
			t.Fatalf("role %q: got %v want %v", role, got, want)
		}
	}
}

func TestUserAndAssistantMessagesWidths(t *testing.T) {
	r := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true, MaxProseWidth: -1})
	user := textMsg("user", strings.Repeat("hello world ", 12))
	asst := textMsg("assistant", strings.Repeat("answer words ", 12))
	for _, width := range []int{30, 60, 100} {
		u := r.UserMessage(user, width)
		a := r.AssistantMessage(asst, width, 0)
		if len(u) == 0 || len(a) == 0 {
			t.Fatalf("width %d empty u=%d a=%d", width, len(u), len(a))
		}
		assertNoNewlineOrOverflow(t, u, width)
		assertNoNewlineOrOverflow(t, a, width)
		if !strings.Contains(strip(strings.Join(u, "")), "hello") {
			t.Fatalf("user lost text at %d", width)
		}
		if !strings.Contains(strip(strings.Join(a, "")), "answer") {
			t.Fatalf("asst lost text at %d", width)
		}
	}
}

func TestToolCardSettledAndRows(t *testing.T) {
	r := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true, ExpandHint: "ctrl+o"})
	call := &model.ToolCall{ID: "t1", Name: "bash", Intent: "list files", Arguments: json.RawMessage(`{"cmd":"ls"}`)}
	exec := &model.ToolExecution{ID: "t1", Name: "bash", Running: true, Intent: "list files"}
	card := view.ToolCardFrom(call, exec, nil)
	if card.Settled() {
		t.Fatal("running card settled")
	}
	for _, width := range []int{30, 60, 100} {
		rows := r.ToolRows(card, width)
		if len(rows) == 0 {
			t.Fatalf("empty tool rows at %d", width)
		}
		assertNoNewlineOrOverflow(t, rows, width)
		joined := strip(strings.Join(rows, "\n"))
		if !strings.Contains(joined, "bash") && !strings.Contains(joined, "list") {
			t.Fatalf("tool header missing at %d: %q", width, joined)
		}
	}

	result := &model.Message{
		Role:       "toolResult",
		ToolCallID: "t1",
		Content:    []model.ContentBlock{{Kind: model.ContentText, Text: "ok\nline2\nline3"}},
	}
	done := view.ToolCardFrom(call, &model.ToolExecution{ID: "t1", Name: "bash", Running: false}, result)
	if !done.HasResult {
		t.Fatal("result message should set HasResult")
	}
	if !done.Settled() {
		t.Fatal("completed card should be settled")
	}
	rows := r.ToolRows(done, 60)
	assertNoNewlineOrOverflow(t, rows, 60)
}

func TestTodoSortedAndBounded(t *testing.T) {
	raw := json.RawMessage(`[
		{"name":"phase","tasks":[
			{"content":"alpha","status":"completed"},
			{"content":"beta","status":"in_progress"},
			{"content":"gamma","status":"pending"}
		]}
	]`)
	items := view.ParseTodos(raw)
	if len(items) != 3 {
		t.Fatalf("items=%d", len(items))
	}
	if items[0].Content != "alpha" || items[1].Content != "beta" {
		t.Fatalf("order: %+v", items)
	}

	flat := view.ParseTodos(json.RawMessage(`[{"content":"one","status":"pending"},{"content":"two","status":"completed"}]`))
	if len(flat) != 2 {
		t.Fatalf("flat %d", len(flat))
	}
	if view.ParseTodos(json.RawMessage(`null`)) != nil {
		t.Fatal("null")
	}
	if view.ParseTodos(json.RawMessage(`{}`)) != nil {
		t.Fatal("object should nil")
	}

	r := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true})
	many := make([]view.TodoItem, 0, 20)
	for i := range 20 {
		st := view.TodoPending
		if i < 8 {
			st = view.TodoDone
		}
		many = append(many, view.TodoItem{Content: "task-" + strings.Repeat("x", i%3+1), Status: st})
	}
	for _, width := range []int{30, 60, 100} {
		rows := r.TodoRows(many, width)
		if len(rows) == 0 {
			t.Fatal("empty todos")
		}
		if len(rows) > 12 {
			t.Fatalf("todo rows unbounded: %d", len(rows))
		}
		assertNoNewlineOrOverflow(t, rows, width)
	}

	sum := view.NewTodoSummary(view.MonoTheme(), view.Options{Tight: true})
	sum.SetItems(items)
	f := sum.Render(60)
	if f.IsEmpty() {
		t.Fatal("todo component empty")
	}
	assertNoNewlineOrOverflow(t, f.Lines, 60)
}

func TestSubagentSortedByIndex(t *testing.T) {
	frames := []json.RawMessage{
		json.RawMessage(`{"payload":{"id":"b","agent":"worker","index":2,"status":"running","description":"second"}}`),
		json.RawMessage(`{"payload":{"id":"a","agent":"scout","index":1,"status":"done","description":"first"}}`),
		json.RawMessage(`{"payload":{"agent":"","description":"noid"}}`),
		json.RawMessage(`not-json`),
	}
	entries := view.ParseSubagents(frames)
	if len(entries) != 2 {
		t.Fatalf("entries=%d %+v", len(entries), entries)
	}
	if entries[0].ID != "a" || entries[1].ID != "b" {
		t.Fatalf("not sorted by index: %+v", entries)
	}

	r := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true})
	big := make([]view.SubagentEntry, 0, 12)
	for i := range 12 {
		big = append(big, view.SubagentEntry{ID: "id", Agent: "ag", Task: "work", Status: "running", Index: i})
	}
	for _, width := range []int{30, 60, 100} {
		rows := r.SubagentRows(big, width)
		if len(rows) == 0 {
			t.Fatal("empty subagents")
		}
		if len(rows) > 10 {
			t.Fatalf("subagent rows unbounded: %d", len(rows))
		}
		assertNoNewlineOrOverflow(t, rows, width)
	}
}

func TestStatusEntriesSortedStable(t *testing.T) {
	r := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true})
	snap := model.Snapshot{
		Session: protocol.SessionState{
			Model:        json.RawMessage(`{"id":"gpt-test","provider":"test"}`),
			ContextUsage: json.RawMessage(`{"tokens":1000,"contextWindow":8000,"percent":12.5}`),
		},
		Status: model.Status{
			StatusEntries: map[string]string{
				"zeta":  "Z-val",
				"alpha": "A-val",
				"mu":    "M-val",
				"empty": "  ",
			},
		},
	}
	info := view.FooterInfo{Path: "/tmp/proj", Branch: "main"}
	var prev string
	for range 5 {
		rows := r.FooterRows(snap, info, 100)
		joined := strip(strings.Join(rows, "\n"))
		if prev == "" {
			prev = joined
		} else if joined != prev {
			t.Fatalf("footer not stable:\n%s\nvs\n%s", prev, joined)
		}
		ai := strings.Index(joined, "A-val")
		mi := strings.Index(joined, "M-val")
		zi := strings.Index(joined, "Z-val")
		if ai < 0 || mi < 0 || zi < 0 || !(ai < mi && mi < zi) {
			t.Fatalf("status entries not key-sorted: %q (a=%d m=%d z=%d)", joined, ai, mi, zi)
		}
		if strings.Contains(joined, "empty") {
			t.Fatal("blank status entry should drop")
		}
	}
	for _, width := range []int{30, 60, 100} {
		rows := r.FooterRows(snap, info, width)
		assertNoNewlineOrOverflow(t, rows, width)
		sl := r.StatusLineRows(snap, width)
		assertNoNewlineOrOverflow(t, sl, width)
	}
}

func TestImageAdapterIdentity(t *testing.T) {
	var seen []view.ImageRequest
	adapter := func(req view.ImageRequest) component.Component {
		seen = append(seen, req)
		return staticLines([]string{"IMG:" + req.Key})
	}
	r := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true, ImageAdapter: adapter})
	req := view.ImageRequest{Key: "m0:0", Base64: "AAAA", MIMEType: "image/png", Filename: "a.png"}
	rows := r.ImageRows(req, 40)
	if len(seen) != 1 || seen[0].Key != "m0:0" {
		t.Fatalf("adapter identity: %#v", seen)
	}
	if len(rows) != 1 || rows[0] != "IMG:m0:0" {
		t.Fatalf("adapter rows: %#v", rows)
	}

	r2 := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true})
	fb := r2.ImageRows(view.ImageRequest{
		MIMEType: "image/png",
		Base64:   "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg==",
	}, 60)
	if len(fb) == 0 {
		t.Fatal("fallback empty")
	}
	joined := strip(strings.Join(fb, ""))
	if !strings.Contains(strings.ToLower(joined), "image") {
		t.Fatalf("fallback label: %q", joined)
	}

	r3 := view.NewRenderer(view.MonoTheme(), view.Options{
		Tight:        true,
		ImageAdapter: func(req view.ImageRequest) component.Component { return nil },
	})
	fb2 := r3.ImageRows(view.ImageRequest{MIMEType: "image/jpeg"}, 40)
	if len(fb2) == 0 {
		t.Fatal("nil adapter should fallback")
	}
}

func TestCacheGenerationAndPointerReuse(t *testing.T) {
	todo := view.NewTodoSummary(view.MonoTheme(), view.Options{Tight: true})
	todo.SetItems([]view.TodoItem{{Content: "only", Status: view.TodoPending}})
	f1 := todo.Render(50)
	f2 := todo.Render(50)
	if f1.Generation != f2.Generation {
		t.Fatalf("unchanged content should keep generation: %d vs %d", f1.Generation, f2.Generation)
	}
	if len(f1.Lines) == 0 {
		t.Fatal("empty")
	}
	if unsafe.SliceData(f1.Lines) != unsafe.SliceData(f2.Lines) {
		t.Fatal("expected Lines pointer reuse on cache hit")
	}
	todo.SetItems([]view.TodoItem{{Content: "changed", Status: view.TodoActive}})
	f3 := todo.Render(50)
	if f3.Generation == f1.Generation {
		t.Fatal("content change must bump generation")
	}
	f4 := todo.Render(80)
	if f4.Generation == f3.Generation && !linesEq(f3.Lines, f4.Lines) {
		t.Fatal("width change without gen bump when lines differ")
	}
}

func TestTranscriptSeamOrderingStreamingTool(t *testing.T) {
	tr := view.NewTranscript(view.MonoTheme(), view.Options{Tight: true, MaxProseWidth: -1})

	snap := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{
			textMsg("user", "first question"),
			textMsg("assistant", "first answer that is done"),
			{
				Role:      "assistant",
				Streaming: true,
				Content:   []model.ContentBlock{{Kind: model.ContentText, Text: "partial answer streaming"}},
			},
		},
	}
	tr.SetSnapshot(snap)
	f := tr.Render(60)
	if f.IsEmpty() {
		t.Fatal("empty transcript")
	}
	assertNoNewlineOrOverflow(t, f.Lines, 60)
	if !f.HasLiveRegion() {
		t.Fatal("streaming assistant should set live region")
	}
	live, commit, snapshot, ok := f.NormalizedSeams()
	if ok {
		if !(live <= commit && commit <= snapshot && snapshot <= len(f.Lines)) {
			t.Fatalf("seam order live=%d commit=%d snapshot=%d rows=%d", live, commit, snapshot, len(f.Lines))
		}
	}

	snap.Generation = 2
	snap.Messages[2].Streaming = false
	snap.Messages[2].Content[0].Text = "partial answer streaming complete now"
	tr.SetSnapshot(snap)
	f2 := tr.Render(60)
	assertNoNewlineOrOverflow(t, f2.Lines, 60)

	snap.Generation = 3
	snap.Messages = append(snap.Messages, model.Message{
		Role: "assistant",
		Content: []model.ContentBlock{{
			Kind:     model.ContentToolCall,
			ToolCall: &model.ToolCall{ID: "c1", Name: "read", Intent: "open file"},
		}},
	})
	snap.Tools = []model.ToolExecution{{
		ID: "c1", Name: "read", Running: true, Intent: "open file",
	}}
	tr.SetSnapshot(snap)
	f3 := tr.Render(60)
	assertNoNewlineOrOverflow(t, f3.Lines, 60)
	if !f3.HasLiveRegion() {
		t.Fatal("running tool should keep live region")
	}
	live, commit, snapshot, ok = f3.NormalizedSeams()
	if ok && !(0 <= live && live <= commit && commit <= snapshot && snapshot <= len(f3.Lines)) {
		t.Fatalf("tool seams live=%d commit=%d snap=%d n=%d", live, commit, snapshot, len(f3.Lines))
	}

	snap.Generation = 4
	snap.Tools[0].Running = false
	snap.Messages = append(snap.Messages, model.Message{
		Role:       "toolResult",
		ToolCallID: "c1",
		Content:    []model.ContentBlock{{Kind: model.ContentText, Text: "file contents here"}},
	})
	tr.SetSnapshot(snap)
	f4 := tr.Render(60)
	assertNoNewlineOrOverflow(t, f4.Lines, 60)
	joined := strip(strings.Join(f4.Lines, "\n"))
	if !strings.Contains(joined, "read") && !strings.Contains(joined, "open") {
		t.Fatalf("tool card missing after complete: %q", joined)
	}

	fNarrow := tr.Render(30)
	assertNoNewlineOrOverflow(t, fNarrow.Lines, 30)
	fWide := tr.Render(100)
	assertNoNewlineOrOverflow(t, fWide.Lines, 100)

	f5 := tr.Render(100)
	if f5.Generation != fWide.Generation {
		if !linesEq(f5.Lines, fWide.Lines) {
			t.Fatal("replay changed lines")
		}
	}
}

func TestHasReflowingMarkdown(t *testing.T) {
	if view.HasReflowingMarkdown("plain text") {
		t.Fatal("plain")
	}
	if !view.HasReflowingMarkdown("```mermaid\ngraph LR\nA-->B\n") {
		t.Fatal("open mermaid")
	}
	table := "| a | b |\n| --- | --- |\n| 1 | 2 |\n"
	if !view.HasReflowingMarkdown(table) {
		t.Fatal("table")
	}
	codey := "```\n| --- | --- |\n```\n"
	if view.HasReflowingMarkdown(codey) {
		t.Fatal("delimiter inside fence should not count")
	}
}

func TestTranscriptPointerReuseUnchanged(t *testing.T) {
	tr := view.NewTranscript(view.MonoTheme(), view.Options{Tight: true})
	tr.SetSnapshot(model.Snapshot{
		Generation: 1,
		Messages:   []model.Message{textMsg("user", "stable"), textMsg("assistant", "also stable")},
	})
	a := tr.Render(70)
	b := tr.Render(70)
	if a.IsEmpty() {
		t.Fatal("empty")
	}
	if a.Generation != b.Generation {
		t.Fatalf("gen churn on identical render: %d vs %d", a.Generation, b.Generation)
	}
	if unsafe.SliceData(a.Lines) != unsafe.SliceData(b.Lines) {
		t.Fatal("transcript lines pointer not reused")
	}
}

func TestTranscriptCommittedRowsConcurrentWithRender(t *testing.T) {
	tr := view.NewTranscript(view.MonoTheme(), view.Options{Tight: true})
	tr.SetSnapshot(model.Snapshot{
		Generation: 1,
		Messages:   []model.Message{textMsg("user", "stable"), textMsg("assistant", "also stable")},
	})
	if frame := tr.Render(70); frame.IsEmpty() {
		t.Fatal("empty initial transcript")
	}

	start := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		<-start
		for i := 0; i < 10_000; i++ {
			tr.SetNativeScrollbackCommittedRows(i)
		}
	}()

	close(start)
	for i := 0; i < 10_000; i++ {
		if frame := tr.Render(70); frame.IsEmpty() {
			t.Fatal("empty transcript during committed-row updates")
		}
	}
	writer.Wait()
}

func TestRendererHermesDetailModes(t *testing.T) {
	thinking := model.Message{
		Role:      "assistant",
		Streaming: true,
		Content:   []model.ContentBlock{{Kind: model.ContentThinking, Text: "reasoning-body"}},
	}
	card := view.ToolCard{
		ID:        "call-1",
		Name:      "inspect",
		Intent:    "useful intent",
		Result:    json.RawMessage(`"tool-body"`),
		HasResult: true,
	}

	tests := []struct {
		name         string
		opts         view.Options
		wantThinking string
		wantTool     string
	}{
		{
			name:         "hidden",
			opts:         view.Options{Tight: true, ThinkingMode: view.DetailModeHidden, ToolsMode: view.DetailModeHidden},
			wantThinking: "x",
			wantTool:     "",
		},
		{
			name:         "collapsed",
			opts:         view.Options{Tight: true, ThinkingMode: view.DetailModeCollapsed, ToolsMode: view.DetailModeCollapsed},
			wantThinking: "`- > Thinking  ~4 tokens",
			wantTool:     "`- > Tool calls (1)",
		},
		{
			name: "expanded",
			opts: view.Options{Tight: true, ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded},
			wantThinking: "`- v Thinking  ~4 tokens\n" +
				"  `- reasoning-body",
			wantTool: "`- v Tool calls (1)\n" +
				"  `- * Inspect(\"useful intent\")\n" +
				"    `- Result:\n" +
				"       tool-body",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := view.NewRenderer(view.MonoTheme(), test.opts)
			gotThinking := strip(strings.Join(r.AssistantMessage(thinking, 80, 2), "\n"))
			if gotThinking != test.wantThinking {
				t.Fatalf("thinking rows = %q, want %q", gotThinking, test.wantThinking)
			}
			gotTool := strip(strings.Join(r.ToolRows(card, 80), "\n"))
			if gotTool != test.wantTool {
				t.Fatalf("tool rows = %q, want %q", gotTool, test.wantTool)
			}
			if test.opts.ThinkingMode == view.DetailModeHidden && strings.Contains(gotThinking, "reasoning-body") {
				t.Fatalf("hidden reasoning leaked: %q", gotThinking)
			}
			if test.opts.ToolsMode == view.DetailModeHidden &&
				(strings.Contains(gotTool, "inspect") || strings.Contains(gotTool, "tool-body")) {
				t.Fatalf("hidden tool leaked: %q", gotTool)
			}
		})
	}
}

func TestHiddenToolFailureBackstop(t *testing.T) {
	failed := view.ToolCard{
		ID:        "call-1",
		Name:      "private-tool",
		Result:    json.RawMessage(`"private failure detail"`),
		HasResult: true,
		IsError:   true,
	}
	hidden := view.NewRenderer(view.MonoTheme(), view.Options{
		Tight: true, ToolsMode: view.DetailModeHidden,
	})
	got := strip(strings.Join(hidden.ToolRows(failed, 80), "\n"))
	if got != "!! Tool call failed" {
		t.Fatalf("hidden failure = %q", got)
	}
	if strings.Contains(got, "private-tool") || strings.Contains(got, "private failure detail") {
		t.Fatalf("hidden failure leaked tool content: %q", got)
	}

	collapsed := view.NewRenderer(view.MonoTheme(), view.Options{
		Tight: true, ToolsMode: view.DetailModeCollapsed,
	})
	got = strip(strings.Join(collapsed.ToolRows(failed, 80), "\n"))
	if got != "`- > Tool calls (1)" {
		t.Fatalf("collapsed failure = %q", got)
	}
	if strings.Contains(got, "private-tool") || strings.Contains(got, "private failure detail") {
		t.Fatalf("collapsed failure leaked tool content: %q", got)
	}
}

func TestDetailModesStayBoundedAtNarrowWidths(t *testing.T) {
	thinking := model.Message{
		Role:      "assistant",
		Streaming: true,
		Content: []model.ContentBlock{{
			Kind: model.ContentThinking,
			Text: "reasoning words that deliberately need wrapping",
		}},
	}
	card := view.ToolCard{
		ID:        "call-1",
		Name:      "inspect",
		Intent:    "a detailed intent that needs truncation",
		Result:    json.RawMessage(`"tool body that deliberately needs wrapping"`),
		HasResult: true,
	}
	for _, mode := range []view.DetailMode{
		view.DetailModeHidden,
		view.DetailModeCollapsed,
		view.DetailModeExpanded,
	} {
		r := view.NewRenderer(view.MonoTheme(), view.Options{
			Tight: true, ThinkingMode: mode, ToolsMode: mode,
		})
		for _, width := range []int{12, 30} {
			rows := append(r.AssistantMessage(thinking, width, 1), r.ToolRows(card, width)...)
			assertNoNewlineOrOverflow(t, rows, width)
		}
	}
}

func TestTranscriptDetailOptionChangesInvalidateRowsAndTail(t *testing.T) {
	call := &model.ToolCall{ID: "call-1", Name: "visible-tool", Intent: "visible intent"}
	snap := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{{
			Role: "assistant",
			Content: []model.ContentBlock{
				{Kind: model.ContentThinking, Text: "reasoning-secret"},
				{Kind: model.ContentToolCall, ToolCall: call},
			},
		}},
		Tools: []model.ToolExecution{{
			ID: "call-1", Name: "visible-tool", Result: json.RawMessage(`"tool-secret"`),
		}},
	}
	tr := view.NewTranscript(view.MonoTheme(), view.Options{
		Tight: true, ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded,
	})
	tr.SetSnapshot(snap)
	expanded := tr.Render(80)
	expandedText := strip(strings.Join(expanded.Lines, "\n"))
	if !strings.Contains(expandedText, "reasoning-secret") || !strings.Contains(expandedText, "tool-secret") {
		t.Fatalf("expanded details missing: %q", expandedText)
	}

	tr.SetOptions(view.Options{
		Tight: true, ThinkingMode: view.DetailModeHidden, ToolsMode: view.DetailModeHidden,
	})
	hidden := tr.Render(80)
	hiddenText := strip(strings.Join(hidden.Lines, "\n"))
	if strings.Contains(hiddenText, "reasoning-secret") ||
		strings.Contains(hiddenText, "visible-tool") ||
		strings.Contains(hiddenText, "tool-secret") {
		t.Fatalf("stale hidden rows leaked: %q", hiddenText)
	}
	if tail := tr.RenderViewportTail(80, 20); len(tail) != 0 {
		t.Fatalf("hidden viewport tail leaked: %q", strip(strings.Join(tail, "\n")))
	}

	tr.SetOptions(view.Options{
		Tight: true, ThinkingMode: view.DetailModeCollapsed, ToolsMode: view.DetailModeCollapsed,
	})
	collapsed := tr.Render(80)
	collapsedText := strip(strings.Join(collapsed.Lines, "\n"))
	if collapsedText != "|- > Thinking  ~4 tokens\n`- > Tool calls (1)" {
		t.Fatalf("collapsed rows = %q", collapsedText)
	}
	if strings.Contains(collapsedText, "reasoning-secret") || strings.Contains(collapsedText, "tool-secret") {
		t.Fatalf("collapsed rows leaked body: %q", collapsedText)
	}
	tail := strip(strings.Join(tr.RenderViewportTail(80, 20), "\n"))
	if tail != collapsedText {
		t.Fatalf("collapsed viewport tail = %q, want %q", tail, collapsedText)
	}

	// An explicit section mode must survive a status update carrying the legacy
	// expanded toggle; otherwise SetSnapshot would re-open the card from a stale
	// cached snapshot.
	snap.Generation++
	snap.Status.ToolsExpanded = true
	tr.SetSnapshot(snap)
	if tr.Options().ToolsMode != view.DetailModeCollapsed {
		t.Fatalf("explicit tools mode overwritten: %q", tr.Options().ToolsMode)
	}
	if got := strip(strings.Join(tr.Render(80).Lines, "\n")); got != collapsedText {
		t.Fatalf("status update changed explicit collapsed rows: %q", got)
	}
}

func TestTranscriptHiddenDetailPulsesAdvanceWithSnapshot(t *testing.T) {
	tests := []struct {
		name string
		opts view.Options
		snap model.Snapshot
	}{
		{
			name: "thinking",
			opts: view.Options{Tight: true, ThinkingMode: view.DetailModeHidden},
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{{
					Role:      "assistant",
					Streaming: true,
					Content:   []model.ContentBlock{{Kind: model.ContentThinking, Text: "hidden reasoning"}},
				}},
			},
		},
		{
			name: "tool",
			opts: view.Options{Tight: true, ToolsMode: view.DetailModeHidden},
			snap: model.Snapshot{
				Generation: 1,
				Tools: []model.ToolExecution{{
					ID: "tool-1", Name: "hidden-tool", Running: true,
					PartialResult: json.RawMessage(`"hidden partial output"`),
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tr := view.NewTranscript(view.MonoTheme(), test.opts)
			tr.SetSnapshot(test.snap)
			first := tr.Render(80)
			firstText := strip(strings.Join(first.Lines, "\n"))
			if firstText != "+" {
				t.Fatalf("first pulse = %q, want +", firstText)
			}
			if !first.HasLiveRegion() {
				t.Fatal("hidden active detail lost its live region")
			}

			test.snap.Generation++
			tr.SetSnapshot(test.snap)
			second := tr.Render(80)
			secondText := strip(strings.Join(second.Lines, "\n"))
			if secondText != "x" {
				t.Fatalf("second pulse = %q, want x", secondText)
			}
			if second.Generation == first.Generation {
				t.Fatal("pulse update reused stale frame generation")
			}
			if strings.Contains(secondText, "hidden") {
				t.Fatalf("hidden content leaked through pulse: %q", secondText)
			}
		})
	}
}

func TestFocusViewKeepsPromptsFinalTextAndErrors(t *testing.T) {
	call := &model.ToolCall{ID: "focus-tool", Name: "private-tool", Intent: "private intent"}
	snap := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{
			textMsg("user", "operator prompt"),
			{
				Role:      "assistant",
				Streaming: true,
				Content: []model.ContentBlock{
					{Kind: model.ContentThinking, Text: "private reasoning"},
					{Kind: model.ContentText, Text: "final answer is streaming"},
					{Kind: model.ContentImage, Data: "not-an-image", MIMEType: "image/png"},
					{Kind: model.ContentToolCall, ToolCall: call},
				},
			},
			textMsg("compactionSummary", "private summary"),
			textMsg("extension.activity", "private activity"),
			{Role: "assistant", StopReason: "error", Error: "provider failure"},
		},
		Tools: []model.ToolExecution{{
			ID: "focus-tool", Name: "private-tool", Result: json.RawMessage(`"private tool failure"`), IsError: true,
		}},
	}
	tr := view.NewTranscript(view.MonoTheme(), view.Options{
		Tight: true, ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded,
	})
	tr.SetSnapshot(snap)
	if initial := strip(strings.Join(tr.Render(80).Lines, "\n")); !strings.Contains(initial, "private reasoning") ||
		!strings.Contains(initial, "private tool failure") {
		t.Fatalf("initial details missing: %q", initial)
	}

	tr.SetOptions(view.Options{
		Tight: true, FocusView: true, ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded,
	})
	focused := tr.Render(80)
	got := strip(strings.Join(focused.Lines, "\n"))
	want := "> operator prompt\n\nfinal answer is streaming\n\n!! Tool call failed\n\nError: provider failure"
	if got != want {
		t.Fatalf("focus rows = %q, want %q", got, want)
	}
	if !focused.HasLiveRegion() {
		t.Fatal("focus view lost the live final-answer stream")
	}
	for _, hidden := range []string{
		"private reasoning",
		"private-tool",
		"private intent",
		"private tool failure",
		"private summary",
		"private activity",
		"[Image:",
	} {
		if strings.Contains(got, hidden) {
			t.Fatalf("focus view leaked %q in %q", hidden, got)
		}
	}
	if tail := strip(strings.Join(tr.RenderViewportTail(80, 20), "\n")); tail != want {
		t.Fatalf("focus viewport tail = %q, want %q", tail, want)
	}

	tr.SetOptions(view.Options{
		Tight: true, ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded,
	})
	restored := strip(strings.Join(tr.Render(80).Lines, "\n"))
	if !strings.Contains(restored, "private reasoning") || !strings.Contains(restored, "private tool failure") {
		t.Fatalf("focus option change reused hidden rows: %q", restored)
	}
}

func TestTranscriptDetailModeTransitionsInvalidateHeightAndRows(t *testing.T) {
	call := &model.ToolCall{ID: "mode-transition", Name: "shell", Intent: "private command"}
	snap := model.Snapshot{
		Generation: 9,
		Messages: []model.Message{{
			Role: "assistant",
			Content: []model.ContentBlock{
				{Kind: model.ContentThinking, Text: "private reasoning body"},
				{Kind: model.ContentText, Text: "public answer"},
				{Kind: model.ContentToolCall, ToolCall: call},
			},
		}},
		Tools: []model.ToolExecution{{
			ID: "mode-transition", Name: "shell", Result: json.RawMessage(`"private result"`),
		}},
	}
	tr := view.NewTranscript(view.MonoTheme(), view.Options{
		Tight: true, ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded,
	})
	tr.SetSnapshot(snap)
	expanded := tr.Render(60)
	expandedText := strip(strings.Join(expanded.Lines, "\n"))
	if !strings.Contains(expandedText, "private reasoning body") || !strings.Contains(expandedText, "private result") {
		t.Fatalf("expanded rows missing details: %q", expandedText)
	}

	tr.SetOptions(view.Options{
		Tight: true, ThinkingMode: view.DetailModeHidden, ToolsMode: view.DetailModeHidden,
	})
	hidden := tr.Render(60)
	hiddenText := strip(strings.Join(hidden.Lines, "\n"))
	if hidden.Generation == expanded.Generation {
		t.Fatal("hidden details reused the expanded cached frame")
	}
	if hidden.RowCount() >= expanded.RowCount() {
		t.Fatalf("hidden height = %d, expanded height = %d", hidden.RowCount(), expanded.RowCount())
	}
	if hiddenText != "public answer" {
		t.Fatalf("hidden rows = %q, want public answer only", hiddenText)
	}

	tr.SetOptions(view.Options{
		Tight: true, ThinkingMode: view.DetailModeCollapsed, ToolsMode: view.DetailModeCollapsed,
	})
	collapsed := tr.Render(60)
	collapsedText := strip(strings.Join(collapsed.Lines, "\n"))
	if collapsed.Generation == hidden.Generation {
		t.Fatal("collapsed details reused the hidden cached frame")
	}
	for _, want := range []string{"> Thinking", "> Tool calls (1)", "public answer"} {
		if !strings.Contains(collapsedText, want) {
			t.Fatalf("collapsed rows missing %q: %q", want, collapsedText)
		}
	}
	for _, secret := range []string{"private reasoning body", "private result"} {
		if strings.Contains(collapsedText, secret) {
			t.Fatalf("collapsed rows leaked %q: %q", secret, collapsedText)
		}
	}
	assertNoNewlineOrOverflow(t, collapsed.Lines, 60)
}

func TestTranscriptHiddenToolDoesNotConsumeViewportSeam(t *testing.T) {
	call := &model.ToolCall{ID: "hidden-seam", Name: "shell"}
	tr := view.NewTranscript(view.MonoTheme(), view.Options{
		Tight: true, ThinkingMode: view.DetailModeHidden, ToolsMode: view.DetailModeHidden,
	})
	tr.SetSnapshot(model.Snapshot{
		Messages: []model.Message{
			{Role: "assistant", Content: []model.ContentBlock{
				{Kind: model.ContentText, Text: "durable answer"},
				{Kind: model.ContentToolCall, ToolCall: call},
			}},
			{Role: "assistant", Streaming: true, Content: []model.ContentBlock{
				{Kind: model.ContentText, Text: "live answer"},
			}},
		},
		Tools: []model.ToolExecution{{
			ID: "hidden-seam", Name: "shell", Result: json.RawMessage(`"private result"`),
		}},
	})
	// Consecutive assistant segments share the model band, so they render flush.
	// The hidden trail between them contributes neither a row nor a separator.
	frame := tr.Render(60)
	if got, want := strip(strings.Join(frame.Lines, "\n")), "durable answer\nlive answer"; got != want {
		t.Fatalf("hidden seam rows = %q, want %q", got, want)
	}
	if frame.LiveRegionStart != 1 {
		t.Fatalf("live region starts at %d, want 1", frame.LiveRegionStart)
	}
	if got, want := strip(strings.Join(tr.RenderViewportTail(60, 3), "\n")), "durable answer\nlive answer"; got != want {
		t.Fatalf("viewport tail = %q, want %q", got, want)
	}
}

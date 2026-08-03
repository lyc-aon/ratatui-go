package view_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/model"
	"github.com/lyc-aon/ratatui-go/ompui/view"
)

// goldenWidths spans the responsive breakpoints the transcript reacts to: below
// micro (20), either side of narrow (39/40), narrow itself (60), the canonical
// reading width (80), and past the prose cap (120/200).
var goldenWidths = []int{20, 39, 40, 60, 80, 120, 200}

const goldenWidth = 80

func strippedRows(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strip(line)
	}
	return out
}

// assertNoPhantomGaps checks the blank-row discipline: a frame never opens or
// closes on a blank row, and no band boundary stacks more than the two margins
// the oracle can legitimately paint (a trailing margin plus a leading one).
func assertNoPhantomGaps(t *testing.T, rows []string) {
	t.Helper()
	if len(rows) == 0 {
		return
	}
	if strings.TrimSpace(rows[0]) == "" {
		t.Fatalf("frame opens on a blank row: %q", rows)
	}
	if strings.TrimSpace(rows[len(rows)-1]) == "" {
		t.Fatalf("frame closes on a blank row: %q", rows)
	}
	run := 0
	for i, row := range rows {
		if strings.TrimSpace(row) != "" {
			run = 0
			continue
		}
		run++
		if run > 2 {
			t.Fatalf("phantom gap of %d blank rows at row %d: %q", run, i, rows)
		}
	}
}

func assertSeamsOrdered(t *testing.T, frame component.Frame) {
	t.Helper()
	live, commit, snapshot, ok := frame.NormalizedSeams()
	if !ok {
		return
	}
	if !(0 <= live && live <= commit && commit <= snapshot && snapshot <= frame.RowCount()) {
		t.Fatalf("seams out of order live=%d commit=%d snapshot=%d rows=%d",
			live, commit, snapshot, frame.RowCount())
	}
}

func hermesOpts(thinking, tools view.DetailMode) view.Options {
	return view.Options{Tight: true, ThinkingMode: thinking, ToolsMode: tools}
}

func rawMsg(role, kind, text string, extra string) model.Message {
	raw := `{"role":"` + role + `"`
	if kind != "" {
		raw += `,"kind":"` + kind + `"`
	}
	if extra != "" {
		raw += "," + extra
	}
	raw += "}"
	msg := model.Message{Role: role, Raw: json.RawMessage(raw)}
	if text != "" {
		msg.Content = []model.ContentBlock{{Kind: model.ContentText, Text: text}}
	}
	return msg
}

type goldenScenario struct {
	name string
	opts view.Options
	snap model.Snapshot
	// want is the exact stripped frame at [goldenWidth]. Empty means the
	// scenario is only checked against the structural invariants.
	want []string
	// secrets must never appear at any width.
	secrets []string
	// liveAt, when non-negative, is the expected live-region start row.
	liveAt int
}

func goldenScenarios() []goldenScenario {
	toolCall := func(id, name, intent string) model.ContentBlock {
		return model.ContentBlock{
			Kind:     model.ContentToolCall,
			ToolCall: &model.ToolCall{ID: id, Name: name, Intent: intent},
		}
	}
	thinkStream := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{
			textMsg("user", "plan this"),
			{
				Role:      "assistant",
				Streaming: true,
				Content:   []model.ContentBlock{{Kind: model.ContentThinking, Text: "weighing options"}},
			},
		},
	}
	parallelTools := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{
			textMsg("user", "run both"),
			{Role: "assistant", Content: []model.ContentBlock{
				toolCall("a", "read_file", "open foo"),
				toolCall("b", "run_tests", "verify"),
			}},
		},
		Tools: []model.ToolExecution{
			{ID: "a", Name: "read_file", Result: json.RawMessage(`"120 lines"`)},
			{ID: "b", Name: "run_tests", Result: json.RawMessage(`"3 failed"`), IsError: true},
		},
	}
	finalAnswer := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{
			textMsg("user", "q"),
			{Role: "assistant", Content: []model.ContentBlock{
				{Kind: model.ContentThinking, Text: "considering"},
				{Kind: model.ContentText, Text: "The answer is 42."},
			}},
		},
	}

	return []goldenScenario{
		{
			name:   "idle",
			opts:   hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap:   model.Snapshot{Generation: 1},
			liveAt: -1,
		},
		{
			name: "thinking stream",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: thinkStream,
			want: []string{
				"> plan this",
				"",
				"`- v Thinking  ~4 tokens",
				"  `- weighing options",
			},
			liveAt: 2,
		},
		{
			name:    "thinking stream collapsed",
			opts:    hermesOpts(view.DetailModeCollapsed, view.DetailModeCollapsed),
			snap:    thinkStream,
			want:    []string{"> plan this", "", "`- > Thinking  ~4 tokens"},
			secrets: []string{"weighing options"},
			liveAt:  2,
		},
		{
			name:    "thinking stream hidden",
			opts:    hermesOpts(view.DetailModeHidden, view.DetailModeHidden),
			snap:    thinkStream,
			want:    []string{"> plan this", "", "+"},
			secrets: []string{"weighing options", "Thinking"},
			liveAt:  2,
		},
		{
			name: "parallel tools mixed outcomes",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: parallelTools,
			want: []string{
				"> run both",
				"",
				"`- v Tool calls (2)",
				"  |- * Read File(\"open foo\")",
				"  | `- Result:",
				"       120 lines",
				"  `- * Run Tests(\"verify\")",
				"    `- Error:",
				"       3 failed",
			},
			liveAt: -1,
		},
		{
			name: "parallel tools collapsed hides every call",
			opts: hermesOpts(view.DetailModeCollapsed, view.DetailModeCollapsed),
			snap: parallelTools,
			want: []string{"> run both", "", "`- > Tool calls (2)"},
			secrets: []string{
				"read_file", "Read File", "run_tests", "Run Tests",
				"open foo", "verify", "120 lines", "3 failed", "Result", "Error",
			},
			liveAt: -1,
		},
		{
			name:    "parallel tools hidden keeps only the failure backstop",
			opts:    hermesOpts(view.DetailModeHidden, view.DetailModeHidden),
			snap:    parallelTools,
			want:    []string{"> run both", "", "!! Tool call failed"},
			secrets: []string{"Read File", "Run Tests", "open foo", "verify", "120 lines", "3 failed"},
			liveAt:  -1,
		},
		{
			name: "final answer",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: finalAnswer,
			want: []string{
				"> q",
				"",
				"`- v Thinking  ~3 tokens",
				"  `- considering",
				"",
				"`- Response",
				"",
				"The answer is 42.",
			},
			liveAt: -1,
		},
		{
			name: "final answer hidden drops the response rule",
			opts: hermesOpts(view.DetailModeHidden, view.DetailModeHidden),
			snap: finalAnswer,
			// With no visible working area there is nothing to separate the
			// answer from, so the rule must not appear over an empty band.
			want:    []string{"> q", "", "The answer is 42."},
			secrets: []string{"considering", "Response", "Thinking"},
			liveAt:  -1,
		},
		{
			name: "errors",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{
					{Role: "assistant", StopReason: "error", Error: "provider exploded"},
				},
			},
			want:   []string{"Error: provider exploded"},
			liveAt: -1,
		},
		{
			name: "todo",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{rawMsg("system", "trail", "",
					`"todoIncomplete":true,"todos":[`+
						`{"content":"write tests","status":"in_progress"},`+
						`{"content":"ship","status":"pending"}]`)},
			},
			want: []string{
				"v Todo (0/2) - incomplete - 2 still pending/in_progress",
				"  [~] write tests",
				"  [ ] ship",
			},
			liveAt: -1,
		},
		{
			name: "todo survives hidden details",
			opts: hermesOpts(view.DetailModeHidden, view.DetailModeHidden),
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{rawMsg("system", "trail", "",
					`"todos":[{"content":"write tests","status":"completed"}]`)},
			},
			want:   []string{"v Todo (1/1)", "  [x] write tests"},
			liveAt: -1,
		},
		{
			name: "images",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{{Role: "assistant", Content: []model.ContentBlock{
					{Kind: model.ContentImage, Data: "not-an-image", MIMEType: "image/png"},
				}}},
			},
			want:   []string{"[Image: [image/png]]"},
			liveAt: -1,
		},
		{
			name: "system event and diff bands",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{
					textMsg("user", "go"),
					rawMsg("system", "event", "switched model", ""),
					rawMsg("assistant", "diff", "```diff\n@@ -1 +1 @@\n-old\n+new\n```", ""),
					rawMsg("system", "", "context loaded", ""),
				},
			},
			want: []string{
				"> go",
				"",
				"i switched model",
				"",
				"",
				" @@ -1 +1 @@",
				" -old",
				" +new",
				"",
				"- context loaded",
			},
			liveAt: -1,
		},
		{
			name: "slash echo",
			opts: hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{
					textMsg("assistant", "before"),
					rawMsg("user", "slash", "/details expanded", ""),
					textMsg("assistant", "after"),
				},
			},
			want:   []string{"before", "", "> /details expanded", "", "after"},
			liveAt: -1,
		},
		{
			name: "focus",
			opts: view.Options{
				Tight: true, FocusView: true,
				ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded,
			},
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{
					textMsg("user", "run both"),
					{Role: "assistant", Content: []model.ContentBlock{
						{Kind: model.ContentThinking, Text: "private plan"},
						{Kind: model.ContentText, Text: "done"},
						toolCall("b", "run_tests", "verify"),
					}},
				},
				Tools: []model.ToolExecution{
					{ID: "b", Name: "run_tests", Result: json.RawMessage(`"3 failed"`), IsError: true},
				},
			},
			want:    []string{"> run both", "", "done", "", "!! Tool call failed"},
			secrets: []string{"private plan", "Run Tests", "verify", "3 failed", "Thinking"},
			liveAt:  -1,
		},
		{
			name: "adjacent turns group into one panel",
			opts: hermesOpts(view.DetailModeCollapsed, view.DetailModeCollapsed),
			snap: model.Snapshot{
				Generation: 1,
				Messages: []model.Message{
					textMsg("user", "three things"),
					{Role: "assistant", Content: []model.ContentBlock{toolCall("a", "read_file", "one")}},
					{Role: "assistant", Content: []model.ContentBlock{toolCall("b", "read_file", "two")}},
					{Role: "assistant", Content: []model.ContentBlock{toolCall("c", "read_file", "three")}},
				},
				Tools: []model.ToolExecution{
					{ID: "a", Name: "read_file", Result: json.RawMessage(`"1"`)},
					{ID: "b", Name: "read_file", Result: json.RawMessage(`"2"`)},
					{ID: "c", Name: "read_file", Result: json.RawMessage(`"3"`)},
				},
			},
			// One panel for the whole run, not one per turn: the count is the
			// point of grouping.
			want:    []string{"> three things", "", "`- > Tool calls (3)"},
			secrets: []string{"Read File", "Tool calls (1)", "Tool calls (2)"},
			liveAt:  -1,
		},
	}
}

func TestTranscriptGoldenRowsAcrossWidths(t *testing.T) {
	for _, scenario := range goldenScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			for _, width := range goldenWidths {
				tr := view.NewTranscript(view.MonoTheme(), scenario.opts)
				tr.SetSnapshot(scenario.snap)
				frame := tr.Render(width)
				rows := strippedRows(frame.Lines)

				assertNoNewlineOrOverflow(t, frame.Lines, width)
				assertNoPhantomGaps(t, rows)
				assertSeamsOrdered(t, frame)

				joined := strings.Join(rows, "\n")
				for _, secret := range scenario.secrets {
					if strings.Contains(joined, secret) {
						t.Fatalf("width %d leaked %q in %q", width, secret, joined)
					}
				}
				if width == goldenWidth && len(scenario.want) > 0 {
					if !linesEq(rows, scenario.want) {
						t.Fatalf("width %d rows =\n%q\nwant\n%q", width, rows, scenario.want)
					}
				}
				// A settled transcript parks all three seams at the frame end,
				// which reads as "every row is committable". A live one opens the
				// region at the first still-mutating block.
				if scenario.liveAt >= 0 {
					if frame.LiveRegionStart >= frame.RowCount() {
						t.Fatalf("width %d parked the live seam at %d over %d rows",
							width, frame.LiveRegionStart, frame.RowCount())
					}
					if width == goldenWidth && frame.LiveRegionStart != scenario.liveAt {
						t.Fatalf("live region starts at %d, want %d", frame.LiveRegionStart, scenario.liveAt)
					}
				} else if frame.LiveRegionStart != frame.RowCount() {
					t.Fatalf("width %d opened a live region at %d on a settled transcript of %d rows",
						width, frame.LiveRegionStart, frame.RowCount())
				}

				// A settled transcript is entirely committable, and a replay of
				// the same width must not churn the frame generation.
				replay := tr.Render(width)
				if replay.Generation != frame.Generation && linesEq(replay.Lines, frame.Lines) {
					t.Fatalf("width %d churned generation on an identical replay", width)
				}
			}
		})
	}
}

func TestTranscriptDetailModeTransitionsProduceFreshFrames(t *testing.T) {
	snap := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{
			textMsg("user", "q"),
			{Role: "assistant", Content: []model.ContentBlock{
				{Kind: model.ContentThinking, Text: "secret reasoning"},
				{Kind: model.ContentText, Text: "public answer"},
				{Kind: model.ContentToolCall, ToolCall: &model.ToolCall{
					ID: "t", Name: "run_tests", Intent: "secret intent",
				}},
			}},
		},
		Tools: []model.ToolExecution{
			{ID: "t", Name: "run_tests", Result: json.RawMessage(`"secret result"`)},
		},
	}

	modes := []view.Options{
		hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
		hermesOpts(view.DetailModeCollapsed, view.DetailModeCollapsed),
		hermesOpts(view.DetailModeHidden, view.DetailModeHidden),
		{Tight: true, FocusView: true, ThinkingMode: view.DetailModeExpanded, ToolsMode: view.DetailModeExpanded},
		hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded),
	}

	tr := view.NewTranscript(view.MonoTheme(), modes[0])
	tr.SetSnapshot(snap)

	var prev []string
	prevGen := uint64(0)
	for i, opts := range modes {
		if i > 0 {
			tr.SetOptions(opts)
		}
		frame := tr.Render(goldenWidth)
		rows := strippedRows(frame.Lines)
		joined := strings.Join(rows, "\n")

		assertNoNewlineOrOverflow(t, frame.Lines, goldenWidth)
		assertNoPhantomGaps(t, rows)

		if !strings.Contains(joined, "public answer") {
			t.Fatalf("mode %d dropped the answer: %q", i, joined)
		}
		expanded := opts.ThinkingMode == view.DetailModeExpanded && !opts.FocusView
		for _, secret := range []string{"secret reasoning", "secret intent", "secret result", "Run Tests"} {
			leaked := strings.Contains(joined, secret)
			if leaked != expanded {
				t.Fatalf("mode %d: %q leaked=%v, want %v in %q", i, secret, leaked, expanded, joined)
			}
		}
		if i > 0 && !linesEq(prev, rows) && frame.Generation == prevGen {
			t.Fatalf("mode %d reused the cached frame generation across a detail change", i)
		}
		// The drag-time tail must agree with the settled assembly row for row.
		tail := strippedRows(tr.RenderViewportTail(goldenWidth, len(rows)))
		if !linesEq(tail, rows) {
			t.Fatalf("mode %d viewport tail =\n%q\nwant\n%q", i, tail, rows)
		}
		prev, prevGen = rows, frame.Generation
	}
}

func TestStreamingTailIsNeverCommitted(t *testing.T) {
	body := "settled paragraph one.\n\nsettled paragraph two.\n\nstill writing the third"
	snap := model.Snapshot{
		Generation: 1,
		Messages: []model.Message{
			textMsg("user", "write three paragraphs"),
			{Role: "assistant", Streaming: true, Content: []model.ContentBlock{
				{Kind: model.ContentText, Text: body},
			}},
		},
	}
	tr := view.NewTranscript(view.MonoTheme(), hermesOpts(view.DetailModeExpanded, view.DetailModeExpanded))
	tr.SetSnapshot(snap)
	frame := tr.Render(goldenWidth)

	if frame.LiveRegionStart >= frame.RowCount() {
		t.Fatalf("streaming turn parked its live seam at %d over %d rows",
			frame.LiveRegionStart, frame.RowCount())
	}
	assertSeamsOrdered(t, frame)
	live, commit, _, ok := frame.NormalizedSeams()
	if !ok {
		t.Fatal("streaming turn reported no seams")
	}
	// The in-flight tail must stay out of the commit-safe prefix: its rows
	// re-render on the next delta, and a committed row can never be rewritten.
	if commit >= frame.RowCount() {
		t.Fatalf("commit seam %d reached the frame end (%d rows) while streaming", commit, frame.RowCount())
	}
	if live > commit {
		t.Fatalf("commit seam %d precedes the live start %d", commit, live)
	}

	// Extending the tail must not rewrite an already-offered row.
	offered := commit
	snap.Generation = 2
	snap.Messages[1].Content[0].Text = body + " paragraph and a little more"
	tr.SetSnapshot(snap)
	next := tr.Render(goldenWidth)
	rows := strippedRows(next.Lines)
	before := strippedRows(frame.Lines)
	for i := 0; i < offered && i < len(rows) && i < len(before); i++ {
		if rows[i] != before[i] {
			t.Fatalf("row %d inside the committed prefix was rewritten: %q → %q", i, before[i], rows[i])
		}
	}
	assertNoPhantomGaps(t, rows)
}

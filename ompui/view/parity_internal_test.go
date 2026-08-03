package view

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// Fixtures ported from the Hermes TUI at d79a6f0: src/__tests__/text.test.ts,
// blockLayout.test.ts, messageLine.test.ts and streamingMarkdown.test.ts. They
// pin the pure functions the transcript's structure is derived from, so a
// divergence shows up here rather than as a misplaced row three layers up.

func TestToolTrailLabelAndCallFixtures(t *testing.T) {
	if got := toolTrailLabel("read_file"); got != "Read File" {
		t.Fatalf("toolTrailLabel = %q, want %q", got, "Read File")
	}
	if got := toolTrailLabel(""); got != "" {
		t.Fatalf("toolTrailLabel(empty) = %q", got)
	}
	if got := toolTrailLabel("__weird__"); got != "Weird" {
		t.Fatalf("toolTrailLabel(underscores) = %q, want %q", got, "Weird")
	}
	if got := formatToolCall("read_file", "x", "…"); got != `Read File("x")` {
		t.Fatalf("formatToolCall = %q", got)
	}
	if got := formatToolCall("read_file", "   ", "…"); got != "Read File" {
		t.Fatalf("formatToolCall(blank context) = %q, want bare label", got)
	}
	if got := formatToolCall("read_file", "a\nb   c", "…"); got != `Read File("a b c")` {
		t.Fatalf("formatToolCall(multiline context) = %q", got)
	}
}

func TestCompactPreviewFixtures(t *testing.T) {
	if got := compactPreview("  a   b\nc  ", 64, "…"); got != "a b c" {
		t.Fatalf("compactPreview = %q", got)
	}
	if got := compactPreview("", 64, "…"); got != "" {
		t.Fatalf("compactPreview(empty) = %q", got)
	}
	// Total length stays within max: one ellipsis rune replaces the last kept one.
	if got := compactPreview("abcdefghij", 5, "…"); got != "abcd…" {
		t.Fatalf("compactPreview(truncated) = %q, want %q", got, "abcd…")
	}
	// A three-cell ASCII ellipsis costs three cells, not one.
	if got := compactPreview("abcdefghij", 5, "..."); got != "ab..." {
		t.Fatalf("compactPreview(ascii ellipsis) = %q, want %q", got, "ab...")
	}
}

func TestToolTrailResultLineFixtures(t *testing.T) {
	if !isToolTrailResultLine("foo ✓") || !isToolTrailResultLine("foo ✗") {
		t.Fatal("terminal markers not detected")
	}
	if isToolTrailResultLine("drafting x…") {
		t.Fatal("transient line detected as settled")
	}

	// buildToolTrailLine('read_file', 'x', false, '', 0.94) → 'Read File("x") (0.9s) ✓'
	line := buildVerboseToolTrailLine("read_file", "x", false, 0.94, true, "", "", "…")
	if line != `Read File("x") (0.9s) ✓` {
		t.Fatalf("settled line = %q", line)
	}
	parsed, ok := parseToolTrailResultLine(line)
	if !ok || parsed.call != `Read File("x") (0.9s)` || parsed.detail != "" || parsed.failed {
		t.Fatalf("parse = %+v ok=%v", parsed, ok)
	}
	label, duration := splitToolDuration(`Read File("x") (0.9s)`)
	if label != `Read File("x")` || duration != " (0.9s)" {
		t.Fatalf("splitToolDuration = %q / %q", label, duration)
	}
	if label, duration := splitToolDuration("Read File"); label != "Read File" || duration != "" {
		t.Fatalf("splitToolDuration(no duration) = %q / %q", label, duration)
	}
}

func TestVerboseToolTrailLineFixtures(t *testing.T) {
	line := buildVerboseToolTrailLine("terminal", "npm test", false, 1.25, true,
		"{\n  \"cmd\": \"npm test\"\n}", "first line\nsecond :: line", "…")
	if !strings.Contains(line, "Args:\n{") {
		t.Fatalf("missing args block: %q", line)
	}
	if !strings.Contains(line, "Result:\nfirst line\nsecond :: line") {
		t.Fatalf("missing result block: %q", line)
	}
	parsed, ok := parseToolTrailResultLine(line)
	// The FIRST " :: " wins, so a result that itself contains " :: " stays whole.
	if !ok || parsed.call != `Terminal("npm test") (1.3s)` {
		t.Fatalf("call = %q ok=%v", parsed.call, ok)
	}
	want := "Args:\n{\n  \"cmd\": \"npm test\"\n}\nResult:\nfirst line\nsecond :: line"
	if parsed.detail != want {
		t.Fatalf("detail = %q, want %q", parsed.detail, want)
	}
	if parsed.failed {
		t.Fatal("success line parsed as failure")
	}

	failure := buildVerboseToolTrailLine("terminal", "npm test", true, 0.5, true, "", "command failed", "…")
	if !strings.Contains(failure, "Error:\ncommand failed") || strings.Contains(failure, "Result:") {
		t.Fatalf("failure labels result as success: %q", failure)
	}
	parsed, ok = parseToolTrailResultLine(failure)
	if !ok || parsed.call != `Terminal("npm test") (0.5s)` || parsed.detail != "Error:\ncommand failed" || !parsed.failed {
		t.Fatalf("failure parse = %+v ok=%v", parsed, ok)
	}
}

func TestVerboseToolTrailLineCapsHugeResults(t *testing.T) {
	huge := strings.Repeat("A", 40_000)
	line := buildVerboseToolTrailLine("browser_snapshot", "https://x.example", false, 2, true, "", huge, "…")
	if !strings.Contains(line, "Result:\n") {
		t.Fatalf("missing result label: %.80q", line)
	}
	// The persisted trail budget is ~1 KB, not the 16 KB live-render budget: a
	// 40 KB payload embedded whole is what balloons a session-long render tree.
	if len(line) >= 2_000 {
		t.Fatalf("verbose line length = %d, want < 2000", len(line))
	}
	if !strings.Contains(line, "omitted") {
		t.Fatalf("truncation not disclosed: %.200q", line)
	}
	if !strings.HasSuffix(line, " ✓") {
		t.Fatalf("marker lost after truncation: %.80q", line)
	}

	small := "ok: 3 files changed"
	fits := buildVerboseToolTrailLine("patch", "index.html", false, 0.1, true, "", small, "…")
	if !strings.Contains(fits, "Result:\n"+small) {
		t.Fatalf("small result reshaped: %q", fits)
	}
	if strings.Contains(fits, "omitted") {
		t.Fatalf("small result truncated: %q", fits)
	}
}

func TestFmtKAndTokenEstimateFixtures(t *testing.T) {
	for n, want := range map[int]string{
		999:           "999",
		1_000:         "1k",
		1_500:         "1.5k",
		1_000_000:     "1m",
		10_000_000:    "10m",
		1_000_000_000: "1b",
	} {
		if got := fmtK(n); got != want {
			t.Fatalf("fmtK(%d) = %q, want %q", n, got, want)
		}
	}
	for text, want := range map[string]int{"": 0, "a": 1, "abcd": 1, "abcde": 2} {
		if got := estimateTokensRough(text); got != want {
			t.Fatalf("estimateTokensRough(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestThinkingPreviewFixtures(t *testing.T) {
	if got := thinkingPreview("  body  ", DetailModeExpanded, thinkingCotMax, false, "…"); got != "body" {
		t.Fatalf("full preview = %q", got)
	}
	if got := thinkingPreview("body", DetailModeCollapsed, thinkingCotMax, false, "…"); got != "" {
		t.Fatalf("collapsed preview leaked %q", got)
	}
	if got := thinkingPreview("body", DetailModeHidden, thinkingCotMax, false, "…"); got != "" {
		t.Fatalf("hidden preview leaked %q", got)
	}
	// Hermes drops empty lines before promoting a mid-line bold run, including
	// the source's retained space before the inserted paragraph boundary.
	got := thinkingPreview("one **Two** three\n\n\n\nfour", DetailModeExpanded, thinkingCotMax, true, "…")
	if got != "one \n\n**Two** three\nfour" {
		t.Fatalf("prose-only preview = %q", got)
	}
	status := "(¬_¬) synthesizing...**Resolving comments on GitHub**\n( ͡° ͜ʖ ͡°) musing...\nActual step\n٩(๑❛ᴗ❛๑)۶ contemplating...next step"
	if got := cleanThinkingText(status); got != "**Resolving comments on GitHub**\nActual step\nnext step" {
		t.Fatalf("status cleanup = %q", got)
	}
}

func TestFindStableBoundaryFixtures(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		prefix string // "" means: no boundary yet
	}{
		{name: "no newline", text: "partial line with no newline yet"},
		{name: "single newlines", text: "line one\nline two\nline three"},
		{name: "empty", text: ""},
		{
			name:   "after last blank line",
			text:   "first paragraph\n\nsecond paragraph\n\nthird",
			prefix: "first paragraph\n\nsecond paragraph\n\n",
		},
		{name: "inside open fence", text: "```ts\nfn();\n\nmore code here"},
		{
			name:   "before open fence",
			text:   "intro paragraph\n\n```ts\nfn();\n\nmore code",
			prefix: "intro paragraph\n\n",
		},
		{
			name:   "after closed fence",
			text:   "```ts\nfn();\n```\n\nnarration continues",
			prefix: "```ts\nfn();\n```\n\n",
		},
		{
			name:   "nested fence boundaries",
			text:   "```js\na\n```\n\nmid text\n\n```python\nstill open",
			prefix: "```js\na\n```\n\nmid text\n\n",
		},
		{name: "inside open display math", text: "$$\nx + y\n\nmore math"},
		{
			name:   "after closed display math",
			text:   "$$\nx + y = z\n$$\n\nnarration continues",
			prefix: "$$\nx + y = z\n$$\n\n",
		},
		{
			name:   "before open display math",
			text:   "intro paragraph\n\n$$\nx + y\n\nmore",
			prefix: "intro paragraph\n\n",
		},
		{
			name:   "single line display math is a zero net toggle",
			text:   "intro\n\n$$x = y$$\n\nnarration",
			prefix: "intro\n\n$$x = y$$\n\n",
		},
		{name: "inside open bracket math", text: "\\[\nx + y\n\nmore"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := FindStableBoundary(test.text)
			if test.prefix == "" {
				if got != -1 {
					t.Fatalf("boundary = %d, want -1 (prefix %q)", got, test.text[:min(got, len(test.text))])
				}
				return
			}
			if got < 0 || got > len(test.text) {
				t.Fatalf("boundary = %d, want a valid index", got)
			}
			if test.text[:got] != test.prefix {
				t.Fatalf("settled prefix = %q, want %q", test.text[:got], test.prefix)
			}
		})
	}
}

// streamCorpus exercises every construct the boundary scanner must respect:
// paragraphs, fenced code (with blank lines and `$$` bait inside), display math
// in both syntaxes, setext headings, tables, lists, quotes, and headings.
const streamCorpus = "Intro paragraph explaining the plan in some detail.\n" +
	"\nSection Title\n=============\n" +
	"\nA paragraph before code.\n" +
	"\n```ts\nconst a = 1\n\nconst b = 2\n// $$ not math $$\n```\n" +
	"\nBetween-blocks narration.\n" +
	"\n$$\nE = mc^2\n\n\\sum_i x_i\n$$\n" +
	"\n- item one\n- item two\n\n1. first\n2. second\n" +
	"\n| a | b |\n|---|---|\n| 1 | 2 |\n" +
	"\n> quoted wisdom\n> second line\n" +
	"\n\\[\nx^2 + y^2 = z^2\n\\]\n" +
	"\n## Closing heading\n" +
	"\nFinal paragraph without a trailing newline"

func TestAdvanceScanIsIncrementalAndIdempotent(t *testing.T) {
	var oneShot streamScan
	oneShot.advance(streamCorpus)
	if oneShot.settledLen <= 0 || oneShot.settledLen > len(streamCorpus) {
		t.Fatalf("one-shot settledLen = %d over %d bytes", oneShot.settledLen, len(streamCorpus))
	}

	// Re-feeding the same text must not move anything.
	before := oneShot
	oneShot.advance(streamCorpus)
	if oneShot != before {
		t.Fatalf("advance is not idempotent: %+v vs %+v", oneShot, before)
	}

	// Fed at arbitrary cut points the scanner must land on the same boundary and
	// never walk a committed prefix backwards.
	for seed := uint64(1); seed <= 8; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0x9e3779b9))
		var state streamScan
		pos := 0
		for pos < len(streamCorpus) {
			pos = min(len(streamCorpus), pos+1+rng.IntN(7))
			prev := state.settledLen
			state.advance(streamCorpus[:pos])
			if state.settledLen < prev {
				t.Fatalf("seed %d: settledLen went backwards %d → %d", seed, prev, state.settledLen)
			}
		}
		if state.settledLen != oneShot.settledLen {
			t.Fatalf("seed %d: incremental settledLen = %d, one-shot = %d", seed, state.settledLen, oneShot.settledLen)
		}
	}
}

func TestAdvanceScanHoldsPartialFenceLikeTail(t *testing.T) {
	var state streamScan
	state.advance("para\n\n``")
	if state.settledLen != len("para\n\n") || state.codeOpen {
		t.Fatalf("partial fence judged early: settled=%d codeOpen=%v", state.settledLen, state.codeOpen)
	}

	state = streamScan{}
	state.advance("para\n\n```ts\ncode\n\nstill code")
	if state.settledLen != len("para\n\n") || !state.codeOpen {
		t.Fatalf("blank line inside open fence committed: settled=%d codeOpen=%v", state.settledLen, state.codeOpen)
	}

	full := "para\n\n```ts\ncode\n\nstill code\n```\n\nafter\n\n"
	state = streamScan{}
	state.advance(full)
	if state.settledLen != len(full) || state.codeOpen {
		t.Fatalf("closed fence not committed: settled=%d of %d", state.settledLen, len(full))
	}
}

func TestAdvanceScanSkipsWhitespaceOnlyBlocks(t *testing.T) {
	text := "alpha\n\n\n\nbeta\n\n"
	var state streamScan
	state.advance(text)
	// The whitespace run stays with the block below it instead of committing an
	// empty block, so the committed prefix is the whole input.
	if state.settledLen != len(text) {
		t.Fatalf("settledLen = %d, want %d", state.settledLen, len(text))
	}

	// A setext underline binds the line above it; the only boundary is the blank
	// line, so the pair can never be torn apart by an incremental feed.
	state = streamScan{}
	state.advance("Title\n")
	state.advance("Title\n====\n")
	state.advance("Title\n====\n\nbody\n\n")
	if state.settledLen != len("Title\n====\n\nbody\n\n") {
		t.Fatalf("setext heading torn: settledLen = %d", state.settledLen)
	}
}

func TestLiveTailRowsHoldsBackTheInFlightTail(t *testing.T) {
	// No boundary yet: the whole body is in flight.
	if got := liveTailRows("one\ntwo\nthree"); got != 3 {
		t.Fatalf("liveTailRows(unsettled) = %d, want 3", got)
	}
	// Settled prefix is excluded; only the tail is held back.
	if got := liveTailRows("settled\n\ntail line one\ntail line two"); got != 2 {
		t.Fatalf("liveTailRows(settled prefix) = %d, want 2", got)
	}
	if got := liveTailRows(""); got != 0 {
		t.Fatalf("liveTailRows(empty) = %d, want 0", got)
	}
}

func TestMessageGroupFixtures(t *testing.T) {
	raw := func(kind string) []byte {
		if kind == "" {
			return nil
		}
		return []byte(`{"kind":"` + kind + `"}`)
	}
	cases := []struct {
		role string
		kind string
		want blockGroup
	}{
		{role: "assistant", want: groupModel},
		{role: "assistant", kind: "diff", want: groupDiff},
		{role: "system", kind: "trail", want: groupTrail},
		{role: "system", want: groupNote},
		{role: "user", want: groupUser},
		{role: "user", kind: "slash", want: groupSlash},
		{role: "system", kind: "event", want: groupEvent},
	}
	for _, test := range cases {
		got := messageGroup(model.Message{Role: test.role, Raw: raw(test.kind)})
		if got != test.want {
			t.Fatalf("messageGroup(%q/%q) = %v, want %v", test.role, test.kind, got, test.want)
		}
	}
}

func TestHasLeadGapFixtures(t *testing.T) {
	// A gap opens only at a boundary between working-area bands.
	for _, pair := range [][2]blockGroup{
		{groupTrail, groupModel},
		{groupModel, groupTrail},
		{groupModel, groupNote},
		{groupNote, groupModel},
	} {
		if !hasLeadGap(pair[0], true, pair[1]) {
			t.Fatalf("no gap at boundary %v → %v", pair[0], pair[1])
		}
	}
	// Same-band neighbours stay flush: that is the grouping.
	for _, group := range []blockGroup{groupTrail, groupModel, groupNote} {
		if hasLeadGap(group, true, group) {
			t.Fatalf("gap inside band %v", group)
		}
	}
	// The first block never gaps.
	if hasLeadGap(groupModel, false, groupModel) || hasLeadGap(groupModel, false, groupTrail) {
		t.Fatal("gap above the first block")
	}
	// Bands that already paint a trailing row suppress the successor's lead gap.
	for _, prev := range []blockGroup{groupUser, groupDiff, groupEvent} {
		if hasLeadGap(prev, true, groupModel) || hasLeadGap(prev, true, groupTrail) {
			t.Fatalf("doubled gap after %v", prev)
		}
	}
	// A slash echo has no trailing margin, so the block below still gaps.
	if !hasLeadGap(groupSlash, true, groupModel) || !hasLeadGap(groupSlash, true, groupTrail) {
		t.Fatal("no gap after a slash echo")
	}
	// user / slash / diff / event own their own spacing.
	for _, cur := range []blockGroup{groupUser, groupSlash, groupDiff, groupEvent} {
		if hasLeadGap(groupModel, true, cur) {
			t.Fatalf("grouping managed %v spacing", cur)
		}
	}
}

func TestSeparatorRowsFixtures(t *testing.T) {
	cases := []struct {
		prev, cur blockGroup
		havePrev  bool
		want      int
	}{
		{prev: groupModel, cur: groupModel, havePrev: true, want: 0},
		{prev: groupTrail, cur: groupTrail, havePrev: true, want: 0},
		{prev: groupModel, cur: groupTrail, havePrev: true, want: 1},
		{prev: groupTrail, cur: groupModel, havePrev: true, want: 1},
		{prev: groupUser, cur: groupModel, havePrev: true, want: 1},
		{prev: groupModel, cur: groupUser, havePrev: true, want: 1},
		{prev: groupUser, cur: groupUser, havePrev: true, want: 2},
		{prev: groupModel, cur: groupModel, want: 0},
	}
	for _, test := range cases {
		got := separatorRows(test.prev, test.havePrev, test.cur)
		if got != test.want {
			t.Fatalf("separatorRows(%v, %v, %v) = %d, want %d",
				test.prev, test.havePrev, test.cur, got, test.want)
		}
	}
}

func TestTrailShowsDetailsFixtures(t *testing.T) {
	expanded := NewRenderer(MonoTheme(), Options{
		Tight: true, ThinkingMode: DetailModeExpanded, ToolsMode: DetailModeExpanded,
	})
	hidden := NewRenderer(MonoTheme(), Options{
		Tight: true, ThinkingMode: DetailModeHidden, ToolsMode: DetailModeHidden,
	})
	card := ToolCard{ID: "c", Name: "shell", HasResult: true}

	if !expanded.trailShowsDetails(Trail{Reasoning: "plan"}) {
		t.Fatal("visible reasoning does not earn a response rule")
	}
	if !expanded.trailShowsDetails(Trail{Cards: []ToolCard{card}}) {
		t.Fatal("visible calls do not earn a response rule")
	}
	if expanded.trailShowsDetails(Trail{}) {
		t.Fatal("an empty trail earned a response rule")
	}
	if hidden.trailShowsDetails(Trail{Reasoning: "plan", Cards: []ToolCard{card}}) {
		t.Fatal("hidden sections earned a response rule")
	}
	// Reasoning that is merely in flight is a pulse, not a panel: no rule.
	if expanded.trailShowsDetails(Trail{ReasoningActive: true}) {
		t.Fatal("a bare pulse earned a response rule")
	}
	if !expanded.trailPulses(Trail{ReasoningActive: true}) {
		t.Fatal("in-flight reasoning with no body should pulse")
	}
	if expanded.trailPulses(Trail{Reasoning: "plan", ReasoningActive: true}) {
		t.Fatal("a visible Thinking panel should not also pulse")
	}
	// A failure outranks the pulse: the operator must see it either way.
	if hidden.trailPulses(Trail{Cards: []ToolCard{{ID: "c", HasResult: true, IsError: true}}}) {
		t.Fatal("a failure should render the backstop, not a pulse")
	}
}

func TestParseDetailModeAndExplicitFixtures(t *testing.T) {
	for _, value := range []string{"hidden", " COLLAPSED ", "Expanded"} {
		mode, ok := ParseDetailMode(value)
		if !ok || !mode.Valid() {
			t.Fatalf("ParseDetailMode(%q) = %q ok=%v", value, mode, ok)
		}
	}
	for _, value := range []string{"truncated", "", "42", "maximised"} {
		if mode, ok := ParseDetailMode(value); ok {
			t.Fatalf("ParseDetailMode(%q) accepted as %q", value, mode)
		}
	}

	// Each section resolves independently, and a legacy boolean only applies
	// where no explicit mode was pinned.
	opts := Options{ThinkingMode: DetailModeCollapsed, ToolsExpanded: true}
	if got := opts.thinkingMode(); got != DetailModeCollapsed {
		t.Fatalf("thinkingMode = %q", got)
	}
	if got := opts.toolsMode(); got != DetailModeExpanded {
		t.Fatalf("toolsMode = %q", got)
	}
	if !opts.detailModesExplicit() {
		t.Fatal("one valid mode should mark the host as explicit")
	}
	if (Options{}).detailModesExplicit() {
		t.Fatal("the zero options should stay on the legacy path")
	}
	if got := (Options{HideThinking: true}).thinkingMode(); got != DetailModeHidden {
		t.Fatalf("legacy HideThinking = %q", got)
	}
}

func TestStripDiffFenceFixtures(t *testing.T) {
	fenced := "```diff\n@@ -1 +1 @@\n-old\n+new\n```"
	if got := stripDiffFence(fenced); got != "@@ -1 +1 @@\n-old\n+new" {
		t.Fatalf("stripDiffFence(fenced) = %q", got)
	}
	bare := "@@ -1 +1 @@\n-old\n+new"
	if got := stripDiffFence(bare); got != bare {
		t.Fatalf("stripDiffFence(bare) = %q", got)
	}
	if got := stripDiffFence("   \n\n"); got != "" {
		t.Fatalf("stripDiffFence(blank) = %q", got)
	}
}

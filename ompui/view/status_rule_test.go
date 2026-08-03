package view_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/model"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/view"
)

func TestStatusRuleWidthsAndProgressiveSegments(t *testing.T) {
	micro := view.StatusRuleWidths(10, "abcd", 7)
	if micro.LeftWidth != 7 || micro.RightWidth != 2 || micro.SeparatorWidth != 1 {
		t.Fatalf("micro widths = %+v", micro)
	}
	wide := view.StatusRuleWidths(80, "/work/project", 30)
	if wide.LeftWidth+wide.RightWidth+wide.SeparatorWidth != 80 || wide.LeftWidth < 30 || wide.RightWidth == 0 {
		t.Fatalf("wide widths = %+v", wide)
	}
	withoutCWD := view.StatusRuleWidths(80, "", 30)
	if withoutCWD != (view.StatusRuleLayout{LeftWidth: 80}) {
		t.Fatalf("empty cwd widths = %+v", withoutCWD)
	}

	if tiny := view.StatusRuleWidths(2, "abcdef"); tiny != (view.StatusRuleLayout{LeftWidth: 2}) {
		t.Fatalf("tiny widths = %+v", tiny)
	}
	cwd := "~/src/hermes-agent/main (some-long-branch)"
	if got, want := view.StatusRuleWidths(80, cwd), view.StatusRuleWidths(80, cwd, 0); got != want {
		t.Fatalf("default reservation drifted: got %+v want %+v", got, want)
	}
	greedy := view.StatusRuleWidths(70, cwd)
	reserved := view.StatusRuleWidths(70, cwd, 40)
	if reserved.LeftWidth < 40 || reserved.LeftWidth <= greedy.LeftWidth {
		t.Fatalf("left reservation did not win: greedy=%+v reserved=%+v", greedy, reserved)
	}

	for _, cols := range []int{8, 12, 20, 40, 100} {
		layout := view.StatusRuleWidths(cols, cwd)
		if layout.LeftWidth+layout.RightWidth+layout.SeparatorWidth > cols || layout.LeftWidth < 1 {
			t.Fatalf("width %d allocation = %+v", cols, layout)
		}
	}
	skinny := view.StatusRuleWidths(24, cwd)
	if skinny.RightWidth >= ansitext.VisibleWidth(cwd) || skinny.LeftWidth < 8 {
		t.Fatalf("skinny cwd allocation = %+v", skinny)
	}
	wideRunes := "目录/分支"
	wideRunesLayout := view.StatusRuleWidths(30, wideRunes)
	if wideRunesLayout.RightWidth != ansitext.VisibleWidth(wideRunes) {
		t.Fatalf("display-width cwd allocation = %+v", wideRunesLayout)
	}

	tests := []struct {
		width                                   int
		compact, bar, duration, compress, voice bool
		background, subagents                   bool
	}{
		{width: 71, compact: true},
		{width: 72, bar: true},
		{width: 76, bar: true, duration: true},
		{width: 80, bar: true, duration: true, compress: true},
		{width: 84, bar: true, duration: true, compress: true, voice: true},
		{width: 88, bar: true, duration: true, compress: true, voice: true, background: true},
		{width: 92, bar: true, duration: true, compress: true, voice: true, background: true, subagents: true},
	}
	for _, test := range tests {
		segments := view.StatusBarSegments(test.width)
		if segments.CompactCtx != test.compact || segments.Bar != test.bar ||
			segments.Duration != test.duration || segments.Compressions != test.compress ||
			segments.Voice != test.voice || segments.Background != test.background ||
			segments.Subagents != test.subagents {
			t.Fatalf("width %d segments = %+v", test.width, segments)
		}
	}
}

func TestStatusRuleUsesEntriesHostFactsAndNow(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	r := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true, FocusView: true, Now: now})
	snap := model.Snapshot{
		Session: protocol.SessionState{
			Model:        json.RawMessage(`{"id":"test-model","provider":"provider"}`),
			ContextUsage: json.RawMessage(`{"tokens":50000,"contextWindow":100000,"percent":50}`),
		},
		Subagents: []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{}`)},
		Status: model.Status{
			AgentRunning:   true,
			WorkingMessage: "answering",
			StatusEntries: map[string]string{
				"battery_percent":  "87",
				"battery_category": "good",
				"background_count": "2",
				"live_sessions":    "3",
				"compressions":     "6",
				"voice":            "voice on",
				"active_subagents": "2",
			},
		},
	}
	info := view.StatusInfo{
		CWD:              "/work/project",
		Home:             "/work",
		SessionStartedAt: now.Add(-2 * time.Minute),
	}
	rows := r.StatusRuleRows(snap, info, 240)
	if len(rows) != 1 {
		t.Fatalf("rows = %q", rows)
	}
	got := strip(rows[0])
	for _, want := range []string{
		"battery 87%",
		"answering",
		"[~] focus",
		"test-model - provider",
		"50K/100K",
		"[#####.....] 50%",
		"2m",
		"cmp 6",
		"voice on",
		"3 sessions",
		"2 bg",
		"ag 2",
		"~/project",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status row missing %q: %q", want, got)
		}
	}
	assertNoNewlineOrOverflow(t, rows, 240)

	compact := view.NewRenderer(view.MonoTheme(), view.Options{Tight: true})
	compactRows := compact.StatusRuleRows(model.Snapshot{
		Session: protocol.SessionState{ContextUsage: json.RawMessage(`{"tokens":50000,"contextWindow":100000,"percent":50}`)},
	}, view.StatusInfo{}, 60)
	compactText := strip(strings.Join(compactRows, "\n"))
	if compactText != "- 50K tok" {
		t.Fatalf("compact context = %q", compactText)
	}
	if strings.Contains(compactText, "[") {
		t.Fatalf("compact status kept bar: %q", compactText)
	}
}

func TestStatusLineInfoInvalidatesCachedStatusRule(t *testing.T) {
	line := view.NewStatusLine(view.MonoTheme(), view.Options{Tight: true})
	line.SetSnapshot(model.Snapshot{Status: model.Status{StatusEntries: map[string]string{"status": "idle"}}})
	first := line.Render(80)
	line.SetInfo(view.StatusInfo{CWD: "/tmp/project"})
	second := line.Render(80)
	if first.Generation == second.Generation {
		t.Fatal("status info update reused stale cached frame")
	}
	if got := strip(strings.Join(second.Lines, "\n")); !strings.Contains(got, "/tmp/project") {
		t.Fatalf("status info cwd missing: %q", got)
	}
}

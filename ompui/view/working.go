package view

import (
	"strconv"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// WorkingRows renders the activity indicator: one row saying what the session
// is doing right now, or nothing at all when it is idle.
//
// This is the only animated surface in the package, and deliberately so. It
// lives in chrome, never enters native scrollback, and it is the single place a
// reader looks to answer "is it stuck?" — a question a hundred spinning tool
// cards answer worse than one moving glyph.
//
// Retry and compaction outrank plain activity: both mean the turn is blocked on
// something the operator may want to act on, and both would otherwise look
// identical to ordinary work.
func (r Renderer) WorkingRows(snap model.Snapshot, width int) []string {
	layout := r.opts.layout(width)
	sym := r.theme.Symbols
	glyph := r.spinnerGlyph(r.opts.frame(snap.Generation))
	inset := padding(layout.Inset)

	status := snap.Status
	switch {
	case status.Retry.Active:
		parts := []string{"retrying " + strconv.Itoa(status.Retry.Attempt)}
		if status.Retry.MaxAttempts > 0 {
			parts[0] += "/" + strconv.Itoa(status.Retry.MaxAttempts)
		}
		if status.Retry.Delay > 0 {
			parts = append(parts, "in "+formatDuration(status.Retry.Delay))
		}
		row := inset + apply(r.theme.Warning, glyph+" "+strings.Join(parts, " "))
		if reason := flattenLine(status.Retry.ErrorMessage); reason != "" && !layout.Narrow {
			row += apply(r.theme.Dim, sym.Sep+reason)
		}
		return []string{fit(row, layout.Width, sym.Ellipsis)}

	case status.Compaction.Active:
		label := "compacting context"
		if status.Compaction.Action != "" {
			label = flattenLine(status.Compaction.Action)
		}
		row := inset + apply(r.theme.Accent, glyph+" "+label)
		if reason := flattenLine(status.Compaction.Reason); reason != "" && !layout.Narrow {
			row += apply(r.theme.Dim, sym.Sep+reason)
		}
		return []string{fit(row, layout.Width, sym.Ellipsis)}
	}

	if !status.AgentRunning && !status.TurnRunning && !status.Streaming {
		return nil
	}

	label := strings.TrimSpace(flattenLine(status.WorkingMessage))
	if label == "" {
		label = "Working"
	}
	row := inset + apply(r.theme.Accent, glyph) + " " + apply(r.theme.Muted, label)

	if running := countRunningTools(snap.Tools); running > 0 && !layout.Narrow {
		row += apply(r.theme.Dim, sym.Sep+strconv.Itoa(running)+" "+pluralize("tool", running)+" running")
	}
	return []string{fit(row, layout.Width, sym.Ellipsis)}
}

func countRunningTools(tools []model.ToolExecution) int {
	running := 0
	for i := range tools {
		if tools[i].Running {
			running++
		}
	}
	return running
}

// spinnerGlyph picks the activity frame for a counter position.
func (r Renderer) spinnerGlyph(frame uint64) string {
	frames := r.theme.Symbols.Spinner
	if len(frames) == 0 {
		return r.theme.Symbols.Running
	}
	return frames[frameIndex(frame, frames)]
}

// WorkingIndicator renders the activity row.
type WorkingIndicator struct {
	widget
	snap model.Snapshot
}

// NewWorkingIndicator constructs an activity indicator.
func NewWorkingIndicator(theme Theme, opts Options) *WorkingIndicator {
	return &WorkingIndicator{widget: newWidget(theme, opts)}
}

// SetSnapshot installs the state to report on.
func (w *WorkingIndicator) SetSnapshot(snap model.Snapshot) { w.snap = snap }

// Active reports whether the indicator currently renders anything.
func (w *WorkingIndicator) Active() bool {
	status := w.snap.Status
	return status.Retry.Active || status.Compaction.Active ||
		status.AgentRunning || status.TurnRunning || status.Streaming
}

// Render implements component.Component.
func (w *WorkingIndicator) Render(width int) component.Frame {
	return w.frame(width, w.r.WorkingRows(w.snap, width))
}

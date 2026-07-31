package view

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// maxSubagentRows bounds the roster. A fan-out wider than this is a number, not
// a list, so the overflow collapses into a count.
const maxSubagentRows = 8

// SubagentEntry is the normalized view of one spawned agent, merged from the
// lifecycle and progress frames the core emits for it.
type SubagentEntry struct {
	ID          string
	Agent       string
	Task        string
	Status      string
	CurrentTool string
	ToolCount   int
	Tokens      int64
	Cost        float64
	Duration    time.Duration
	Detached    bool
	Index       int
}

// wireSubagentFrame covers the lifecycle, progress, and event frame shapes with
// one decode. Absent fields simply stay zero.
type wireSubagentFrame struct {
	Type    string `json:"type"`
	Payload struct {
		ID          string `json:"id"`
		SubagentID  string `json:"subagentId"`
		Agent       string `json:"agent"`
		Description string `json:"description"`
		Task        string `json:"task"`
		Assignment  string `json:"assignment"`
		Status      string `json:"status"`
		Index       int    `json:"index"`
		Detached    bool   `json:"detached"`
		Progress    struct {
			ID          string  `json:"id"`
			Index       int     `json:"index"`
			Agent       string  `json:"agent"`
			Status      string  `json:"status"`
			Task        string  `json:"task"`
			Description string  `json:"description"`
			CurrentTool string  `json:"currentTool"`
			ToolCount   int     `json:"toolCount"`
			Tokens      int64   `json:"tokens"`
			Cost        float64 `json:"cost"`
			DurationMS  int64   `json:"durationMs"`
		} `json:"progress"`
	} `json:"payload"`
}

// ParseSubagents normalizes the retained subagent frames into a roster ordered
// by spawn index. Frames that carry no identity are skipped rather than shown
// as an anonymous row.
func ParseSubagents(frames []json.RawMessage) []SubagentEntry {
	if len(frames) == 0 {
		return nil
	}
	out := make([]SubagentEntry, 0, len(frames))
	for _, raw := range frames {
		var frame wireSubagentFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}
		p := frame.Payload
		entry := SubagentEntry{
			ID:          firstNonEmpty(p.ID, p.SubagentID, p.Progress.ID),
			Agent:       firstNonEmpty(p.Agent, p.Progress.Agent),
			Task:        firstNonEmpty(p.Description, p.Progress.Description, p.Task, p.Progress.Task, p.Assignment),
			Status:      firstNonEmpty(p.Progress.Status, p.Status),
			CurrentTool: p.Progress.CurrentTool,
			ToolCount:   p.Progress.ToolCount,
			Tokens:      p.Progress.Tokens,
			Cost:        p.Progress.Cost,
			Detached:    p.Detached,
			Index:       p.Index,
		}
		if p.Progress.DurationMS > 0 {
			entry.Duration = time.Duration(p.Progress.DurationMS) * time.Millisecond
		}
		if entry.Index == 0 && p.Progress.Index != 0 {
			entry.Index = p.Progress.Index
		}
		if entry.ID == "" && entry.Agent == "" {
			continue
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (r Renderer) subagentGlyph(status string) (string, StyleFunc) {
	sym := r.theme.Symbols
	switch status {
	case "running", "started":
		return sym.Running, r.theme.Accent
	case "completed":
		return sym.Success, r.theme.Success
	case "failed":
		return sym.Error, r.theme.Error
	case "aborted":
		return sym.Aborted, r.theme.Warning
	default:
		return sym.Pending, r.theme.Muted
	}
}

// SubagentRows renders the roster: a count header, then one line per agent —
// state, name, assignment, and a dim work cluster.
//
// The assignment gets the elastic space because it is what distinguishes two
// agents of the same kind; counters get fixed, abbreviated widths so the rows
// stay column-comparable as numbers grow.
func (r Renderer) SubagentRows(entries []SubagentEntry, width int) []string {
	if len(entries) == 0 {
		return nil
	}
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)

	running, done, failed := 0, 0, 0
	for _, entry := range entries {
		switch entry.Status {
		case "running", "started":
			running++
		case "completed":
			done++
		case "failed", "aborted":
			failed++
		}
	}
	counts := make([]string, 0, 3)
	if running > 0 {
		counts = append(counts, strconv.Itoa(running)+" running")
	}
	if done > 0 {
		counts = append(counts, strconv.Itoa(done)+" done")
	}
	if failed > 0 {
		counts = append(counts, strconv.Itoa(failed)+" failed")
	}

	header := apply(r.theme.Muted, apply(r.theme.Bold, "Agents"))
	if len(counts) > 0 {
		header += apply(r.theme.Dim, "  "+strings.Join(counts, r.theme.Symbols.Sep))
	}
	out := make([]string, 0, len(entries)+2)
	out = append(out, inset+fit(header, layout.Body, r.theme.Symbols.Ellipsis))

	visible := entries
	hidden := 0
	if len(visible) > maxSubagentRows {
		hidden = len(visible) - maxSubagentRows
		visible = visible[:maxSubagentRows]
	}
	for _, entry := range visible {
		out = append(out, r.subagentRow(entry, layout, inset))
	}
	if hidden > 0 {
		out = append(out, inset+" "+apply(r.theme.Dim,
			r.theme.Symbols.Ellipsis+" "+strconv.Itoa(hidden)+" more"))
	}
	return out
}

func (r Renderer) subagentRow(entry SubagentEntry, layout Layout, inset string) string {
	glyph, style := r.subagentGlyph(entry.Status)
	name := firstNonEmpty(entry.Agent, entry.ID)
	row := inset + " " + apply(style, glyph) + " " + apply(r.theme.Text, name)

	if entry.Task != "" && !layout.Micro {
		row += apply(r.theme.Muted, r.theme.Symbols.Sep+flattenLine(entry.Task))
	}
	if layout.Narrow {
		return fit(row, layout.Width, r.theme.Symbols.Ellipsis)
	}

	meta := make([]string, 0, 4)
	if entry.CurrentTool != "" {
		meta = append(meta, entry.CurrentTool)
	}
	if entry.ToolCount > 0 {
		meta = append(meta, strconv.Itoa(entry.ToolCount)+" "+pluralize("tool", entry.ToolCount))
	}
	if entry.Tokens > 0 {
		meta = append(meta, formatNumber(entry.Tokens))
	}
	if entry.Duration > 0 {
		meta = append(meta, formatDuration(entry.Duration))
	}
	if len(meta) > 0 {
		row += apply(r.theme.Dim, r.theme.Symbols.Sep+strings.Join(meta, r.theme.Symbols.Sep))
	}
	return fit(row, layout.Width, r.theme.Symbols.Ellipsis)
}

// SubagentSummary renders the spawned-agent roster.
type SubagentSummary struct {
	widget
	entries []SubagentEntry
}

// NewSubagentSummary constructs a roster component.
func NewSubagentSummary(theme Theme, opts Options) *SubagentSummary {
	return &SubagentSummary{widget: newWidget(theme, opts)}
}

// SetEntries installs an explicit roster.
func (s *SubagentSummary) SetEntries(entries []SubagentEntry) { s.entries = entries }

// SetSnapshot parses the roster out of a snapshot.
func (s *SubagentSummary) SetSnapshot(snap model.Snapshot) {
	s.entries = ParseSubagents(snap.Subagents)
}

// Entries returns the current roster.
func (s *SubagentSummary) Entries() []SubagentEntry { return s.entries }

// Render implements component.Component.
func (s *SubagentSummary) Render(width int) component.Frame {
	return s.frame(width, s.r.SubagentRows(s.entries, width))
}

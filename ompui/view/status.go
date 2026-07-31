package view

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/model"
)

// Context-pressure thresholds, mirroring OMP. Percent and absolute-token
// thresholds are both checked so a huge window does not hide a huge context.
const (
	contextWarnPercent = 50.0
	contextWarnTokens  = 150_000.0
	contextHighPercent = 70.0
	contextHighTokens  = 270_000.0
	contextCritPercent = 90.0
	contextCritTokens  = 500_000.0
)

// ContextUsage is the session's context-window pressure.
type ContextUsage struct {
	Tokens        int64
	ContextWindow int64
	Percent       float64
	Known         bool
}

// ParseContextUsage decodes the raw contextUsage payload.
func ParseContextUsage(raw json.RawMessage) ContextUsage {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return ContextUsage{}
	}
	var wire struct {
		Tokens        int64   `json:"tokens"`
		ContextWindow int64   `json:"contextWindow"`
		Percent       float64 `json:"percent"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ContextUsage{}
	}
	return ContextUsage{
		Tokens:        wire.Tokens,
		ContextWindow: wire.ContextWindow,
		Percent:       wire.Percent,
		Known:         true,
	}
}

// Label renders the usage as OMP does: `12.4%/200K` when the window is known,
// and `48K/?` when it is not — `0.0%/0` would read as an empty context rather
// than as missing provider metadata.
func (u ContextUsage) Label() string {
	if u.ContextWindow <= 0 {
		return formatNumber(u.Tokens) + "/?"
	}
	return strconv.FormatFloat(u.Percent, 'f', 1, 64) + "%/" + formatNumber(u.ContextWindow)
}

// pressure grades the usage into the color tiers the footer paints with.
func (u ContextUsage) pressure() int {
	if !u.Known || u.Percent <= 0 {
		return 0
	}
	reaches := func(percentThreshold, tokenThreshold float64) bool {
		if u.ContextWindow <= 0 {
			return u.Percent >= percentThreshold
		}
		tokenPercent := tokenThreshold / float64(u.ContextWindow) * 100
		if tokenPercent < percentThreshold {
			percentThreshold = tokenPercent
		}
		return u.Percent >= percentThreshold
	}
	switch {
	case reaches(contextCritPercent, contextCritTokens):
		return 3
	case reaches(contextHighPercent, contextHighTokens):
		return 2
	case reaches(contextWarnPercent, contextWarnTokens):
		return 1
	default:
		return 0
	}
}

func (r Renderer) contextStyle(u ContextUsage) StyleFunc {
	switch u.pressure() {
	case 3:
		return r.theme.Error
	case 2, 1:
		return r.theme.Warning
	default:
		return r.theme.StatusContext
	}
}

// ModelInfo is the resolved model identity shown in chrome.
type ModelInfo struct {
	ID       string
	Provider string
}

// ParseModel decodes the raw model payload from session state.
func ParseModel(raw json.RawMessage) ModelInfo {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return ModelInfo{}
	}
	var wire struct {
		ID       string `json:"id"`
		Provider string `json:"provider"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ModelInfo{}
	}
	return ModelInfo{ID: firstNonEmpty(wire.ID, wire.Name), Provider: wire.Provider}
}

// Label renders `model` or `model · provider`.
func (m ModelInfo) Label(sep string) string {
	switch {
	case m.ID == "" && m.Provider == "":
		return ""
	case m.Provider == "":
		return m.ID
	case m.ID == "":
		return m.Provider
	default:
		return m.ID + sep + m.Provider
	}
}

// parseThinkingLevel decodes the thinking level, which the core sends as a bare
// string or as an object with a level field.
func parseThinkingLevel(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if text == "off" {
			return ""
		}
		return text
	}
	var wire struct {
		Level string `json:"level"`
	}
	if json.Unmarshal(raw, &wire) == nil && wire.Level != "off" {
		return wire.Level
	}
	return ""
}

// sortedStatusEntries returns keyed extension statuses in stable key order.
// Sorting is not cosmetic: an unstable order would rewrite the footer row on
// every render and make the line flicker as extensions report.
func sortedStatusEntries(entries map[string]string) []string {
	if len(entries) == 0 {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(flattenLine(entries[key]))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// StatusLineRows renders the single-row status summary: what model is answering,
// how full the context is, and whether anything is running.
//
// Segments shed right-to-left as the terminal narrows, so the model identity —
// the one thing that changes the meaning of everything else on screen — is the
// last thing to go.
func (r Renderer) StatusLineRows(snap model.Snapshot, width int) []string {
	layout := r.opts.layout(width)
	sym := r.theme.Symbols
	usage := ParseContextUsage(snap.Session.ContextUsage)
	modelInfo := ParseModel(snap.Session.Model)

	segments := make([]string, 0, 5)
	if label := modelInfo.Label(sym.Sep); label != "" {
		text := label
		if !layout.Narrow {
			text = sym.IconModel + " " + label
		}
		segments = append(segments, apply(r.theme.StatusModel, text))
	}
	if level := parseThinkingLevel(snap.Session.ThinkingLevel); level != "" && !layout.Narrow {
		segments = append(segments, apply(r.theme.Dim, level))
	}
	if usage.Known && !layout.Micro {
		segments = append(segments, apply(r.contextStyle(usage), sym.IconContext+" "+usage.Label()))
	}
	if agents := len(snap.Subagents); agents > 0 && !layout.Narrow {
		segments = append(segments, apply(r.theme.Muted, sym.IconAgents+" "+strconv.Itoa(agents)))
	}
	if queued := snap.Session.QueuedMessageCount; queued > 0 {
		segments = append(segments, apply(r.theme.Warning, strconv.Itoa(queued)+" queued"))
	}
	if len(segments) == 0 {
		return nil
	}

	row := padding(layout.Inset) + strings.Join(segments, apply(r.theme.Dim, sym.Sep))
	return []string{fit(row, layout.Width, sym.Ellipsis)}
}

// FooterInfo carries the environment facts the view cannot read for itself.
// The view never touches the filesystem or shells out to git.
type FooterInfo struct {
	// Path is the working directory, already absolute.
	Path string
	// Home, when set, is collapsed to "~" in Path.
	Home string
	// Branch is the current git branch, empty outside a repository.
	Branch string
	// Dirty marks an unclean working tree.
	Dirty bool
	// Cost is the session spend in USD; zero hides the segment.
	Cost float64
	// InputTokens and OutputTokens are cumulative session totals.
	InputTokens  int64
	OutputTokens int64
	// CacheReadTokens and CacheWriteTokens are cumulative session totals.
	CacheReadTokens  int64
	CacheWriteTokens int64
}

// FooterRows renders the two-row footer: location on top, session economics
// below with the model right-aligned.
//
// The split is deliberate. The path answers "where am I", which you read once
// and then ignore; the counters answer "what is this costing", which you glance
// at repeatedly. Giving them separate rows keeps the glance target in a fixed
// position instead of sliding as the path length changes.
func (r Renderer) FooterRows(snap model.Snapshot, info FooterInfo, width int) []string {
	layout := r.opts.layout(width)
	sym := r.theme.Symbols
	out := make([]string, 0, 3)

	if location := r.footerLocation(info, layout); location != "" {
		out = append(out, location)
	}
	out = append(out, r.footerStats(snap, info, layout))

	if entries := sortedStatusEntries(snap.Status.StatusEntries); len(entries) > 0 {
		row := padding(layout.Inset) + apply(r.theme.Dim, strings.Join(entries, sym.Sep))
		out = append(out, fit(row, layout.Width, sym.Ellipsis))
	}
	return out
}

func (r Renderer) footerLocation(info FooterInfo, layout Layout) string {
	path := shortenPath(info.Path, info.Home)
	if path == "" && info.Branch == "" {
		return ""
	}
	branch := ""
	if info.Branch != "" {
		style := r.theme.StatusGitClean
		if info.Dirty {
			style = r.theme.StatusGitDirty
		}
		branch = " " + apply(style, r.theme.Symbols.IconGit+" "+info.Branch)
	}
	budget := layout.Body - ansitext.VisibleWidth(stripANSI(branch))
	if budget < 1 {
		budget = 1
	}
	return padding(layout.Inset) + apply(r.theme.StatusPath, elidePath(path, budget, r.theme.Symbols.Ellipsis)) + branch
}

// footerStats builds the counters row, right-aligning the model identity when
// the terminal is wide enough to hold both clusters apart.
func (r Renderer) footerStats(snap model.Snapshot, info FooterInfo, layout Layout) string {
	sym := r.theme.Symbols
	usage := ParseContextUsage(snap.Session.ContextUsage)

	left := make([]string, 0, 6)
	if info.InputTokens > 0 {
		left = append(left, apply(r.theme.Dim, "in "+formatNumber(info.InputTokens)))
	}
	if info.OutputTokens > 0 {
		left = append(left, apply(r.theme.Dim, "out "+formatNumber(info.OutputTokens)))
	}
	if !layout.Narrow && (info.CacheReadTokens > 0 || info.CacheWriteTokens > 0) {
		left = append(left, apply(r.theme.Dim, "cache "+formatNumber(info.CacheReadTokens)+
			"/"+formatNumber(info.CacheWriteTokens)))
	}
	if info.Cost > 0 {
		left = append(left, apply(r.theme.StatusCost, sym.IconCost+strconv.FormatFloat(info.Cost, 'f', 3, 64)))
	}
	if usage.Known {
		label := sym.IconContext + " " + usage.Label()
		if snap.Session.AutoCompactionEnabled && !layout.Narrow {
			label += " auto"
		}
		left = append(left, apply(r.contextStyle(usage), label))
	}

	right := ""
	if !layout.Narrow {
		modelInfo := ParseModel(snap.Session.Model)
		label := modelInfo.ID
		if level := parseThinkingLevel(snap.Session.ThinkingLevel); level != "" {
			label = joinMeta(sym.Sep, label, level)
		}
		right = apply(r.theme.StatusModel, label)
	}

	inset := padding(layout.Inset)
	leftText := strings.Join(left, apply(r.theme.Dim, sym.Sep))
	leftWidth := ansitext.VisibleWidth(leftText)
	rightWidth := ansitext.VisibleWidth(right)

	if right == "" || leftWidth+2+rightWidth > layout.Body {
		if right != "" && leftWidth == 0 {
			return inset + fit(right, layout.Body, sym.Ellipsis)
		}
		return inset + fit(leftText, layout.Body, sym.Ellipsis)
	}
	return inset + leftText + padding(layout.Body-leftWidth-rightWidth) + right
}

// StatusLine renders the compact one-row session status.
type StatusLine struct {
	widget
	snap model.Snapshot
}

// NewStatusLine constructs a status line.
func NewStatusLine(theme Theme, opts Options) *StatusLine {
	return &StatusLine{widget: newWidget(theme, opts)}
}

// SetSnapshot installs the state to summarize.
func (s *StatusLine) SetSnapshot(snap model.Snapshot) { s.snap = snap }

// Render implements component.Component.
func (s *StatusLine) Render(width int) component.Frame {
	return s.frame(width, s.r.StatusLineRows(s.snap, width))
}

// Footer renders the two-row session footer beneath the editor.
type Footer struct {
	widget
	snap model.Snapshot
	info FooterInfo
}

// NewFooter constructs a footer.
func NewFooter(theme Theme, opts Options) *Footer {
	return &Footer{widget: newWidget(theme, opts)}
}

// SetSnapshot installs the session state.
func (f *Footer) SetSnapshot(snap model.Snapshot) { f.snap = snap }

// SetInfo installs the environment facts the view cannot read for itself.
func (f *Footer) SetInfo(info FooterInfo) { f.info = info }

// Info returns the current environment facts.
func (f *Footer) Info() FooterInfo { return f.info }

// Render implements component.Component.
func (f *Footer) Render(width int) component.Frame {
	return f.frame(width, f.r.FooterRows(f.snap, f.info, width))
}

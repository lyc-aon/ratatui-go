package view

import (
	"strconv"
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// StatusRuleLayout reserves a stable left status region and an optional right
// working-directory region. The right side yields first as the terminal narrows.
type StatusRuleLayout struct {
	LeftWidth      int
	RightWidth     int
	SeparatorWidth int
}

// StatusRuleWidths mirrors Hermes' status-rule allocation. minLeftContent is
// the visible width of the pinned left segments (status, model, and context).
func StatusRuleWidths(cols int, cwdLabel string, minLeftContent ...int) StatusRuleLayout {
	width := cols
	if width < 1 {
		width = 1
	}
	desiredSeparatorWidth := 1
	baseMinLeft := 1
	if width >= 24 {
		desiredSeparatorWidth = 3
		baseMinLeft = 8
	}
	reservedLeft := 0
	if len(minLeftContent) > 0 {
		reservedLeft = minLeftContent[0]
	}
	minLeftWidth := min(width, max(baseMinLeft, reservedLeft))
	maxRightWidth := max(0, width-desiredSeparatorWidth-minLeftWidth)
	if cwdLabel == "" || maxRightWidth == 0 {
		return StatusRuleLayout{LeftWidth: width}
	}
	rightWidth := min(ansitext.VisibleWidth(cwdLabel), maxRightWidth)
	if rightWidth == 0 {
		return StatusRuleLayout{LeftWidth: width}
	}
	separatorWidth := desiredSeparatorWidth
	leftWidth := max(1, width-separatorWidth-rightWidth)
	return StatusRuleLayout{LeftWidth: leftWidth, RightWidth: rightWidth, SeparatorWidth: separatorWidth}
}

// StatusBarSegmentSet describes which lower-priority status segments fit at a
// terminal width. Pinned status, model, context, and focus are intentionally
// absent: they are budgeted separately and never gated by these breakpoints.
type StatusBarSegmentSet struct {
	Bar          bool
	Background   bool
	CompactCtx   bool
	Compressions bool
	Duration     bool
	Subagents    bool
	Voice        bool
}

// StatusBarSegments mirrors Hermes' progressive-disclosure breakpoints.
func StatusBarSegments(cols int) StatusBarSegmentSet {
	width := cols
	if width < 1 {
		width = 1
	}
	return StatusBarSegmentSet{
		CompactCtx:   width < 72,
		Bar:          width >= 72,
		Duration:     width >= 76,
		Compressions: width >= 80,
		Voice:        width >= 84,
		Background:   width >= 88,
		Subagents:    width >= 92,
	}
}

// BatteryInfo is a host-supplied battery reading. Percent is meaningful when
// Available is true; set it to -1 when the platform can detect a battery but
// cannot obtain a charge percentage.
type BatteryInfo struct {
	Available bool
	Charging  bool
	Category  string
	Percent   int
}

// StatusInfo carries host facts which are intentionally absent from the model
// stream. Zero values defer to compatible Snapshot.StatusEntries keys.
type StatusInfo struct {
	CWD  string
	Home string

	Battery          BatteryInfo
	BackgroundCount  int
	LiveSessionCount int
	Compressions     int
	VoiceLabel       string
	Status           string

	SessionStartedAt time.Time
	LastTurnEndedAt  time.Time
	TurnStartedAt    time.Time
}

type statusRulePiece struct {
	plain string
	row   string
}

// StatusRuleRows renders Hermes' one-row status rule using only supplied facts.
// It deliberately sheds complete tail segments rather than truncating each one,
// so the status/model/context cluster remains readable at narrow widths.
func (r Renderer) StatusRuleRows(snap model.Snapshot, info StatusInfo, width int) []string {
	layout := r.opts.layout(width)
	if layout.Width < 1 {
		return nil
	}
	sym := r.theme.Symbols
	entries := snap.Status.StatusEntries
	segments := StatusBarSegments(layout.Body)
	sep := statusRuleSeparator(sym)

	cwd := firstNonEmpty(info.CWD, statusEntry(entries, "cwd", "path", "workdir"))
	if cwd != "" {
		cwd = shortenPath(cwd, info.Home)
	}
	usage := ParseContextUsage(snap.Session.ContextUsage)
	modelInfo := ParseModel(snap.Session.Model)
	modelLabel := modelInfo.Label(sym.Sep)
	if modelLabel == "" {
		modelLabel = statusEntry(entries, "model", "model_label")
	}
	contextLabel := statusContextLabel(usage, segments.CompactCtx)
	busy := snap.Status.AgentRunning || snap.Status.Streaming || snap.Status.TurnRunning
	statusText := firstNonEmpty(info.Status, snap.Status.WorkingMessage, statusEntry(entries, "status", "state", "working"))
	if statusText == "" && busy {
		statusText = "working"
	}

	pieces := make([]statusRulePiece, 0, 6)
	if battery := statusBattery(info.Battery, entries); battery.Available {
		label := statusBatteryLabel(battery)
		pieces = append(pieces, statusRulePiece{plain: label, row: apply(r.batteryStyle(battery), label)})
	}
	if statusText != "" {
		style := r.theme.Muted
		if busy {
			style = r.theme.Accent
		}
		pieces = append(pieces, statusRulePiece{plain: statusText, row: apply(style, statusText)})
	}
	if modelLabel != "" {
		pieces = append(pieces, statusRulePiece{plain: modelLabel, row: apply(r.theme.StatusModel, modelLabel)})
	}
	if contextLabel != "" {
		pieces = append(pieces, statusRulePiece{plain: contextLabel, row: apply(r.contextStyle(usage), contextLabel)})
	}
	if r.opts.FocusView {
		focus := sym.CheckActive + " focus"
		// Hermes keeps the reduced-output badge in the pinned essentials
		// cluster, after model and context. It is never tail-budgeted.
		pieces = append(pieces, statusRulePiece{plain: focus, row: apply(r.theme.Warning, focus)})
	}

	if len(pieces) == 0 && cwd == "" {
		return nil
	}
	prefix := sym.Rule + " "
	leftPlain := prefix
	leftRow := apply(r.theme.Border, prefix)
	for i, piece := range pieces {
		if i > 0 {
			leftPlain += sep
			leftRow += apply(r.theme.Dim, sep)
		}
		leftPlain += piece.plain
		leftRow += piece.row
	}
	essentialWidth := ansitext.VisibleWidth(leftPlain)
	ruleLayout := StatusRuleWidths(layout.Body, cwd, essentialWidth)
	tailBudget := max(0, ruleLayout.LeftWidth-essentialWidth)
	appendTail := func(plain string, style StyleFunc) bool {
		if plain == "" {
			return false
		}
		needed := ansitext.VisibleWidth(sep) + ansitext.VisibleWidth(plain)
		if tailBudget < needed {
			return false
		}
		tailBudget -= needed
		leftPlain += sep + plain
		leftRow += apply(r.theme.Dim, sep) + apply(style, plain)
		return true
	}

	if segments.Bar && usage.ContextWindow > 0 {
		bar := statusContextBar(usage.Percent, 10, sym.Rule == "-")
		percent := trimOneDecimal(usage.Percent) + "%"
		appendTail("["+bar+"] "+percent, r.contextStyle(usage))
	}
	now := r.opts.Now
	startedAt := firstStatusTime(info.SessionStartedAt, statusEntry(entries, "session_started_at", "sessionStartedAt"))
	lastEndedAt := firstStatusTime(info.LastTurnEndedAt, statusEntry(entries, "last_turn_ended_at", "lastTurnEndedAt"))
	if segments.Duration && !now.IsZero() && !startedAt.IsZero() {
		appendTail(formatDuration(now.Sub(startedAt)), r.theme.Muted)
	}
	if segments.Duration && !busy && !now.IsZero() && !lastEndedAt.IsZero() {
		appendTail(sym.Success+" "+formatDuration(now.Sub(lastEndedAt)), r.theme.Success)
	}
	compressions := firstPositive(info.Compressions, statusEntryInt(entries, "compressions", "compression_count"))
	if segments.Compressions && compressions > 0 {
		style := r.theme.Muted
		if compressions >= 10 {
			style = r.theme.Error
		} else if compressions >= 5 {
			style = r.theme.Warning
		}
		appendTail("cmp "+strconv.Itoa(compressions), style)
	}
	voice := firstNonEmpty(info.VoiceLabel, statusEntry(entries, "voice", "voice_label"))
	if segments.Voice && voice != "" {
		voiceStyle := r.theme.Muted
		if strings.HasPrefix(voice, "●") {
			voiceStyle = r.theme.Error
		} else if strings.HasPrefix(voice, "◉") {
			voiceStyle = r.theme.Warning
		}
		appendTail(voice, voiceStyle)
	}
	liveSessions := firstPositive(info.LiveSessionCount, statusEntryInt(entries, "live_sessions", "session_count", "sessions"))
	if liveSessions > 0 {
		appendTail(strconv.Itoa(liveSessions)+" "+pluralize("session", liveSessions), r.theme.Muted)
	}
	background := firstPositive(info.BackgroundCount, statusEntryInt(entries, "background_count", "background_tasks", "bg"))
	if segments.Background && background > 0 {
		appendTail(strconv.Itoa(background)+" bg", r.theme.Muted)
	}
	subagents := max(len(snap.Subagents), statusEntryInt(entries, "active_subagents", "subagents"))
	if segments.Subagents && subagents > 0 {
		appendTail(sym.IconAgents+" "+strconv.Itoa(subagents), r.theme.Muted)
	}

	leftRow = fit(leftRow, ruleLayout.LeftWidth, sym.Ellipsis)
	row := padding(layout.Inset) + leftRow
	if ruleLayout.RightWidth > 0 {
		rule := " "
		if ruleLayout.SeparatorWidth >= 3 {
			rule = " " + sym.Rule + " "
		}
		row += apply(r.theme.Border, rule) + apply(r.theme.StatusPath, fit(cwd, ruleLayout.RightWidth, sym.Ellipsis))
	}
	return []string{fit(row, layout.Width, sym.Ellipsis)}
}

func statusRuleSeparator(sym Symbols) string {
	if sym.Rule == "-" {
		return " | "
	}
	return " │ "
}

func statusContextLabel(usage ContextUsage, compact bool) string {
	if !usage.Known || (usage.ContextWindow <= 0 && usage.Tokens <= 0) {
		return ""
	}
	if compact || usage.ContextWindow <= 0 {
		return formatNumber(usage.Tokens) + " tok"
	}
	return formatNumber(usage.Tokens) + "/" + formatNumber(usage.ContextWindow)
}

func statusContextBar(percent float64, width int, ascii bool) string {
	if width < 1 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent/100*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	full, empty := "█", "░"
	if ascii {
		full, empty = "#", "."
	}
	return strings.Repeat(full, filled) + strings.Repeat(empty, width-filled)
}

func statusEntry(entries map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(flattenLine(entries[key])); value != "" {
			return value
		}
	}
	return ""
}

func statusEntryInt(entries map[string]string, keys ...string) int {
	value := statusEntry(entries, keys...)
	if value == "" {
		return 0
	}
	value = strings.TrimSuffix(value, "%")
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstStatusTime(explicit time.Time, raw string) time.Time {
	if !explicit.IsZero() {
		return explicit
	}
	if raw == "" {
		return time.Time{}
	}
	if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if value > 0 && value < 1_000_000_000_000 {
			return time.Unix(value, 0)
		}
		if value >= 1_000_000_000_000 {
			return time.UnixMilli(value)
		}
	}
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		return value
	}
	return time.Time{}
}

func statusBattery(explicit BatteryInfo, entries map[string]string) BatteryInfo {
	if explicit.Available {
		return explicit
	}
	label := statusEntry(entries, "battery", "battery_label")
	percent := statusEntryInt(entries, "battery_percent", "battery.percent")
	if label == "" && percent == 0 && statusEntry(entries, "battery_percent", "battery.percent") == "" {
		return BatteryInfo{}
	}
	battery := BatteryInfo{
		Available: true,
		Charging:  strings.Contains(strings.ToLower(label), "charg") || statusEntry(entries, "battery_charging") == "true",
		Category:  statusEntry(entries, "battery_category", "battery.category"),
		Percent:   -1,
	}
	if raw := statusEntry(entries, "battery_percent", "battery.percent"); raw != "" {
		battery.Percent = percent
		return battery
	}
	for _, token := range strings.Fields(label) {
		if value, err := strconv.Atoi(strings.TrimSuffix(token, "%")); err == nil {
			battery.Percent = value
			break
		}
	}
	return battery
}

func statusBatteryLabel(battery BatteryInfo) string {
	kind := "battery"
	if battery.Charging {
		kind = "charge"
	}
	percent := "--"
	if battery.Percent >= 0 {
		percent = strconv.Itoa(battery.Percent)
	}
	return kind + " " + percent + "%"
}

func (r Renderer) batteryStyle(battery BatteryInfo) StyleFunc {
	switch strings.ToLower(battery.Category) {
	case "good", "full", "ok":
		return r.theme.Success
	case "warn", "warning":
		return r.theme.Warning
	case "bad", "critical", "low":
		return r.theme.Error
	default:
		return r.theme.Muted
	}
}

package view

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Bounds ported from Hermes config/limits.ts. The verbose trail budget is
// deliberately far below the live-render budget: a persisted trail block is kept
// for the whole session and rendered expanded by default, so a 40 KB browser
// snapshot embedded whole is what balloons the row tree.
const (
	liveRenderMaxChars = 16_000
	liveRenderMaxLines = 240

	verboseTrailMaxChars = 800
	verboseTrailMaxLines = 12

	// thinkingCotMax bounds a one-line reasoning preview.
	thinkingCotMax = 160

	// toolCallContextMax bounds the intent quoted inside a tool-call label.
	toolCallContextMax = 64
)

// trailResultMarks are the two terminal markers a settled trail line ends with.
const (
	trailMarkOK   = "✓"
	trailMarkFail = "✗"
)

var whitespaceRun = regexp.MustCompile(`\s+`)

// boldParagraph matches the position just before an inline-bold run that starts
// mid-line. Hermes promotes those to their own paragraph so a reasoning body
// reads as prose instead of one run-on line.
var boldParagraph = regexp.MustCompile(`\*\*[^*\n][^\n]*?\*\*`)

var blankRun = regexp.MustCompile(`\n{3,}`)

const thinkingStatusVerbs = `pondering|contemplating|musing|cogitating|ruminating|deliberating|mulling|reflecting|processing|reasoning|analyzing|computing|synthesizing|formulating|brainstorming`

var (
	thinkingStatus      = regexp.MustCompile(`(?i)^(?:` + thinkingStatusVerbs + `)\.{0,3}$`)
	thinkingStatusChunk = regexp.MustCompile(`(?i)[^A-Za-z\n]+\s*(?:` + thinkingStatusVerbs + `)\.{0,3}\s*`)
)

// compactPreview collapses whitespace runs, trims, and truncates to max cells
// with ellipsis. Port of Hermes compactPreview, generalized so the 7-bit glyph
// preset can spend three cells on its ellipsis without overrunning max.
func compactPreview(s string, max int, ellipsis string) string {
	one := strings.TrimSpace(whitespaceRun.ReplaceAllString(s, " "))
	if one == "" {
		return ""
	}
	if utf8.RuneCountInString(one) <= max {
		return one
	}
	keep := max - utf8.RuneCountInString(ellipsis)
	if keep < 0 {
		keep = 0
	}
	return string([]rune(one)[:keep]) + ellipsis
}

// estimateTokensRough is Hermes' 4-chars-per-token estimate, rounding up.
func estimateTokensRough(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) >> 2
}

// fmtK renders a compact token count with a lowercase magnitude suffix
// (999, 1k, 1.5k, 25k, 1m). Port of Hermes fmtK over the shared compact
// number formatter, so both frontends print the same digits.
func fmtK(n int) string {
	return strings.ToLower(formatNumber(int64(n)))
}

// toolTrailLabel title-cases a snake_case tool name: `read_file` → `Read File`.
func toolTrailLabel(name string) string {
	parts := strings.Split(name, "_")
	kept := parts[:0]
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		kept = append(kept, string(runes))
	}
	if len(kept) == 0 {
		return name
	}
	return strings.Join(kept, " ")
}

// formatToolCall renders the trail's call label: the title-cased tool name plus
// the model's one-line context in quotes.
func formatToolCall(name, context string, ellipsis string) string {
	label := toolTrailLabel(name)
	preview := compactPreview(context, toolCallContextMax, ellipsis)
	if preview == "" {
		return label
	}
	return label + "(\"" + preview + "\")"
}

// fixed1 formats seconds with one decimal, rounding halves away from zero so a
// 1.25 s call prints `1.3s` exactly as the TypeScript `toFixed(1)` does.
func fixed1(v float64) string {
	if v < 0 {
		return "-" + fixed1(-v)
	}
	return strconv.FormatFloat(math.Floor(v*10+0.5)/10, 'f', 1, 64)
}

// trailDuration renders the ` (1.3s)` suffix splitToolDuration later peels off.
func trailDuration(seconds float64, ok bool) string {
	if !ok {
		return ""
	}
	return " (" + fixed1(seconds) + "s)"
}

// buildVerboseToolTrailLine renders the settled trail line with the full
// argument and result blocks behind ` :: `, each bounded to the persisted trail
// budget rather than the live-render budget.
func buildVerboseToolTrailLine(
	name, context string,
	isError bool,
	seconds float64, hasDuration bool,
	argsText, resultText string,
	ellipsis string,
) string {
	resultLabel := "Result"
	if isError {
		resultLabel = "Error"
	}
	detail := joinNonEmpty("\n",
		verboseToolBlock("Args", argsText),
		verboseToolBlock(resultLabel, resultText),
	)
	return joinTrailLine(formatToolCall(name, context, ellipsis)+trailDuration(seconds, hasDuration), detail, isError)
}

func joinTrailLine(call, detail string, isError bool) string {
	mark := trailMarkOK
	if isError {
		mark = trailMarkFail
	}
	if detail != "" {
		return call + " :: " + detail + " " + mark
	}
	return call + " " + mark
}

func verboseToolBlock(label, text string) string {
	body := strings.TrimSpace(text)
	if body == "" {
		return ""
	}
	return label + ":\n" + boundedRenderText(body, "showing live tail", verboseTrailMaxChars, verboseTrailMaxLines)
}

func joinNonEmpty(sep string, parts ...string) string {
	kept := parts[:0]
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, sep)
}

// trailResult is a parsed settled trail line.
type trailResult struct {
	call   string
	detail string
	failed bool
}

// isToolTrailResultLine reports whether a trail line carries a terminal marker.
func isToolTrailResultLine(line string) bool {
	return strings.HasSuffix(line, " "+trailMarkOK) || strings.HasSuffix(line, " "+trailMarkFail)
}

// parseToolTrailResultLine splits a settled trail line into its call label and
// detail block. The ` :: ` separator wins over the legacy `: ` form so a detail
// containing `: ` cannot be mistaken for the boundary.
func parseToolTrailResultLine(line string) (trailResult, bool) {
	if !isToolTrailResultLine(line) {
		return trailResult{}, false
	}
	failed := strings.HasSuffix(line, " "+trailMarkFail)
	body := line[:len(line)-len(" "+trailMarkOK)]
	if i := strings.Index(body, " :: "); i >= 0 {
		return trailResult{call: body[:i], detail: body[i+len(" :: "):], failed: failed}, true
	}
	if i := strings.Index(body, ": "); i > 0 {
		return trailResult{call: body[:i], detail: body[i+len(": "):], failed: failed}, true
	}
	return trailResult{call: body, failed: failed}, true
}

// durationSuffix matches the trailing ` (1.3s)` of a call label.
var durationSuffix = regexp.MustCompile(`^(.*?)( \(\d+(?:\.\d)?s\))$`)

// splitToolDuration peels the duration off a call label so the renderer can dim
// it without dimming the label.
func splitToolDuration(call string) (label, duration string) {
	if m := durationSuffix.FindStringSubmatch(call); m != nil {
		return m[1], m[2]
	}
	return call, ""
}

// boundedLiveRenderText front-trims an in-flight body to the live-render budget,
// stating what it dropped rather than silently losing the head.
func boundedLiveRenderText(text string) string {
	return boundedRenderText(text, "showing live tail", liveRenderMaxChars, liveRenderMaxLines)
}

// boundedRenderText keeps the tail of text within maxChars and maxLines,
// prefixing a label that names the omission. Port of Hermes boundedRenderText:
// the line budget is applied first, then the character budget is snapped forward
// to the next line boundary so the kept body still starts on a whole line.
func boundedRenderText(text, labelPrefix string, maxChars, maxLines int) string {
	if len(text) <= maxChars && countNewlines(text, len(text)) < maxLines {
		return text
	}

	start := 0
	idx := len(text)
	for seen := 0; seen < maxLines && idx > 0; seen++ {
		idx = strings.LastIndexByte(text[:idx], '\n')
		if idx < 0 {
			start = 0
			break
		}
		start = idx + 1
	}

	lineStart := start
	if trimmed := len(text) - maxChars; trimmed > start {
		start = trimmed
	}
	if start > lineStart {
		if next := strings.IndexByte(text[start:], '\n'); next >= 0 && start+next < len(text)-1 {
			start += next + 1
		}
	}
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}

	tail := strings.TrimLeft(text[start:], " \t\n\r")
	omittedLines := countNewlines(text, start)
	omittedChars := len(text) - len(tail)
	if omittedChars < 0 {
		omittedChars = 0
	}

	label := "[" + labelPrefix + "; omitted "
	if omittedLines > 0 {
		label += fmtK(omittedLines) + " lines / "
	}
	label += fmtK(omittedChars) + " chars]\n"
	return label + tail
}

func countNewlines(text string, end int) int {
	if end > len(text) {
		end = len(text)
	}
	return strings.Count(text[:end], "\n")
}

// thinkingPreview normalizes a reasoning body for display. `full` keeps the
// prose; `collapsed` renders nothing; the truncated form is the one-line preview
// the collapsed chevron used to carry.
func thinkingPreview(reasoning string, mode DetailMode, max int, proseOnly bool, ellipsis string) string {
	raw := strings.TrimSpace(reasoning)
	if proseOnly {
		raw = cleanThinkingText(raw)
	}
	switch {
	case raw == "" || mode == DetailModeCollapsed || mode == DetailModeHidden:
		return ""
	case mode == DetailModeExpanded:
		return raw
	default:
		return compactPreview(raw, max, ellipsis)
	}
}

// cleanThinkingText ports Hermes' reasoning prose cleanup: remove decorative
// face/status fragments, discard standalone status lines, promote a mid-line
// bold run to its own paragraph, and collapse runs of three or more newlines.
func cleanThinkingText(reasoning string) string {
	lines := strings.Split(reasoning, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(thinkingStatusChunk.ReplaceAllString(line, ""))
		statusCandidate := strings.TrimSpace(strings.TrimSuffix(line, "..."))
		if line == "" || thinkingStatus.MatchString(statusCandidate) {
			continue
		}
		kept = append(kept, line)
	}
	joined := splitBeforeBold(strings.Join(kept, "\n"))
	return strings.TrimSpace(blankRun.ReplaceAllString(joined, "\n\n"))
}

// splitBeforeBold inserts a paragraph break before every inline bold run that
// does not already start its line.
func splitBeforeBold(s string) string {
	matches := boldParagraph.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2*len(matches))
	prev := 0
	for _, m := range matches {
		if m[0] == 0 || s[m[0]-1] == '\n' {
			continue
		}
		b.WriteString(s[prev:m[0]])
		b.WriteString("\n\n")
		prev = m[0]
	}
	b.WriteString(s[prev:])
	return b.String()
}

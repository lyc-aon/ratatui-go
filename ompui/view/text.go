package view

import (
	"strconv"
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

const spaceRun = "                                                                "

// padding returns n spaces without allocating for common small widths.
func padding(n int) string {
	if n <= 0 {
		return ""
	}
	if n <= len(spaceRun) {
		return spaceRun[:n]
	}
	return strings.Repeat(" ", n)
}

// trimPad removes trailing plain spaces so transcript rows copy clean. Rows
// whose tail is an escape sequence (a painted background) are left alone: their
// spaces are part of the paint, not padding.
func trimPad(line string) string {
	return strings.TrimRight(line, " ")
}

// trimPadAll applies trimPad to a freshly allocated slice, in place.
func trimPadAll(lines []string) []string {
	for i, line := range lines {
		lines[i] = trimPad(line)
	}
	return lines
}

// stripANSI returns the visible text of s with every escape sequence removed.
func stripANSI(s string) string {
	if !strings.ContainsRune(s, '\x1b') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, seg := range ansitext.ParseSegments(s) {
		if seg.Kind == "text" {
			b.WriteString(seg.Text)
		}
	}
	return b.String()
}

// hasControls reports whether s contains a C0 control other than tab/newline,
// a DEL, or a C1 control.
func hasControls(s string) bool {
	for _, r := range s {
		if r == '\t' || r == '\n' {
			continue
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return true
		}
	}
	return false
}

// sanitizeText mirrors OMP sanitizeText: untrusted text that carries control
// bytes loses its escapes and controls entirely. Clean text is returned
// unchanged, so ordinary tool output keeps whatever styling the tool emitted.
func sanitizeText(s string) string {
	if !hasControls(s) {
		return s
	}
	stripped := stripANSI(s)
	var b strings.Builder
	b.Grow(len(stripped))
	for _, r := range stripped {
		if r == '\t' || r == '\n' {
			b.WriteRune(r)
			continue
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// flattenLine collapses CR/LF runs to single spaces so a caller-supplied
// fragment can never expand a single-row header into several rows.
func flattenLine(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

// replaceTabs expands tabs to the fixed terminal tab width.
func replaceTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	return strings.ReplaceAll(s, "\t", padding(ansitext.DefaultTabWidth))
}

// previewLines returns up to maxLines non-blank lines, each truncated to
// maxWidth cells. Mirrors OMP getPreviewLines.
func previewLines(text string, maxLines, maxWidth int, ellipsis string) []string {
	if text == "" || maxLines <= 0 {
		return nil
	}
	out := make([]string, 0, maxLines)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, ansitext.TruncateToWidth(replaceTabs(line), maxWidth, ellipsis))
		if len(out) == maxLines {
			break
		}
	}
	return out
}

// fit truncates s to width cells, appending ellipsis when it does not fit.
func fit(s string, width int, ellipsis string) string {
	if width <= 0 {
		return ""
	}
	return ansitext.TruncateToWidth(s, width, ellipsis)
}

// joinMeta joins non-empty fragments with sep.
func joinMeta(sep string, parts ...string) string {
	kept := parts[:0]
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, sep)
}

// rule returns a horizontal rule of width cells built from the given glyph.
func rule(glyph string, width int) string {
	if width <= 0 || glyph == "" {
		return ""
	}
	unit := ansitext.VisibleWidth(glyph)
	if unit <= 0 {
		return ""
	}
	return strings.Repeat(glyph, width/unit)
}

// formatNumber renders a compact count (999, 1.5K, 25K, 1.5M). Mirrors OMP
// formatNumber so the two frontends read identically.
func formatNumber(n int64) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	switch {
	case n < 1_000:
		return strconv.FormatInt(n, 10)
	case n < 10_000:
		return trimOneDecimal(float64(n)/1_000) + "K"
	case n < 1_000_000:
		return strconv.FormatInt((n+500)/1_000, 10) + "K"
	case n < 10_000_000:
		return trimOneDecimal(float64(n)/1_000_000) + "M"
	case n < 1_000_000_000:
		return strconv.FormatInt((n+500_000)/1_000_000, 10) + "M"
	case n < 10_000_000_000:
		return trimOneDecimal(float64(n)/1_000_000_000) + "B"
	default:
		return strconv.FormatInt((n+500_000_000)/1_000_000_000, 10) + "B"
	}
}

func trimOneDecimal(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}

// formatDuration renders a compact elapsed time (123ms, 1.5s, 30m15s, 2h30m).
// Mirrors OMP formatDuration.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0ms"
	}
	ms := d.Milliseconds()
	switch {
	case ms < 1_000:
		return strconv.FormatInt(ms, 10) + "ms"
	case ms < 60_000:
		return trimOneDecimal(float64(ms)/1000) + "s"
	case ms < 3_600_000:
		mins := ms / 60_000
		secs := (ms % 60_000) / 1_000
		if secs > 0 {
			return strconv.FormatInt(mins, 10) + "m" + strconv.FormatInt(secs, 10) + "s"
		}
		return strconv.FormatInt(mins, 10) + "m"
	case ms < 86_400_000:
		hours := ms / 3_600_000
		mins := (ms % 3_600_000) / 60_000
		if mins > 0 {
			return strconv.FormatInt(hours, 10) + "h" + strconv.FormatInt(mins, 10) + "m"
		}
		return strconv.FormatInt(hours, 10) + "h"
	default:
		days := ms / 86_400_000
		hours := (ms % 86_400_000) / 3_600_000
		if hours > 0 {
			return strconv.FormatInt(days, 10) + "d" + strconv.FormatInt(hours, 10) + "h"
		}
		return strconv.FormatInt(days, 10) + "d"
	}
}

// pluralize appends the English plural suffix when count is not 1.
func pluralize(label string, count int) string {
	if count == 1 {
		return label
	}
	switch {
	case strings.HasSuffix(label, "ch"), strings.HasSuffix(label, "sh"),
		strings.HasSuffix(label, "s"), strings.HasSuffix(label, "x"), strings.HasSuffix(label, "z"):
		return label + "es"
	case strings.HasSuffix(label, "y") && len(label) > 1 && !strings.ContainsRune("aeiou", rune(label[len(label)-2])):
		return label[:len(label)-1] + "ies"
	default:
		return label + "s"
	}
}

// clampInt bounds v to [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// frameIndex picks a cycle position for a frame set from a monotonic counter.
// Deterministic: the same snapshot always renders the same glyph.
func frameIndex(counter uint64, frames []string) int {
	if len(frames) == 0 {
		return 0
	}
	return int(counter % uint64(len(frames)))
}

// shortenPath replaces a leading home prefix with "~".
func shortenPath(path, home string) string {
	if home == "" || !strings.HasPrefix(path, home) {
		return path
	}
	suffix := path[len(home):]
	if suffix == "" {
		return "~"
	}
	if suffix[0] == '/' || suffix[0] == '\\' {
		return "~" + suffix
	}
	return path
}

// elidePath shortens a path to width cells by cutting out its middle, so both
// the repo root and the current leaf stay legible.
func elidePath(path string, width int, ellipsis string) string {
	if width <= 0 {
		return ""
	}
	if ansitext.VisibleWidth(path) <= width {
		return path
	}
	half := width/2 - 1
	if half <= 1 {
		return fit(path, width, "")
	}
	head := fit(path, half, "")
	tailBudget := width - ansitext.VisibleWidth(head) - ansitext.VisibleWidth(ellipsis)
	if tailBudget <= 0 {
		return head + ellipsis
	}
	runes := []rune(path)
	tail := string(runes[clampInt(len(runes)-tailBudget, 0, len(runes)):])
	return head + ellipsis + tail
}

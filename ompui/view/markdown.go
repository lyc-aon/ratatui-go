package view

import (
	"regexp"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

// ── Reflow detection ────────────────────────────────────────────────────

// codeFencePattern matches an opening or closing fence: three or more
// backticks/tildes plus an info string.
var codeFencePattern = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")

// tableDelimiterPattern matches a GFM table delimiter row (`| --- | :--: |`,
// with or without bounding pipes). The header row alone renders as prose; this
// delimiter is what makes markdown lay a table out.
var tableDelimiterPattern = regexp.MustCompile(`^ {0,3}\|?(?:[ \t]*:?-+:?[ \t]*\|)+[ \t]*:?-*:?[ \t]*$`)

// mermaidInfoPattern matches a mermaid fence info string.
var mermaidInfoPattern = regexp.MustCompile(`^mermaid\b`)

// HasReflowingMarkdown reports whether text currently contains markdown whose
// layout is not yet permanent: an open mermaid fence (the diagram reshapes as
// source arrives) or a GFM table (columns re-align as rows arrive).
//
// This is what keeps a streaming table out of native scrollback. Committing an
// intermediate layout strands a stale fragment in immutable history that only a
// full repaint can clear, so such a block stays wholly repaintable and commits
// once, at its final layout.
//
// Fence-aware: table delimiters inside ordinary fenced code (shell pipes, ASCII
// separators, doc examples) are ignored, so a long streamed code block is never
// held back. A delimiter counts only directly under a pipe-bearing header row,
// outside any code fence.
func HasReflowingMarkdown(text string) bool {
	fence := ""
	prev := ""
	for _, line := range strings.Split(text, "\n") {
		match := codeFencePattern.FindStringSubmatch(line)
		if fence != "" {
			// Inside a code block: only a bare matching closing fence ends it.
			if match != nil && strings.TrimSpace(match[2]) == "" &&
				match[1][0] == fence[0] && len(match[1]) >= len(fence) {
				fence = ""
			}
			continue
		}
		if match != nil {
			if mermaidInfoPattern.MatchString(strings.TrimSpace(match[2])) {
				return true
			}
			fence = match[1]
			prev = ""
			continue
		}
		if strings.Contains(prev, "|") && tableDelimiterPattern.MatchString(line) {
			return true
		}
		prev = line
	}
	return false
}

// ── Streaming block boundaries ──────────────────────────────────────────

// Display-math openers. `$$` and `\[` open a block unless the same line also
// closes it, which is how `$$x = y$$` stays a zero net toggle.
var (
	fenceOpenPattern = regexp.MustCompile("^(?:`{3,}|~{3,})")
	mathOpenDollar   = regexp.MustCompile(`^\$\$`)
	mathCloseDollar  = regexp.MustCompile(`\$\$$`)
	mathOpenBracket  = regexp.MustCompile(`^\\\[`)
	mathCloseBracket = regexp.MustCompile(`\\\]$`)
)

// streamScan is the forward scanner over an in-flight assistant body. It keeps
// fence and display-math state plus the scan position, so a delta only touches
// newly arrived complete lines.
//
// Port of Hermes streamingMarkdown.tsx. The invariants that make it correct:
//   - only newline-terminated lines are judged; a partial trailing line may yet
//     become a fence opener, so it stays in the tail;
//   - blank-line boundaries can never be retroactively merged, so a committed
//     block is final;
//   - an unmatched math opener counts as open forever, which is the
//     conservative direction: it withholds a boundary instead of inventing one.
type streamScan struct {
	// settledLen is the length of the committed prefix.
	settledLen int
	// scanned is how far complete lines have been folded into the state.
	scanned int
	// codeOpen marks an unclosed ``` / ~~~ fence at the scan position.
	codeOpen bool
	// mathOpener is the unclosed display-math opener, or "" outside one.
	mathOpener string
}

// advance consumes newly arrived complete lines of text, committing the prefix at
// every blank-line boundary outside a fence. Re-calling with the same text is a
// no-op, so the scanner is idempotent; text must only ever extend what it was
// last called with, which is what streaming guarantees.
func (s *streamScan) advance(text string) {
	i := s.scanned
	for i < len(text) {
		nl := strings.IndexByte(text[i:], '\n')
		if nl < 0 {
			break // partial trailing line: it could still open a fence
		}
		nl += i
		if nl == i {
			// Second half of a "\n\n" outside any fence: the prefix is settled.
			if i > 0 && !s.codeOpen && s.mathOpener == "" {
				if block := text[s.settledLen : nl+1]; strings.TrimSpace(block) != "" {
					s.settledLen = nl + 1
				}
			}
		} else {
			s.applyLine(strings.TrimSpace(text[i:nl]))
		}
		i = nl + 1
	}
	s.scanned = i
}

// applyLine folds one complete line into the fence and math state. A fence
// toggles; math inside an open fence is inert; a closer counts only against a
// pending opener.
func (s *streamScan) applyLine(line string) {
	if fenceOpenPattern.MatchString(line) {
		s.codeOpen = !s.codeOpen
		return
	}
	if s.codeOpen {
		return
	}
	switch {
	case s.mathOpener == "":
		switch {
		case mathOpenDollar.MatchString(line) && !(len(line) >= 4 && mathCloseDollar.MatchString(line)):
			s.mathOpener = "$$"
		case mathOpenBracket.MatchString(line) && !mathCloseBracket.MatchString(line):
			s.mathOpener = `\[`
		}
	case s.mathOpener == "$$" && mathCloseDollar.MatchString(line):
		s.mathOpener = ""
	case s.mathOpener == `\[` && mathCloseBracket.MatchString(line):
		s.mathOpener = ""
	}
}

// FindStableBoundary returns the index just past the last committed block
// boundary of a streaming body, or -1 when nothing has settled yet. Everything
// before it is final markdown; everything after is the in-flight tail that still
// re-renders on every delta.
func FindStableBoundary(text string) int {
	var scan streamScan
	scan.advance(text)
	if scan.settledLen > 0 {
		return scan.settledLen
	}
	return -1
}

// liveTailRows is a lower bound on the rendered rows of the in-flight tail: its
// physical lines, since wrapping only ever adds rows. The transcript holds these
// back from the commit-safe prefix, which is the seam equivalent of the oracle's
// "only the tail re-parses" invariant — a settled paragraph reaches immutable
// scrollback while the paragraph still being written does not.
func liveTailRows(text string) int {
	start := 0
	if b := FindStableBoundary(text); b > 0 {
		start = b
	}
	tail := text[start:]
	if tail == "" {
		return 0
	}
	return strings.Count(tail, "\n") + 1
}

// ── Diff segments ───────────────────────────────────────────────────────

// diffRows renders a patch segment: additions, deletions, hunk headers, and
// context, each in its own voice. Mirrors the `diff` fence branch of Hermes'
// markdown renderer, applied to a whole inline-diff segment so a patch pushed
// between narration segments reads as a patch rather than as prose.
func (r Renderer) diffRows(text string, layout Layout) []string {
	body := strings.Trim(stripDiffFence(text), "\n")
	if body == "" {
		return nil
	}
	inset := padding(layout.Inset)
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, trimPad(inset+" "+apply(r.diffStyle(line),
			fit(replaceTabs(line), layout.Body-1, r.theme.Symbols.Ellipsis))))
	}
	return out
}

// diffStyle picks the voice for one patch line.
func (r Renderer) diffStyle(line string) StyleFunc {
	switch {
	case strings.HasPrefix(line, "@@"):
		return r.theme.Muted
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		return r.theme.Muted
	case strings.HasPrefix(line, "+"):
		return r.theme.Success
	case strings.HasPrefix(line, "-"):
		return r.theme.Error
	default:
		return r.theme.Dim
	}
}

// stripDiffFence unwraps a ```diff fence so a segment arrives at the renderer as
// bare patch lines whether or not the core wrapped it.
func stripDiffFence(text string) string {
	lines := strings.Split(text, "\n")
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= end {
		return ""
	}
	match := codeFencePattern.FindStringSubmatch(lines[start])
	if match == nil {
		return text
	}
	start++
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if end > start {
		if closing := codeFencePattern.FindStringSubmatch(lines[end-1]); closing != nil &&
			strings.TrimSpace(closing[2]) == "" {
			end--
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// eventRow renders a timeline marker (a model switch, a delegation completion):
// a dim diamond behind the assistant gutter, never an opaque message row.
func (r Renderer) eventRow(text string, layout Layout) []string {
	flat := flattenLine(strings.TrimSpace(text))
	if flat == "" {
		return nil
	}
	row := apply(r.theme.Dim, r.theme.Symbols.Info+" "+flat)
	return []string{padding(layout.Inset) + fit(row, layout.Body, r.theme.Symbols.Ellipsis)}
}

// wrapPlain wraps plain text to width, styling every row. Used where the oracle
// renders a bare <Text wrap="wrap-trim"> rather than markdown.
func (r Renderer) wrapPlain(text string, width int, style StyleFunc) []string {
	if width < 1 {
		width = 1
	}
	rows := ansitext.WrapANSI(replaceTabs(text), width)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, apply(style, row))
	}
	return out
}

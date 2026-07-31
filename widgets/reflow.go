package widgets

import (
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/text"
)

// wrappedLine is one reflowed output line ready for painting.
type wrappedLine struct {
	Graphemes []text.StyledGrapheme
	Width     int
	Alignment layout.Alignment
}

// lineComposer yields wrapped/truncated lines one at a time.
type lineComposer interface {
	NextLine() (wrappedLine, bool)
}

// inputLine is one source line already materialised into graphemes.
type inputLine struct {
	Graphemes []text.StyledGrapheme
	Alignment layout.Alignment
}

// wordWrapper wraps on word boundaries; long words hard-break.
// Symbols wider than maxWidth are dropped.
type wordWrapper struct {
	lines        []inputLine
	lineIdx      int
	maxLineWidth int
	trim         bool
	wrappedLines [][]text.StyledGrapheme
	currentAlign layout.Alignment
	pendingWord  []text.StyledGrapheme
	pendingWS    []text.StyledGrapheme
}

func newWordWrapper(lines []inputLine, maxWidth int, trim bool) *wordWrapper {
	return &wordWrapper{
		lines:        lines,
		maxLineWidth: maxWidth,
		trim:         trim,
	}
}

func (w *wordWrapper) NextLine() (wrappedLine, bool) {
	if w.maxLineWidth == 0 {
		return wrappedLine{}, false
	}

	for {
		if len(w.wrappedLines) > 0 {
			line := w.wrappedLines[0]
			w.wrappedLines = w.wrappedLines[1:]
			width := 0
			for i := range line {
				width += text.GraphemeWidth(line[i].Symbol)
			}
			return wrappedLine{
				Graphemes: line,
				Width:     width,
				Alignment: w.currentAlign,
			}, true
		}
		if w.lineIdx >= len(w.lines) {
			return wrappedLine{}, false
		}
		src := w.lines[w.lineIdx]
		w.lineIdx++
		w.currentAlign = src.Alignment
		w.processInput(src.Graphemes)
	}
}

func (w *wordWrapper) processInput(lineSymbols []text.StyledGrapheme) {
	pendingLine := make([]text.StyledGrapheme, 0, len(lineSymbols))
	lineWidth := 0
	wordWidth := 0
	whitespaceWidth := 0
	nonWhitespacePrevious := false

	w.pendingWord = w.pendingWord[:0]
	w.pendingWS = w.pendingWS[:0]

	for _, grapheme := range lineSymbols {
		isWhitespace := grapheme.IsWhitespace()
		symbolWidth := text.GraphemeWidth(grapheme.Symbol)

		// Ignore symbols wider than the line limit.
		if symbolWidth > w.maxLineWidth {
			continue
		}

		wordFound := nonWhitespacePrevious && isWhitespace
		// Current word would overflow after removing whitespace.
		trimmedOverflow := len(pendingLine) == 0 &&
			w.trim &&
			wordWidth+symbolWidth > w.maxLineWidth
		// Separated whitespace would overflow on its own.
		whitespaceOverflow := len(pendingLine) == 0 &&
			w.trim &&
			whitespaceWidth+symbolWidth > w.maxLineWidth
		// Current full word (including whitespace) would overflow.
		untrimmedOverflow := len(pendingLine) == 0 &&
			!w.trim &&
			wordWidth+whitespaceWidth+symbolWidth > w.maxLineWidth

		// Append finished segment to current line.
		if wordFound || trimmedOverflow || whitespaceOverflow || untrimmedOverflow {
			if len(pendingLine) > 0 || !w.trim {
				pendingLine = append(pendingLine, w.pendingWS...)
				lineWidth += whitespaceWidth
			}
			pendingLine = append(pendingLine, w.pendingWord...)
			lineWidth += wordWidth

			w.pendingWS = w.pendingWS[:0]
			whitespaceWidth = 0
			wordWidth = 0
			w.pendingWord = w.pendingWord[:0]
		}

		lineFull := lineWidth >= w.maxLineWidth
		pendingWordOverflow := symbolWidth > 0 &&
			lineWidth+whitespaceWidth+wordWidth >= w.maxLineWidth

		// Emit finished wrapped line.
		if lineFull || pendingWordOverflow {
			remainingWidth := w.maxLineWidth - lineWidth
			if remainingWidth < 0 {
				remainingWidth = 0
			}

			w.wrappedLines = append(w.wrappedLines, pendingLine)
			pendingLine = make([]text.StyledGrapheme, 0, len(lineSymbols))
			lineWidth = 0

			// Drop whitespace that fits in the remainder of the finished line.
			for len(w.pendingWS) > 0 {
				width := text.GraphemeWidth(w.pendingWS[0].Symbol)
				if width > remainingWidth {
					break
				}
				whitespaceWidth -= width
				remainingWidth -= width
				w.pendingWS = w.pendingWS[1:]
			}

			// Don't count first whitespace toward next word.
			if isWhitespace && len(w.pendingWS) == 0 {
				continue
			}
		}

		// Append symbol to a pending buffer.
		if isWhitespace {
			whitespaceWidth += symbolWidth
			w.pendingWS = append(w.pendingWS, grapheme)
		} else {
			wordWidth += symbolWidth
			w.pendingWord = append(w.pendingWord, grapheme)
		}

		nonWhitespacePrevious = !isWhitespace
	}

	// Append remaining text parts.
	if len(pendingLine) == 0 &&
		len(w.pendingWord) == 0 &&
		len(w.pendingWS) > 0 &&
		w.trim {
		w.wrappedLines = append(w.wrappedLines, []text.StyledGrapheme{})
	}
	if len(pendingLine) > 0 || !w.trim {
		pendingLine = append(pendingLine, w.pendingWS...)
		w.pendingWS = w.pendingWS[:0]
	}
	pendingLine = append(pendingLine, w.pendingWord...)
	w.pendingWord = w.pendingWord[:0]

	if len(pendingLine) > 0 {
		w.wrappedLines = append(w.wrappedLines, pendingLine)
	}
	if len(w.wrappedLines) == 0 {
		w.wrappedLines = append(w.wrappedLines, []text.StyledGrapheme{})
	}
}

// lineTruncator truncates overhanging lines and optionally applies horizontal scroll.
type lineTruncator struct {
	lines            []inputLine
	lineIdx          int
	maxLineWidth     int
	horizontalOffset int
}

func newLineTruncator(lines []inputLine, maxWidth, hScroll int) *lineTruncator {
	if hScroll < 0 {
		hScroll = 0
	}
	return &lineTruncator{
		lines:            lines,
		maxLineWidth:     maxWidth,
		horizontalOffset: hScroll,
	}
}

func (t *lineTruncator) setHorizontalOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	t.horizontalOffset = offset
}

func (t *lineTruncator) NextLine() (wrappedLine, bool) {
	if t.maxLineWidth == 0 {
		return wrappedLine{}, false
	}
	if t.lineIdx >= len(t.lines) {
		return wrappedLine{}, false
	}

	src := t.lines[t.lineIdx]
	t.lineIdx++
	alignment := src.Alignment

	current := make([]text.StyledGrapheme, 0, len(src.Graphemes))
	currentWidth := 0
	horizontalOffset := t.horizontalOffset

	for _, g := range src.Graphemes {
		symWidth := text.GraphemeWidth(g.Symbol)
		// Ignore characters wider than the total max width.
		if symWidth > t.maxLineWidth {
			continue
		}
		if currentWidth+symWidth > t.maxLineWidth {
			break
		}

		symbol := g.Symbol
		if horizontalOffset > 0 && alignment == layout.AlignLeft {
			w := text.GraphemeWidth(symbol)
			if w > horizontalOffset {
				symbol = trimOffset(symbol, horizontalOffset)
				horizontalOffset = 0
			} else {
				horizontalOffset -= w
				symbol = ""
			}
		}
		currentWidth += text.GraphemeWidth(symbol)
		current = append(current, text.StyledGrapheme{Symbol: symbol, Style: g.Style})
	}

	return wrappedLine{
		Graphemes: current,
		Width:     currentWidth,
		Alignment: alignment,
	}, true
}

// trimOffset returns the suffix of src after skipping offset terminal cells
// from the start. Does not split grapheme clusters.
func trimOffset(src string, offset int) string {
	if offset <= 0 || src == "" {
		return src
	}
	// Walk grapheme clusters (control clusters already filtered upstream).
	gs := text.Graphemes(src)
	if len(gs) == 0 {
		w := text.GraphemeWidth(src)
		if w <= offset {
			return ""
		}
		return src
	}
	start := 0
	for _, g := range gs {
		w := text.GraphemeWidth(g)
		if w <= offset {
			offset -= w
			start += len(g)
			continue
		}
		break
	}
	return src[start:]
}

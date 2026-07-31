package editor

import (
	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/textutil"
	"github.com/rivo/uniseg"
)

// textChunk is one word-wrapped visual segment of a logical line.
// start/end are UTF-8 byte offsets into the original logical line.
type textChunk struct {
	text  string
	start int
	end   int
}

type wrapToken struct {
	text       string
	start, end int
	ws         bool
}

// wordWrapLine splits a plain logical line into visual chunks at maxWidth cells.
// Matches OMP editor wordWrapLine: word boundaries preferred, long tokens hard-
// break by grapheme, leading wrap whitespace is skipped but span-mapped.
func wordWrapLine(line string, maxWidth int) []textChunk {
	if line == "" || maxWidth <= 0 {
		return []textChunk{{text: "", start: 0, end: 0}}
	}
	if ansitext.VisibleWidth(line) <= maxWidth {
		return []textChunk{{text: line, start: 0, end: len(line)}}
	}

	tokens := tokenizeWrap(line)
	chunks := make([]textChunk, 0, 4)

	current := ""
	currentW := 0
	chunkStart := 0
	atLineStart := true

	push := func(text string, start, end int) {
		chunks = append(chunks, textChunk{text: text, start: start, end: end})
	}
	extendPrevEnd := func(end int) {
		if n := len(chunks); n > 0 {
			chunks[n-1].end = end
		}
	}

	for _, tok := range tokens {
		tw := ansitext.VisibleWidth(tok.text)

		if atLineStart && tok.ws {
			extendPrevEnd(tok.end)
			chunkStart = tok.end
			continue
		}
		atLineStart = false

		if tw > maxWidth {
			consumed, consumedLen := "", 0
			if current != "" && currentW < maxWidth {
				consumed, consumedLen = consumePrefixToWidth(tok.text, maxWidth-currentW)
			}
			if current != "" {
				if consumed != "" {
					push(current+consumed, chunkStart, tok.start+consumedLen)
					current = ""
					currentW = 0
					chunkStart = tok.start + consumedLen
				} else {
					push(current, chunkStart, tok.start)
					current = ""
					currentW = 0
					chunkStart = tok.start
					consumed = ""
					consumedLen = 0
				}
			}
			rest := tok.text
			restStart := tok.start
			if consumedLen > 0 {
				rest = tok.text[consumedLen:]
				restStart = tok.start + consumedLen
			}
			var tc string
			tcW := 0
			tcStart := restStart
			off := restStart
			gr := uniseg.NewGraphemes(rest)
			for gr.Next() {
				g := gr.Str()
				gw := ansitext.VisibleWidth(g)
				if tc != "" && tcW+gw > maxWidth {
					push(tc, tcStart, off)
					tc = g
					tcW = gw
					tcStart = off
				} else {
					tc += g
					tcW += gw
				}
				off += len(g)
			}
			if tc != "" {
				current = tc
				currentW = tcW
				chunkStart = tcStart
			}
			continue
		}

		if currentW+tw > maxWidth {
			if current != "" && !tok.ws && currentW < maxWidth && hasWideGrapheme(tok.text) {
				consumed, consumedLen := consumePrefixToWidth(tok.text, maxWidth-currentW)
				if consumed != "" {
					push(current+consumed, chunkStart, tok.start+consumedLen)
					rest := tok.text[consumedLen:]
					current = rest
					currentW = ansitext.VisibleWidth(rest)
					chunkStart = tok.start + consumedLen
					atLineStart = false
					continue
				}
			}
			trimmed := trimRightSpaces(current)
			if trimmed != "" || len(chunks) == 0 {
				push(trimmed, chunkStart, chunkStart+len(current))
			} else {
				extendPrevEnd(chunkStart + len(current))
			}
			atLineStart = true
			if tok.ws {
				extendPrevEnd(tok.end)
				current = ""
				currentW = 0
				chunkStart = tok.end
			} else {
				current = tok.text
				currentW = tw
				chunkStart = tok.start
				atLineStart = false
			}
			continue
		}

		current += tok.text
		currentW += tw
	}

	if current != "" {
		push(current, chunkStart, len(line))
	}
	if len(chunks) == 0 {
		return []textChunk{{text: "", start: 0, end: 0}}
	}
	return chunks
}

func tokenizeWrap(line string) []wrapToken {
	var tokens []wrapToken
	var cur string
	start := 0
	inWS := false
	off := 0
	gr := uniseg.NewGraphemes(line)
	for gr.Next() {
		g := gr.Str()
		ws := textutil.Kind(g) == textutil.WordWhitespace
		if cur == "" {
			inWS = ws
			start = off
		} else if ws != inWS {
			tokens = append(tokens, wrapToken{text: cur, start: start, end: off, ws: inWS})
			cur = ""
			start = off
			inWS = ws
		}
		cur += g
		off += len(g)
	}
	if cur != "" {
		tokens = append(tokens, wrapToken{text: cur, start: start, end: off, ws: inWS})
	}
	return tokens
}

func consumePrefixToWidth(text string, available int) (string, int) {
	if available <= 0 || text == "" {
		return "", 0
	}
	var prefix string
	pw := 0
	lenBytes := 0
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		g := gr.Str()
		gw := ansitext.VisibleWidth(g)
		if pw+gw > available {
			break
		}
		prefix += g
		pw += gw
		lenBytes += len(g)
		if pw == available {
			break
		}
	}
	return prefix, lenBytes
}

func hasWideGrapheme(text string) bool {
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		if ansitext.VisibleWidth(gr.Str()) > 1 {
			return true
		}
	}
	return false
}

func trimRightSpaces(s string) string {
	i := len(s)
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return s[:i]
}

// visualColAtOffset returns the cell column of UTF-8 byte offset within text.
func visualColAtOffset(text string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	col := 0
	off := 0
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		if off >= offset {
			break
		}
		g := gr.Str()
		col += ansitext.VisibleWidth(g)
		off += len(g)
	}
	return col
}

// offsetAtVisualCol returns the UTF-8 byte offset of visual cell col in text,
// snapped to a grapheme boundary (never splits a cluster).
func offsetAtVisualCol(text string, col int) int {
	if col <= 0 {
		return 0
	}
	cur := 0
	off := 0
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		g := gr.Str()
		w := ansitext.VisibleWidth(g)
		if cur+w > col {
			return off
		}
		cur += w
		off += len(g)
	}
	return len(text)
}

// maxSegmentVisualCol is the highest caret cell on a wrap segment.
// Non-last segments clamp before the final grapheme (segment end is next start).
func maxSegmentVisualCol(text string, isLast bool) int {
	total := 0
	lastW := 0
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		lastW = ansitext.VisibleWidth(gr.Str())
		total += lastW
	}
	if isLast {
		return total
	}
	if total-lastW < 0 {
		return 0
	}
	return total - lastW
}

// graphemeBefore returns the last grapheme ending at or before byte offset, and its byte length.
func graphemeBefore(s string, offset int) (g string, n int) {
	if offset <= 0 || s == "" {
		return "", 0
	}
	if offset > len(s) {
		offset = len(s)
	}
	prefix := s[:offset]
	var last string
	gr := uniseg.NewGraphemes(prefix)
	for gr.Next() {
		last = gr.Str()
	}
	return last, len(last)
}

// graphemeAfter returns the first grapheme starting at byte offset, and its byte length.
func graphemeAfter(s string, offset int) (g string, n int) {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(s) {
		return "", 0
	}
	gr := uniseg.NewGraphemes(s[offset:])
	if gr.Next() {
		g = gr.Str()
		return g, len(g)
	}
	return "", 0
}

// clampByteOffset snaps offset onto a UTF-8 code-point boundary within s.
func clampByteOffset(s string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(s) {
		return len(s)
	}
	for offset > 0 && (s[offset]&0xC0) == 0x80 {
		offset--
	}
	return offset
}

// isWordGrapheme reports whether g is non-whitespace for undo-coalesce typing.
func isWordGrapheme(g string) bool {
	return textutil.Kind(g) != textutil.WordWhitespace
}

// allWordGraphemes reports whether every grapheme in s is non-whitespace.
func allWordGraphemes(s string) bool {
	if s == "" {
		return false
	}
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		if !isWordGrapheme(gr.Str()) {
			return false
		}
	}
	return true
}

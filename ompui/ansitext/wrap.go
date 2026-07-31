package ansitext

import (
	"strings"

	"github.com/rivo/uniseg"
)

// SGR attribute bits matching pi-natives AnsiState (bit 5 intentionally unused).
const (
	attrBold      uint16 = 1 << 0
	attrDim       uint16 = 1 << 1
	attrItalic    uint16 = 1 << 2
	attrUnderline uint16 = 1 << 3
	attrBlink     uint16 = 1 << 4
	attrInverse   uint16 = 1 << 6
	attrHidden    uint16 = 1 << 7
	attrStrike    uint16 = 1 << 8
)

// colorVal encodes a tracked fg/bg color the way pi-natives does:
//
//	0            unset (COLOR_NONE)
//	1..8         basic ANSI (30-37 / 40-47)
//	9..16        bright ANSI (90-97 / 100-107)
//	0x100|idx    indexed 256-color
//	0x1rrggbb    truecolor (high bit 0x1000000 set)
type colorVal uint32

const colorNone colorVal = 0

// ansiState is the active SGR snapshot carried across wrap line breaks.
type ansiState struct {
	attrs uint16
	fg    colorVal
	bg    colorVal
}

func (s *ansiState) reset() {
	*s = ansiState{}
}

func (s *ansiState) empty() bool {
	return s.attrs == 0 && s.fg == colorNone && s.bg == colorNone
}

// applySGR applies the parameter body of a CSI … m sequence (no ESC/[ /m).
func (s *ansiState) applySGR(params string) {
	if params == "" {
		s.reset()
		return
	}
	i := 0
	for i < len(params) {
		code, ni := parseSGRNum(params, i)
		i = ni
		switch {
		case code == 0:
			s.reset()
		case code == 1:
			s.attrs |= attrBold
		case code == 2:
			s.attrs |= attrDim
		case code == 3:
			s.attrs |= attrItalic
		case code == 4:
			s.attrs |= attrUnderline
		case code == 5:
			s.attrs |= attrBlink
		case code == 7:
			s.attrs |= attrInverse
		case code == 8:
			s.attrs |= attrHidden
		case code == 9:
			s.attrs |= attrStrike
		case code == 21:
			s.attrs &^= attrBold
		case code == 22:
			s.attrs &^= attrBold | attrDim
		case code == 23:
			s.attrs &^= attrItalic
		case code == 24:
			s.attrs &^= attrUnderline
		case code == 25:
			s.attrs &^= attrBlink
		case code == 27:
			s.attrs &^= attrInverse
		case code == 28:
			s.attrs &^= attrHidden
		case code == 29:
			s.attrs &^= attrStrike
		case code >= 30 && code <= 37:
			s.fg = colorVal(code - 29)
		case code == 39:
			s.fg = colorNone
		case code >= 40 && code <= 47:
			s.bg = colorVal(code - 39)
		case code == 49:
			s.bg = colorNone
		case code >= 90 && code <= 97:
			s.fg = colorVal(code - 81)
		case code >= 100 && code <= 107:
			s.bg = colorVal(code - 91)
		case code == 38 || code == 48:
			mode, ni := parseSGRNum(params, i)
			i = ni
			var col colorVal
			switch mode {
			case 5:
				idx, ni := parseSGRNum(params, i)
				i = ni
				col = 0x100 | colorVal(idx&0xff)
			case 2:
				r, ni := parseSGRNum(params, i)
				g, ni := parseSGRNum(params, ni)
				b, ni := parseSGRNum(params, ni)
				i = ni
				col = 0x1000000 |
					colorVal(r&0xff)<<16 |
					colorVal(g&0xff)<<8 |
					colorVal(b&0xff)
			default:
				continue
			}
			if code == 38 {
				s.fg = col
			} else {
				s.bg = col
			}
		}
	}
}

// parseSGRNum reads a decimal parameter starting at i, skipping a leading ';'.
// Returns (value, index-after-token). Non-digit bytes inside a token are ignored
// for the value (matching natives' saturating parse that still advances).
func parseSGRNum(params string, i int) (uint32, int) {
	for i < len(params) && params[i] == ';' {
		i++
	}
	var val uint32
	for i < len(params) {
		b := params[i]
		if b == ';' {
			i++
			break
		}
		if b >= '0' && b <= '9' {
			val = val*10 + uint32(b-'0')
		}
		i++
	}
	return val, i
}

func (s *ansiState) writeRestore(b *strings.Builder) {
	if s.empty() {
		return
	}
	b.WriteByte(esc)
	b.WriteByte('[')
	first := true
	push := func(code uint32) {
		if !first {
			b.WriteByte(';')
		}
		first = false
		writeUint(b, code)
	}
	if s.attrs&attrBold != 0 {
		push(1)
	}
	if s.attrs&attrDim != 0 {
		push(2)
	}
	if s.attrs&attrItalic != 0 {
		push(3)
	}
	if s.attrs&attrUnderline != 0 {
		push(4)
	}
	if s.attrs&attrBlink != 0 {
		push(5)
	}
	if s.attrs&attrInverse != 0 {
		push(7)
	}
	if s.attrs&attrHidden != 0 {
		push(8)
	}
	if s.attrs&attrStrike != 0 {
		push(9)
	}
	writeColor(b, s.fg, 38, &first)
	writeColor(b, s.bg, 48, &first)
	b.WriteByte('m')
}

func writeColor(b *strings.Builder, color colorVal, base uint32, first *bool) {
	if color == colorNone {
		return
	}
	if !*first {
		b.WriteByte(';')
	}
	*first = false
	switch {
	case color < 0x100:
		code := uint32(color)
		if code <= 8 {
			code = code + 29
		} else {
			code = code + 81
		}
		if base == 48 {
			code += 10
		}
		writeUint(b, code)
	case color < 0x1000000:
		writeUint(b, base)
		b.WriteString(";5;")
		writeUint(b, uint32(color&0xff))
	default:
		writeUint(b, base)
		b.WriteString(";2;")
		writeUint(b, uint32((color>>16)&0xff))
		b.WriteByte(';')
		writeUint(b, uint32((color>>8)&0xff))
		b.WriteByte(';')
		writeUint(b, uint32(color&0xff))
	}
}

func writeUint(b *strings.Builder, val uint32) {
	if val == 0 {
		b.WriteByte('0')
		return
	}
	var buf [10]byte
	n := 0
	for val > 0 {
		buf[n] = byte('0' + val%10)
		val /= 10
		n++
	}
	for i := n - 1; i >= 0; i-- {
		b.WriteByte(buf[i])
	}
}

func writeActiveCodes(state *ansiState, b *strings.Builder) {
	state.writeRestore(b)
}

// writeLineEndReset closes underline/strikethrough only so colors stay live for
// the continuation line's restore prefix (OMP wrap contract).
func writeLineEndReset(state *ansiState, b *strings.Builder) {
	hasU := state.attrs&attrUnderline != 0
	hasS := state.attrs&attrStrike != 0
	if !hasU && !hasS {
		return
	}
	b.WriteByte(esc)
	b.WriteByte('[')
	if hasU {
		b.WriteString("24")
		if hasS {
			b.WriteByte(';')
		}
	}
	if hasS {
		b.WriteString("29")
	}
	b.WriteByte('m')
}

func updateStateFromText(data string, state *ansiState) {
	i := 0
	n := len(data)
	for i < n {
		if data[i] == esc {
			if seqLen := ansiSeqLen(data, i); seqLen > 0 {
				seq := data[i : i+seqLen]
				if isSGR(seq) {
					// body between ESC[ and trailing m
					state.applySGR(seq[2 : len(seq)-1])
				}
				i += seqLen
				continue
			}
		}
		i++
	}
}

// WrapANSI wraps text to a visible cell width while preserving ANSI/OSC/APC
// sequences across breaks. Matches OMP wrapTextWithAnsi / pi-natives
// wrap_text_with_ansi:
//
//   - Explicit '\n' splits first; each physical line is then word-wrapped.
//   - Words break on ASCII space runs; tabs stay inside tokens (width 3).
//   - Long tokens hard-break grapheme-safely; OSC 66 spans stay atomic (may
//     alone exceed width); malformed ESC advances one byte (no spin).
//   - Continuation lines restore active SGR; underline/strike close at soft
//     breaks without a full CSI 0 m so colors survive.
//   - Trailing ASCII spaces on each output row are trimmed.
//
// Empty input returns a single empty string. width < 1 hard-breaks every cell.
func WrapANSI(text string, width int) []string {
	if width < 0 {
		width = 0
	}
	if text == "" {
		return []string{""}
	}

	var result []string
	var state ansiState
	lineStart := 0
	n := len(text)

	for i := 0; i <= n; i++ {
		if i != n && text[i] != '\n' {
			continue
		}
		line := text[lineStart:i]
		var lineWithPrefix strings.Builder
		if len(result) > 0 {
			writeActiveCodes(&state, &lineWithPrefix)
		}
		lineWithPrefix.WriteString(line)
		wrapped := wrapSingleLine(lineWithPrefix.String(), width)
		result = append(result, wrapped...)
		updateStateFromText(line, &state)
		lineStart = i + 1
	}

	if len(result) == 0 {
		result = []string{""}
	}
	return result
}

func wrapSingleLine(line string, width int) []string {
	if line == "" {
		return []string{""}
	}
	if VisibleWidth(line) <= width {
		return []string{line}
	}

	tokens := splitIntoTokensWithANSI(line)
	var wrapped []string
	var cur strings.Builder
	curW := 0
	var state ansiState

	for _, token := range tokens {
		tokenW := VisibleWidth(token)
		isWS := tokenIsWhitespace(token)

		if tokenW > width && !isWS {
			if cur.Len() > 0 {
				writeLineEndReset(&state, &cur)
				wrapped = append(wrapped, cur.String())
				cur.Reset()
				curW = 0
			}
			broken := breakLongWord(token, width, &state)
			if len(broken) > 0 {
				last := broken[len(broken)-1]
				wrapped = append(wrapped, broken[:len(broken)-1]...)
				cur.Reset()
				cur.WriteString(last)
				curW = VisibleWidth(last)
			}
			continue
		}

		total := curW + tokenW
		if total > width && curW > 0 {
			lineStr := trimEndSpaces(cur.String())
			var lb strings.Builder
			lb.WriteString(lineStr)
			writeLineEndReset(&state, &lb)
			wrapped = append(wrapped, lb.String())

			cur.Reset()
			writeActiveCodes(&state, &cur)
			if isWS {
				curW = 0
			} else {
				cur.WriteString(token)
				curW = tokenW
			}
		} else {
			cur.WriteString(token)
			curW += tokenW
		}
		updateStateFromText(token, &state)
	}

	if cur.Len() > 0 {
		wrapped = append(wrapped, cur.String())
	}

	for i := range wrapped {
		wrapped[i] = trimEndSpaces(wrapped[i])
	}
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}
	return wrapped
}

func splitIntoTokensWithANSI(line string) []string {
	var tokens []string
	var cur strings.Builder
	var pending strings.Builder
	inWS := false
	i := 0
	n := len(line)

	for i < n {
		if line[i] == esc {
			if seqLen := ansiSeqLen(line, i); seqLen > 0 {
				pending.WriteString(line[i : i+seqLen])
				i += seqLen
				continue
			}
		}

		ch := line[i]
		// Only ASCII space is a wrap separator (tabs stay in word tokens).
		charIsSpace := ch == ' '
		if charIsSpace != inWS && cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
		if pending.Len() > 0 {
			cur.WriteString(pending.String())
			pending.Reset()
		}
		inWS = charIsSpace
		cur.WriteByte(ch)
		i++
	}
	if pending.Len() > 0 {
		cur.WriteString(pending.String())
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func tokenIsWhitespace(token string) bool {
	i := 0
	n := len(token)
	for i < n {
		if token[i] == esc {
			if seqLen := ansiSeqLen(token, i); seqLen > 0 {
				seq := token[i : i+seqLen]
				if _, payload, ok := osc66MetaPayload(seq); ok {
					for j := range payload {
						if payload[j] != ' ' {
							return false
						}
					}
				}
				i += seqLen
				continue
			}
		}
		if token[i] != ' ' {
			return false
		}
		i++
	}
	return true
}

func trimEndSpaces(line string) string {
	end := len(line)
	for end > 0 && line[end-1] == ' ' {
		end--
	}
	return line[:end]
}

func breakLongWord(word string, width int, state *ansiState) []string {
	var lines []string
	var cur strings.Builder
	writeActiveCodes(state, &cur)
	curW := 0
	i := 0
	n := len(word)

	flush := func() {
		writeLineEndReset(state, &cur)
		lines = append(lines, cur.String())
		cur.Reset()
		writeActiveCodes(state, &cur)
		curW = 0
	}

	for i < n {
		if word[i] == esc {
			if seqLen := ansiSeqLen(word, i); seqLen > 0 {
				seq := word[i : i+seqLen]
				if info, ok := osc66Info(seq); ok {
					// Atomic OSC 66: new line if needed, then whole span
					// (may still exceed width alone — matches natives).
					if curW+info.width > width {
						flush()
					}
					cur.WriteString(seq)
					curW += info.width
					i += seqLen
					continue
				}
				cur.WriteString(seq)
				if isSGR(seq) {
					state.applySGR(seq[2 : len(seq)-1])
				}
				i += seqLen
				continue
			}
			// Unclassified ESC: emit zero-width and advance (no spin).
			cur.WriteByte(esc)
			i++
			continue
		}

		start := i
		ascii := true
		for i < n && word[i] != esc {
			if word[i] > 0x7f {
				ascii = false
			}
			i++
		}
		seg := word[start:i]

		if ascii {
			for j := range seg {
				u := seg[j]
				gw := asciiCellWidth(u)
				if curW+gw > width {
					flush()
				}
				cur.WriteByte(u)
				curW += gw
			}
			continue
		}

		// Grapheme-safe hard break. Emit every cluster (incl. zero-width),
		// matching natives for_each_grapheme which always keeps the slice.
		walkAllGraphemes(seg, func(g string, gw int) {
			if gw > 0 && curW+gw > width {
				flush()
			}
			cur.WriteString(g)
			curW += gw
		})
	}

	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// walkAllGraphemes invokes fn for every grapheme cluster in s, including
// zero-width and control clusters. Used by hard-break so wrap never drops bytes.
func walkAllGraphemes(s string, fn func(g string, gw int)) {
	if s == "" {
		return
	}
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		fn(g, graphemeCellWidth(g))
	}
}

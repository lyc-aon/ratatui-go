package richtext

import (
	"strings"
	"unicode/utf8"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/rivo/uniseg"
)

// WrapTextWithAnsi wraps text to width cells, preserving ANSI/OSC sequences and
// carrying active SGR across soft wraps (OMP wrapTextWithAnsi / pi-natives).
// width <= 0 is treated as 1. Empty input yields a single empty line.
func WrapTextWithAnsi(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	if text == "" {
		return []string{""}
	}

	// Split on hard newlines first; carry SGR state across hard breaks too.
	var result []string
	state := ansiState{}
	start := 0
	for i := 0; i <= len(text); i++ {
		if i == len(text) || text[i] == '\n' {
			line := text[start:i]
			lineWithPrefix := line
			if len(result) > 0 {
				lineWithPrefix = state.activeCodes() + line
			}
			wrapped := wrapSingleLine(lineWithPrefix, width, &state)
			result = append(result, wrapped...)
			// Update state from the original line (not the re-prefixed wrap).
			state.updateFrom(line)
			start = i + 1
		}
	}
	if len(result) == 0 {
		return []string{""}
	}
	return result
}

func wrapSingleLine(line string, width int, outer *ansiState) []string {
	if line == "" {
		return []string{""}
	}
	if ansitext.VisibleWidth(line) <= width {
		return []string{line}
	}

	tokens := splitTokensWithANSI(line)
	var wrapped []string
	var current strings.Builder
	currentWidth := 0
	// Track SGR only for codes emitted on this wrap pass so end-reset is correct.
	state := ansiState{}
	// Seed from outer open codes already present at line start? The line itself
	// carries them as leading SGR tokens; updateFrom handles them as we consume.

	flushLine := func(trimTrailing bool) {
		s := current.String()
		if trimTrailing {
			s = trimTrailingSpacesKeepANSI(s)
		}
		s = state.writeLineEndReset(s)
		wrapped = append(wrapped, s)
		current.Reset()
		currentWidth = 0
	}

	for _, tok := range tokens {
		tokW := ansitext.VisibleWidth(tok)
		isWS := tokenIsWhitespace(tok)

		if tokW > width && !isWS {
			if current.Len() > 0 {
				flushLine(true)
			}
			broken := breakLongWord(tok, width, &state)
			if len(broken) == 0 {
				continue
			}
			// All but last are complete lines; last continues.
			for i := 0; i < len(broken)-1; i++ {
				wrapped = append(wrapped, broken[i])
			}
			last := broken[len(broken)-1]
			current.WriteString(last)
			currentWidth = ansitext.VisibleWidth(last)
			// state already advanced through breakLongWord
			continue
		}

		if currentWidth+tokW > width && currentWidth > 0 {
			flushLine(true)
			// Re-open active styles on the new line.
			if prefix := state.activeCodes(); prefix != "" {
				current.WriteString(prefix)
			}
			if isWS {
				// Drop leading whitespace after wrap.
				state.updateFrom(tok)
				continue
			}
			current.WriteString(tok)
			currentWidth = tokW
			state.updateFrom(tok)
			continue
		}

		current.WriteString(tok)
		currentWidth += tokW
		state.updateFrom(tok)
	}

	if current.Len() > 0 {
		s := trimTrailingSpacesKeepANSI(current.String())
		wrapped = append(wrapped, s)
	}

	// Final pass: trim trailing spaces on every line (OMP does this).
	for i := range wrapped {
		wrapped[i] = trimTrailingSpacesKeepANSI(wrapped[i])
	}
	if len(wrapped) == 0 {
		return []string{""}
	}

	// Sync outer with final open state of this source line.
	if outer != nil {
		*outer = state
	}
	return wrapped
}

func breakLongWord(word string, width int, state *ansiState) []string {
	var lines []string
	var current strings.Builder
	if prefix := state.activeCodes(); prefix != "" {
		current.WriteString(prefix)
	}
	curW := 0

	flush := func() {
		s := state.writeLineEndReset(current.String())
		lines = append(lines, s)
		current.Reset()
		if prefix := state.activeCodes(); prefix != "" {
			current.WriteString(prefix)
		}
		curW = 0
	}

	segs := ansitext.ParseSegments(word)
	for _, seg := range segs {
		switch seg.Kind {
		case "sgr":
			current.WriteString(seg.Text)
			state.applySGR(seg.Text)
		case "osc66":
			w := seg.Width
			if curW+w > width && curW > 0 {
				flush()
			}
			current.WriteString(seg.Text)
			curW += w
		case "osc8", "other":
			current.WriteString(seg.Text)
		default: // text
			text := seg.Text
			// Walk graphemes.
			gr := uniseg.NewGraphemes(text)
			for gr.Next() {
				g := gr.Str()
				gw := ansitext.VisibleWidth(g)
				if gw == 0 {
					current.WriteString(g)
					continue
				}
				if curW+gw > width && curW > 0 {
					flush()
				}
				// If single grapheme wider than width, still emit (OMP keeps it).
				current.WriteString(g)
				curW += gw
			}
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func splitTokensWithANSI(line string) []string {
	// Split into whitespace / non-whitespace tokens; ANSI attaches to the
	// following visible run (or trailing if none).
	var tokens []string
	var current strings.Builder
	var pendingANSI strings.Builder
	inWS := false
	started := false

	segs := ansitext.ParseSegments(line)
	for _, seg := range segs {
		if seg.Kind != "text" {
			pendingANSI.WriteString(seg.Text)
			continue
		}
		// Walk runes in text, splitting on spaces.
		for i := 0; i < len(seg.Text); {
			r, size := utf8.DecodeRuneInString(seg.Text[i:])
			isSpace := r == ' '
			if started && isSpace != inWS && current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			if pendingANSI.Len() > 0 {
				current.WriteString(pendingANSI.String())
				pendingANSI.Reset()
			}
			current.WriteString(seg.Text[i : i+size])
			inWS = isSpace
			started = true
			i += size
		}
	}
	if pendingANSI.Len() > 0 {
		current.WriteString(pendingANSI.String())
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func tokenIsWhitespace(tok string) bool {
	// True if every visible cell is a space (ANSI ignored).
	for _, seg := range ansitext.ParseSegments(tok) {
		if seg.Kind != "text" {
			// OSC66 with non-space payload is not whitespace.
			if seg.Kind == "osc66" && seg.Width > 0 {
				// Check payload roughly: if width>0 and not pure spaces, false.
				// Conservative: any osc66 with width is content.
				return false
			}
			continue
		}
		for i := 0; i < len(seg.Text); i++ {
			if seg.Text[i] != ' ' {
				return false
			}
		}
	}
	return true
}

func trimTrailingSpacesKeepANSI(s string) string {
	// Remove trailing ASCII spaces that are outside/after the last non-space
	// text cell. Trailing pure-space text segments are dropped; trailing ANSI kept.
	segs := ansitext.ParseSegments(s)
	if len(segs) == 0 {
		return s
	}
	// Find last non-space text content.
	end := len(segs)
	for end > 0 {
		seg := segs[end-1]
		if seg.Kind != "text" {
			// Keep trailing ANSI — stop.
			break
		}
		// Trim trailing spaces from this text segment.
		t := strings.TrimRight(seg.Text, " ")
		if t == "" {
			end--
			continue
		}
		if t != seg.Text {
			segs[end-1].Text = t
		}
		break
	}
	var b strings.Builder
	for i := 0; i < end; i++ {
		b.WriteString(segs[i].Text)
	}
	// Also keep any trailing non-text after end? We broke on non-text, so end
	// includes them only if we stopped without trimming. If last was non-text
	// we keep all. Good.
	if end < len(segs) {
		// We stopped because of non-text at end-1... actually break keeps it in range.
	}
	// Append remaining segments from end if we only trimmed pure-space texts
	// but left ANSI after? OMP trims end spaces in place on the byte buffer
	// which can strip spaces even after SGR. Match simple: strip trailing ' ' bytes
	// only when they are outside escapes — already done via segments.
	for i := end; i < len(segs); i++ {
		if segs[i].Kind != "text" {
			b.WriteString(segs[i].Text)
		}
	}
	return b.String()
}

// ansiState tracks open SGR attributes for wrap carry-over.
type ansiState struct {
	// codes holds the last full SGR parameter strings that establish current look.
	// Simplified model: store the concatenation of open style SGR sequences since
	// last full reset, excluding the reset itself.
	open string
	// bold, dim, italic, underline, blink, inverse, hidden, strike
	attrs [9]bool
	fg    string // full SGR params for fg e.g. "38;2;1;2;3" or "31" or ""
	bg    string
}

func (s *ansiState) reset() {
	*s = ansiState{}
}

func (s *ansiState) applySGR(seq string) {
	// seq is ESC [ ... m
	if len(seq) < 3 || seq[len(seq)-1] != 'm' {
		return
	}
	params := seq[2 : len(seq)-1]
	if params == "" {
		s.reset()
		return
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch p {
		case "", "0":
			s.reset()
		case "1":
			s.attrs[0] = true
		case "2":
			s.attrs[1] = true
		case "3":
			s.attrs[2] = true
		case "4":
			s.attrs[3] = true
		case "5", "6":
			s.attrs[4] = true
		case "7":
			s.attrs[5] = true
		case "8":
			s.attrs[6] = true
		case "9":
			s.attrs[7] = true
		case "22":
			s.attrs[0] = false
			s.attrs[1] = false
		case "23":
			s.attrs[2] = false
		case "24":
			s.attrs[3] = false
		case "25":
			s.attrs[4] = false
		case "27":
			s.attrs[5] = false
		case "28":
			s.attrs[6] = false
		case "29":
			s.attrs[7] = false
		case "39":
			s.fg = ""
		case "49":
			s.bg = ""
		default:
			// 30-37, 90-97 fg; 40-47, 100-107 bg; 38/48 extended
			if p == "38" || p == "48" {
				isFG := p == "38"
				rest, consumed := takeColor(parts, i)
				i += consumed
				if isFG {
					s.fg = rest
				} else {
					s.bg = rest
				}
				continue
			}
			if isNum(p) {
				n := atoi(p)
				switch {
				case (n >= 30 && n <= 37) || (n >= 90 && n <= 97):
					s.fg = p
				case (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
					s.bg = p
				}
			}
		}
	}
}

func takeColor(parts []string, i int) (string, int) {
	// parts[i] is 38 or 48
	if i+1 >= len(parts) {
		return parts[i], 0
	}
	mode := parts[i+1]
	switch mode {
	case "5": // 256 color: 38;5;N
		if i+2 < len(parts) {
			return parts[i] + ";" + mode + ";" + parts[i+2], 2
		}
		return parts[i] + ";" + mode, 1
	case "2": // truecolor: 38;2;R;G;B
		if i+4 < len(parts) {
			return parts[i] + ";" + mode + ";" + parts[i+2] + ";" + parts[i+3] + ";" + parts[i+4], 4
		}
		// consume what's left
		var b strings.Builder
		b.WriteString(parts[i])
		n := 0
		for j := i + 1; j < len(parts) && n < 4; j++ {
			b.WriteByte(';')
			b.WriteString(parts[j])
			n++
		}
		return b.String(), n
	default:
		return parts[i] + ";" + mode, 1
	}
}

func (s *ansiState) activeCodes() string {
	var b strings.Builder
	write := func(p string) {
		b.WriteString("\x1b[")
		b.WriteString(p)
		b.WriteByte('m')
	}
	if s.attrs[0] {
		write("1")
	}
	if s.attrs[1] {
		write("2")
	}
	if s.attrs[2] {
		write("3")
	}
	if s.attrs[3] {
		write("4")
	}
	if s.attrs[4] {
		write("5")
	}
	if s.attrs[5] {
		write("7")
	}
	if s.attrs[6] {
		write("8")
	}
	if s.attrs[7] {
		write("9")
	}
	if s.fg != "" {
		write(s.fg)
	}
	if s.bg != "" {
		write(s.bg)
	}
	return b.String()
}

func (s *ansiState) writeLineEndReset(line string) string {
	// Close attributes that would bleed; keep it simple: if any open attr/color, append selective resets
	// matching OMP write_line_end_reset spirit — emit 0m only when something is open,
	// but then next line re-opens via activeCodes. Using full reset is fine if we re-open.
	if s.fg == "" && s.bg == "" && !anyAttr(s.attrs) {
		return line
	}
	return line + ansitext.SegmentReset
}

func anyAttr(a [9]bool) bool {
	for _, v := range a {
		if v {
			return true
		}
	}
	return false
}

func (s *ansiState) updateFrom(text string) {
	for _, seg := range ansitext.ParseSegments(text) {
		if seg.Kind == "sgr" {
			s.applySGR(seg.Text)
		}
	}
}

func isNum(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

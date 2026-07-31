package latex

import (
	"strings"
	"unicode/utf8"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
)

// Box is a rectangular rendered fragment. Every entry in Lines is padded to
// exactly Width visible columns; Baseline is the row that aligns with
// surrounding text when boxes are placed side by side (e.g. the fraction bar).
type Box struct {
	Lines    []string
	Baseline int
	Width    int
}

const barChar = "─"

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func padRight(line string, width int) string {
	return line + spaces(width-ansitext.VisibleWidth(line))
}

func center(line string, width int) string {
	extra := width - ansitext.VisibleWidth(line)
	if extra <= 0 {
		return line
	}
	left := extra >> 1
	return spaces(left) + line + spaces(extra-left)
}

func textBox(text string) Box {
	raw := strings.Split(text, "\n")
	width := 0
	for _, line := range raw {
		if w := ansitext.VisibleWidth(line); w > width {
			width = w
		}
	}
	lines := make([]string, len(raw))
	for i, line := range raw {
		lines[i] = padRight(line, width)
	}
	return Box{Lines: lines, Baseline: (len(raw) - 1) >> 1, Width: width}
}

func hconcat(boxes []Box) Box {
	if len(boxes) == 0 {
		return textBox("")
	}
	if len(boxes) == 1 {
		return boxes[0]
	}
	above, below := 0, 0
	width := 0
	for _, b := range boxes {
		if b.Baseline > above {
			above = b.Baseline
		}
		if d := len(b.Lines) - 1 - b.Baseline; d > below {
			below = d
		}
		width += b.Width
	}
	height := above + below + 1
	lines := make([]string, height)
	for row := range height {
		var bld strings.Builder
		for _, b := range boxes {
			local := row - (above - b.Baseline)
			if local >= 0 && local < len(b.Lines) {
				bld.WriteString(b.Lines[local])
			} else {
				bld.WriteString(spaces(b.Width))
			}
		}
		lines[row] = bld.String()
	}
	return Box{Lines: lines, Baseline: above, Width: width}
}

func fracBox(num, den Box) Box {
	width := num.Width
	if den.Width > width {
		width = den.Width
	}
	width += 2
	lines := make([]string, 0, len(num.Lines)+1+len(den.Lines))
	for _, line := range num.Lines {
		lines = append(lines, center(line, width))
	}
	lines = append(lines, strings.Repeat(barChar, width))
	for _, line := range den.Lines {
		lines = append(lines, center(line, width))
	}
	return Box{Lines: lines, Baseline: len(num.Lines), Width: width}
}

func vconcat(boxes []Box) Box {
	if len(boxes) == 0 {
		return textBox("")
	}
	if len(boxes) == 1 {
		return boxes[0]
	}
	width := 0
	for _, b := range boxes {
		if b.Width > width {
			width = b.Width
		}
	}
	var lines []string
	for _, b := range boxes {
		for _, line := range b.Lines {
			lines = append(lines, padRight(line, width))
		}
	}
	return Box{Lines: lines, Baseline: (len(lines) - 1) >> 1, Width: width}
}

type span struct {
	text string
	end  int
}

func readBraceGroup(src string, i int) span {
	depth := 0
	var out strings.Builder
	j := i
	for j < len(src) {
		c := src[j]
		if c == '\\' {
			out.WriteByte(c)
			if j+1 < len(src) {
				out.WriteByte(src[j+1])
				j += 2
			} else {
				j++
			}
			continue
		}
		if c == '{' {
			depth++
			if depth > 1 {
				out.WriteByte(c)
			}
			j++
			continue
		}
		if c == '}' {
			depth--
			if depth == 0 {
				j++
				break
			}
			out.WriteByte(c)
			j++
			continue
		}
		out.WriteByte(c)
		j++
	}
	return span{text: out.String(), end: j}
}

func readArg(src string, i int) span {
	for i < len(src) && src[i] == ' ' {
		i++
	}
	if i >= len(src) {
		return span{text: "", end: i}
	}
	if src[i] == '{' {
		return readBraceGroup(src, i)
	}
	if src[i] != '\\' {
		_, size := utf8.DecodeRuneInString(src[i:])
		return span{text: src[i : i+size], end: i + size}
	}
	j := i + 1
	nameStart := j
	for j < len(src) && isLetter(src[j]) {
		j++
	}
	name := src[nameStart:j]
	if name == "begin" {
		if env := consumeEnvironment(src, i); env != nil {
			return *env
		}
	}
	if name == "" {
		end := i + 2
		if end > len(src) {
			end = len(src)
		}
		return span{text: src[i:end], end: end}
	}
	end := j
	for end < len(src) && (src[end] == '[' || src[end] == '{') {
		if src[end] == '{' {
			end = readBraceGroup(src, end).end
		} else {
			closeIdx := strings.IndexByte(src[end:], ']')
			if closeIdx < 0 {
				end = len(src)
			} else {
				end = end + closeIdx + 1
			}
		}
	}
	return span{text: src[i:end], end: end}
}

type envParts struct {
	env       string
	bodyStart int
	bodyEnd   int
	end       int
}

func readEnvironment(src string, start int) *envParts {
	// start points at backslash of \begin
	i := start + 6 // past "\begin"
	for i < len(src) && src[i] == ' ' {
		i++
	}
	if i >= len(src) || src[i] != '{' {
		return nil
	}
	nameGroup := readBraceGroup(src, i)
	k := nameGroup.end
	depth := 1
	bodyEnd := len(src)
	for k < len(src) && depth > 0 {
		if strings.HasPrefix(src[k:], `\begin`) {
			depth++
			k += 6
			continue
		}
		if strings.HasPrefix(src[k:], `\end`) {
			depth--
			if depth == 0 {
				bodyEnd = k
			}
			k += 4
			for k < len(src) && src[k] == ' ' {
				k++
			}
			if k < len(src) && src[k] == '{' {
				k = readBraceGroup(src, k).end
			}
			if depth == 0 {
				break
			}
			continue
		}
		k++
	}
	return &envParts{
		env:       strings.TrimSpace(nameGroup.text),
		bodyStart: nameGroup.end,
		bodyEnd:   bodyEnd,
		end:       k,
	}
}

func consumeEnvironment(src string, start int) *span {
	env := readEnvironment(src, start)
	if env == nil {
		return nil
	}
	return &span{text: src[start:env.end], end: env.end}
}

func splitRows(body string) []string {
	var rows []string
	braceDepth := 0
	envDepth := 0
	last := 0
	i := 0
	for i < len(body) {
		if strings.HasPrefix(body[i:], `\begin`) {
			envDepth++
			i += 6
			continue
		}
		if strings.HasPrefix(body[i:], `\end`) {
			envDepth--
			i += 4
			continue
		}
		c := body[i]
		if c == '\\' {
			if i+1 < len(body) && body[i+1] == '\\' && braceDepth == 0 && envDepth == 0 {
				rows = append(rows, body[last:i])
				i += 2
				for i < len(body) && body[i] == ' ' {
					i++
				}
				if i < len(body) && body[i] == '[' {
					closeIdx := strings.IndexByte(body[i:], ']')
					if closeIdx < 0 {
						i = len(body)
					} else {
						i = i + closeIdx + 1
					}
				}
				last = i
				continue
			}
			i += 2 // skip escaped char
			if i > len(body) {
				i = len(body)
			}
			continue
		}
		if c == '{' {
			braceDepth++
		} else if c == '}' {
			braceDepth--
		}
		i++
	}
	rows = append(rows, body[last:])
	return rows
}

func parseEnvironment(src string, start int) (box Box, end int, ok bool) {
	env := readEnvironment(src, start)
	if env == nil {
		return Box{}, 0, false
	}
	base := env.env
	if strings.HasSuffix(base, "*") {
		base = base[:len(base)-1]
	}
	if _, isRow := displayRowEnvironments[base]; !isRow {
		return textBox(ToUnicode(src[start:env.end])), env.end, true
	}
	bodyStart := env.bodyStart
	if base == "alignat" || base == "alignedat" || base == "gatheredat" {
		p := bodyStart
		for p < len(src) && (src[p] == ' ' || src[p] == '\n') {
			p++
		}
		if p < len(src) && src[p] == '{' {
			bodyStart = readBraceGroup(src, p).end
		}
	}
	rawRows := splitRows(src[bodyStart:env.bodyEnd])
	var boxes []Box
	for _, row := range rawRows {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}
		boxes = append(boxes, parseExpr(row))
	}
	if len(boxes) == 0 {
		return textBox(""), env.end, true
	}
	return vconcat(boxes), env.end, true
}

func readScript(src string, i int) span {
	out := string(src[i])
	i++
	for i < len(src) && src[i] == ' ' {
		out += string(src[i])
		i++
	}
	if i < len(src) && src[i] == '{' {
		group := readBraceGroup(src, i)
		return span{text: out + "{" + group.text + "}", end: group.end}
	}
	if i < len(src) && src[i] == '\\' {
		j := i + 1
		if j < len(src) && isLetter(src[j]) {
			for j < len(src) && isLetter(src[j]) {
				j++
			}
		} else if j < len(src) {
			j++
		}
		return span{text: out + src[i:j], end: j}
	}
	if i < len(src) {
		_, size := utf8.DecodeRuneInString(src[i:])
		return span{text: out + src[i:i+size], end: i + size}
	}
	return span{text: out, end: i}
}

func parseExpr(src string) Box {
	var boxes []Box
	var inline strings.Builder
	flush := func() {
		if inline.Len() > 0 {
			boxes = append(boxes, textBox(ToUnicode(inline.String())))
			inline.Reset()
		}
	}
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '\\' {
			j := i + 1
			nameStart := j
			for j < len(src) && isLetter(src[j]) {
				j++
			}
			name := src[nameStart:j]
			if name != "" {
				if _, isFrac := fracCommands[name]; isFrac {
					flush()
					num := readArg(src, j)
					den := readArg(src, num.end)
					boxes = append(boxes, fracBox(parseExpr(num.text), parseExpr(den.text)))
					i = den.end
					continue
				}
				if name == "begin" {
					if box, end, ok := parseEnvironment(src, i); ok {
						flush()
						boxes = append(boxes, box)
						i = end
						continue
					}
				}
			}
			if name == "" {
				inline.WriteByte('\\')
				if j < len(src) {
					inline.WriteByte(src[j])
					i = j + 1
				} else {
					i = j
				}
				continue
			}
			inline.WriteByte('\\')
			inline.WriteString(name)
			i = j
			for i < len(src) && (src[i] == '[' || src[i] == '{') {
				if src[i] == '{' {
					group := readBraceGroup(src, i)
					inline.WriteByte('{')
					inline.WriteString(group.text)
					inline.WriteByte('}')
					i = group.end
				} else {
					closeIdx := strings.IndexByte(src[i:], ']')
					var end int
					if closeIdx < 0 {
						end = len(src)
					} else {
						end = i + closeIdx + 1
					}
					inline.WriteString(src[i:end])
					i = end
				}
			}
			continue
		}
		if c == '^' || c == '_' {
			script := readScript(src, i)
			inline.WriteString(script.text)
			i = script.end
			continue
		}
		if c == '{' {
			group := readBraceGroup(src, i)
			flush()
			boxes = append(boxes, parseExpr(group.text))
			i = group.end
			continue
		}
		inline.WriteByte(c)
		i++
	}
	flush()
	if len(boxes) == 0 {
		return textBox("")
	}
	return hconcat(boxes)
}

func splitLines(src string) []string {
	var lines []string
	braceDepth := 0
	envDepth := 0
	last := 0
	i := 0
	for i < len(src) {
		if strings.HasPrefix(src[i:], `\begin`) {
			envDepth++
			i += 6
			continue
		}
		if strings.HasPrefix(src[i:], `\end`) {
			envDepth--
			i += 4
			continue
		}
		c := src[i]
		if c == '\\' {
			i += 2
			if i > len(src) {
				i = len(src)
			}
			continue
		}
		if c == '{' {
			braceDepth++
		} else if c == '}' {
			braceDepth--
		} else if c == '\n' && braceDepth == 0 && envDepth == 0 {
			lines = append(lines, src[last:i])
			last = i + 1
		}
		i++
	}
	lines = append(lines, src[last:])
	return lines
}

// ToBlock renders a display LaTeX math fragment to lines, stacking \frac
// vertically. Top-level source newlines become vertical rows. Inline math
// should use ToUnicode instead.
func ToBlock(src string) []string {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	raw := splitLines(strings.TrimSpace(src))
	var rows []Box
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rows = append(rows, parseExpr(line))
	}
	if len(rows) == 0 {
		return nil
	}
	lines := vconcat(rows).Lines
	for len(lines) > 1 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 1 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return lines
}

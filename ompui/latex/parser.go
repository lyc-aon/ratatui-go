package latex

import (
	"strings"
	"unicode/utf8"
)

type argument struct {
	text  string
	group bool
}

type latexParser struct {
	s          string
	i          int
	foreground *string
	background *string
}

func newLatexParser(src string) *latexParser {
	return &latexParser{s: src}
}

func (p *latexParser) render() string {
	out := p.parse("", false)
	return restoreAnsi(out, p.foreground, nil, p.background, nil)
}

func (p *latexParser) parse(style string, stopAtBrace bool) string {
	var b strings.Builder
	for p.i < len(p.s) {
		if p.s[p.i] == '}' {
			if stopAtBrace {
				break
			}
			p.i++ // stray close
			continue
		}
		b.WriteString(p.node(style))
	}
	return b.String()
}

func (p *latexParser) node(style string) string {
	if p.i >= len(p.s) {
		return ""
	}
	c := p.s[p.i]
	switch c {
	case '\\':
		return p.command(style)
	case '{':
		return p.group(style)
	case '^':
		p.i++
		return p.script(style, true)
	case '_':
		p.i++
		return p.script(style, false)
	case '$':
		p.i++
		return ""
	case '~':
		p.i++
		return " "
	case '&':
		p.i++
		return "  "
	case '\'':
		k := 0
		for p.i < len(p.s) && p.s[p.i] == '\'' {
			k++
			p.i++
		}
		if k <= 4 {
			return primes[k]
		}
		return strings.Repeat(primes[1], k)
	case '%':
		nl := strings.IndexByte(p.s[p.i:], '\n')
		if nl < 0 {
			p.i = len(p.s)
		} else {
			p.i += nl + 1
		}
		return ""
	default:
		r, size := utf8.DecodeRuneInString(p.s[p.i:])
		p.i += size
		return styleChar(r, style)
	}
}

func (p *latexParser) command(style string) string {
	p.i++ // past backslash
	if p.i >= len(p.s) {
		return ""
	}
	c := p.s[p.i]
	if !isLetter(c) {
		p.i++
		switch c {
		case '\\':
			return "\n"
		case '{', '}', '$', '%', '&', '#', '_', ' ', '.':
			return string(c)
		case ',', ':', ';', '>':
			return " "
		case '!':
			return ""
		case '/':
			return ""
		case '|':
			return "‖"
		case '(', ')', '[', ']':
			return ""
		default:
			return string(c)
		}
	}
	start := p.i
	for p.i < len(p.s) && isLetter(p.s[p.i]) {
		p.i++
	}
	name := p.s[start:p.i]
	if p.i < len(p.s) && p.s[p.i] == '*' {
		p.i++ // starred variants
	}
	return p.applyCommand(name, style)
}

func (p *latexParser) applyCommand(name, style string) string {
	if font, ok := fonts[name]; ok {
		return p.argument(font).text
	}
	if _, ok := textCommands[name]; ok {
		return unescapeText(p.rawArgument())
	}
	if name == "operatorname" {
		fn := unescapeText(p.rawArgument())
		return fn + p.spaceBeforeArg()
	}
	if accent, ok := accents[name]; ok {
		return applyCombining(p.argument(style).text, accent)
	}
	if name == "frac" || name == "dfrac" || name == "tfrac" || name == "cfrac" {
		num := p.argument(style)
		den := p.argument(style)
		return p.fraction(num, den)
	}
	if name == "genfrac" {
		left := p.argument(style).text
		right := p.argument(style).text
		p.rawArgument() // rule thickness
		p.rawArgument() // math style
		num := p.argument(style)
		den := p.argument(style)
		return left + p.fraction(num, den) + right
	}
	if name == "binom" || name == "dbinom" || name == "tbinom" {
		n := p.argument(style)
		k := p.argument(style)
		return "C(" + n.text + ", " + k.text + ")"
	}
	if name == "sqrt" {
		return p.sqrt(style)
	}
	if name == "not" {
		arg := p.argument(style)
		if m, ok := notMap[arg.text]; ok {
			return m
		}
		return applyCombining(arg.text, "\u0338")
	}
	if name == "overset" || name == "stackrel" {
		return p.scriptedAbove(style)
	}
	if name == "underset" {
		return p.scriptedBelow(style)
	}
	if name == "prescript" {
		return p.prescript(style)
	}
	if arrow, ok := extensibleArrows[name]; ok {
		return p.extensibleArrow(style, arrow)
	}
	if name == "boxed" || name == "fbox" {
		return "[" + p.argument(style).text + "]"
	}
	if name == "overbrace" {
		return "⏞(" + p.argument(style).text + ")"
	}
	if name == "underbrace" {
		return "⏟(" + p.argument(style).text + ")"
	}
	if name == "overbracket" {
		return "⎴(" + p.argument(style).text + ")"
	}
	if name == "underbracket" {
		return "⎵(" + p.argument(style).text + ")"
	}
	if name == "overparen" {
		return "⏜(" + p.argument(style).text + ")"
	}
	if name == "underparen" {
		return "⏝(" + p.argument(style).text + ")"
	}
	if name == "cancel" {
		return applyCombining(p.argument(style).text, "\u0338")
	}
	if name == "bcancel" {
		return applyCombining(p.argument(style).text, "\u20E5")
	}
	if name == "xcancel" {
		t := applyCombining(p.argument(style).text, "\u0338")
		return applyCombining(t, "\u20E5")
	}
	if name == "sout" {
		return applyCombining(p.argument(style).text, "\u0336")
	}
	if name == "substack" {
		arg := p.argument(style).text
		return strings.ReplaceAll(arg, "\n", ",")
	}
	if name == "left" || name == "right" || name == "middle" {
		return p.delimiter(style)
	}
	if bigDelimName(name) {
		return p.delimiter(style)
	}
	if name == "begin" {
		return p.environment(style)
	}
	if name == "end" {
		p.rawArgument()
		return ""
	}
	if name == "bmod" {
		return " mod "
	}
	if name == "pmod" {
		return "(mod " + p.argument(style).text + ")"
	}
	if name == "pod" {
		return "(" + p.argument(style).text + ")"
	}
	if name == "tag" {
		return "(" + p.argument(style).text + ")"
	}
	if name == "label" {
		p.rawArgument()
		return ""
	}
	if name == "ref" || name == "eqref" {
		return "(" + unescapeText(p.rawArgument()) + ")"
	}
	if name == "url" {
		return unescapeText(p.rawArgument())
	}
	if name == "href" {
		p.rawArgument()
		return p.argument(style).text
	}
	if name == "textcolor" {
		return p.scopedForeground(p.readAnsiColor(), style)
	}
	if name == "colorbox" {
		return p.scopedBackground(p.readAnsiColor(), style)
	}
	if name == "fcolorbox" {
		return p.fcolorbox(style)
	}
	if name == "color" {
		return p.setForeground()
	}
	if name == "normalcolor" {
		prev := p.foreground
		p.foreground = nil
		if prev == nil {
			return ""
		}
		return ansiFGReset
	}
	if name == "phantom" || name == "hphantom" {
		return strings.Repeat(" ", codePointLength(p.argument(style).text))
	}
	if name == "vphantom" {
		p.argument(style)
		return ""
	}
	if _, ok := functions[name]; ok {
		return name + p.spaceBeforeArg()
	}
	if symbol, ok := symbols[name]; ok {
		return symbol
	}
	switch name {
	case "displaystyle", "textstyle", "scriptstyle", "scriptscriptstyle",
		"limits", "nolimits", "nonumber", "notag":
		return ""
	case "quad":
		return "  "
	case "qquad":
		return "    "
	case "thinspace", "enspace", "medspace", "thickspace", "space":
		return " "
	case "negthinspace", "negmedspace", "negthickspace":
		return ""
	}
	// Unknown command: surface bare name.
	return name
}

func (p *latexParser) group(style string) string {
	p.i++ // past {
	outerFG := p.foreground
	outerBG := p.background
	inner := p.parse(style, true)
	innerFG := p.foreground
	innerBG := p.background
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
	}
	p.foreground = outerFG
	p.background = outerBG
	return restoreAnsi(inner, innerFG, outerFG, innerBG, outerBG)
}

func (p *latexParser) readAnsiColor() *ansiColor {
	model := p.optionalRawArgument()
	return ansiColorFrom(model, p.rawArgument())
}

func (p *latexParser) setForeground() string {
	color := p.readAnsiColor()
	if color == nil {
		return ""
	}
	fg := color.foreground
	p.foreground = &fg
	return color.foreground
}

func (p *latexParser) scopedForeground(color *ansiColor, style string) string {
	outerFG := p.foreground
	if color == nil {
		return p.argument(style).text
	}
	fg := color.foreground
	p.foreground = &fg
	arg := p.argument(style).text
	innerFG := p.foreground
	p.foreground = outerFG
	return color.foreground + restoreAnsi(arg, innerFG, outerFG, p.background, p.background)
}

func (p *latexParser) scopedBackground(color *ansiColor, style string) string {
	outerBG := p.background
	if color == nil {
		return p.argument(style).text
	}
	bg := color.background
	p.background = &bg
	arg := p.argument(style).text
	innerBG := p.background
	p.background = outerBG
	return color.background + restoreAnsi(arg, p.foreground, p.foreground, innerBG, outerBG)
}

func (p *latexParser) fcolorbox(style string) string {
	frameModel := p.optionalRawArgument()
	frame := ansiColorFrom(frameModel, p.rawArgument())
	bgModel := p.optionalRawArgument()
	if bgModel == nil {
		bgModel = frameModel
	}
	background := ansiColorFrom(bgModel, p.rawArgument())
	body := p.scopedBackground(background, style)
	if frame == nil {
		return "[" + body + "]"
	}
	fgRest := ansiFGReset
	if p.foreground != nil {
		fgRest = *p.foreground
	}
	return frame.foreground + "[" + fgRest + body + frame.foreground + "]" + fgRest
}

func (p *latexParser) argument(style string) argument {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
	if p.i >= len(p.s) {
		return argument{}
	}
	c := p.s[p.i]
	if c == '{' {
		p.i++
		inner := p.parse(style, true)
		if p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
		}
		return argument{text: inner, group: true}
	}
	if c == '\\' {
		return argument{text: p.command(style), group: false}
	}
	if c == '^' || c == '_' {
		p.i++
		return argument{text: p.script(style, c == '^'), group: false}
	}
	r, size := utf8.DecodeRuneInString(p.s[p.i:])
	p.i += size
	return argument{text: styleChar(r, style), group: false}
}

func (p *latexParser) rawArgument() string {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
	if p.i >= len(p.s) {
		return ""
	}
	if p.s[p.i] != '{' {
		c := p.s[p.i]
		if c == '\\' {
			t := "\\"
			p.i++
			if p.i < len(p.s) && isLetter(p.s[p.i]) {
				for p.i < len(p.s) && isLetter(p.s[p.i]) {
					t += string(p.s[p.i])
					p.i++
				}
			} else if p.i < len(p.s) {
				t += string(p.s[p.i])
				p.i++
			}
			return t
		}
		p.i++
		return string(c)
	}
	p.i++ // past {
	depth := 1
	var out strings.Builder
	for p.i < len(p.s) && depth > 0 {
		c := p.s[p.i]
		if c == '\\' {
			out.WriteByte(c)
			if p.i+1 < len(p.s) {
				out.WriteByte(p.s[p.i+1])
				p.i += 2
			} else {
				p.i++
			}
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				p.i++
				break
			}
		}
		out.WriteByte(c)
		p.i++
	}
	return out.String()
}

func (p *latexParser) script(style string, sup bool) string {
	arg := p.argument(style)
	if sup {
		return toSuperscript(arg.text, arg.group)
	}
	return toSubscript(arg.text, arg.group)
}

func (p *latexParser) wrapFrac(arg argument) string {
	if arg.group && codePointLength(arg.text) > 1 {
		return "(" + arg.text + ")"
	}
	return arg.text
}

func (p *latexParser) fraction(num, den argument) string {
	if v, ok := vulgar[num.text+"/"+den.text]; ok {
		return v
	}
	return p.wrapFrac(num) + "/" + p.wrapFrac(den)
}

func (p *latexParser) scriptedAbove(style string) string {
	above := p.argument(style)
	base := p.argument(style)
	return base.text + toSuperscript(above.text, true)
}

func (p *latexParser) scriptedBelow(style string) string {
	below := p.argument(style)
	base := p.argument(style)
	return base.text + toSubscript(below.text, true)
}

func (p *latexParser) prescript(style string) string {
	sup := p.argument(style)
	sub := p.argument(style)
	base := p.argument(style)
	return toSuperscript(sup.text, true) + toSubscript(sub.text, true) + base.text
}

func (p *latexParser) extensibleArrow(style, arrow string) string {
	below := p.optionalArgument(style)
	above := p.argument(style)
	out := arrow + toSuperscript(above.text, true)
	if below != nil {
		out += toSubscript(below.text, true)
	}
	return out
}

func (p *latexParser) delimiter(style string) string {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
	if p.i >= len(p.s) {
		return ""
	}
	c := p.s[p.i]
	if c == '.' {
		p.i++
		return ""
	}
	if c != '\\' {
		r, size := utf8.DecodeRuneInString(p.s[p.i:])
		p.i += size
		return styleChar(r, style)
	}
	p.i++
	if p.i >= len(p.s) {
		return ""
	}
	d := p.s[p.i]
	if !isLetter(d) {
		p.i++
		switch d {
		case '.':
			return ""
		case '{':
			return "{"
		case '}':
			return "}"
		case '|':
			return "‖"
		default:
			return string(d)
		}
	}
	start := p.i
	for p.i < len(p.s) && isLetter(p.s[p.i]) {
		p.i++
	}
	name := p.s[start:p.i]
	if sym, ok := symbols[name]; ok {
		return sym
	}
	return name
}

func (p *latexParser) optionalArgument(style string) *argument {
	source := p.optionalRawArgument()
	if source == nil {
		return nil
	}
	inner := newLatexParser(*source).parse(style, false)
	return &argument{text: inner, group: true}
}

func (p *latexParser) optionalRawArgument() *string {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
	if p.i >= len(p.s) || p.s[p.i] != '[' {
		return nil
	}
	p.i++
	bracketDepth := 1
	braceDepth := 0
	var out strings.Builder
	for p.i < len(p.s) && bracketDepth > 0 {
		c := p.s[p.i]
		if c == '\\' {
			out.WriteByte(c)
			if p.i+1 < len(p.s) {
				out.WriteByte(p.s[p.i+1])
				p.i += 2
			} else {
				p.i++
			}
			continue
		}
		if c == '{' {
			braceDepth++
		} else if c == '}' && braceDepth > 0 {
			braceDepth--
		} else if braceDepth == 0 && c == '[' {
			bracketDepth++
		} else if braceDepth == 0 && c == ']' {
			bracketDepth--
			if bracketDepth == 0 {
				p.i++
				break
			}
		}
		out.WriteByte(c)
		p.i++
	}
	s := out.String()
	return &s
}

func (p *latexParser) sqrt(style string) string {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
	radical := "√"
	if idx := p.optionalArgument(style); idx != nil {
		switch idx.text {
		case "2":
			radical = "√"
		case "3":
			radical = "∛"
		case "4":
			radical = "∜"
		default:
			radical = toSuperscript(idx.text, true) + "√"
		}
	}
	radicand := p.argument(style).text
	if codePointLength(radicand) > 1 {
		return radical + "(" + radicand + ")"
	}
	return radical + radicand
}

func (p *latexParser) environment(style string) string {
	env := strings.TrimSpace(p.rawArgument())
	if env == "array" || env == "tabular" || env == "array*" || env == "tabular*" {
		p.optionalRawArgument()
		if p.i < len(p.s) && p.s[p.i] == '{' {
			p.rawArgument() // column spec
		}
	} else if env == "alignedat" || env == "alignedat*" || env == "alignat" || env == "alignat*" || env == "gatheredat" {
		p.optionalRawArgument()
		if p.i < len(p.s) && p.s[p.i] == '{' {
			p.rawArgument() // column count
		}
	}
	var body strings.Builder
	for p.i < len(p.s) {
		if strings.HasPrefix(p.s[p.i:], `\end`) {
			p.i += 4
			p.rawArgument()
			break
		}
		body.WriteString(p.node(style))
	}
	b := strings.TrimSpace(body.String())
	switch env {
	case "cases", "cases*", "dcases", "dcases*", "rcases", "drcases":
		b = replaceCasesNewlines(b)
	}
	if delims, ok := envDelims[env]; ok {
		return delims[0] + b + delims[1]
	}
	return b
}

func (p *latexParser) spaceBeforeArg() string {
	if p.i >= len(p.s) {
		return ""
	}
	c := p.s[p.i]
	if isAlphaNum(c) || c == '\\' {
		return " "
	}
	return ""
}


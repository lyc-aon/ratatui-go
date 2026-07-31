package latex

import (
	"strings"
	"unicode/utf8"
)

func mapAll(text string, table map[string]string) (string, bool) {
	var b strings.Builder
	for _, r := range text {
		mapped, ok := table[string(r)]
		if !ok {
			return "", false
		}
		b.WriteString(mapped)
	}
	return b.String(), true
}

func codePointLength(s string) int {
	return utf8.RuneCountInString(s)
}

func styleAlnum(ch rune, style string) string {
	s := string(ch)
	if hole, ok := alphaHoles[style+":"+s]; ok {
		return hole
	}
	plane, ok := planes[style]
	if !ok {
		return s
	}
	if ch >= 'A' && ch <= 'Z' {
		return string(rune(plane.upper + int(ch-'A')))
	}
	if ch >= 'a' && ch <= 'z' {
		return string(rune(plane.lower + int(ch-'a')))
	}
	if ch >= '0' && ch <= '9' && plane.digit >= 0 {
		return string(rune(plane.digit + int(ch-'0')))
	}
	return s
}

func styleChar(ch rune, style string) string {
	if style == "" {
		return string(ch)
	}
	if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
		return styleAlnum(ch, style)
	}
	return string(ch)
}

func applyCombining(text, mark string) string {
	if text == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range text {
		if r == ' ' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(r)
		b.WriteString(mark)
	}
	return b.String()
}

func unescapeText(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			c := s[i+1]
			switch c {
			case '&', '%', '$', '#', '_', '{', '}', ' ', '\t', '\n':
				b.WriteByte(c)
				i += 2
				continue
			}
		}
		if s[i] == '~' {
			b.WriteByte(' ')
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func toSuperscript(text string, group bool) string {
	if text == "" {
		return ""
	}
	if mapped, ok := mapAll(text, superscript); ok {
		return mapped
	}
	if group {
		return "^(" + text + ")"
	}
	return "^" + text
}

func toSubscript(text string, group bool) string {
	if text == "" {
		return ""
	}
	if mapped, ok := mapAll(text, subscript); ok {
		return mapped
	}
	if group {
		return "_(" + text + ")"
	}
	return "_" + text
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isAlphaNum(b byte) bool {
	return isLetter(b) || isDigit(b)
}

func bigDelimName(name string) bool {
	if name == "" {
		return false
	}
	if name[0] != 'b' && name[0] != 'B' {
		return false
	}
	rest := name[1:]
	if !strings.HasPrefix(rest, "ig") {
		return false
	}
	rest = rest[2:]
	if rest == "" || rest == "g" {
		return true
	}
	if rest == "l" || rest == "r" || rest == "m" {
		return true
	}
	if strings.HasPrefix(rest, "g") {
		rest = rest[1:]
		return rest == "l" || rest == "r" || rest == "m"
	}
	return false
}

func replaceNewlinesWithSpace(s string) string {
	var b strings.Builder
	prevNL := false
	for _, r := range s {
		if r == '\n' {
			if !prevNL {
				b.WriteByte(' ')
			}
			prevNL = true
			continue
		}
		prevNL = false
		b.WriteRune(r)
	}
	return b.String()
}

func replaceCasesNewlines(body string) string {
	// body.replace(/[ \t]*\n+[ \t]*/g, "; ").replace(/ {3,}/g, "  ")
	var b strings.Builder
	i := 0
	for i < len(body) {
		if body[i] == '\n' {
			for b.Len() > 0 {
				bs := b.String()
				last := bs[len(bs)-1]
				if last == ' ' || last == '\t' {
					b.Reset()
					b.WriteString(bs[:len(bs)-1])
					continue
				}
				break
			}
			for i < len(body) && body[i] == '\n' {
				i++
			}
			for i < len(body) && (body[i] == ' ' || body[i] == '\t') {
				i++
			}
			b.WriteString("; ")
			continue
		}
		b.WriteByte(body[i])
		i++
	}
	s := b.String()
	for strings.Contains(s, "   ") {
		s = strings.ReplaceAll(s, "   ", "  ")
	}
	return s
}

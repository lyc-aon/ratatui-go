package richtext

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	hexColorExactRe = regexp.MustCompile(`(?i)^#([0-9a-fA-F]{3,8})$`)
)

// classifyHexColorOMP reports whether hex digits (without #) are a CSS color.
// Length must be exactly 3, 6, or 8 (OMP).
func classifyHexColorOMP(hex string, strict bool) bool {
	n := len(hex)
	if n != 3 && n != 6 && n != 8 {
		return false
	}
	_ = strict
	return true
}

func expandHex(hex string) (r, g, b int, ok bool) {
	hex = strings.ToLower(hex)
	switch len(hex) {
	case 3:
		r = hexNibble(hex[0])*17
		g = hexNibble(hex[1])*17
		b = hexNibble(hex[2])*17
		return r, g, b, true
	case 4:
		r = hexNibble(hex[0]) * 17
		g = hexNibble(hex[1]) * 17
		b = hexNibble(hex[2]) * 17
		return r, g, b, true
	case 6, 8:
		rv, err1 := strconv.ParseInt(hex[0:2], 16, 0)
		gv, err2 := strconv.ParseInt(hex[2:4], 16, 0)
		bv, err3 := strconv.ParseInt(hex[4:6], 16, 0)
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, 0, 0, false
		}
		return int(rv), int(gv), int(bv), true
	default:
		return 0, 0, 0, false
	}
}

func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	}
	return 0
}

func colorSwatchANSI(hex, glyph string) string {
	if glyph == "" {
		return ""
	}
	r, g, b, ok := expandHex(hex)
	if !ok {
		return ""
	}
	// Truecolor fg chip + reset only foreground so enclosing styles survive.
	var bld strings.Builder
	bld.WriteString("\x1b[38;2;")
	bld.WriteString(itoa(r))
	bld.WriteByte(';')
	bld.WriteString(itoa(g))
	bld.WriteByte(';')
	bld.WriteString(itoa(b))
	bld.WriteByte('m')
	bld.WriteString(glyph)
	bld.WriteString("\x1b[39m")
	return bld.String()
}

// renderTextWithSwatches inserts a color chip before each #hex mention.
func renderTextWithSwatches(text string, applySegment func(string) string, glyph string) string {
	if glyph == "" || !strings.Contains(text, "#") {
		return applySegment(text)
	}
	// Manual scan to match OMP lookbehind/lookahead semantics.
	var result strings.Builder
	last := 0
	for i := 0; i < len(text); i++ {
		if text[i] != '#' {
			continue
		}
		// lookbehind: not word/#/&
		if i > 0 {
			c := text[i-1]
			if isWordish(c) || c == '#' || c == '&' {
				continue
			}
		}
		// gather hex
		j := i + 1
		for j < len(text) && isHexByte(text[j]) {
			j++
		}
		hexLen := j - (i + 1)
		if hexLen < 3 || hexLen > 8 {
			continue
		}
		// lookahead: not more hex
		if j < len(text) && isHexByte(text[j]) {
			continue
		}
		hex := text[i+1 : j]
		if !classifyHexColorOMP(hex, true) {
			continue
		}
		sw := colorSwatchANSI(hex, glyph)
		if sw == "" {
			continue
		}
		if i > last {
			result.WriteString(applySegment(text[last:i]))
		}
		result.WriteString(sw)
		result.WriteString(applySegment(text[i:j]))
		last = j
		i = j - 1
	}
	if last == 0 {
		return applySegment(text)
	}
	if last < len(text) {
		result.WriteString(applySegment(text[last:]))
	}
	return result.String()
}

func codespanSwatch(code, glyph string) string {
	if glyph == "" {
		return ""
	}
	m := hexColorExactRe.FindStringSubmatch(code)
	if m == nil {
		return ""
	}
	if !classifyHexColorOMP(m[1], false) {
		return ""
	}
	return colorSwatchANSI(m[1], glyph)
}

func isHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isWordish(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

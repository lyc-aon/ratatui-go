package latex

import (
	"math"
	"strconv"
	"strings"
	"sync/atomic"
)

const (
	ansiFGReset = "\x1b[39m"
	ansiBGReset = "\x1b[49m"
)

// ColorMode selects ANSI emission for LaTeX color commands.
type ColorMode uint32

const (
	ColorNone ColorMode = iota
	ColorANSI256
	ColorTrueColor
)

var colorMode atomic.Uint32

func init() {
	colorMode.Store(uint32(ColorTrueColor))
}

// SetColorMode sets process-wide LaTeX color output. A frontend owns one
// terminal, so a process-wide atomic avoids threading mode through every AST.
func SetColorMode(mode ColorMode) { colorMode.Store(uint32(mode)) }

// CurrentColorMode reports the active LaTeX color output mode.
func CurrentColorMode() ColorMode { return ColorMode(colorMode.Load()) }

// SetTrueColor keeps the original two-mode API: false selects ANSI-256.
func SetTrueColor(on bool) {
	if on {
		SetColorMode(ColorTrueColor)
	} else {
		SetColorMode(ColorANSI256)
	}
}

// TrueColor reports whether 24-bit output is active.
func TrueColor() bool { return CurrentColorMode() == ColorTrueColor }

type ansiColor struct {
	foreground string
	background string
}

type rgb struct {
	r, g, b float64
}

func clamp01(n float64) float64 {
	if n <= 0 {
		return 0
	}
	if n >= 1 {
		return 1
	}
	return n
}

func clampByte(n float64) int {
	if n <= 0 {
		return 0
	}
	if n >= 255 {
		return 255
	}
	return int(math.Round(n))
}

func cssRGB(c rgb) string {
	return "rgb(" + strconv.Itoa(clampByte(c.r)) + ", " + strconv.Itoa(clampByte(c.g)) + ", " + strconv.Itoa(clampByte(c.b)) + ")"
}

func parseNumber(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	if strings.HasSuffix(trimmed, "%") {
		v, err := strconv.ParseFloat(trimmed[:len(trimmed)-1], 64)
		if err != nil {
			return 0, false
		}
		return v / 100, true
	}
	v, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func parseColorComponents(spec string, expected int) ([]float64, bool) {
	// split on comma or whitespace
	fields := strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			parts = append(parts, f)
		}
	}
	if len(parts) != expected {
		return nil, false
	}
	vals := make([]float64, 0, expected)
	for _, p := range parts {
		v, ok := parseNumber(p)
		if !ok {
			return nil, false
		}
		vals = append(vals, v)
	}
	return vals, true
}

func rgbFromUnit(values []float64) (string, bool) {
	if len(values) != 3 {
		return "", false
	}
	return cssRGB(rgb{clamp01(values[0]) * 255, clamp01(values[1]) * 255, clamp01(values[2]) * 255}), true
}

func rgbFromByte(values []float64) (string, bool) {
	if len(values) != 3 {
		return "", false
	}
	return cssRGB(rgb{values[0], values[1], values[2]}), true
}

func rgbFromCmyk(values []float64) (string, bool) {
	if len(values) != 4 {
		return "", false
	}
	c, m, y, k := clamp01(values[0]), clamp01(values[1]), clamp01(values[2]), clamp01(values[3])
	return cssRGB(rgb{
		255 * (1 - c) * (1 - k),
		255 * (1 - m) * (1 - k),
		255 * (1 - y) * (1 - k),
	}), true
}

func rgbFromHsv(values []float64, hueScale float64) (string, bool) {
	if len(values) != 3 {
		return "", false
	}
	h := math.Mod(values[0]*hueScale, 360) / 60
	if h < 0 {
		h += 6
	}
	s := clamp01(values[1])
	v := clamp01(values[2])
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 1:
		r, g = c, x
	case h < 2:
		r, g = x, c
	case h < 3:
		g, b = c, x
	case h < 4:
		g, b = x, c
	case h < 5:
		r, b = x, c
	default:
		r, b = c, x
	}
	return cssRGB(rgb{(r + m) * 255, (g + m) * 255, (b + m) * 255}), true
}

func rgbFromWave(spec string) (string, bool) {
	wavelength, ok := parseNumber(spec)
	if !ok || wavelength < 380 || wavelength > 780 {
		return "", false
	}
	var r, g, b float64
	switch {
	case wavelength < 440:
		r = -(wavelength - 440) / 60
		b = 1
	case wavelength < 490:
		g = (wavelength - 440) / 50
		b = 1
	case wavelength < 510:
		g = 1
		b = -(wavelength - 510) / 20
	case wavelength < 580:
		r = (wavelength - 510) / 70
		g = 1
	case wavelength < 645:
		r = 1
		g = -(wavelength - 645) / 65
	default:
		r = 1
	}
	factor := 1.0
	if wavelength < 420 {
		factor = 0.3 + (0.7*(wavelength-380))/40
	} else if wavelength > 700 {
		factor = 0.3 + (0.7*(780-wavelength))/80
	}
	return cssRGB(rgb{r * factor * 255, g * factor * 255, b * factor * 255}), true
}

func parseHexRGB(spec string) (r, g, b int, ok bool) {
	s := strings.TrimSpace(spec)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 3 && len(s) != 6 && len(s) != 8 {
		return 0, 0, 0, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return 0, 0, 0, false
		}
	}
	nib := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0')
		case c >= 'a' && c <= 'f':
			return int(c - 'a' + 10)
		default:
			return int(c - 'A' + 10)
		}
	}
	switch len(s) {
	case 3:
		return nib(s[0]) * 17, nib(s[1]) * 17, nib(s[2]) * 17, true
	case 6, 8:
		return nib(s[0])*16 + nib(s[1]), nib(s[2])*16 + nib(s[3]), nib(s[4])*16 + nib(s[5]), true
	}
	return 0, 0, 0, false
}

func parseCSSColor(spec string) (r, g, b int, ok bool) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return 0, 0, 0, false
	}
	if named, ok := latexNamedColors[s]; ok {
		return parseHexRGB(named)
	}
	if named, ok := latexNamedColors[strings.ToLower(s)]; ok {
		return parseHexRGB(named)
	}
	if strings.HasPrefix(s, "#") {
		return parseHexRGB(s)
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "rgb(") && strings.HasSuffix(s, ")") {
		inner := s[4 : len(s)-1]
		vals, ok := parseColorComponents(inner, 3)
		if !ok {
			return 0, 0, 0, false
		}
		// if any value > 1 treat as bytes
		if vals[0] > 1 || vals[1] > 1 || vals[2] > 1 {
			return clampByte(vals[0]), clampByte(vals[1]), clampByte(vals[2]), true
		}
		return clampByte(vals[0] * 255), clampByte(vals[1] * 255), clampByte(vals[2] * 255), true
	}
	// bare hex without #
	if r, g, b, ok := parseHexRGB(s); ok {
		return r, g, b, true
	}
	return 0, 0, 0, false
}

func normalizeCSSColor(spec string, allowMix bool) (string, bool) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return "", false
	}
	if allowMix && strings.Contains(trimmed, "!") {
		if mixed, ok := resolveMixedColor(trimmed); ok {
			return mixed, true
		}
	}
	if named, ok := latexNamedColors[trimmed]; ok {
		return named, true
	}
	if named, ok := latexNamedColors[strings.ToLower(trimmed)]; ok {
		return named, true
	}
	if r, g, b, ok := parseCSSColor(trimmed); ok {
		return cssRGB(rgb{float64(r), float64(g), float64(b)}), true
	}
	return "", false
}

func resolveModeledColor(model, spec string) (string, bool) {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" || trimmedModel == "named" {
		return normalizeCSSColor(spec, true)
	}
	if trimmedModel == "HTML" || trimmedModel == "Html" || trimmedModel == "html" {
		hex := strings.TrimPrefix(strings.TrimSpace(spec), "#")
		if r, g, b, ok := parseHexRGB(hex); ok {
			_ = r
			_ = g
			_ = b
			return "#" + hex, true
		}
		return "", false
	}
	if trimmedModel == "wave" {
		return rgbFromWave(spec)
	}
	lower := strings.ToLower(trimmedModel)
	if trimmedModel == "RGB" {
		vals, ok := parseColorComponents(spec, 3)
		if !ok {
			return "", false
		}
		return rgbFromByte(vals)
	}
	if lower == "rgb" {
		vals, ok := parseColorComponents(spec, 3)
		if !ok {
			return "", false
		}
		return rgbFromUnit(vals)
	}
	if lower == "cmyk" {
		vals, ok := parseColorComponents(spec, 4)
		if !ok {
			return "", false
		}
		return rgbFromCmyk(vals)
	}
	if lower == "gray" || lower == "grey" {
		vals, ok := parseColorComponents(spec, 1)
		if !ok {
			return "", false
		}
		value := vals[0]
		unit := value
		if trimmedModel == "Gray" || trimmedModel == "Grey" {
			unit = value / 15
		}
		byteV := clamp01(unit) * 255
		return cssRGB(rgb{byteV, byteV, byteV}), true
	}
	if lower == "hsb" || lower == "hsv" {
		vals, ok := parseColorComponents(spec, 3)
		if !ok {
			return "", false
		}
		scale := 360.0
		if trimmedModel == "Hsb" || trimmedModel == "HSV" {
			scale = 1
		}
		return rgbFromHsv(vals, scale)
	}
	return normalizeCSSColor(spec, true)
}

func resolveLatexColor(model *string, spec string) (string, bool) {
	unescaped := strings.TrimSpace(unescapeText(spec))
	if unescaped == "" {
		return "", false
	}
	if model == nil {
		return normalizeCSSColor(unescaped, true)
	}
	return resolveModeledColor(*model, unescaped)
}

func resolveMixedColor(spec string) (string, bool) {
	parts := strings.Split(spec, "!")
	if len(parts) < 2 {
		return "", false
	}
	first, ok := normalizeCSSColor(parts[0], false)
	if !ok {
		return "", false
	}
	cr, cg, cb, ok := parseCSSColor(first)
	if !ok {
		return "", false
	}
	current := rgb{float64(cr), float64(cg), float64(cb)}
	for i := 1; i < len(parts); i += 2 {
		percent, ok := parseNumber(parts[i])
		if !ok {
			return "", false
		}
		nextSpec := "white"
		if i+1 < len(parts) {
			nextSpec = parts[i+1]
		}
		nextColor, ok := normalizeCSSColor(nextSpec, false)
		if !ok {
			return "", false
		}
		nr, ng, nb, ok := parseCSSColor(nextColor)
		if !ok {
			return "", false
		}
		t := clamp01(percent / 100)
		current = rgb{
			current.r*t + float64(nr)*(1-t),
			current.g*t + float64(ng)*(1-t),
			current.b*t + float64(nb)*(1-t),
		}
	}
	return cssRGB(current), true
}

func colorToRGB(css string) (r, g, b int, ok bool) {
	return parseCSSColor(css)
}

// rgbToAnsi256 approximates a 24-bit color as an xterm 256-color index (same idea as common converters).
func rgbToAnsi256(r, g, b int) int {
	// grayscale ramp 232-255
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 248 {
			return 231
		}
		return int(math.Round(((float64(r)-8)/247)*24)) + 232
	}
	ri := int(math.Round(float64(r) / 255 * 5))
	gi := int(math.Round(float64(g) / 255 * 5))
	bi := int(math.Round(float64(b) / 255 * 5))
	return 16 + 36*ri + 6*gi + bi
}

func ansiFromRGB(r, g, b int) ansiColor {
	if CurrentColorMode() == ColorTrueColor {
		fg := "\x1b[38;2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b) + "m"
		bg := "\x1b[48;2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b) + "m"
		return ansiColor{foreground: fg, background: bg}
	}
	idx := rgbToAnsi256(r, g, b)
	fg := "\x1b[38;5;" + strconv.Itoa(idx) + "m"
	bg := "\x1b[48;5;" + strconv.Itoa(idx) + "m"
	return ansiColor{foreground: fg, background: bg}
}

func ansiColorFrom(model *string, spec string) *ansiColor {
	if CurrentColorMode() == ColorNone {
		return nil
	}
	css, ok := resolveLatexColor(model, spec)
	if !ok {
		return nil
	}
	r, g, b, ok := colorToRGB(css)
	if !ok {
		// try #hex direct
		if strings.HasPrefix(css, "#") {
			r, g, b, ok = parseHexRGB(css)
		}
		if !ok {
			return nil
		}
	}
	c := ansiFromRGB(r, g, b)
	return &c
}

func restoreAnsi(text string, fromFG, toFG, fromBG, toBG *string) string {
	if fromFG != nil && (toFG == nil || *fromFG != *toFG) {
		if toFG != nil {
			text += *toFG
		} else {
			text += ansiFGReset
		}
	}
	if fromBG != nil && (toBG == nil || *fromBG != *toBG) {
		if toBG != nil {
			text += *toBG
		} else {
			text += ansiBGReset
		}
	}
	return text
}

package event

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// modifierOrder is the canonical left-to-right order of modifiers in a Key.ID.
var modifierOrder = []string{"ctrl", "shift", "alt", "super"}

// shiftedSymbolKeys are characters that already encode Shift on US keyboards.
// Bindings may alias them as shift+<sym>.
var shiftedSymbolKeys = map[string]struct{}{
	"!": {}, "@": {}, "#": {}, "$": {}, "%": {}, "^": {}, "&": {}, "*": {},
	"(": {}, ")": {}, "_": {}, "+": {}, "{": {}, "}": {}, "|": {}, ":": {},
	"<": {}, ">": {}, "?": {}, "~": {},
}

// CanonicalKeyID normalizes a key identifier:
//   - modifier order becomes ctrl, shift, alt, super
//   - "esc" → "escape", "return" → "enter"
//   - bare uppercase ASCII letter implies shift
//   - base key is lowercased (pageUp → pageup), matching keybindings.ts
func CanonicalKeyID(key string) string {
	if key == "" {
		return ""
	}
	offset := 0
	var modifiers []string
	for {
		found := false
		for _, mod := range modifierOrder {
			if startsWithModifier(key, offset, mod) {
				modifiers = append(modifiers, mod)
				offset += len(mod) + 1
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	rawBase := key[offset:]
	lowerBase := strings.ToLower(rawBase)
	base := normalizeBaseName(lowerBase)
	if isASCIIUppercaseLetter(rawBase) && !containsStr(modifiers, "shift") {
		modifiers = append(modifiers, "shift")
	}
	if len(modifiers) == 0 {
		return base
	}
	sortModifiers(modifiers)
	var b strings.Builder
	b.Grow(len(key) + 8)
	for i, m := range modifiers {
		if i > 0 {
			b.WriteByte('+')
		}
		b.WriteString(m)
	}
	b.WriteByte('+')
	b.WriteString(base)
	return b.String()
}

// FormatKeyID builds a canonical key ID from mods and a base code name.
// Lock bits are stripped. Empty base yields "".
// Modifier order is ctrl+shift+alt+super (binding order).
func FormatKeyID(mods Modifiers, base string) string {
	if base == "" {
		return ""
	}
	base = normalizeBaseName(strings.ToLower(base))
	mods = mods.WithoutLocks()
	if mods == 0 {
		return base
	}
	var parts []string
	if mods&ModCtrl != 0 {
		parts = append(parts, "ctrl")
	}
	if mods&ModShift != 0 {
		parts = append(parts, "shift")
	}
	if mods&ModAlt != 0 {
		parts = append(parts, "alt")
	}
	if mods&ModSuper != 0 {
		parts = append(parts, "super")
	}
	return strings.Join(parts, "+") + "+" + base
}

// AddKeyAliases inserts canonical and shifted-symbol aliases of key into set.
func AddKeyAliases(set map[string]struct{}, key string) {
	canonical := CanonicalKeyID(key)
	set[canonical] = struct{}{}
	if _, ok := shiftedSymbolKeys[canonical]; ok {
		set["shift+"+canonical] = struct{}{}
	}
}

// MatchesKey reports whether a decoded key matches one configured binding.
// It applies the same canonicalization and shifted-symbol alias rule as OMP's
// KeybindingsManager. Key-release policy remains the caller's responsibility.
func MatchesKey(k Key, binding string) bool {
	actual := CanonicalKeyID(k.ID)
	if actual == "" {
		actual = FormatKeyID(k.Mods, string(k.Code))
	}
	configured := CanonicalKeyID(binding)
	if actual == configured {
		return true
	}
	return IsShiftedSymbol(configured) && actual == "shift+"+configured
}

// MatchesAnyKey reports whether k matches at least one configured binding.
func MatchesAnyKey(k Key, bindings ...string) bool {
	for _, binding := range bindings {
		if MatchesKey(k, binding) {
			return true
		}
	}
	return false
}

// IsShiftedSymbol reports whether s is a US-keyboard shifted symbol character.
func IsShiftedSymbol(s string) bool {
	_, ok := shiftedSymbolKeys[s]
	return ok
}

// SplitKeyID parses "ctrl+shift+p" into mods + base.
// Base casing is preserved from the last non-modifier token (after plus/esc aliases).
func SplitKeyID(keyID string) (mods Modifiers, base string, ok bool) {
	s := strings.TrimSpace(keyID)
	if s == "" {
		return 0, "", false
	}
	// Support bare "+" and "...++" (trailing key is '+').
	forcedPlus := false
	prefix := s
	if s == "+" {
		return 0, "+", true
	}
	if strings.HasSuffix(s, "++") {
		prefix = s[:len(s)-2]
		forcedPlus = true
	}
	var keyTok string
	if forcedPlus {
		keyTok = "+"
	}
	for _, part := range strings.Split(prefix, "+") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		switch {
		case strings.EqualFold(p, "ctrl"):
			mods |= ModCtrl
		case strings.EqualFold(p, "shift"):
			mods |= ModShift
		case strings.EqualFold(p, "super"):
			mods |= ModSuper
		case strings.EqualFold(p, "alt"):
			mods |= ModAlt
		default:
			keyTok = p
		}
	}
	if keyTok == "" {
		return 0, "", false
	}
	keyTok = normalizeBaseName(strings.ToLower(keyTok))
	if keyTok == "plus" {
		keyTok = "+"
	}
	return mods, keyTok, true
}

// normalizeBaseName maps lowercased base tokens to the stable binding spelling.
// Matches keybindings.ts canonicalKeyId: only esc/return are renamed.
func normalizeBaseName(lower string) string {
	switch lower {
	case "esc":
		return "escape"
	case "return":
		return "enter"
	default:
		return lower
	}
}

func startsWithModifier(key string, offset int, modifier string) bool {
	if len(key) <= offset+len(modifier) {
		return false
	}
	if key[offset+len(modifier)] != '+' {
		return false
	}
	for i := 0; i < len(modifier); i++ {
		actual := key[offset+i]
		expected := modifier[i]
		if actual != expected && !asciiEqualFoldByte(actual, expected) {
			return false
		}
	}
	return true
}

func asciiEqualFoldByte(a, b byte) bool {
	if a == b {
		return true
	}
	if a >= 'A' && a <= 'Z' {
		a += 32
	}
	if b >= 'A' && b <= 'Z' {
		b += 32
	}
	return a == b
}

func isASCIIUppercaseLetter(key string) bool {
	if len(key) != 1 {
		return false
	}
	c := key[0]
	return c >= 'A' && c <= 'Z'
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func sortModifiers(mods []string) {
	order := map[string]int{"ctrl": 0, "shift": 1, "alt": 2, "super": 3}
	for i := 1; i < len(mods); i++ {
		j := i
		for j > 0 && order[mods[j]] < order[mods[j-1]] {
			mods[j], mods[j-1] = mods[j-1], mods[j]
			j--
		}
	}
}

// FirstRune returns the first UTF-8 rune of s, or 0.
func FirstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return 0
	}
	return r
}

// IsGraphicASCII reports whether r is ASCII graphic (0x21..0x7E).
func IsGraphicASCII(r rune) bool {
	return r >= 0x21 && r <= 0x7e
}

// LowerASCIILetter lowercases an ASCII letter rune; otherwise returns r.
func LowerASCIILetter(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

// IsLetter reports unicode letter (used sparingly; prefer ASCII paths).
func IsLetter(r rune) bool { return unicode.IsLetter(r) }

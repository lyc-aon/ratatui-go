package input

import (
	"strings"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// legacySequences maps exact byte sequences to canonical key IDs.
// Port of LEGACY_SEQUENCES in keys.rs.
var legacySequences = map[string]string{
	// Arrow keys (SS3 and CSI)
	"\x1bOA": "up", "\x1bOB": "down", "\x1bOC": "right", "\x1bOD": "left",
	"\x1b[A": "up", "\x1b[B": "down", "\x1b[C": "right", "\x1b[D": "left",
	// Home/End
	"\x1bOH": "home", "\x1bOF": "end",
	"\x1b[H": "home", "\x1b[F": "end",
	"\x1b[1~": "home", "\x1b[7~": "home",
	"\x1b[4~": "end", "\x1b[8~": "end",
	// Clear
	"\x1b[E": "clear", "\x1bOE": "clear", "\x1bOe": "ctrl+clear", "\x1b[e": "shift+clear",
	// Insert/Delete
	"\x1b[2~": "insert", "\x1b[2$": "shift+insert", "\x1b[2^": "ctrl+insert",
	"\x1b[3~": "delete", "\x1b[3$": "shift+delete", "\x1b[3^": "ctrl+delete",
	// Page Up/Down
	"\x1b[5~": "pageUp", "\x1b[6~": "pageDown",
	"\x1b[[5~": "pageUp", "\x1b[[6~": "pageDown",
	// Shift+arrow
	"\x1b[a": "shift+up", "\x1b[b": "shift+down", "\x1b[c": "shift+right", "\x1b[d": "shift+left",
	// Ctrl+arrow
	"\x1bOa": "ctrl+up", "\x1bOb": "ctrl+down", "\x1bOc": "ctrl+right", "\x1bOd": "ctrl+left",
	// Shift+page/home/end
	"\x1b[5$": "shift+pageUp", "\x1b[6$": "shift+pageDown",
	"\x1b[7$": "shift+home", "\x1b[8$": "shift+end",
	// Ctrl+page/home/end
	"\x1b[5^": "ctrl+pageUp", "\x1b[6^": "ctrl+pageDown",
	"\x1b[7^": "ctrl+home", "\x1b[8^": "ctrl+end",
	// Function keys
	"\x1bOP": "f1", "\x1bOQ": "f2", "\x1bOR": "f3", "\x1bOS": "f4",
	"\x1b[11~": "f1", "\x1b[12~": "f2", "\x1b[13~": "f3", "\x1b[14~": "f4",
	"\x1b[[A": "f1", "\x1b[[B": "f2", "\x1b[[C": "f3", "\x1b[[D": "f4", "\x1b[[E": "f5",
	"\x1b[15~": "f5", "\x1b[17~": "f6", "\x1b[18~": "f7", "\x1b[19~": "f8",
	"\x1b[20~": "f9", "\x1b[21~": "f10", "\x1b[23~": "f11", "\x1b[24~": "f12",
}

// parsedKitty is the internal Kitty sequence parse result.
type parsedKitty struct {
	codepoint      int32
	shiftedKey     int32 // 0 = absent
	baseLayoutKey  int32 // 0 = absent
	textCodepoint  int32 // 0 = absent
	hasText        bool
	modifier       uint32
	eventType      uint32 // 0 = absent; 1 press 2 repeat 3 release
	hasEventType   bool
}

// KeyParseOptions controls platform-sensitive key decoding.
type KeyParseOptions struct {
	// KittyActive is true when the Kitty keyboard protocol is engaged.
	KittyActive bool
	// WindowsTerminal enables the raw-0x08 → ctrl+backspace heuristic.
	WindowsTerminal bool
}

// ParseKey returns the canonical key ID for a framed sequence, or "" if unrecognized.
func ParseKey(data []byte, opts KeyParseOptions) string {
	id, _, ok := parseKeyFull(data, opts)
	if !ok {
		return ""
	}
	return id
}

// parseKeyFull returns canonical ID, structured Key, and ok.
func parseKeyFull(data []byte, opts KeyParseOptions) (id string, key event.Key, ok bool) {
	if len(data) == 0 {
		return "", event.Key{}, false
	}

	// Single byte fast path
	if len(data) == 1 {
		return parseSingleByte(data[0], opts)
	}

	if data[0] != esc {
		// Multi-byte UTF-8 printable text — not a "key id" path; caller handles Text.
		return "", event.Key{}, false
	}

	// Two-byte ESC sequences are Meta/Alt before legacy table.
	if len(data) == 2 {
		if id, key, ok = parseEscPair(data[1], opts); ok {
			return id, key, true
		}
	}

	// Legacy table
	if id, found := legacySequences[string(data)]; found {
		return keyFromID(id, event.ActionPress, data)
	}

	// modifyOtherKeys: CSI 27 ; mod ; keycode ~
	if mods, keycode, mok := parseModifyOtherKeys(data); mok {
		name := formatKeyName(keycode)
		if name == "" {
			return "", event.Key{}, false
		}
		eff := event.Modifiers(mods).WithoutLocks()
		id = event.FormatKeyID(eff, name)
		return keyFromParts(id, event.Code(baseCode(name)), eff, event.ActionPress, keycode, data)
	}

	// Kitty protocol
	if pk, kittyOK := parseKittySequence(data); kittyOK {
		if pk.hasEventType && pk.eventType == 3 {
			// Release: still produce a Key with ActionRelease so callers can see it.
			// parse_key_inner returns None for release (matchesKey ignores them);
			// we surface the structured event and leave ID empty for release.
			k, id2, ok2 := formatKittyKey(pk)
			if !ok2 {
				// Still emit structured release with action set
				act := event.ActionRelease
				code := formatKeyName(pk.codepoint)
				return "", event.Key{
					Code:          event.Code(baseCode(code)),
					Mods:          event.Modifiers(pk.modifier).WithoutLocks(),
					Action:        act,
					Codepoint:     runeOrZero(pk.codepoint),
					ShiftedKey:    runeOrZero(pk.shiftedKey),
					BaseLayoutKey: runeOrZero(pk.baseLayoutKey),
				}, true
			}
			k.Action = event.ActionRelease
			return id2, k, true
		}
		k, id2, ok2 := formatKittyKey(pk)
		if !ok2 {
			return "", event.Key{}, false
		}
		if pk.hasEventType && pk.eventType == 2 {
			k.Action = event.ActionRepeat
		} else {
			k.Action = event.ActionPress
		}
		return id2, k, true
	}

	// ESC-prefixed meta: ESC ESC [ / O ...
	if len(data) > 2 && data[0] == esc && data[1] == esc && (data[2] == '[' || data[2] == 'O') {
		innerID, innerKey, innerOK := parseKeyFull(data[1:], KeyParseOptions{KittyActive: true, WindowsTerminal: opts.WindowsTerminal})
		if innerOK {
			// Prepend alt+
			if innerID != "" {
				id = "alt+" + innerID
			}
			innerKey.Mods |= event.ModAlt
			innerKey.ID = event.CanonicalKeyID(id)
			return innerKey.ID, innerKey, true
		}
	}

	// Fixed extras
	switch string(data) {
	case "\x1b[Z":
		return keyFromID("shift+tab", event.ActionPress, data)
	case "\x1bOM":
		return keyFromID("enter", event.ActionPress, data)
	}

	// Focus events CSI I / CSI O (bare, no params)
	if string(data) == "\x1b[I" || string(data) == "\x1b[O" {
		return "", event.Key{}, false // handled as focus elsewhere
	}

	return "", event.Key{}, false
}

func parseSingleByte(code byte, opts KeyParseOptions) (string, event.Key, bool) {
	switch code {
	case 0x1b:
		return keyFromID("escape", event.ActionPress, []byte{code})
	case '\t':
		return keyFromID("tab", event.ActionPress, []byte{code})
	case '\r', '\n':
		return keyFromID("enter", event.ActionPress, []byte{code})
	case 0x00:
		return keyFromID("ctrl+space", event.ActionPress, []byte{code})
	case ' ':
		return keyFromID("space", event.ActionPress, []byte{code})
	case 0x7f:
		return keyFromID("backspace", event.ActionPress, []byte{code})
	case 0x08:
		// Windows Terminal: 0x08 = Ctrl+Backspace; elsewhere plain Backspace.
		if opts.WindowsTerminal {
			return keyFromID("ctrl+backspace", event.ActionPress, []byte{code})
		}
		return keyFromID("backspace", event.ActionPress, []byte{code})
	case 28:
		return keyFromID("ctrl+\\", event.ActionPress, []byte{code})
	case 29:
		return keyFromID("ctrl+]", event.ActionPress, []byte{code})
	case 30:
		return keyFromID("ctrl+^", event.ActionPress, []byte{code})
	case 31:
		return keyFromID("ctrl+_", event.ActionPress, []byte{code})
	}
	if code >= 1 && code <= 26 {
		letter := 'a' + rune(code-1)
		id := "ctrl+" + string(letter)
		return keyFromID(id, event.ActionPress, []byte{code})
	}
	if code >= 'a' && code <= 'z' {
		id := string(code)
		return keyFromParts(id, event.Code(id), 0, event.ActionPress, int32(code), []byte{code})
	}
	if code >= 33 && code <= 126 {
		id := string(code)
		// Uppercase letter → shift+letter in canonical form for Key.ID bindings?
		// parse_single_byte returns the literal ASCII ("A"), while CanonicalKeyID
		// later adds shift. We store the literal id matching native parse_key.
		return keyFromParts(id, event.Code(strings.ToLower(id)), 0, event.ActionPress, int32(code), []byte{code})
	}
	return "", event.Key{}, false
}

func parseEscPair(code byte, opts KeyParseOptions) (string, event.Key, bool) {
	switch code {
	case 0x7f, 0x08:
		return keyFromID("alt+backspace", event.ActionPress, []byte{esc, code})
	case '\r', '\n':
		return keyFromID("alt+enter", event.ActionPress, []byte{esc, code})
	case '\t':
		return keyFromID("alt+tab", event.ActionPress, []byte{esc, code})
	}
	// Historical cursor aliases only in legacy mode
	if !opts.KittyActive {
		switch code {
		case ' ':
			return keyFromID("alt+space", event.ActionPress, []byte{esc, code})
		case 'B':
			return keyFromID("alt+left", event.ActionPress, []byte{esc, code})
		case 'F':
			return keyFromID("alt+right", event.ActionPress, []byte{esc, code})
		}
	}
	if code >= 1 && code <= 26 {
		letter := 'a' + rune(code-1)
		return keyFromID("ctrl+alt+"+string(letter), event.ActionPress, []byte{esc, code})
	}
	if code >= 'a' && code <= 'z' {
		return keyFromID("alt+"+string(code), event.ActionPress, []byte{esc, code})
	}
	if code >= 'A' && code <= 'Z' {
		letter := byte(code + 32)
		return keyFromID("alt+shift+"+string(letter), event.ActionPress, []byte{esc, code})
	}
	return "", event.Key{}, false
}

// parseModifyOtherKeys parses CSI 27 ; modifiers ; keycode ~ (tilde optional).
func parseModifyOtherKeys(bytes []byte) (modifier uint32, keycode int32, ok bool) {
	// \x1b[27;
	if len(bytes) < 7 || !hasPrefix(bytes, []byte{0x1b, '[', '2', '7', ';'}) {
		return 0, 0, false
	}
	end := len(bytes)
	if bytes[end-1] == '~' {
		end--
	}
	if end <= 5 {
		return 0, 0, false
	}
	idx := 5
	modValue, next, ok := parseDigits(bytes, idx, end)
	if !ok {
		return 0, 0, false
	}
	idx = next
	if idx >= end || bytes[idx] != ';' {
		return 0, 0, false
	}
	idx++
	kc, next, ok := parseDigits(bytes, idx, end)
	if !ok {
		return 0, 0, false
	}
	idx = next
	if idx != end || modValue == 0 {
		return 0, 0, false
	}
	return modValue - 1, int32(kc), true
}

func parseKittySequence(bytes []byte) (parsedKitty, bool) {
	if len(bytes) < 4 || bytes[0] != esc || bytes[1] != '[' {
		return parsedKitty{}, false
	}
	switch bytes[len(bytes)-1] {
	case 'u':
		return parseCSIU(bytes)
	case '~':
		return parseFunctional(bytes)
	case 'A', 'B', 'C', 'D', 'E', 'F', 'H', 'P', 'Q', 'R', 'S':
		return parseCSI1Letter(bytes)
	default:
		return parsedKitty{}, false
	}
}

func parseCSIU(bytes []byte) (parsedKitty, bool) {
	end := len(bytes) - 1 // index of 'u'
	idx := 2
	cp, next, ok := parseDigits(bytes, idx, end)
	if !ok {
		return parsedKitty{}, false
	}
	codepoint := int32(cp)
	idx = next

	var shiftedKey, baseLayoutKey int32
	if idx < end && bytes[idx] == ':' {
		idx++
		sv, next2, has := parseOptionalDigits(bytes, idx, end)
		if has {
			shiftedKey = int32(sv)
		}
		idx = next2
		if idx < end && bytes[idx] == ':' {
			idx++
			bv, next3, ok3 := parseDigits(bytes, idx, end)
			if !ok3 {
				return parsedKitty{}, false
			}
			baseLayoutKey = int32(bv)
			idx = next3
		}
	}

	modValue := uint32(1)
	var eventType uint32
	hasEvent := false
	if idx < end && bytes[idx] == ';' {
		idx++
		if idx < end && bytes[idx] >= '0' && bytes[idx] <= '9' {
			v, next4, ok4 := parseDigits(bytes, idx, end)
			if !ok4 {
				return parsedKitty{}, false
			}
			modValue = v
			idx = next4
		} else {
			modValue = 1
		}
		if idx < end && bytes[idx] == ':' {
			idx++
			ev, next5, ok5 := parseDigits(bytes, idx, end)
			if !ok5 {
				return parsedKitty{}, false
			}
			eventType = ev
			hasEvent = true
			idx = next5
		}
	}

	var textCP int32
	hasText := false
	textCount := 0
	if idx < end && bytes[idx] == ';' {
		idx++
		for idx < end {
			if bytes[idx] == ':' {
				idx++
				continue
			}
			tcp, next6, ok6 := parseDigits(bytes, idx, end)
			if !ok6 {
				return parsedKitty{}, false
			}
			textCount++
			if textCount == 1 {
				if tcp >= 32 && utf8.ValidRune(rune(tcp)) {
					textCP = int32(tcp)
					hasText = true
				}
			} else {
				textCP = 0
				hasText = false
			}
			idx = next6
			if idx < end && bytes[idx] == ':' {
				idx++
			}
		}
	}

	if idx != end || modValue == 0 {
		return parsedKitty{}, false
	}
	return parsedKitty{
		codepoint:     codepoint,
		shiftedKey:    shiftedKey,
		baseLayoutKey: baseLayoutKey,
		textCodepoint: textCP,
		hasText:       hasText,
		modifier:      modValue - 1,
		eventType:     eventType,
		hasEventType:  hasEvent,
	}, true
}

func parseCSI1Letter(bytes []byte) (parsedKitty, bool) {
	if !hasPrefix(bytes, []byte{0x1b, '[', '1', ';'}) {
		return parsedKitty{}, false
	}
	end := len(bytes)
	idx := 4
	modValue, next, ok := parseDigits(bytes, idx, end)
	if !ok {
		return parsedKitty{}, false
	}
	idx = next
	var eventType uint32
	hasEvent := false
	if idx < end && bytes[idx] == ':' {
		idx++
		ev, next2, ok2 := parseDigits(bytes, idx, end)
		if !ok2 {
			return parsedKitty{}, false
		}
		eventType = ev
		hasEvent = true
		idx = next2
	}
	if idx+1 != end || modValue == 0 {
		return parsedKitty{}, false
	}
	var codepoint int32
	switch bytes[idx] {
	case 'A':
		codepoint = arrowUp
	case 'B':
		codepoint = arrowDown
	case 'C':
		codepoint = arrowRight
	case 'D':
		codepoint = arrowLeft
	case 'H':
		codepoint = funcHome
	case 'F':
		codepoint = funcEnd
	case 'E':
		codepoint = funcClear
	case 'P':
		codepoint = funcF1
	case 'Q':
		codepoint = funcF2
	case 'R':
		codepoint = funcF3
	case 'S':
		codepoint = funcF4
	default:
		return parsedKitty{}, false
	}
	return parsedKitty{
		codepoint:    codepoint,
		modifier:     modValue - 1,
		eventType:    eventType,
		hasEventType: hasEvent,
	}, true
}

func parseFunctional(bytes []byte) (parsedKitty, bool) {
	end := len(bytes) - 1 // '~'
	idx := 2
	keyNum, next, ok := parseDigits(bytes, idx, end)
	if !ok {
		return parsedKitty{}, false
	}
	idx = next
	modValue := uint32(1)
	if idx < end && bytes[idx] == ';' {
		idx++
		v, next2, ok2 := parseDigits(bytes, idx, end)
		if !ok2 {
			return parsedKitty{}, false
		}
		modValue = v
		idx = next2
	}
	var eventType uint32
	hasEvent := false
	if idx < end && bytes[idx] == ':' {
		idx++
		ev, next3, ok3 := parseDigits(bytes, idx, end)
		if !ok3 {
			return parsedKitty{}, false
		}
		eventType = ev
		hasEvent = true
		idx = next3
	}
	if idx != end || modValue == 0 {
		return parsedKitty{}, false
	}
	var codepoint int32
	switch keyNum {
	case 2:
		codepoint = funcInsert
	case 3:
		codepoint = funcDelete
	case 5:
		codepoint = funcPageUp
	case 6:
		codepoint = funcPageDown
	case 1, 7:
		codepoint = funcHome
	case 4, 8:
		codepoint = funcEnd
	case 11:
		codepoint = funcF1
	case 12:
		codepoint = funcF2
	case 13:
		codepoint = funcF3
	case 14:
		codepoint = funcF4
	case 15:
		codepoint = funcF5
	case 17:
		codepoint = funcF6
	case 18:
		codepoint = funcF7
	case 19:
		codepoint = funcF8
	case 20:
		codepoint = funcF9
	case 21:
		codepoint = funcF10
	case 23:
		codepoint = funcF11
	case 24:
		codepoint = funcF12
	default:
		return parsedKitty{}, false
	}
	return parsedKitty{
		codepoint:    codepoint,
		modifier:     modValue - 1,
		eventType:    eventType,
		hasEventType: hasEvent,
	}, true
}

func formatKittyKey(p parsedKitty) (event.Key, string, bool) {
	effMod := event.Modifiers(p.modifier).WithoutLocks()
	supported := event.ModShift | event.ModCtrl | event.ModAlt | event.ModSuper
	if effMod&^supported != 0 {
		return event.Key{}, "", false
	}

	effectiveCP := p.codepoint
	if tcp, ok := keypadOperatorTextCodepoint(p.codepoint); ok {
		effectiveCP = tcp
	} else {
		cp := p.codepoint
		if isASCIILetterCP(cp) || isSymbolKey(cp) {
			effectiveCP = cp
		} else if p.baseLayoutKey != 0 {
			effectiveCP = p.baseLayoutKey
		}
	}

	if effMod == 0 {
		if p.hasText {
			if name := formatKeyName(p.textCodepoint); name != "" {
				return keyStruct(name, event.Code(baseCode(name)), 0, event.ActionPress, p)
			}
		}
		if tcp, ok := keypadNumLockTextCodepoint(p.codepoint); ok {
			if name := formatKeyName(tcp); name != "" {
				return keyStruct(name, event.Code(baseCode(name)), 0, event.ActionPress, p)
			}
		}
		name := formatKeyName(effectiveCP)
		if name == "" {
			return event.Key{}, "", false
		}
		return keyStruct(name, event.Code(baseCode(name)), 0, event.ActionPress, p)
	}

	name := formatKeyName(effectiveCP)
	if name == "" {
		return event.Key{}, "", false
	}
	id := event.FormatKeyID(effMod, name)
	// Native format_with_mods uses shift+ctrl+alt+super order (shift before ctrl).
	// event.FormatKeyID uses ctrl+shift+alt+super (canonical binding order).
	// parse_key returns the native order; CanonicalKeyID normalizes for bindings.
	// Prefer native wire-order here to match parseKey() output, then expose
	// CanonicalKeyID separately. Match native:
	id = formatWithModsNative(uint32(effMod), name)
	k := event.Key{
		ID:            id,
		Code:          event.Code(baseCode(name)),
		Mods:          effMod,
		Action:        event.ActionPress,
		Codepoint:     runeOrZero(p.codepoint),
		ShiftedKey:    runeOrZero(p.shiftedKey),
		BaseLayoutKey: runeOrZero(p.baseLayoutKey),
	}
	return k, id, true
}

// formatWithModsNative matches keys.rs format_with_mods: shift, ctrl, alt, super.
func formatWithModsNative(mods uint32, keyName string) string {
	var b strings.Builder
	b.Grow(16 + len(keyName))
	if mods&kittyModShift != 0 {
		b.WriteString("shift+")
	}
	if mods&kittyModCtrl != 0 {
		b.WriteString("ctrl+")
	}
	if mods&kittyModAlt != 0 {
		b.WriteString("alt+")
	}
	if mods&kittyModSuper != 0 {
		b.WriteString("super+")
	}
	b.WriteString(keyName)
	return b.String()
}

func keyStruct(id string, code event.Code, mods event.Modifiers, act event.Action, p parsedKitty) (event.Key, string, bool) {
	k := event.Key{
		ID:            id,
		Code:          code,
		Mods:          mods,
		Action:        act,
		Codepoint:     runeOrZero(p.codepoint),
		ShiftedKey:    runeOrZero(p.shiftedKey),
		BaseLayoutKey: runeOrZero(p.baseLayoutKey),
	}
	// Printable text for unmodified / shift-only keys filled by decodePrintable.
	return k, id, true
}

func formatKeyName(codepoint int32) string {
	switch codepoint {
	case cpEscape:
		return "escape"
	case cpTab:
		return "tab"
	case cpEnter, cpKPEnter:
		return "enter"
	case cpSpace:
		return "space"
	case cpBackspace:
		return "backspace"
	case cpKP0:
		return "insert"
	case cpKP1:
		return "end"
	case cpKP2:
		return "down"
	case cpKP3:
		return "pageDown"
	case cpKP4:
		return "left"
	case cpKP5:
		return "clear"
	case cpKP6:
		return "right"
	case cpKP7:
		return "home"
	case cpKP8:
		return "up"
	case cpKP9:
		return "pageUp"
	case cpKPDecimal:
		return "delete"
	case funcDelete:
		return "delete"
	case funcInsert:
		return "insert"
	case funcHome:
		return "home"
	case funcEnd:
		return "end"
	case funcPageUp:
		return "pageUp"
	case funcPageDown:
		return "pageDown"
	case funcClear:
		return "clear"
	case arrowUp:
		return "up"
	case arrowDown:
		return "down"
	case arrowLeft:
		return "left"
	case arrowRight:
		return "right"
	case funcF1:
		return "f1"
	case funcF2:
		return "f2"
	case funcF3:
		return "f3"
	case funcF4:
		return "f4"
	case funcF5:
		return "f5"
	case funcF6:
		return "f6"
	case funcF7:
		return "f7"
	case funcF8:
		return "f8"
	case funcF9:
		return "f9"
	case funcF10:
		return "f10"
	case funcF11:
		return "f11"
	case funcF12:
		return "f12"
	}
	if codepoint >= 33 && codepoint <= 126 {
		return string(rune(codepoint))
	}
	return ""
}

func keypadOperatorTextCodepoint(cp int32) (int32, bool) {
	switch cp {
	case cpKPDivide:
		return 47, true
	case cpKPMultiply:
		return 42, true
	case cpKPSubtract:
		return 45, true
	case cpKPAdd:
		return 43, true
	case cpKPEquals:
		return 61, true
	}
	return 0, false
}

func keypadNumLockTextCodepoint(cp int32) (int32, bool) {
	switch cp {
	case cpKP0:
		return 48, true
	case cpKP1:
		return 49, true
	case cpKP2:
		return 50, true
	case cpKP3:
		return 51, true
	case cpKP4:
		return 52, true
	case cpKP5:
		return 53, true
	case cpKP6:
		return 54, true
	case cpKP7:
		return 55, true
	case cpKP8:
		return 56, true
	case cpKP9:
		return 57, true
	case cpKPDecimal:
		return 46, true
	}
	return 0, false
}

func isSymbolKey(cp int32) bool {
	switch cp {
	case 96, 34, 45, 61, 91, 93, 92, 59, 39, 44, 46, 47,
		33, 64, 35, 36, 37, 94, 38, 42, 40, 41, 95, 43,
		124, 126, 123, 125, 58, 60, 62, 63:
		return true
	}
	return false
}

func isASCIILetterCP(cp int32) bool {
	return (cp >= 'A' && cp <= 'Z') || (cp >= 'a' && cp <= 'z')
}

func parseDigits(b []byte, idx, end int) (uint32, int, bool) {
	if idx >= end || b[idx] < '0' || b[idx] > '9' {
		return 0, idx, false
	}
	var v uint32
	for idx < end && b[idx] >= '0' && b[idx] <= '9' {
		v = v*10 + uint32(b[idx]-'0')
		idx++
	}
	return v, idx, true
}

func parseOptionalDigits(b []byte, idx, end int) (uint32, int, bool) {
	if idx >= end || b[idx] < '0' || b[idx] > '9' {
		return 0, idx, false
	}
	return parseDigits(b, idx, end)
}

func keyFromID(id string, act event.Action, raw []byte) (string, event.Key, bool) {
	mods, base, ok := event.SplitKeyID(id)
	if !ok {
		// id may already be a simple name
		base = id
		mods = 0
	}
	// SplitKeyID lowercases poorly for mixed; use Canonical for Code
	canon := event.CanonicalKeyID(id)
	// Extract base from canon
	_, cbase, _ := event.SplitKeyID(canon)
	if cbase == "" {
		cbase = base
	}
	// Prefer the id as produced (native order for multi-mod from legacy table)
	k := event.Key{
		ID:     id,
		Code:   event.Code(baseCode(cbase)),
		Mods:   mods,
		Action: act,
	}
	_ = raw
	return id, k, true
}

func keyFromParts(id string, code event.Code, mods event.Modifiers, act event.Action, cp int32, raw []byte) (string, event.Key, bool) {
	_ = raw
	k := event.Key{
		ID:        id,
		Code:      code,
		Mods:      mods,
		Action:    act,
		Codepoint: runeOrZero(cp),
	}
	return id, k, true
}

func baseCode(name string) string {
	switch strings.ToLower(name) {
	case "esc", "escape":
		return "escape"
	case "return", "enter":
		return "enter"
	case "tab":
		return "tab"
	case "backspace":
		return "backspace"
	case "delete":
		return "delete"
	case "insert":
		return "insert"
	case "clear":
		return "clear"
	case "home":
		return "home"
	case "end":
		return "end"
	case "pageup":
		return "pageUp"
	case "pagedown":
		return "pageDown"
	case "up":
		return "up"
	case "down":
		return "down"
	case "left":
		return "left"
	case "right":
		return "right"
	case "space":
		return "space"
	case "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
		return strings.ToLower(name)
	}
	// Single ASCII letter → lowercase code
	if len(name) == 1 {
		c := name[0]
		if c >= 'A' && c <= 'Z' {
			return string(c + 32)
		}
	}
	return name
}

func runeOrZero(cp int32) rune {
	if cp <= 0 {
		return 0
	}
	if cp > 0x10ffff {
		return 0
	}
	return rune(cp)
}

// decodePrintableKey returns the printable character a sequence inserts, if any.
// Port of keys.ts decodePrintableKey / decodeKittyPrintable / decodeModifyOtherKeysPrintable.
func decodePrintableKey(data []byte) (string, bool) {
	if s, ok := decodeKittyPrintable(data); ok {
		return s, true
	}
	return decodeModifyOtherKeysPrintable(data)
}

func decodeKittyPrintable(data []byte) (string, bool) {
	pk, ok := parseCSIUOnly(data)
	if !ok {
		return "", false
	}
	// release → not text
	if pk.hasEventType && pk.eventType == 3 {
		return "", false
	}
	eff := pk.modifier &^ kittyLockMask
	supported := uint32(kittyModShift | kittyModAlt | kittyModCtrl | kittyModSuper)
	if eff&^supported != 0 {
		return "", false
	}
	if eff&(kittyModAlt|kittyModCtrl|kittyModSuper) != 0 {
		return "", false
	}
	// text field
	if pk.hasText && pk.textCodepoint >= 32 && pk.textCodepoint != 127 {
		return string(rune(pk.textCodepoint)), true
	}
	if op, ok := keypadOperatorTextCodepoint(pk.codepoint); ok {
		return string(rune(op)), true
	}
	if eff == 0 {
		if n, ok := keypadNumLockTextCodepoint(pk.codepoint); ok {
			return string(rune(n)), true
		}
	}
	effCP := pk.codepoint
	if eff&kittyModShift != 0 && pk.shiftedKey != 0 {
		effCP = pk.shiftedKey
	}
	// Reject PUA
	if effCP >= 0xe000 && effCP <= 0xf8ff {
		return "", false
	}
	if effCP < 32 || effCP == 127 {
		return "", false
	}
	if effCP > 0x10ffff {
		return "", false
	}
	return string(rune(effCP)), true
}

// parseCSIUOnly is parseKittySequence restricted to 'u' terminator (for printable path).
func parseCSIUOnly(data []byte) (parsedKitty, bool) {
	if len(data) < 4 || data[len(data)-1] != 'u' {
		return parsedKitty{}, false
	}
	return parseCSIU(data)
}

func decodeModifyOtherKeysPrintable(data []byte) (string, bool) {
	mods, keycode, ok := parseModifyOtherKeys(data)
	if !ok {
		return "", false
	}
	mod := mods &^ kittyLockMask
	if mod&^kittyModShift != 0 {
		return "", false
	}
	if keycode < 32 || keycode == 127 {
		return "", false
	}
	return string(rune(keycode)), true
}

// decodeKittyKeypadText returns text for keypad digit/operator CSI-u sequences only.
func decodeKittyKeypadText(data []byte) (string, bool) {
	pk, ok := parseCSIUOnly(data)
	if !ok {
		return "", false
	}
	if _, ok := keypadNumLockTextCodepoint(pk.codepoint); !ok {
		if _, ok2 := keypadOperatorTextCodepoint(pk.codepoint); !ok2 {
			return "", false
		}
	}
	return decodeKittyPrintable(data)
}

// extractPrintableText returns insertion text for a framed sequence.
func extractPrintableText(data []byte) (string, bool) {
	if s, ok := decodePrintableKey(data); ok {
		return s, true
	}
	if len(data) == 0 || hasControlChars(data) {
		return "", false
	}
	return string(data), true
}

func hasControlChars(data []byte) bool {
	i := 0
	for i < len(data) {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			c := data[i]
			if c < 32 || c == 0x7f || (c >= 0x80 && c <= 0x9f) {
				return true
			}
			i++
			continue
		}
		if r < 32 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
		i += size
	}
	return false
}

// parseUnmodifiedKittyPrintableCodepoint returns the codepoint for
// unmodified CSI-u printables used by the dedup window, or -1.
func parseUnmodifiedKittyPrintableCodepoint(seq []byte) int32 {
	// /^\x1b\[(\d+)(?::\d*)?(?::\d+)?u$/
	if len(seq) < 4 || seq[0] != esc || seq[1] != '[' || seq[len(seq)-1] != 'u' {
		return -1
	}
	// Must not contain ';' (modifiers / text)
	body := seq[2 : len(seq)-1]
	for _, c := range body {
		if c == ';' {
			return -1
		}
	}
	// Parse leading digits
	i := 0
	if i >= len(body) || body[i] < '0' || body[i] > '9' {
		return -1
	}
	var cp int32
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		cp = cp*10 + int32(body[i]-'0')
		i++
	}
	// optional :shifted optional :base
	if i < len(body) {
		if body[i] != ':' {
			return -1
		}
		i++
		// optional digits
		for i < len(body) && body[i] >= '0' && body[i] <= '9' {
			i++
		}
		if i < len(body) {
			if body[i] != ':' {
				return -1
			}
			i++
			if i >= len(body) || body[i] < '0' || body[i] > '9' {
				return -1
			}
			for i < len(body) && body[i] >= '0' && body[i] <= '9' {
				i++
			}
		}
	}
	if i != len(body) {
		return -1
	}
	if cp >= 32 {
		return cp
	}
	return -1
}

// IsKeyRelease reports Kitty release when protocol is active.
func IsKeyRelease(data []byte, kittyActive bool) bool {
	if !kittyActive {
		return false
	}
	if indexOfBytes(data, pasteStart) >= 0 {
		return false
	}
	return kittyEventTypeSuffix(data, '3')
}

// IsKeyRepeat reports Kitty repeat when protocol is active.
func IsKeyRepeat(data []byte, kittyActive bool) bool {
	if !kittyActive {
		return false
	}
	if indexOfBytes(data, pasteStart) >= 0 {
		return false
	}
	return kittyEventTypeSuffix(data, '2')
}

// kittyEventTypeSuffix matches /^\x1b\[[\d:;]*:X[u~ABCDHF]$/ for event type digit X.
func kittyEventTypeSuffix(data []byte, eventDigit byte) bool {
	if len(data) < 5 || data[0] != esc || data[1] != '[' {
		return false
	}
	last := data[len(data)-1]
	switch last {
	case 'u', '~', 'A', 'B', 'C', 'D', 'H', 'F':
	default:
		return false
	}
	// Must contain :eventDigit immediately before terminator... actually
	// pattern is [\d:;]*:3[u~ABCDHF] — the :3 is just before the final.
	// Scan body for :eventDigit at end-2.
	if len(data) < 5 {
		return false
	}
	// Verify all middle bytes are digits, ':', or ';'
	for i := 2; i < len(data)-1; i++ {
		c := data[i]
		if (c >= '0' && c <= '9') || c == ':' || c == ';' {
			continue
		}
		return false
	}
	// Need :eventDigit right before final byte
	if data[len(data)-2] != eventDigit {
		return false
	}
	if len(data) < 4 || data[len(data)-3] != ':' {
		return false
	}
	return true
}

// MatchesRawBackspace implements the Windows Terminal 0x08 heuristic.
func MatchesRawBackspace(data []byte, expectedMod event.Modifiers, windowsTerminal bool) bool {
	if len(data) != 1 {
		return false
	}
	switch data[0] {
	case 0x7f:
		return expectedMod == 0
	case 0x08:
		if windowsTerminal {
			return expectedMod == event.ModCtrl
		}
		return expectedMod == 0
	}
	return false
}

package input

// decodeReencodedPasteControls expands tmux extended-keys re-encodings inside
// a bracketed-paste payload back to literal control bytes.
//
// Observed formats (extended-keys-format):
//   - csi-u:  ESC [ <codepoint> ; 5 u        (Ctrl+J → ESC [ 106 ; 5 u)
//   - xterm:  ESC [ 27 ; 5 ; <codepoint> ~   (Ctrl+J → ESC [ 27 ; 5 ; 106 ~)
//
// Only Ctrl+letter is decoded (a-z/A-Z → 0x01..0x1A). Non-letter Ctrl combos
// are left untouched. Port of bracketed-paste.ts decodeReencodedPasteControls.
func decodeReencodedPasteControls(text []byte) []byte {
	if len(text) == 0 {
		return text
	}
	// Fast path: no ESC → unchanged
	if indexOfBytes(text, []byte{esc}) < 0 {
		return text
	}
	out := make([]byte, 0, len(text))
	i := 0
	for i < len(text) {
		if text[i] != esc {
			out = append(out, text[i])
			i++
			continue
		}
		// Try CSI-u: ESC [ <digits> ; 5 u
		if n, cp, ok := matchReencodedCSIU(text[i:]); ok {
			if b, ok := ctrlByteFromCodepoint(cp); ok {
				out = append(out, b)
			} else {
				out = append(out, text[i:i+n]...)
			}
			i += n
			continue
		}
		// Try xterm: ESC [ 27 ; 5 ; <digits> ~
		if n, cp, ok := matchReencodedXTerm(text[i:]); ok {
			if b, ok := ctrlByteFromCodepoint(cp); ok {
				out = append(out, b)
			} else {
				out = append(out, text[i:i+n]...)
			}
			i += n
			continue
		}
		out = append(out, text[i])
		i++
	}
	return out
}

// matchReencodedCSIU matches \x1b\[(\d+);5u at the start of b.
// Returns consumed byte count and codepoint.
func matchReencodedCSIU(b []byte) (n int, cp int, ok bool) {
	// ESC [ digits ; 5 u
	if len(b) < 6 || b[0] != esc || b[1] != '[' {
		return 0, 0, false
	}
	i := 2
	if i >= len(b) || b[i] < '0' || b[i] > '9' {
		return 0, 0, false
	}
	v := 0
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		v = v*10 + int(b[i]-'0')
		i++
	}
	if i+2 >= len(b) { // need ;5u
		return 0, 0, false
	}
	if b[i] != ';' || b[i+1] != '5' || b[i+2] != 'u' {
		return 0, 0, false
	}
	return i + 3, v, true
}

// matchReencodedXTerm matches \x1b\[27;5;(\d+)~ at the start of b.
func matchReencodedXTerm(b []byte) (n int, cp int, ok bool) {
	// ESC [ 2 7 ; 5 ; digits ~
	const prefix = "\x1b[27;5;"
	if len(b) < len(prefix)+2 { // + digit + ~
		return 0, 0, false
	}
	for i := 0; i < len(prefix); i++ {
		if b[i] != prefix[i] {
			return 0, 0, false
		}
	}
	i := len(prefix)
	if b[i] < '0' || b[i] > '9' {
		return 0, 0, false
	}
	v := 0
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		v = v*10 + int(b[i]-'0')
		i++
	}
	if i >= len(b) || b[i] != '~' {
		return 0, 0, false
	}
	return i + 1, v, true
}

func ctrlByteFromCodepoint(cp int) (byte, bool) {
	// a-z → Ctrl+A..Ctrl+Z
	if cp >= 97 && cp <= 122 {
		return byte(cp - 96), true
	}
	// A-Z → Ctrl+A..Ctrl+Z
	if cp >= 65 && cp <= 90 {
		return byte(cp - 64), true
	}
	return 0, false
}

// DecodeReencodedPasteControls is the exported form operating on strings.
func DecodeReencodedPasteControls(text string) string {
	return string(decodeReencodedPasteControls([]byte(text)))
}

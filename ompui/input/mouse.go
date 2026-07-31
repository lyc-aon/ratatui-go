package input

import "github.com/lyc-aon/ratatui-go/ompui/event"

// parseSGRMouse decodes an SGR mouse report `\x1b[<btn;col;rowM|m`.
// Returns ok=false when data is not a complete SGR mouse sequence.
// Col/Row are converted to 0-based.
//
// Port of packages/tui/src/mouse.ts parseSgrMouse.
func parseSGRMouse(data []byte) (event.Mouse, bool) {
	// Minimum: ESC [ < 0 ; 0 ; 0 M  => 9 bytes? actually \x1b[<0;0;0M = 9
	if len(data) < 8 {
		return event.Mouse{}, false
	}
	if data[0] != esc || data[1] != '[' || data[2] != '<' {
		return event.Mouse{}, false
	}
	term := data[len(data)-1]
	if term != 'M' && term != 'm' {
		return event.Mouse{}, false
	}
	body := data[3 : len(data)-1] // between '<' and terminator
	// Parse three decimal fields separated by ';'
	btn, rest, ok := parseDecField(body)
	if !ok {
		return event.Mouse{}, false
	}
	col1, rest, ok := parseDecField(rest)
	if !ok {
		return event.Mouse{}, false
	}
	row1, rest, ok := parseDecField(rest)
	if !ok || len(rest) != 0 {
		return event.Mouse{}, false
	}
	release := term == 'm'
	wheel := 0
	if btn&64 != 0 {
		if btn&1 != 0 {
			wheel = 1
		} else {
			wheel = -1
		}
	}
	motion := btn&32 != 0 && wheel == 0
	leftClick := !release && wheel == 0 && !motion && (btn&3) == 0

	// Modifier bits in SGR button code: +4 shift, +8 alt, +16 ctrl
	var mods event.Modifiers
	if btn&4 != 0 {
		mods |= event.ModShift
	}
	if btn&8 != 0 {
		mods |= event.ModAlt
	}
	if btn&16 != 0 {
		mods |= event.ModCtrl
	}

	return event.Mouse{
		Button:    btn,
		Col:       col1 - 1,
		Row:       row1 - 1,
		Release:   release,
		Motion:    motion,
		LeftClick: leftClick,
		Wheel:     wheel,
		Mods:      mods,
	}, true
}

// parseX10Mouse decodes legacy `\x1b[M` + 3 bytes (cb, cx, cy) with
// coordinates encoded as byte-32. Rare; still framed by the splitter.
func parseX10Mouse(data []byte) (event.Mouse, bool) {
	if len(data) != 6 {
		return event.Mouse{}, false
	}
	if data[0] != esc || data[1] != '[' || data[2] != 'M' {
		return event.Mouse{}, false
	}
	cb := int(data[3]) - 32
	cx := int(data[4]) - 32 - 1
	cy := int(data[5]) - 32 - 1
	if cx < 0 {
		cx = 0
	}
	if cy < 0 {
		cy = 0
	}
	release := false
	wheel := 0
	motion := false
	// X10 button low bits
	btnLow := cb & 3
	if cb&64 != 0 {
		if cb&1 != 0 {
			wheel = 1
		} else {
			wheel = -1
		}
	}
	if cb&32 != 0 && wheel == 0 {
		motion = true
	}
	// button 3 = release in X10
	if btnLow == 3 {
		release = true
	}
	leftClick := !release && wheel == 0 && !motion && btnLow == 0
	return event.Mouse{
		Button:    cb,
		Col:       cx,
		Row:       cy,
		Release:   release,
		Motion:    motion,
		LeftClick: leftClick,
		Wheel:     wheel,
	}, true
}

// parseDecField parses a leading unsigned decimal and optional trailing ';'.
// On success rest is after the field and its separator (if any).
// The final field has no trailing ';'; rest will be empty when that is last.
func parseDecField(b []byte) (value int, rest []byte, ok bool) {
	if len(b) == 0 {
		return 0, nil, false
	}
	i := 0
	if b[i] < '0' || b[i] > '9' {
		return 0, nil, false
	}
	v := 0
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		v = v*10 + int(b[i]-'0')
		i++
	}
	if i < len(b) {
		if b[i] != ';' {
			return 0, nil, false
		}
		return v, b[i+1:], true
	}
	return v, nil, true
}

// isSGRMousePrefix is a cheap hot-path gate: data starts with ESC [ <
func isSGRMousePrefix(data []byte) bool {
	return len(data) >= 3 && data[0] == esc && data[1] == '[' && data[2] == '<'
}

// isX10MousePrefix is ESC [ M
func isX10MousePrefix(data []byte) bool {
	return len(data) >= 3 && data[0] == esc && data[1] == '[' && data[2] == 'M'
}

package interact

import (
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/event"
)

// OMP default key IDs for common actions (see packages/tui keybindings.ts).
var (
	keysCancel             = []string{"escape", "ctrl+c"}
	keysSelectUp           = []string{"up"}
	keysSelectDown         = []string{"down"}
	keysSelectPageUp       = []string{"pageup"}
	keysSelectPageDown     = []string{"pagedown"}
	keysConfirm            = []string{"enter"}
	keysSubmit             = []string{"enter"}
	keysTab                = []string{"tab"}
	keysShiftTab           = []string{"shift+tab"}
	keysUndo               = []string{"ctrl+-", "ctrl+_"}
	keysDeleteCharBackward = []string{"backspace"}
	keysDeleteCharForward  = []string{"delete", "ctrl+d"}
	keysDeleteWordBackward = []string{"ctrl+w", "alt+backspace", "ctrl+backspace", "super+alt+backspace"}
	keysDeleteWordForward  = []string{"alt+delete", "alt+d", "super+alt+delete", "super+alt+d"}
	keysDeleteToLineStart  = []string{"ctrl+u"}
	keysDeleteToLineEnd    = []string{"ctrl+k"}
	keysYank               = []string{"ctrl+y"}
	keysYankPop            = []string{"alt+y"}
	keysCursorLeft         = []string{"left", "ctrl+b"}
	keysCursorRight        = []string{"right", "ctrl+f"}
	keysCursorLineStart    = []string{"home", "ctrl+a"}
	keysCursorLineEnd      = []string{"end", "ctrl+e"}
	keysCursorWordLeft     = []string{"alt+left", "ctrl+left", "alt+b"}
	keysCursorWordRight    = []string{"alt+right", "ctrl+right", "alt+f"}
	keysScrollUp           = []string{"up"}
	keysScrollDown         = []string{"down"}
	keysScrollFastUp       = []string{"shift+up"}
	keysScrollFastDown     = []string{"shift+down"}
	keysHome               = []string{"home"}
	keysEnd                = []string{"end"}
	keysLeft               = []string{"left"}
	keysRight              = []string{"right"}
	keysVimUp              = []string{"k"}
	keysVimDown            = []string{"j"}
)

// keyPress reports a non-release key event.
func keyPress(ev event.Event) bool {
	return ev.Kind == event.KindKey && ev.Key.Action != event.ActionRelease
}

// matchKeys reports whether ev is a press/repeat matching any binding key id.
func matchKeys(ev event.Event, bindings []string) bool {
	if !keyPress(ev) {
		return false
	}
	return event.MatchesAnyKey(ev.Key, bindings...)
}

// PrintableText returns insertion text from a key or text event.
// Empty when the event is a pure binding.
func PrintableText(ev event.Event) string {
	switch ev.Kind {
	case event.KindText:
		return filterPrintable(ev.Text)
	case event.KindKey:
		if ev.Key.Action == event.ActionRelease {
			return ""
		}
		switch ev.Key.Code {
		case event.CodeEscape, event.CodeEnter, event.CodeTab, event.CodeBackspace,
			event.CodeDelete, event.CodeInsert, event.CodeHome, event.CodeEnd,
			event.CodePageUp, event.CodePageDown, event.CodeUp, event.CodeDown,
			event.CodeLeft, event.CodeRight,
			event.CodeF1, event.CodeF2, event.CodeF3, event.CodeF4,
			event.CodeF5, event.CodeF6, event.CodeF7, event.CodeF8,
			event.CodeF9, event.CodeF10, event.CodeF11, event.CodeF12:
			return ""
		}
		mods := ev.Key.Mods.WithoutLocks()
		if mods&^event.ModShift != 0 {
			return ""
		}
		if ev.Key.Text != "" {
			return filterPrintable(ev.Key.Text)
		}
		if len(ev.Key.Code) == 1 {
			r := rune(ev.Key.Code[0])
			if r >= 0x20 && r != 0x7f {
				return string(r)
			}
		}
		return ""
	default:
		return ""
	}
}

func filterPrintable(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' {
			b.WriteString(strings.Repeat(" ", 3))
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// IsEnter reports enter / bare newline submit.
func IsEnter(ev event.Event) bool {
	if matchKeys(ev, keysConfirm) {
		return true
	}
	if ev.Kind == event.KindText && (ev.Text == "\n" || ev.Text == "\r") {
		return true
	}
	return false
}

// IsSpace reports a space key or text " ".
func IsSpace(ev event.Event) bool {
	if ev.Kind == event.KindText && ev.Text == " " {
		return true
	}
	if keyPress(ev) {
		if ev.Key.Code == event.CodeSpace || ev.Key.Text == " " {
			mods := ev.Key.Mods.WithoutLocks()
			return mods&^event.ModShift == 0
		}
	}
	return false
}

// IsCancel reports escape / ctrl+c.
func IsCancel(ev event.Event) bool {
	return matchKeys(ev, keysCancel)
}

package interact

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// KeyMatcher resolves a configurable keybinding action id against a decoded key.
//
// *ompui/keymap.Registry satisfies this interface. Components take the
// interface rather than importing keymap so leaf widgets keep no dependency on
// configuration loading and hosts may supply their own resolver.
type KeyMatcher interface {
	Matches(k event.Key, action string) bool
}

// keyAction pairs a keymap action id with the bindings used when no KeyMatcher
// is injected. Fallback slices are read-only.
type keyAction struct {
	id       string
	fallback []string
}

// Actions declared by ompui/keymap and consumed by interact components.
// Fallbacks mirror the OMP defaults for the same ids (see packages/tui
// keybindings.ts); an injected matcher replaces them wholesale.
var (
	actSelectUp       = keyAction{"tui.select.up", []string{"up"}}
	actSelectDown     = keyAction{"tui.select.down", []string{"down"}}
	actSelectPageUp   = keyAction{"tui.select.pageUp", []string{"pageup"}}
	actSelectPageDown = keyAction{"tui.select.pageDown", []string{"pagedown"}}
	actSelectConfirm  = keyAction{"tui.select.confirm", []string{"enter"}}
	actSelectCancel   = keyAction{"tui.select.cancel", []string{"escape", "ctrl+c"}}
	actInputSubmit    = keyAction{"tui.input.submit", []string{"enter"}}

	actUndo               = keyAction{"tui.editor.undo", []string{"ctrl+-", "ctrl+_"}}
	actDeleteCharBackward = keyAction{"tui.editor.deleteCharBackward", []string{"backspace"}}
	actDeleteCharForward  = keyAction{"tui.editor.deleteCharForward", []string{"delete", "ctrl+d"}}
	actDeleteWordBackward = keyAction{"tui.editor.deleteWordBackward", []string{"ctrl+w", "alt+backspace", "ctrl+backspace", "super+alt+backspace"}}
	actDeleteWordForward  = keyAction{"tui.editor.deleteWordForward", []string{"alt+delete", "alt+d", "super+alt+delete", "super+alt+d"}}
	actDeleteToLineStart  = keyAction{"tui.editor.deleteToLineStart", []string{"ctrl+u"}}
	actDeleteToLineEnd    = keyAction{"tui.editor.deleteToLineEnd", []string{"ctrl+k"}}
	actYank               = keyAction{"tui.editor.yank", []string{"ctrl+y"}}
	actYankPop            = keyAction{"tui.editor.yankPop", []string{"alt+y"}}
	actCursorLeft         = keyAction{"tui.editor.cursorLeft", []string{"left", "ctrl+b"}}
	actCursorRight        = keyAction{"tui.editor.cursorRight", []string{"right", "ctrl+f"}}
	actCursorLineStart    = keyAction{"tui.editor.cursorLineStart", []string{"home", "ctrl+a"}}
	actCursorLineEnd      = keyAction{"tui.editor.cursorLineEnd", []string{"end", "ctrl+e"}}
	actCursorWordLeft     = keyAction{"tui.editor.cursorWordLeft", []string{"alt+left", "ctrl+left", "alt+b"}}
	actCursorWordRight    = keyAction{"tui.editor.cursorWordRight", []string{"alt+right", "ctrl+right", "alt+f"}}
)

// Component-local keys with no declared keymap action. They stay fixed so that
// rebinding a tui.* action never silently steals a widget's own navigation.
var (
	keysCancel         = []string{"escape", "ctrl+c"}
	keysScrollUp       = []string{"up"}
	keysScrollDown     = []string{"down"}
	keysScrollFastUp   = []string{"shift+up"}
	keysScrollFastDown = []string{"shift+down"}
	keysScrollPageUp   = []string{"pageup"}
	keysScrollPageDown = []string{"pagedown"}
	keysHome           = []string{"home"}
	keysEnd            = []string{"end"}
	keysLeft           = []string{"left"}
	keysRight          = []string{"right"}
	keysTab            = []string{"tab"}
	keysShiftTab       = []string{"shift+tab"}
	keysVimUp          = []string{"k"}
	keysVimDown        = []string{"j"}
)

// keyBindings is the instance-scoped action resolver embedded by input
// components. Its zero value resolves through the package fallbacks, so
// components constructed without a host behave exactly as before injection.
type keyBindings struct {
	matcher KeyMatcher
}

// SetKeyMatcher injects the host's resolved keybinding source, replacing the
// built-in defaults for every action this component resolves. Passing nil
// restores the defaults. The matcher is stored per instance; no package state
// is mutated.
func (b *keyBindings) SetKeyMatcher(m KeyMatcher) { b.matcher = m }

// match reports whether ev triggers action a for this instance.
func (b *keyBindings) match(ev event.Event, a keyAction) bool {
	if !keyPress(ev) {
		return false
	}
	if b.matcher != nil {
		return b.matcher.Matches(ev.Key, a.id)
	}
	return event.MatchesAnyKey(ev.Key, a.fallback...)
}

// isCancel reports the resolved tui.select.cancel action.
func (b *keyBindings) isCancel(ev event.Event) bool { return b.match(ev, actSelectCancel) }

// isConfirm reports the resolved tui.select.confirm action, or a bare newline
// delivered as text (some terminals submit that way).
func (b *keyBindings) isConfirm(ev event.Event) bool {
	return b.match(ev, actSelectConfirm) || isTextNewline(ev)
}

// isSubmit reports the resolved tui.input.submit action, or a bare newline
// delivered as text.
func (b *keyBindings) isSubmit(ev event.Event) bool {
	return b.match(ev, actInputSubmit) || isTextNewline(ev)
}

func isTextNewline(ev event.Event) bool {
	return ev.Kind == event.KindText && (ev.Text == "\n" || ev.Text == "\r")
}

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

// IsEnter reports enter / bare newline submit using the default bindings.
// Components with an injected KeyMatcher resolve tui.select.confirm instead.
func IsEnter(ev event.Event) bool {
	return matchKeys(ev, actSelectConfirm.fallback) || isTextNewline(ev)
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

// IsCancel reports escape / ctrl+c using the default bindings. Components with
// an injected KeyMatcher resolve tui.select.cancel instead.
func IsCancel(ev event.Event) bool {
	return matchKeys(ev, keysCancel)
}

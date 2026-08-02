package editor

import "github.com/lyc-aon/ratatui-go/ompui/event"

// KeyMatcher resolves a configurable keybinding action id against a decoded key.
//
// *ompui/keymap.Registry satisfies this interface. The editor takes the
// interface rather than importing keymap so the leaf component keeps no
// dependency on configuration loading and hosts may supply their own resolver.
type KeyMatcher interface {
	Matches(k event.Key, action string) bool
}

// keyAction pairs a keymap action id with the bindings used when no KeyMatcher
// is injected. Fallback slices are read-only and reproduce the editor's
// historical hardcoded keys, so embedders that never inject a matcher keep
// byte-identical behavior.
type keyAction struct {
	id       string
	fallback []string
}

// Actions declared by ompui/keymap and resolved by the editor.
var (
	actCursorUp        = keyAction{"tui.editor.cursorUp", []string{"up"}}
	actCursorDown      = keyAction{"tui.editor.cursorDown", []string{"down"}}
	actCursorLeft      = keyAction{"tui.editor.cursorLeft", []string{"left"}}
	actCursorRight     = keyAction{"tui.editor.cursorRight", []string{"right"}}
	actCursorWordLeft  = keyAction{"tui.editor.cursorWordLeft", []string{"alt+left", "ctrl+left", "alt+b"}}
	actCursorWordRight = keyAction{"tui.editor.cursorWordRight", []string{"alt+right", "ctrl+right", "alt+f"}}
	actCursorLineStart = keyAction{"tui.editor.cursorLineStart", []string{"ctrl+a", "home"}}
	actCursorLineEnd   = keyAction{"tui.editor.cursorLineEnd", []string{"ctrl+e", "end"}}
	actJumpForward     = keyAction{"tui.editor.jumpForward", []string{"ctrl+]"}}
	actJumpBackward    = keyAction{"tui.editor.jumpBackward", []string{"ctrl+alt+]"}}
	actPageUp          = keyAction{"tui.editor.pageUp", []string{"pageup"}}
	actPageDown        = keyAction{"tui.editor.pageDown", []string{"pagedown"}}

	actDeleteCharBackward = keyAction{"tui.editor.deleteCharBackward", []string{"backspace"}}
	actDeleteCharForward  = keyAction{"tui.editor.deleteCharForward", []string{"delete", "ctrl+d"}}
	actDeleteWordBackward = keyAction{"tui.editor.deleteWordBackward", []string{"ctrl+w", "alt+backspace", "super+alt+backspace"}}
	actDeleteWordForward  = keyAction{"tui.editor.deleteWordForward", []string{"alt+d", "alt+delete", "super+alt+d", "super+alt+delete"}}
	actDeleteToLineStart  = keyAction{"tui.editor.deleteToLineStart", []string{"ctrl+u"}}
	actDeleteToLineEnd    = keyAction{"tui.editor.deleteToLineEnd", []string{"ctrl+k"}}
	actYank               = keyAction{"tui.editor.yank", []string{"ctrl+y"}}
	actYankPop            = keyAction{"tui.editor.yankPop", []string{"alt+y"}}
	actUndo               = keyAction{"tui.editor.undo", []string{"ctrl+z", "ctrl+_"}}

	actNewLine = keyAction{"tui.input.newLine", []string{"shift+enter", "ctrl+enter"}}
	actSubmit  = keyAction{"tui.input.submit", []string{"enter"}}
	actTab     = keyAction{"tui.input.tab", []string{"tab"}}

	// Autocomplete dropdown navigation is a selection surface.
	actSelectUp       = keyAction{"tui.select.up", []string{"up"}}
	actSelectDown     = keyAction{"tui.select.down", []string{"down"}}
	actSelectPageUp   = keyAction{"tui.select.pageUp", []string{"pageup"}}
	actSelectPageDown = keyAction{"tui.select.pageDown", []string{"pagedown"}}
	actSelectConfirm  = keyAction{"tui.select.confirm", []string{"enter"}}
)

// Keys the editor owns outright: no keymap action declares them, so rebinding a
// tui.* action must never steal or silence them.
//
//   - escape / ctrl+c   host interrupt contract (OnInterrupt)
//   - ctrl+d            host exit contract (OnEOF, empty buffer only)
//   - ctrl+q            SubmitOnCtrlEnter submit alias
//   - ctrl+enter        SubmitOnCtrlEnter submit
//   - ctrl+shift+z      redo (no declared action)
//   - shift+backspace   terminal alias for backspace
//   - shift+delete      terminal alias for delete
//   - shift+tab         reserved by the host (app.thinking.cycle); never inserts
//   - alt+enter         host newline/dispatch hook
//   - ctrl+home / alt+< message start; ctrl+end / alt+> message end
//   - shift+space       literal space
//   - ctrl+p / ctrl+n   emacs prev/next inside the autocomplete dropdown
//   - enter             physical-key probe for the backslash-continuation rule
const (
	keyInterruptEscape = "escape"
	keyInterruptCtrlC  = "ctrl+c"
	keyEOF             = "ctrl+d"
	keyCtrlEnterSubmit = "ctrl+enter"
	keyCtrlQSubmit     = "ctrl+q"
	keyRedo            = "ctrl+shift+z"
	keyShiftBackspace  = "shift+backspace"
	keyShiftDelete     = "shift+delete"
	keyShiftTab        = "shift+tab"
	keyAltEnter        = "alt+enter"
	keyShiftSpace      = "shift+space"
	keyDropdownPrev    = "ctrl+p"
	keyDropdownNext    = "ctrl+n"
	keyPhysicalEnter   = "enter"
)

var (
	keysMessageStart = []string{"ctrl+home", "alt+<"}
	keysMessageEnd   = []string{"ctrl+end", "alt+>"}
)

// matches reports whether k triggers action a for this editor instance.
// Callers must hold e.mu.
func (e *Editor) matches(k event.Key, a keyAction) bool {
	if e.keys != nil {
		return e.keys.Matches(k, a.id)
	}
	return event.MatchesAnyKey(k, a.fallback...)
}

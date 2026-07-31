package component

// FocusState is an embeddable helper that implements [Focusable] and
// [TerminalCursorAware].
//
// Leaf components can embed FocusState and promote its methods, or call them
// from their own wrappers:
//
//	type Editor struct {
//	    component.FocusState
//	    // ...
//	}
type FocusState struct {
	focused           bool
	useTerminalCursor bool
}

// Focused reports whether the host currently focuses this component.
func (f *FocusState) Focused() bool {
	if f == nil {
		return false
	}
	return f.focused
}

// SetFocused updates the focused flag. The host calls this on focus changes.
// When focused, the component should emit a cursor via Frame.Cursor and/or
// [CursorMarker] in its rows.
func (f *FocusState) SetFocused(focused bool) {
	if f == nil {
		return
	}
	f.focused = focused
}

// UseTerminalCursor reports the host hardware-cursor preference last applied
// via SetUseTerminalCursor.
func (f *FocusState) UseTerminalCursor() bool {
	if f == nil {
		return false
	}
	return f.useTerminalCursor
}

// SetUseTerminalCursor records whether the component should rely on the
// terminal hardware cursor (true) or draw a software cursor (false).
func (f *FocusState) SetUseTerminalCursor(use bool) {
	if f == nil {
		return
	}
	f.useTerminalCursor = use
}

// ApplyFocus transitions focus from prev to next.
//
// Clears Focused on prev when it is Focusable, sets Focused on next, and
// syncs TerminalCursorAware on next with useTerminalCursor.
// prev and next may be nil. No-op when prev == next (still re-syncs cursor
// mode on next so preference changes apply).
func ApplyFocus(prev, next Component, useTerminalCursor bool) {
	if prev != nil && prev != next {
		if f, ok := prev.(Focusable); ok {
			f.SetFocused(false)
		}
	}
	if next == nil {
		return
	}
	if f, ok := next.(Focusable); ok {
		f.SetFocused(true)
	}
	if t, ok := next.(TerminalCursorAware); ok {
		t.SetUseTerminalCursor(useTerminalCursor)
	}
}

// SyncTerminalCursorMode pushes useTerminalCursor to c when it implements
// TerminalCursorAware.
func SyncTerminalCursorMode(c Component, useTerminalCursor bool) {
	if c == nil {
		return
	}
	if t, ok := c.(TerminalCursorAware); ok {
		t.SetUseTerminalCursor(useTerminalCursor)
	}
}

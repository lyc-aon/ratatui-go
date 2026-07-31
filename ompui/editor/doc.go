// Package editor implements the production OMP multiline editor as a
// [component.Component] and [component.InputHandler].
//
// # Index unit invariant
//
// External cursor and selection indexes are UTF-8 byte offsets into each
// logical line string (and into the joined buffer for flat absolute offsets).
// Navigation and deletion move by grapheme cluster; conversion between
// grapheme steps and byte offsets is always explicit. Never treat a public
// col as a rune count or terminal cell column unless a helper says so.
//
// Visual (cell) columns are used only for wrap layout and sticky vertical
// movement. They are never exposed as the primary cursor API.
//
// # Ownership
//
// The editor owns its buffer, undo/history stacks, kill ring, and autocomplete
// dropdown state. Autocomplete providers are injected; path/command candidate
// production is the provider's job. Render never blocks on provider I/O —
// suggestion results are applied asynchronously via request generation.
package editor

// Package component defines immutable line-frame contracts, optional component
// capabilities, a retained Container, and a protocol-backed remote leaf.
//
// # Render contract
//
// [Component.Render] returns a [Frame]. The Frame's Lines slice (and each row
// string) is owned by the component. Callers must not mutate it. An unchanged
// component should return the same Lines slice reference and the same
// Generation it returned last time. A content change must bump Generation and
// typically allocate a fresh Lines slice. Reference/generation equality is how
// [Container] memoizes concatenation and how the renderer derives stable
// prefixes.
//
// # Native scrollback seams
//
// Frame carries optional component-local seam indices (unset = [BoundaryUnset]):
//
//   - LiveRegionStart — first row of the live/mutating suffix
//   - CommitSafeEnd — rows in [LiveRegionStart, CommitSafeEnd) are append-only
//     (byte-stable) even while technically live
//   - SnapshotSafeEnd — rows in [CommitSafeEnd, SnapshotSafeEnd) are durable
//     snapshots (may drift later) that must not be dropped from history
//
// [Container] translates child seams by row offset and keeps the topmost seam
// (first child that reports a live region).
//
// # Optional capabilities
//
// Components opt into behavior by implementing the matching interface:
// [InputHandler], [KeyReleaseInterest], [Focusable], [TerminalCursorAware],
// [StablePrefix], [ViewportTailProvider], [Disposable], [Invalidator],
// [OverlayFocusOwner], [TightLayoutAware], [CommittedRowsAware].
//
// # Imports
//
// This package imports ompui/event and ompui/ansitext only. It performs no
// terminal I/O and contains no renderer.
package component

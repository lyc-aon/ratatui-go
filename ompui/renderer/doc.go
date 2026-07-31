// Package renderer is the OMP normal-screen, append-only scrollback engine.
//
// It ports the hot path of oh-my-pi packages/tui/src/tui.ts:
// frame prepare, ledger-driven window/commit math, full-paint and incremental
// emitters, overlay composition into the window slice only, hardware-cursor
// parking, and a coalescing 30fps render scheduler.
//
// # Ownership
//
//   - Caller owns terminal open/raw/restore lifecycle.
//   - This package writes deterministic ANSI to an injected io.Writer.
//   - ledger.State is the sole commit math. Emitters never rewrite rows
//     below CommittedRows; ED3 lives in exactly one full-paint callsite.
//
// # Invariants
//
//   - scrollback == frame[0:CommittedRows], each row once, in order.
//   - Transcript stays on the normal screen; overlays may freeze commits and
//     optionally borrow the alternate screen when marked fullscreen.
//   - Errors leave prior ledger/window state retryable (no silent advance):
//     ledger.Step mutations roll back on emit failure; Finish runs only after
//     a successful write.
//   - No viewport-position probes. No global terminal writes.
//   - Unchanged-frame hot path avoids allocations where practical (one write
//     buffer, prepared-line cache reuse, early cursor-only exit).
//
// # Typical use
//
//	eng := renderer.New(os.Stdout, renderer.CapsFromSnapshot(snap, env))
//	eng.Draw(renderer.Request{
//	    Frame:  componentFrame, // component.Frame{Lines, seams, Cursor}
//	    Width:  w, Height: h,
//	    Reason: renderer.ReasonUpdate,
//	})
//
// Or via the scheduler for coalesced ordinary repaints:
//
//	s := renderer.NewScheduler(eng, renderer.DefaultScheduler())
//	s.Request(renderer.Request{...})           // throttled ~30fps
//	s.RequestImmediate(renderer.Request{...})  // force/flush path
//	s.Flush(nil)                               // drain pending synchronously
package renderer

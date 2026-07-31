// Package ledger is pure append-only native-scrollback math for the OMP UI.
//
// It ports the committed-prefix contract from oh-my-pi's tui.ts:
// findCommittedPrefixResync, audit/update marks, boundary resolution, and
// window/commit planning. No terminal I/O lives here — callers feed []string
// frames and apply the returned plan.
//
// # Invariants
//
//   - scrollback == frame[0:CommittedRows], each row exactly once, in order.
//   - Committed rows are immutable. On component violation the audit re-anchors
//     at the first changed audited row: duplication, never content loss.
//   - Three zones under CommittedRows, split by AuditRows ≤ DurableRows:
//     [0, AuditRows)               byte-stable — audited
//     [AuditRows, DurableRows)     durable snapshot — audit-exempt (in-place drift OK)
//     [DurableRows, CommittedRows) forced-overflow — audited
//   - Boundaries no longer gate the commit floor; windowTop does. Boundaries
//     define only the audit-exempt span.
//   - Ordinary updates never rewrite history. Full-paint ED3 is a caller concern
//     (gestures only; never in multiplexers).
//   - Unchanged prefix storage is extended in place; wholesale replace only on
//     full paint, shrink-into-prefix, geometry re-base, or resync truncate.
//
// # Typical call sequence
//
//	st := ledger.NewState()
//	in := ledger.NewFrameInput(frame, height) // seams default to Unset (shell)
//	in.LiveStart, in.CommitSafe, in.SnapshotSafe = ... // absolute indices
//	res := st.Step(in)
//	// emit using res.Plan (ChunkTo, WindowTop, Kind, ClearScrollback)
//	st.Finish(frame, res)
//
// Seam fields use Unset (-1) when omitted. Zero is a valid live seam at row 0.
// Prefer NewFrameInput so the zero-value footgun (LiveStart==0) cannot silently
// pin a commit-unstable barrier.
//
// # Sources
//
// OMP packages/tui/src/tui.ts (findCommittedPrefixResync, #doRender commit math,
// #auditCommittedPrefix, #updateCommittedAuditRows), docs/tui-core-renderer.md,
// render-stable-prefix.test.ts, streaming-scrollback-defer.test.ts.
package ledger

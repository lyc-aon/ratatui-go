package ledger

// Tail-alignment sampling bounds: look back through up to Lookback rows of the
// committed prefix to collect Samples non-blank comparisons.
// Mirrors tui.ts RESYNC_TAIL_LOOKBACK / RESYNC_TAIL_SAMPLES.
const (
	ResyncTailLookback = 24
	ResyncTailSamples  = 8
)

// FindCommittedPrefixResync decides whether frame still aligns with the
// committed prefix, and where to re-anchor the commit index when it does not.
// Returns the resync row index, or -1 when no resync is needed.
//
// Audits the committed prefix [0, auditTo) EXCEPT the exempt window
// [exemptFrom, exemptTo): rows in the window are durable snapshots (a streaming
// table re-aligning its columns) that may drift legitimately, so their drift
// never triggers a re-anchor. Rows below the window — including forced-overflow
// rows committed only because they scrolled above the viewport under a
// commit-unstable barrier — ARE audited.
//
// Two detectors run over the audited rows:
//
//  1. Hard scan of the now-permanent forced suffix [exemptTo, permanentEnd):
//     forced-overflow rows that THIS frame asserts are durable/permanent
//     (index < permanentEnd). A content change there is real finalized content,
//     so ANY mismatch re-anchors. Scanned in FULL, not sampled, so a single edit
//     far above the commit boundary with an unchanged tail still re-anchors
//     (duplication, never loss) instead of being committed nowhere and painted
//     nowhere.
//
//  2. Tail sample (only when the hard scan is clean): exploits the asymmetry
//     between the two mutation classes — an in-place edit/restyle of a committed
//     row disturbs only the touched rows (alignment below intact; the stale copy
//     in history is the long-accepted artifact), while an insertion/deletion
//     shifts EVERY row below it. So up to 8 non-blank rows within the last 24
//     audited rows are compared SGR-stripped (theme changes stay quiet),
//     tolerating a SINGLE non-hard mismatch (a legitimate one-row edit): aligned ⇒
//     no resync; misaligned ⇒ resync at the first non-equivalent audited row.
//
// Highly repetitive tails (identical filler rows) can mask a shift in the tail
// sample, in which case the skipped rows are content-identical to the committed
// ones — observationally harmless.
//
// Defaults matching the TS signature when callers pass len(prefix) / 0:
//
//	auditTo = len(prefix)
//	exemptFrom = auditTo
//	exemptTo = exemptFrom
//	permanentEnd = 0
//
// Mirrors tui.ts findCommittedPrefixResync exactly.
func FindCommittedPrefixResync(frame, prefix []string, auditTo, exemptFrom, exemptTo, permanentEnd int) int {
	committed := min(len(prefix), max(0, auditTo))
	if committed == 0 {
		return -1
	}
	// Exempt window [exFrom, exTo) clamped into the committed prefix.
	exFrom := max(0, min(committed, exemptFrom))
	exTo := max(exFrom, min(committed, exemptTo))
	audited := func(i int) bool {
		return i < exFrom || i >= exTo
	}

	if len(frame) >= committed {
		// 1. Hard scan: forced-overflow rows now asserted permanent. Full scan,
		// no tolerance — a finalized row that changed must re-anchor.
		hardEnd := min(committed, max(0, permanentEnd))
		hardMismatch := false
		for i := exTo; i < hardEnd; i++ {
			if !RowsEquivalent(frame[i], prefix[i]) {
				hardMismatch = true
				break
			}
		}
		if !hardMismatch {
			// 2. Tail sample. Walk up from the commit boundary, skipping exempt
			// rows, until LOOKBACK audited rows or SAMPLES non-blank comparisons.
			samples := 0
			mismatches := 0
			scanned := 0
			for j := 1; j <= committed && scanned < ResyncTailLookback && samples < ResyncTailSamples; j++ {
				idx := committed - j
				if !audited(idx) {
					continue
				}
				scanned++
				row := frame[idx]
				old := prefix[idx]
				if row == old {
					if !IsBlankRow(row) {
						samples++
					}
					continue
				}
				if IsBlankRow(row) && IsBlankRow(old) {
					continue
				}
				samples++
				if !RowsEquivalent(row, old) {
					mismatches++
				}
			}
			// No signal (all-blank/all-exempt tail) or at most one edited row: aligned.
			if samples == 0 || mismatches <= 1 {
				return -1
			}
		}
	}

	// Misaligned (hard mismatch, tail-sample shift, or the frame no longer covers
	// the prefix): re-anchor at the first audited row whose content changed.
	limit := min(committed, len(frame))
	for i := range limit {
		if !audited(i) {
			continue
		}
		if !RowsEquivalent(frame[i], prefix[i]) {
			return i
		}
	}
	if limit < committed {
		return limit
	}
	return -1
}

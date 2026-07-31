package ledger

// Unset marks an absent optional seam index (live/commit/snapshot end).
// OMP uses undefined; Go callers pass Unset.
const Unset = -1

// Boundaries are the byte-stable and durable ends for one composed frame.
//
//	ByteStable (B): rows [0, B) are asserted never to re-layout; audited.
//	Durable    (D): rows [B, D) are permanent on scroll-off but may drift
//	                bytes later; committed audit-exempt.
//
// Rows at/beyond D that still commit (because they scrolled above the window
// under a commit-unstable barrier) are forced-overflow and stay audited.
//
// Invariant: 0 ≤ ByteStable ≤ Durable ≤ frameLen.
type Boundaries struct {
	ByteStable int
	Durable    int
}

// ResolveBoundaries computes B and D from optional NativeScrollbackLiveRegion
// seam ends for a frame of length frameLen.
//
//	B = clamp(commitSafe ?? liveStart ?? frameLen) into [0, frameLen]
//	D = max(B, clamp(snapshotSafe ?? B) into [0, frameLen])
//
// liveStart / commitSafe / snapshotSafe use Unset when the component omits them.
// Mirrors tui.ts #doRender boundary math.
func ResolveBoundaries(frameLen, liveStart, commitSafe, snapshotSafe int) Boundaries {
	if frameLen < 0 {
		frameLen = 0
	}

	var rawB int
	switch {
	case commitSafe != Unset:
		rawB = commitSafe
	case liveStart != Unset:
		rawB = liveStart
	default:
		rawB = frameLen
	}
	b := clampInt(rawB, 0, frameLen)

	rawD := b
	if snapshotSafe != Unset {
		rawD = snapshotSafe
	}
	d := clampInt(rawD, 0, frameLen)
	if d < b {
		d = b
	}
	return Boundaries{ByteStable: b, Durable: d}
}

// ClampLiveRegionStart clamps a child-local live-region start into [0, childLen].
func ClampLiveRegionStart(raw, childLen int) int {
	return clampInt(raw, 0, max(0, childLen))
}

// ClampCommitSafeEnd clamps a child-local commit-safe end to [liveStart, childLen].
func ClampCommitSafeEnd(raw, liveStart, childLen int) int {
	childLen = max(0, childLen)
	liveStart = clampInt(liveStart, 0, childLen)
	return clampInt(raw, liveStart, childLen)
}

// ClampSnapshotSafeEnd clamps a child-local snapshot-safe end to [floor, childLen],
// where floor is commitSafe ?? liveStart so durable never sits above byte-stable.
func ClampSnapshotSafeEnd(raw, floor, childLen int) int {
	childLen = max(0, childLen)
	floor = clampInt(floor, 0, childLen)
	return clampInt(raw, floor, childLen)
}

// AbsSeam shifts a child-local seam index into absolute frame coordinates.
func AbsSeam(offset, local int) int {
	if local < 0 {
		return local
	}
	return offset + local
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

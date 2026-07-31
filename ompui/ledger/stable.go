package ledger

// ClampReportedStableCount ports the compose-chain stableCount math for one
// root child segment.
//
//	hasReport: child implements RenderStablePrefix (consume-on-read).
//	sameRef:   previous segment lines are the same slice header as this render
//	           (reference equality proves byte-identical rows).
//
// When hasReport is set the report overrides reference equality; rows beyond
// the previous row count cannot be "unchanged". Without a report, sameRef
// yields the full child length, else 0.
//
// Mirrors tui.ts Container.render stableCount block.
func ClampReportedStableCount(reported, childLen, prevRowCount int, hasReport, sameRef bool) int {
	if childLen < 0 {
		childLen = 0
	}
	if prevRowCount < 0 {
		prevRowCount = 0
	}
	if hasReport {
		n := reported
		if n < 0 {
			n = 0
		}
		if n > childLen {
			n = childLen
		}
		if n > prevRowCount {
			n = prevRowCount
		}
		return n
	}
	if sameRef {
		return childLen
	}
	return 0
}

// ChainContinues reports whether the leading stable chain survives past a
// segment: fully stable rows AND identical row count (a grown/shrunk segment
// shifts every row below it).
func ChainContinues(stableCount, childLen, prevRowCount int) bool {
	return stableCount >= childLen && prevRowCount == childLen
}

// ClampStablePrefixRows defends the composed stable-row floor: it can never
// exceed what the previous compose actually materialized.
func ClampStablePrefixRows(stableRows, prevFrameLen int) int {
	if stableRows < 0 {
		return 0
	}
	if prevFrameLen < 0 {
		prevFrameLen = 0
	}
	if stableRows > prevFrameLen {
		return prevFrameLen
	}
	return stableRows
}

// PrefixFloor is a consume-on-read stable-prefix floor for in-place mutators
// (OMP RenderStablePrefix). Reading Consume re-bases the baseline to the
// current length so the next report measures only later mutations.
//
// Pure bookkeeping for components; the engine uses ClampReportedStableCount
// on the reported value.
type PrefixFloor struct {
	floor  int
	length int
}

// NoteAppend grows the logical row count by count (tail append).
func (p *PrefixFloor) NoteAppend(count int) {
	if count > 0 {
		p.length += count
	}
}

// NoteMutate records an in-place rewrite at index, lowering the floor to it.
func (p *PrefixFloor) NoteMutate(index int) {
	if index < p.floor {
		p.floor = index
	}
}

// NoteSetLength sets the current length and clamps the floor into it.
func (p *PrefixFloor) NoteSetLength(n int) {
	if n < 0 {
		n = 0
	}
	p.length = n
	if p.floor > n {
		p.floor = n
	}
	if p.floor < 0 {
		p.floor = 0
	}
}

// Length is the current logical row count.
func (p *PrefixFloor) Length() int { return p.length }

// Peek returns the floor without re-basing.
func (p *PrefixFloor) Peek() int {
	v := p.floor
	if v < 0 {
		return 0
	}
	if v > p.length {
		return p.length
	}
	return v
}

// Consume returns the stable floor and re-bases it to the current length.
// Mirrors getRenderStablePrefixRows: read re-bases the baseline to the state
// this compose is about to ingest.
func (p *PrefixFloor) Consume() int {
	v := p.Peek()
	p.floor = p.length
	return v
}

// RebaseForce sets the floor to the current length without returning it.
// Used when a compose path ingests the rows without reading a report
// (e.g. after a full replace).
func (p *PrefixFloor) RebaseForce() {
	p.floor = p.length
}

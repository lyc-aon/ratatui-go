package media

// DefaultMaxInlineImages is the default count of live graphics before demotion.
const DefaultMaxInlineImages = 8

// ImageBudget bounds how many inline images render as live terminal graphics.
//
// Terminal graphics protocols — Kitty especially — keep every transmitted image
// in a per-terminal store and re-draw placements as content scrolls; text-clear
// escapes do not remove them. The budget keeps the most recent cap images live
// and demotes older ones to text fallback. Demotion needs a full redraw plus an
// explicit graphics purge of the demoted ids.
//
// cap <= 0 disables budgeting: every image stays a live graphic.
//
// Not safe for concurrent use; owned by the single render path.
type ImageBudget struct {
	cap           int
	requestRender func()
	nextID        IDSource

	keyToID map[string]uint32

	// Display-order image ids observed during the in-flight pass.
	passIDs []uint32
	// Suppress threshold reflected in the frame currently on the terminal:
	// images at display indices [0, onTerminal) are shown as text there.
	onTerminal int
	// Suppress threshold the current/next render should apply.
	planned int
	// True while the in-flight pass applies a stricter threshold than the
	// terminal shows — the demotion frame that must purge and fully repaint.
	applyingReset bool
	lastTotal     int
	purgeIDs      []uint32
	// Image ids whose data is believed loaded in the terminal store.
	transmitted map[uint32]struct{}
	// Transmit sequences (full base64) to write once before this frame's placements.
	pendingTransmits []string
	// True while the in-flight pass is a partial/throwaway pass (resize viewport
	// fast path) that walks only the visible tail. Suppression replays the
	// committed split; the pass must NOT be closed with EndPass.
	stablePass bool
	// Image ids shown as text in the frame currently on the terminal.
	suppressedIDs map[uint32]struct{}
}

// NewImageBudget constructs a budget with the given cap and optional render
// request callback. nil requestRender is a no-op. nil idSource uses DefaultIDSource.
func NewImageBudget(cap int, requestRender func(), idSource IDSource) *ImageBudget {
	if requestRender == nil {
		requestRender = func() {}
	}
	if idSource == nil {
		idSource = DefaultIDSource()
	}
	return &ImageBudget{
		cap:           normalizeCap(cap),
		requestRender: requestRender,
		nextID:        idSource,
		keyToID:       make(map[string]uint32),
		transmitted:   make(map[uint32]struct{}),
		suppressedIDs: make(map[uint32]struct{}),
	}
}

func normalizeCap(cap int) int {
	if cap < 0 {
		return 0
	}
	return cap
}

// Cap returns the current live-graphics ceiling (0 = unlimited).
func (b *ImageBudget) Cap() int { return b.cap }

// Enabled reports whether budgeting is active (cap > 0).
func (b *ImageBudget) Enabled() bool { return b.cap > 0 }

// SetRequestRender replaces the redraw callback.
func (b *ImageBudget) SetRequestRender(fn func()) {
	if fn == nil {
		fn = func() {}
	}
	b.requestRender = fn
}

// SetCap updates the live-graphics ceiling and reconciles thresholds.
func (b *ImageBudget) SetCap(cap int) {
	next := normalizeCap(cap)
	if next == b.cap {
		return
	}
	b.cap = next
	b.reconcile(b.lastTotal)
}

// AcquireID returns a stable 24-bit graphics id. A non-empty key maps to the
// same id across re-creations; a missing key gets a fresh id every call.
func (b *ImageBudget) AcquireID(key string) uint32 {
	if key != "" {
		if id, ok := b.keyToID[key]; ok {
			return id
		}
		id := clampImageID(b.nextID())
		b.keyToID[key] = id
		return id
	}
	return clampImageID(b.nextID())
}

// BeginPass starts a render pass. stable=true is the partial/throwaway resize
// path: Observe replays the committed per-id decision and EndPass must not run.
func (b *ImageBudget) BeginPass(stable bool) {
	b.passIDs = b.passIDs[:0]
	b.stablePass = stable
	b.applyingReset = !stable && b.cap > 0 && b.planned > b.onTerminal
}

// Observe records an image in display order and reports whether it must render
// its text fallback this frame. Called by every Image during Render — including
// on a cache hit — so the image keeps its display-order slot.
func (b *ImageBudget) Observe(imageID uint32) bool {
	if b.stablePass {
		if b.cap <= 0 {
			return false
		}
		_, ok := b.suppressedIDs[imageID]
		return ok
	}
	index := len(b.passIDs)
	b.passIDs = append(b.passIDs, imageID)
	return b.cap > 0 && index < b.planned
}

// EndPass finishes a full render pass. Returns true when this frame must purge
// graphics and fully repaint; read ids via TakePurgeIDs.
func (b *ImageBudget) EndPass() bool {
	total := len(b.passIDs)
	b.lastTotal = total
	reset := false
	if b.applyingReset {
		for i := b.onTerminal; i < b.planned && i < total; i++ {
			id := b.passIDs[i]
			b.purgeIDs = append(b.purgeIDs, id)
			// d=I frees the data too, so the image must re-transmit if it returns.
			delete(b.transmitted, id)
		}
		b.onTerminal = b.planned
		b.applyingReset = false
		reset = true
	}
	b.reconcile(total)
	// Snapshot committed display-order suppression by id.
	b.suppressedIDs = make(map[uint32]struct{}, b.onTerminal)
	limit := b.onTerminal
	if limit > total {
		limit = total
	}
	for i := range limit {
		b.suppressedIDs[b.passIDs[i]] = struct{}{}
	}
	return reset
}

// TakePurgeIDs returns image ids to delete this frame and clears the pending set.
func (b *ImageBudget) TakePurgeIDs() []uint32 {
	if len(b.purgeIDs) == 0 {
		return nil
	}
	ids := b.purgeIDs
	b.purgeIDs = nil
	return ids
}

// TakeAllTransmittedIDs returns every id believed loaded and clears tracking
// (session cleanup). Also clears pending purges and transmits.
func (b *ImageBudget) TakeAllTransmittedIDs() []uint32 {
	if len(b.transmitted) == 0 {
		b.purgeIDs = nil
		b.pendingTransmits = nil
		return nil
	}
	ids := make([]uint32, 0, len(b.transmitted))
	for id := range b.transmitted {
		ids = append(ids, id)
	}
	b.transmitted = make(map[uint32]struct{})
	b.purgeIDs = nil
	b.pendingTransmits = nil
	return ids
}

// ShouldTransmit reports whether imageID still needs its data sent.
func (b *ImageBudget) ShouldTransmit(imageID uint32) bool {
	_, ok := b.transmitted[imageID]
	return !ok
}

// EnqueueTransmit queues a one-time transmit. No-op if already transmitted.
func (b *ImageBudget) EnqueueTransmit(imageID uint32, sequence string) {
	if _, ok := b.transmitted[imageID]; ok {
		return
	}
	b.transmitted[imageID] = struct{}{}
	b.pendingTransmits = append(b.pendingTransmits, sequence)
}

// HasPendingTransmits reports whether a frame has image data queued but not yet taken.
func (b *ImageBudget) HasPendingTransmits() bool {
	return len(b.pendingTransmits) > 0
}

// Quiescent is true when the budget has nothing in flight: no live images on
// the last pass, no queued transmits, no pending purges, and no stricter
// threshold left to apply.
func (b *ImageBudget) Quiescent() bool {
	return b.lastTotal == 0 &&
		len(b.pendingTransmits) == 0 &&
		len(b.purgeIDs) == 0 &&
		b.planned == b.onTerminal
}

// TakeTransmits returns transmit sequences to write before placements and clears the queue.
func (b *ImageBudget) TakeTransmits() []string {
	if len(b.pendingTransmits) == 0 {
		return nil
	}
	seq := b.pendingTransmits
	b.pendingTransmits = nil
	return seq
}

// TakeTransmitString joins pending transmits into one buffer for Request.ImageTransmit.
func (b *ImageBudget) TakeTransmitString() string {
	seq := b.TakeTransmits()
	if len(seq) == 0 {
		return ""
	}
	if len(seq) == 1 {
		return seq[0]
	}
	n := 0
	for _, s := range seq {
		n += len(s)
	}
	buf := make([]byte, 0, n)
	for _, s := range seq {
		buf = append(buf, s...)
	}
	return string(buf)
}

// TakePurgeString builds concatenated Kitty d=I deletes for pending purge ids.
func (b *ImageBudget) TakePurgeString() string {
	ids := b.TakePurgeIDs()
	return EncodeKittyDeleteImages(ids)
}

// ForgetTransmitted drops transmit tracking so every still-live image re-enqueues
// its data (a=t) on the next render. Recovers when the terminal dropped the
// original transmit. Keeps no base64 in budget state.
func (b *ImageBudget) ForgetTransmitted() {
	if len(b.transmitted) == 0 && len(b.pendingTransmits) == 0 {
		return
	}
	b.transmitted = make(map[uint32]struct{})
	b.pendingTransmits = nil
}

func (b *ImageBudget) reconcile(total int) {
	desired := 0
	if b.cap > 0 {
		desired = total - b.cap
		if desired < 0 {
			desired = 0
		}
	}
	if desired == b.planned {
		// Budget relaxed without a stricter frame: catch tracking up.
		if b.planned < b.onTerminal {
			b.onTerminal = b.planned
		}
		return
	}
	b.planned = desired
	// More images must be demoted: schedule purge + full redraw.
	// Fewer: no ghosts to clear — just catch tracking up.
	if desired <= b.onTerminal {
		b.onTerminal = desired
	}
	b.requestRender()
}

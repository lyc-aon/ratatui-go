package view

import (
	"sort"
	"strconv"
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/model"
)

// tailHoldback is the number of trailing rows of a live block never offered as
// commit-safe. Real streaming is not strictly append-only at the bottom: the
// in-flight paragraph re-wraps as words arrive, an unclosed markdown token
// re-renders when its closer streams in, and a wrap shrink moves the last word
// onto a new row. Holding the bottom rows back means those legitimate rewrites
// can never touch a row the engine already committed to native scrollback.
const tailHoldback = 4

// blockKind identifies what a transcript block renders.
type blockKind uint8

const (
	blockUser blockKind = iota
	blockAssistant
	blockTool
	blockSummary
	blockCustom
)

// blockSpec is the render recipe for one transcript block, resolved from the
// snapshot once per SetSnapshot rather than once per frame.
type blockSpec struct {
	key  string
	kind blockKind

	msg  model.Message
	card ToolCard

	// live marks a block that can still change: a streaming turn, a tool
	// without a terminal result.
	live bool
	// durable marks a block whose rendered rows are permanent content even if
	// they re-lay out. A pending tool card is not durable: its provisional
	// preview is replaced wholesale by the result render, so committing those
	// rows would strand a stale copy in immutable history.
	durable bool
}

// transcriptBlock is one block's retained render state.
type transcriptBlock struct {
	key   string
	hash  uint64
	width int
	lines []string
	built bool

	// sep is the separator row count this block contributed in the last
	// assembly (0 or 1).
	sep int
	// startRow is where this block's content began in the last assembly.
	startRow int

	live    bool
	durable bool

	// rewritten latches once the block re-laid out a row it had already
	// rendered outside the streaming edge, or rewrote a row already offered as
	// commit-safe. A rewritten block stops growing its commit offer for life.
	rewritten bool
	// offered is the high-water mark of rows reported commit-safe. It never
	// decreases: the engine may already hold those rows, and the seam contract
	// is "duplication, never loss".
	offered int
}

// Transcript renders a whole conversation as one line frame.
//
// Caching is per message, keyed by identity and content hash, so an unchanged
// historical turn is never re-laid-out and its exact line slice is reused.
// Blocks that the engine has already committed to native scrollback skip even
// the hash: their rows are immutable history and replaying them is free.
//
// Seams follow the OMP contract. LiveRegionStart marks the first still-mutating
// block; CommitSafeEnd is how far into the live run the rows are byte-stable;
// SnapshotSafeEnd is how far they are durable-but-possibly-reflowing. When
// nothing is live all three sit at the end of the frame, which reads as "every
// row is committable".
type Transcript struct {
	r Renderer

	snap    model.Snapshot
	specs   []blockSpec
	blocks  []*transcriptBlock
	missing []string

	lines []string
	frame component.Frame
	gen   component.Gen
	width int
	valid bool

	committedRows int
	stableFloor   int
}

// NewTranscript constructs a transcript view.
func NewTranscript(theme Theme, opts Options) *Transcript {
	return &Transcript{r: NewRenderer(theme, opts)}
}

// SetTheme rebinds the theme and drops every cached block.
func (t *Transcript) SetTheme(theme Theme) {
	t.r = NewRenderer(theme, t.r.opts)
	t.Invalidate()
}

// SetOptions rebinds render options and drops every cached block.
func (t *Transcript) SetOptions(opts Options) {
	t.r = NewRenderer(t.r.theme, opts)
	t.Invalidate()
}

// SetIgnoreTight implements component.TightLayoutAware.
func (t *Transcript) SetIgnoreTight(ignore bool) {
	opts := t.r.opts
	opts.Tight = !ignore
	t.SetOptions(opts)
}

// Theme returns the bound theme.
func (t *Transcript) Theme() Theme { return t.r.theme }

// Options returns the bound options.
func (t *Transcript) Options() Options { return t.r.opts }

// Invalidate implements component.Invalidator: every block re-renders on the
// next frame. Retained seam bookkeeping (rewrite latches, commit high-water
// marks) is preserved — the engine's committed history did not move just
// because the theme did.
func (t *Transcript) Invalidate() {
	for _, block := range t.blocks {
		block.built = false
		block.lines = nil
		block.width = 0
	}
	t.valid = false
	t.lines = nil
	t.stableFloor = 0
}

// SetNativeScrollbackCommittedRows implements component.CommittedRowsAware.
func (t *Transcript) SetNativeScrollbackCommittedRows(rows int) {
	if rows < 0 {
		rows = 0
	}
	t.committedRows = rows
}

// Snapshot returns the state currently rendered.
func (t *Transcript) Snapshot() model.Snapshot { return t.snap }

// UnsupportedKinds lists the custom message kinds the current snapshot carries
// that have no dedicated Go view. They still render — labelled, with their
// bodies intact — but the list tells a host which surfaces are worth building.
func (t *Transcript) UnsupportedKinds() []string { return t.missing }

// SetSnapshot installs new state and resolves the block list. Resolution walks
// the messages once per snapshot, not once per frame, so a resize replays the
// same recipe without re-deriving it.
//
// Tool expansion follows the snapshot: the core owns that toggle, and a view
// that kept its own copy would drift out of sync the first time the user
// pressed the expand key. A host that wants to override it calls SetOptions
// after SetSnapshot.
func (t *Transcript) SetSnapshot(snap model.Snapshot) {
	t.snap = snap
	if snap.Status.ToolsExpanded != t.r.opts.ToolsExpanded {
		opts := t.r.opts
		opts.ToolsExpanded = snap.Status.ToolsExpanded
		t.SetOptions(opts)
	}
	t.specs = t.buildSpecs(snap)
}

func (t *Transcript) buildSpecs(snap model.Snapshot) []blockSpec {
	messages := snap.Messages

	// Tool results are folded into the card of the call that produced them, and
	// executions are matched to their call block. Both indexes are built once.
	resultByCall := make(map[string]int, len(messages)/2+1)
	calledIDs := make(map[string]struct{}, len(messages)/2+1)
	for i := range messages {
		switch ClassifyMessage(messages[i]) {
		case KindToolResult:
			if id := messages[i].ToolCallID; id != "" {
				resultByCall[id] = i
			}
		case KindAssistant:
			for _, block := range messages[i].Content {
				if block.Kind == model.ContentToolCall && block.ToolCall != nil {
					calledIDs[block.ToolCall.ID] = struct{}{}
				}
			}
		}
	}
	execByID := make(map[string]int, len(snap.Tools))
	for i := range snap.Tools {
		execByID[snap.Tools[i].ID] = i
	}

	specs := make([]blockSpec, 0, len(messages)+len(snap.Tools))
	usedExec := make(map[string]struct{}, len(snap.Tools))
	var missing []string
	seenMissing := make(map[string]struct{})

	for i := range messages {
		msg := messages[i]
		index := strconv.Itoa(i)
		switch ClassifyMessage(msg) {
		case KindUser:
			specs = append(specs, blockSpec{key: index + ":user", kind: blockUser, msg: msg, durable: true})

		case KindAssistant:
			// A streaming turn is durable content unless its markdown is still
			// reflowing: a mermaid diagram reshaping or a table re-aligning its
			// columns re-lays-out rows that were already on screen, and those
			// rows must not reach immutable scrollback until the layout settles.
			specs = append(specs, blockSpec{
				key:     index + ":assistant",
				kind:    blockAssistant,
				msg:     msg,
				live:    msg.Streaming,
				durable: !msg.Streaming || !messageHasReflowingMarkdown(msg),
			})
			for _, block := range msg.Content {
				if block.Kind != model.ContentToolCall || block.ToolCall == nil {
					continue
				}
				call := block.ToolCall
				spec := t.toolSpec(call, snap, execByID, resultByCall, messages)
				usedExec[call.ID] = struct{}{}
				specs = append(specs, spec)
			}

		case KindToolResult:
			// Folded into its card. A result whose call never appeared in the
			// transcript would otherwise vanish, so it gets its own card.
			if _, ok := calledIDs[msg.ToolCallID]; ok {
				continue
			}
			result := msg
			card := ToolCardFrom(nil, execAt(snap, execByID, msg.ToolCallID), &result)
			usedExec[msg.ToolCallID] = struct{}{}
			specs = append(specs, blockSpec{
				key: index + ":toolresult", kind: blockTool, card: card, durable: true,
			})

		case KindSummary:
			specs = append(specs, blockSpec{key: index + ":summary", kind: blockSummary, msg: msg, durable: true})

		default:
			extras := readExtras(msg)
			kindName := firstNonEmpty(extras.CustomType, msg.Role)
			if kindName != "" {
				if _, seen := seenMissing[kindName]; !seen {
					seenMissing[kindName] = struct{}{}
					missing = append(missing, kindName)
				}
			}
			specs = append(specs, blockSpec{key: index + ":custom", kind: blockCustom, msg: msg, durable: true})
		}
	}

	// Executions the assistant has not yet published a call block for still
	// belong on screen: they are the work happening right now.
	for i := range snap.Tools {
		exec := snap.Tools[i]
		if _, used := usedExec[exec.ID]; used {
			continue
		}
		card := ToolCardFrom(nil, &snap.Tools[i], nil)
		specs = append(specs, blockSpec{
			key:     "exec:" + exec.ID,
			kind:    blockTool,
			card:    card,
			live:    !card.Settled(),
			durable: card.Settled(),
		})
	}

	sort.Strings(missing)
	t.missing = missing
	return specs
}

func (t *Transcript) toolSpec(
	call *model.ToolCall,
	snap model.Snapshot,
	execByID map[string]int,
	resultByCall map[string]int,
	messages []model.Message,
) blockSpec {
	var result *model.Message
	if idx, ok := resultByCall[call.ID]; ok {
		result = &messages[idx]
	}
	card := ToolCardFrom(call, execAt(snap, execByID, call.ID), result)
	return blockSpec{
		key:     "tool:" + call.ID,
		kind:    blockTool,
		card:    card,
		live:    !card.Settled(),
		durable: card.Settled(),
	}
}

func execAt(snap model.Snapshot, execByID map[string]int, id string) *model.ToolExecution {
	if id == "" {
		return nil
	}
	if idx, ok := execByID[id]; ok {
		return &snap.Tools[idx]
	}
	return nil
}

// Render implements component.Component.
func (t *Transcript) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	widthChanged := t.width != width
	if widthChanged {
		t.valid = false
	}

	t.syncBlocks()

	// First pass: bring every block's rows up to date.
	rowCursor := 0
	for i := range t.specs {
		block := t.blocks[i]
		spec := &t.specs[i]

		sep := 0
		if rowCursor > 0 {
			sep = 1
		}

		if t.canReplayCommitted(block, width, rowCursor, sep) {
			// Rows already in immutable scrollback: reuse verbatim, skipping
			// both the content hash and the layout.
			rowCursor += sep + len(block.lines)
			continue
		}

		hash := t.specHash(spec)
		if block.built && block.width == width && block.hash == hash {
			block.sep = sep
			block.startRow = rowCursor + sep
			rowCursor += sep + len(block.lines)
			continue
		}

		lines := t.renderSpec(spec, width)
		t.observeChange(block, lines, widthChanged)
		block.lines = lines
		block.hash = hash
		block.width = width
		block.built = true
		block.live = spec.live
		block.durable = spec.durable
		block.sep = sep
		block.startRow = rowCursor + sep
		rowCursor += sep + len(lines)
	}

	total := rowCursor
	if total == 0 {
		t.lines = nil
		t.width = width
		t.valid = true
		t.stableFloor = 0
		t.frame = component.EmptyFrame(t.gen.Current()).WithSeams(0, 0, 0)
		return t.frame
	}

	lines := make([]string, 0, total)
	for i := range t.blocks {
		block := t.blocks[i]
		if len(block.lines) == 0 {
			continue
		}
		if block.sep == 1 && len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block.lines...)
	}

	stableNow := commonPrefixRows(t.lines, lines)
	if !t.valid {
		stableNow = 0
	}
	if stableNow < t.stableFloor {
		t.stableFloor = stableNow
	}

	live, commit, snapshot := t.seams(len(lines))

	unchanged := t.valid && linesEqual(t.lines, lines) &&
		t.frame.LiveRegionStart == live &&
		t.frame.CommitSafeEnd == commit &&
		t.frame.SnapshotSafeEnd == snapshot
	if unchanged {
		return t.frame
	}
	t.lines = lines
	t.width = width
	t.valid = true
	t.frame = component.NewFrame(lines, t.gen.Next()).WithSeams(live, commit, snapshot)
	return t.frame
}

// canReplayCommitted reports whether a block sits wholly inside rows the engine
// already committed to native scrollback. Those rows are immutable on-screen
// history, so replaying the previous render is not just an optimization: it is
// what "committed rows are never rewritten" means in practice, and it removes
// hashing and layout for the bulk of a long session.
func (t *Transcript) canReplayCommitted(block *transcriptBlock, width, rowCursor, sep int) bool {
	if !t.valid || !block.built || block.width != width || t.committedRows <= 0 {
		return false
	}
	if block.startRow != rowCursor+sep || block.sep != sep {
		return false
	}
	return rowCursor+sep+len(block.lines) <= t.committedRows
}

// syncBlocks aligns the retained block slice with the current spec list.
// Transcripts grow at the tail, so a positional match is the common case; a key
// mismatch (history replaced by compaction) discards that block's state.
func (t *Transcript) syncBlocks() {
	if cap(t.blocks) < len(t.specs) {
		grown := make([]*transcriptBlock, len(t.specs))
		copy(grown, t.blocks)
		t.blocks = grown
	} else {
		t.blocks = t.blocks[:len(t.specs)]
	}
	for i := range t.specs {
		if t.blocks[i] == nil || t.blocks[i].key != t.specs[i].key {
			t.blocks[i] = &transcriptBlock{key: t.specs[i].key}
			t.valid = false
		}
	}
}

func (t *Transcript) renderSpec(spec *blockSpec, width int) []string {
	switch spec.kind {
	case blockUser:
		return t.r.UserMessage(spec.msg, width)
	case blockAssistant:
		return t.r.AssistantMessage(spec.msg, width, t.r.opts.frame(t.snap.Generation))
	case blockTool:
		return t.r.ToolRows(spec.card, width)
	case blockSummary:
		return t.r.SummaryMessage(spec.msg, width)
	default:
		return t.r.CustomMessage(spec.msg, width)
	}
}

// observeChange updates a live block's rewrite latch by diffing the new rows
// against the previous ones. A width change re-lays-out everything, so it is
// not evidence of volatility and resets the comparison instead.
func (t *Transcript) observeChange(block *transcriptBlock, lines []string, widthChanged bool) {
	if widthChanged || !block.built || len(block.lines) == 0 {
		return
	}
	prefix := visiblePrefixRows(block.lines, lines)
	appendOnly := prefix >= len(block.lines)
	tailConfined := prefix >= len(block.lines)-tailHoldback
	if !appendOnly && !tailConfined {
		block.rewritten = true
	}
	if prefix < block.offered {
		// The block re-laid-out a row already reported commit-safe. Latch it:
		// every further promote-then-mutate cycle would spray another stale
		// snapshot into immutable history.
		block.rewritten = true
	}
}

// commitOffer returns how many of a block's rows may be reported byte-stable.
func (b *transcriptBlock) commitOffer() int {
	if !b.live {
		return len(b.lines)
	}
	if !b.durable || b.rewritten {
		return b.offered
	}
	offer := len(b.lines) - tailHoldback
	if offer < b.offered {
		offer = b.offered
	}
	if offer < 0 {
		offer = 0
	}
	b.offered = offer
	return offer
}

// snapshotOffer returns how many rows are durable — permanent content even if
// they later re-lay out. The engine commits these audit-exempt.
func (b *transcriptBlock) snapshotOffer() int {
	if !b.live {
		return len(b.lines)
	}
	if !b.durable {
		return b.offered
	}
	return len(b.lines)
}

// seams computes the three native-scrollback boundaries for the assembled frame.
func (t *Transcript) seams(total int) (live, commit, snapshot int) {
	first := -1
	for i := range t.blocks {
		if t.blocks[i].live && len(t.blocks[i].lines) > 0 {
			first = i
			break
		}
	}
	if first < 0 {
		// Nothing is mutating: the whole frame is committable.
		return total, total, total
	}

	live = t.blocks[first].startRow
	commit, snapshot = live, live
	cursor := live
	commitOpen, snapshotOpen := true, true

	for i := first; i < len(t.blocks) && (commitOpen || snapshotOpen); i++ {
		block := t.blocks[i]
		if len(block.lines) == 0 {
			continue
		}
		if i > first {
			// The separator above this block is a blank row and always safe.
			cursor += block.sep
			if commitOpen {
				commit = cursor
			}
			if snapshotOpen {
				snapshot = cursor
			}
		}
		if commitOpen {
			offer := block.commitOffer()
			commit = cursor + offer
			if offer < len(block.lines) {
				commitOpen = false
			}
		}
		if snapshotOpen {
			offer := block.snapshotOffer()
			snapshot = cursor + offer
			if offer < len(block.lines) {
				snapshotOpen = false
			}
		}
		cursor += len(block.lines)
	}

	if commit > snapshot {
		snapshot = commit
	}
	return live, clampInt(commit, live, total), clampInt(snapshot, live, total)
}

// RenderStablePrefixRows implements component.StablePrefix. Reading consumes
// the report and re-bases the baseline to the rows just returned.
func (t *Transcript) RenderStablePrefixRows() int {
	value := clampInt(t.stableFloor, 0, len(t.lines))
	t.stableFloor = len(t.lines)
	return value
}

// RenderViewportTail implements component.ViewportTailProvider: during a resize
// drag only the visible bottom of the transcript is laid out, and none of the
// persistent cache or seam state is touched, so the settle render reconciles
// exactly as if this never ran.
func (t *Transcript) RenderViewportTail(width, maxRows int) []string {
	if maxRows <= 0 || len(t.specs) == 0 {
		return nil
	}
	if width < 1 {
		width = 1
	}
	collected := make([][]string, 0, 8)
	rows := 0
	for i := len(t.specs) - 1; i >= 0 && rows < maxRows; i-- {
		spec := t.specs[i]
		lines := t.renderSpec(&spec, width)
		if len(lines) == 0 {
			continue
		}
		if len(collected) > 0 {
			rows++ // separator above the block below
		}
		collected = append(collected, lines)
		rows += len(lines)
	}
	if len(collected) == 0 {
		return nil
	}

	out := make([]string, 0, rows)
	for i := len(collected) - 1; i >= 0; i-- {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, collected[i]...)
	}
	if len(out) > maxRows {
		out = out[len(out)-maxRows:]
	}
	return out
}

// commonPrefixRows counts leading rows that are byte-identical.
func commonPrefixRows(prev, next []string) int {
	limit := len(prev)
	if len(next) < limit {
		limit = len(next)
	}
	i := 0
	for i < limit && prev[i] == next[i] {
		i++
	}
	return i
}

// visiblePrefixRows counts leading rows whose painted cells match, ignoring
// escape placement and trailing pad drift. A styled row's closing escape moves
// when the row stops being the last of its span, and that is not a rewrite.
func visiblePrefixRows(prev, next []string) int {
	limit := len(prev)
	if len(next) < limit {
		limit = len(next)
	}
	i := 0
	for i < limit {
		if prev[i] != next[i] && normalizeRow(prev[i]) != normalizeRow(next[i]) {
			break
		}
		i++
	}
	return i
}

func normalizeRow(line string) string {
	return strings.TrimRight(stripANSI(line), " \t")
}

// FNV-1a over the fields that decide a block's rendering.
const (
	fnvOffset uint64 = 14695981039346656037
	fnvPrime  uint64 = 1099511628211
)

func hashBytes(h uint64, b []byte) uint64 {
	for _, c := range b {
		h ^= uint64(c)
		h *= fnvPrime
	}
	return h
}

func hashString(h uint64, s string) uint64 {
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

func hashInt(h uint64, v int64) uint64 {
	for i := range 8 {
		h ^= uint64(byte(v >> (8 * i)))
		h *= fnvPrime
	}
	return h
}

func hashBool(h uint64, v bool) uint64 {
	if v {
		return hashInt(h, 1)
	}
	return hashInt(h, 0)
}

// specHash fingerprints everything a block's render depends on. The preserved
// raw payload covers message content; derived flags are folded in explicitly
// because they are not part of the wire bytes.
func (t *Transcript) specHash(spec *blockSpec) uint64 {
	h := hashString(fnvOffset, spec.key)
	switch spec.kind {
	case blockTool:
		card := spec.card
		h = hashString(h, card.Name)
		h = hashString(h, card.Intent)
		h = hashBytes(h, card.Arguments)
		h = hashBytes(h, card.PartialResult)
		h = hashBytes(h, card.Result)
		h = hashBool(h, card.Running)
		h = hashBool(h, card.HasResult)
		h = hashBool(h, card.IsError)
		h = hashInt(h, card.StartedAt.UnixMilli())
		h = hashInt(h, card.EndedAt.UnixMilli())
		if card.Running && !t.r.opts.Now.IsZero() {
			// An injected clock makes running cards tick; fold it in at second
			// resolution so the row actually refreshes.
			h = hashInt(h, t.r.opts.Now.Unix())
		}
	case blockAssistant:
		h = hashBytes(h, spec.msg.Raw)
		h = hashBool(h, spec.msg.Streaming)
		h = hashString(h, spec.msg.StopReason)
		h = hashString(h, spec.msg.Error)
		if spec.msg.Streaming && t.r.opts.HideThinking {
			// The reasoning pulse advances with the snapshot generation.
			h = hashInt(h, int64(t.r.opts.frame(t.snap.Generation)))
		}
	default:
		h = hashBytes(h, spec.msg.Raw)
		h = hashString(h, spec.msg.Role)
		h = hashBool(h, spec.msg.Synthetic)
		h = hashBool(h, spec.msg.IsError)
	}
	return h
}

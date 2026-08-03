package view

import (
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/model"
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
	blockTrail
	blockSummary
	blockCustom
)

// blockGroup is the visual band a block belongs to. Blocks in the same band
// render flush; a single blank row opens where the band changes. So a run of
// tool trails — or of model paragraphs — reads as one section and the eye only
// catches a gap where the kind of content actually changed.
//
// Port of Hermes domain/blockLayout.ts.
type blockGroup uint8

const (
	// groupModel is assistant prose, the model's voice.
	groupModel blockGroup = iota
	// groupTrail is the agent's working area: reasoning and tool calls.
	groupTrail
	// groupNote is system notes and errors, a quieter band.
	groupNote
	// groupUser is the human turn; it owns its own margins.
	groupUser
	// groupSlash is a slash-command echo; it owns its top margin.
	groupSlash
	// groupDiff is an inline patch segment; an island owning both margins.
	groupDiff
	// groupEvent is a timeline marker; it owns its trailing margin.
	groupEvent
)

// selfSpaced groups own the blank row above them through their own chrome, so the
// grouping rule must not add a second one.
func (g blockGroup) selfSpaced() bool {
	switch g {
	case groupUser, groupSlash, groupDiff, groupEvent:
		return true
	default:
		return false
	}
}

// paintsTrailingGap groups already draw a blank row beneath themselves, so the
// block below must not add its own leading gap or one boundary becomes two.
func (g blockGroup) paintsTrailingGap() bool {
	switch g {
	case groupUser, groupDiff, groupEvent:
		return true
	default:
		return false
	}
}

// topMargin is the blank row a group always draws above itself.
func (g blockGroup) topMargin() bool {
	switch g {
	case groupUser, groupSlash, groupDiff:
		return true
	default:
		return false
	}
}

// hasLeadGap reports whether cur opens one blank row against the block rendered
// directly above it. True only where the band changes, and only for the
// working-area bands — user, slash, diff, and event keep their own spacing.
//
// Streaming-safe by construction: the result depends on the predecessor's group,
// never on cur's own live content, so an actively streaming block computes the
// same gap while it streams as the settled segment does once it flushes.
func hasLeadGap(prev blockGroup, havePrev bool, cur blockGroup) bool {
	if cur.selfSpaced() || !havePrev {
		return false
	}
	return prev != cur && !prev.paintsTrailingGap()
}

// separatorRows is how many blank rows sit between two rendered blocks: the
// predecessor's trailing margin plus the successor's own leading gap. The two are
// independent margins, not one collapsing gap — which is exactly why the groups
// that paint a trailing row suppress the successor's lead gap.
func separatorRows(prev blockGroup, havePrev bool, cur blockGroup) int {
	if !havePrev {
		return 0
	}
	rows := 0
	if prev.paintsTrailingGap() {
		rows++
	}
	if cur.topMargin() || hasLeadGap(prev, havePrev, cur) {
		rows++
	}
	return rows
}

// messageGroup maps a message onto its visual band.
func messageGroup(msg model.Message) blockGroup {
	return groupFor(readExtras(msg), msg)
}

// groupFor resolves a band from already-parsed extras, so a spec walk pays for
// one payload parse per message rather than one per lookup.
func groupFor(extras messageExtras, msg model.Message) blockGroup {
	switch extras.Kind {
	case blockKindSlash:
		return groupSlash
	case blockKindEvent:
		return groupEvent
	case blockKindDiff:
		return groupDiff
	case blockKindTrail:
		return groupTrail
	}
	switch ClassifyMessage(msg) {
	case KindUser:
		return groupUser
	case KindAssistant:
		return groupModel
	default:
		return groupNote
	}
}

// blockSpec is the render recipe for one transcript block, resolved from the
// snapshot once per SetSnapshot rather than once per frame.
type blockSpec struct {
	key   string
	kind  blockKind
	group blockGroup

	msg   model.Message
	card  ToolCard
	trail Trail

	// responseSep marks an assistant body whose turn also rendered a trail, so
	// the answer earns Hermes' `Response` rule above it.
	responseSep bool

	// live marks a block that can still change: a streaming turn, a tool
	// without a terminal result.
	live bool
	// durable marks a block whose rendered rows are permanent content even if
	// they re-lay out. A pending tool card is not durable: its provisional
	// preview is replaced wholesale by the result render, so committing those
	// rows would strand a stale copy in immutable history.
	durable bool
	// holdback is how many trailing rows must never be offered commit-safe. A
	// streaming body's in-flight tail re-renders on every delta, so its rows
	// cannot reach immutable scrollback until the tail settles.
	holdback int
}

// transcriptBlock is one block's retained render state.
type transcriptBlock struct {
	key   string
	hash  uint64
	width int
	lines []string
	built bool

	// sep is the separator row count this block contributed in the last
	// assembly.
	sep int
	// startRow is where this block's content began in the last assembly.
	startRow int
	// group is the visual band this block rendered in, retained so the next
	// assembly can compute its own separators without re-resolving specs.
	group blockGroup

	live     bool
	durable  bool
	holdback int

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

	committedRows atomic.Int64
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

// SetOptions rebinds render options, drops every cached block, and re-resolves
// the block list. Detail modes decide the block structure itself — an explicit
// mode groups a turn's calls into one trail where the legacy layout gives each
// call its own card — so a mode change must rebuild specs, not just rows.
func (t *Transcript) SetOptions(opts Options) {
	t.r = NewRenderer(t.r.theme, opts)
	t.Invalidate()
	t.specs = t.buildSpecs(t.snap)
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
	t.committedRows.Store(int64(rows))
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
// Tool expansion follows the snapshot only while ToolsMode uses its legacy
// fallback. An explicit detail mode belongs to the host and must not be
// overwritten by a status update.
func (t *Transcript) SetSnapshot(snap model.Snapshot) {
	t.snap = snap
	if !t.r.opts.ToolsMode.Valid() && snap.Status.ToolsExpanded != t.r.opts.ToolsExpanded {
		opts := t.r.opts
		opts.ToolsExpanded = snap.Status.ToolsExpanded
		t.r = NewRenderer(t.r.theme, opts)
		t.Invalidate()
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

	// Hermes' grouped trail replaces the per-call card once a host opts into an
	// explicit detail mode. Focus view keeps the legacy shape so its failure
	// backstop stays beneath the answer it belongs to.
	grouped := t.r.opts.detailModesExplicit() && !t.r.opts.FocusView

	specs := make([]blockSpec, 0, len(messages)+len(snap.Tools))
	usedExec := make(map[string]struct{}, len(snap.Tools))
	var missing []string
	seenMissing := make(map[string]struct{})

	for i := range messages {
		msg := messages[i]
		extras := readExtras(msg)
		index := strconv.Itoa(i)

		// A Hermes block kind outranks the message role: an event marker, a diff
		// segment, and a plan block have their own voice regardless of who the
		// core attributed them to. A slash echo keeps the operator gutter, so it
		// stays on the user path and only changes band.
		switch {
		case extras.Kind == blockKindEvent, extras.Kind == blockKindDiff,
			extras.Kind == blockKindTrail, len(extras.Todos) > 0:
			specs = append(specs, blockSpec{
				key: index + ":block", kind: blockCustom, group: groupFor(extras, msg), msg: msg, durable: true,
			})
			continue
		}

		switch ClassifyMessage(msg) {
		case KindUser:
			specs = append(specs, blockSpec{
				key: index + ":user", kind: blockUser, group: groupFor(extras, msg), msg: msg, durable: true,
			})

		case KindAssistant:
			cards := t.messageCards(msg, snap, execByID, resultByCall, messages, usedExec)
			responseSep := false
			if grouped {
				trail := t.r.MessageTrail(msg)
				trail.Cards = cards
				specs = appendTrailSpec(specs, index, trail)
				// The rule belongs to the turn that did the work. A trail merged
				// in from an earlier turn already sits above its own answer, so
				// only this message's own details earn it.
				responseSep = t.r.trailShowsDetails(trail)
				if !t.r.assistantBodyRenders(msg) {
					// Nothing to answer with yet. Skipping the block keeps it
					// transparent to grouping, so the trail above it never
					// strands a floating blank row against the next turn.
					continue
				}
			}
			// A streaming turn is durable content unless its markdown is still
			// reflowing: a mermaid diagram reshaping or a table re-aligning its
			// columns re-lays-out rows that were already on screen, and those
			// rows must not reach immutable scrollback until the layout settles.
			specs = append(specs, blockSpec{
				key:         index + ":assistant",
				kind:        blockAssistant,
				group:       groupModel,
				msg:         msg,
				responseSep: responseSep,
				live:        msg.Streaming,
				durable:     !msg.Streaming || !messageHasReflowingMarkdown(msg),
				holdback:    streamHoldback(msg),
			})
			if !grouped {
				for _, card := range cards {
					specs = append(specs, toolCardSpec("tool:"+card.ID, card))
				}
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
			if grouped {
				specs = appendTrailSpec(specs, index, Trail{Cards: []ToolCard{card}})
				continue
			}
			specs = append(specs, blockSpec{
				key: index + ":toolresult", kind: blockTool, group: groupTrail, card: card, durable: true,
			})

		case KindSummary:
			specs = append(specs, blockSpec{
				key: index + ":summary", kind: blockSummary, group: groupNote, msg: msg, durable: true,
			})

		default:
			kindName := firstNonEmpty(extras.CustomType, msg.Role)
			if kindName != "" && extras.Kind == "" {
				if _, seen := seenMissing[kindName]; !seen {
					seenMissing[kindName] = struct{}{}
					missing = append(missing, kindName)
				}
			}
			specs = append(specs, blockSpec{
				key: index + ":custom", kind: blockCustom, group: groupFor(extras, msg), msg: msg, durable: true,
			})
		}
	}

	// Executions the assistant has not yet published a call block for still
	// belong on screen: they are the work happening right now.
	var orphans []ToolCard
	for i := range snap.Tools {
		exec := snap.Tools[i]
		if _, used := usedExec[exec.ID]; used {
			continue
		}
		card := ToolCardFrom(nil, &snap.Tools[i], nil)
		if grouped {
			orphans = append(orphans, card)
			continue
		}
		specs = append(specs, toolCardSpec("exec:"+exec.ID, card))
	}
	if len(orphans) > 0 {
		specs = appendTrailSpec(specs, "exec:"+orphans[0].ID, Trail{Cards: orphans})
	}

	sort.Strings(missing)
	t.missing = missing
	return specs
}

// appendTrailSpec adds a trail block, merging into the trail immediately above it
// when there is one. Adjacent calls belong to one working area, so a turn that
// only emits more calls extends the run rather than opening a second panel — that
// is what makes `Tool calls (N)` count the whole run.
//
// Merging is keyed on the snapshot's own message and tool-call identity, so the
// grouping is a pure function of the snapshot and identical across frames.
func appendTrailSpec(specs []blockSpec, key string, trail Trail) []blockSpec {
	if trail.Empty() {
		return specs
	}
	if n := len(specs); n > 0 && specs[n-1].kind == blockTrail {
		prev := &specs[n-1]
		if reasoning := strings.TrimSpace(trail.Reasoning); reasoning != "" {
			if strings.TrimSpace(prev.trail.Reasoning) == "" {
				prev.trail.Reasoning = reasoning
			} else {
				prev.trail.Reasoning += "\n\n" + reasoning
			}
		}
		prev.trail.Cards = append(prev.trail.Cards, trail.Cards...)
		prev.trail.ReasoningRedacted = prev.trail.ReasoningRedacted || trail.ReasoningRedacted
		prev.trail.ReasoningActive = trail.ReasoningActive
		prev.trail.ReasoningTokens += trail.ReasoningTokens
		prev.trail.ToolTokens += trail.ToolTokens
		prev.key += "+" + key
		prev.live = !prev.trail.Settled()
		prev.durable = prev.trail.Settled()
		return specs
	}
	return append(specs, blockSpec{
		key:     key + ":trail",
		kind:    blockTrail,
		group:   groupTrail,
		trail:   trail,
		live:    !trail.Settled(),
		durable: trail.Settled(),
	})
}

// toolCardSpec is the legacy one-card-per-call block.
func toolCardSpec(key string, card ToolCard) blockSpec {
	return blockSpec{
		key:     key,
		kind:    blockTool,
		group:   groupTrail,
		card:    card,
		live:    !card.Settled(),
		durable: card.Settled(),
	}
}

// messageCards assembles every tool card a message requested, folding in the live
// execution and the eventual result.
func (t *Transcript) messageCards(
	msg model.Message,
	snap model.Snapshot,
	execByID map[string]int,
	resultByCall map[string]int,
	messages []model.Message,
	usedExec map[string]struct{},
) []ToolCard {
	var cards []ToolCard
	for _, block := range msg.Content {
		if block.Kind != model.ContentToolCall || block.ToolCall == nil {
			continue
		}
		call := block.ToolCall
		var result *model.Message
		if idx, ok := resultByCall[call.ID]; ok {
			result = &messages[idx]
		}
		cards = append(cards, ToolCardFrom(call, execAt(snap, execByID, call.ID), result))
		usedExec[call.ID] = struct{}{}
	}
	return cards
}

// streamHoldback is how many trailing rows of a streaming turn must stay out of
// the commit-safe prefix: the physical lines of the in-flight tail, which is the
// only region a later delta can re-render. See [FindStableBoundary].
func streamHoldback(msg model.Message) int {
	if !msg.Streaming {
		return 0
	}
	rows := 0
	for _, block := range msg.Content {
		if block.Kind == model.ContentText {
			rows += liveTailRows(block.Text)
		}
	}
	return rows
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
	committedRows := int(t.committedRows.Load())
	if width < 1 {
		width = 1
	}
	widthChanged := t.width != width
	if widthChanged {
		t.valid = false
	}

	t.syncBlocks()

	// First pass: bring every block's rows up to date. Separators come from the
	// visual bands of the last block that actually rendered and this one, so a
	// block that draws nothing is transparent to grouping.
	rowCursor := 0
	var prevGroup blockGroup
	havePrev := false
	for i := range t.specs {
		block := t.blocks[i]
		spec := &t.specs[i]

		sep := 0
		if len(block.lines) > 0 {
			sep = separatorRows(prevGroup, havePrev, spec.group)
		}

		if t.canReplayCommitted(block, width, rowCursor, sep, committedRows) {
			// Rows already in immutable scrollback: reuse verbatim, skipping
			// both the content hash and the layout.
			rowCursor += sep + len(block.lines)
			prevGroup, havePrev = spec.group, true
			continue
		}

		hash := t.specHash(spec)
		if block.built && block.width == width && block.hash == hash {
			block.sep = sep
			block.startRow = rowCursor + sep
			rowCursor += sep + len(block.lines)
			if len(block.lines) > 0 {
				prevGroup, havePrev = spec.group, true
			}
			continue
		}

		lines := t.renderSpec(spec, width)
		// An omitted detail section occupies no row and therefore cannot
		// consume a phantom seam. This matters when hidden thinking/tools sit
		// between durable transcript blocks: their later block coordinates must
		// agree with the assembled frame for viewport virtualization.
		sep = 0
		if len(lines) > 0 {
			sep = separatorRows(prevGroup, havePrev, spec.group)
			prevGroup, havePrev = spec.group, true
		}
		t.observeChange(block, lines, widthChanged)
		block.lines = lines
		block.hash = hash
		block.width = width
		block.built = true
		block.live = spec.live
		block.durable = spec.durable
		block.holdback = spec.holdback
		block.group = spec.group
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
		if len(lines) > 0 {
			for range block.sep {
				lines = append(lines, "")
			}
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
func (t *Transcript) canReplayCommitted(block *transcriptBlock, width, rowCursor, sep, committedRows int) bool {
	if !t.valid || !block.built || len(block.lines) == 0 || block.width != width || committedRows <= 0 {
		return false
	}
	if block.startRow != rowCursor+sep || block.sep != sep {
		return false
	}
	return rowCursor+sep+len(block.lines) <= committedRows
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
	frame := t.r.opts.frame(t.snap.Generation)
	switch spec.kind {
	case blockUser:
		return t.r.UserMessage(spec.msg, width)
	case blockAssistant:
		if t.r.opts.detailModesExplicit() && !t.r.opts.FocusView {
			return t.r.AssistantBody(spec.msg, width, spec.responseSep)
		}
		return t.r.AssistantMessage(spec.msg, width, frame)
	case blockTrail:
		return t.r.TrailRows(spec.trail, width, frame)
	case blockTool:
		return t.r.toolRows(spec.card, width, frame)
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
//
// A live block always withholds its bottom rows: real streaming is not strictly
// append-only at the edge. The floor is [tailHoldback]; a streaming body raises it
// to cover its whole in-flight markdown tail, which is the only region a later
// delta can re-render (see [FindStableBoundary]).
func (b *transcriptBlock) commitOffer() int {
	if !b.live {
		return len(b.lines)
	}
	if !b.durable || b.rewritten {
		return b.offered
	}
	holdback := tailHoldback
	if b.holdback > holdback {
		holdback = b.holdback
	}
	offer := len(b.lines) - holdback
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
	// Walking upward, each collected block's separator is the one it draws
	// against the block above it, so the count is resolved once the block above
	// is known. This must agree with Render's assembly or the drag frame would
	// disagree with the settle frame by a row.
	type tailBlock struct {
		lines []string
		group blockGroup
		sep   int
	}
	collected := make([]tailBlock, 0, 8)
	rows := 0
	for i := len(t.specs) - 1; i >= 0 && rows < maxRows; i-- {
		spec := t.specs[i]
		lines := t.renderSpec(&spec, width)
		if len(lines) == 0 {
			continue
		}
		if n := len(collected); n > 0 {
			collected[n-1].sep = separatorRows(spec.group, true, collected[n-1].group)
			rows += collected[n-1].sep
		}
		collected = append(collected, tailBlock{lines: lines, group: spec.group})
		rows += len(lines)
	}
	if len(collected) == 0 {
		return nil
	}

	out := make([]string, 0, rows)
	for i := len(collected) - 1; i >= 0; i-- {
		if len(out) > 0 {
			for range collected[i].sep {
				out = append(out, "")
			}
		}
		out = append(out, collected[i].lines...)
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
		h = t.hashCard(h, spec.card)
		if t.r.trailPulses(Trail{Cards: []ToolCard{spec.card}}) {
			// A hidden active card renders the generic pulse instead of its
			// content, so each host-controlled frame must invalidate it.
			h = hashInt(h, int64(t.r.opts.frame(t.snap.Generation)))
		}
		return h
	case blockTrail:
		h = hashString(h, spec.trail.Reasoning)
		h = hashBool(h, spec.trail.ReasoningRedacted)
		h = hashBool(h, spec.trail.ReasoningActive)
		h = hashInt(h, int64(spec.trail.ReasoningTokens))
		h = hashInt(h, int64(spec.trail.ToolTokens))
		for i := range spec.trail.Cards {
			h = t.hashCard(h, spec.trail.Cards[i])
		}
		if t.r.trailPulses(spec.trail) {
			// A trail with no visible panel renders the generic pulse instead of
			// its content, so each host-controlled frame must invalidate it.
			h = hashInt(h, int64(t.r.opts.frame(t.snap.Generation)))
		}
		return h
	case blockAssistant:
		h = hashBytes(h, spec.msg.Raw)
		h = hashBool(h, spec.msg.Streaming)
		h = hashString(h, spec.msg.StopReason)
		h = hashString(h, spec.msg.Error)
		h = hashBool(h, spec.responseSep)
		if t.r.shouldPulseThinking(spec.msg) {
			// The reasoning pulse advances with the snapshot generation.
			h = hashInt(h, int64(t.r.opts.frame(t.snap.Generation)))
		}
		return h
	default:
		h = hashBytes(h, spec.msg.Raw)
		h = hashString(h, spec.msg.Role)
		h = hashBool(h, spec.msg.Synthetic)
		h = hashBool(h, spec.msg.IsError)
		return h
	}
}

// hashCard folds one tool card's rendered inputs into h.
func (t *Transcript) hashCard(h uint64, card ToolCard) uint64 {
	h = hashString(h, card.ID)
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
	return h
}

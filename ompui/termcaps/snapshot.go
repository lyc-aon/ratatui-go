package termcaps

import "sync"

// Snapshot is a mutable runtime capability set resolved from env + static table.
// Probe-driven fields (image protocol, hyperlinks, DECCARA, screen-to-scrollback,
// text sizing, sync output) may be updated after construction. Owned by the
// single TTY backend/render path — not safe for concurrent mutation.
type Snapshot struct {
	ID                         TerminalID
	ImageProtocol              ImageProtocol
	TrueColor                  bool
	Hyperlinks                 bool
	NotifyProtocol             NotifyProtocol
	DECCARA                    bool
	SupportsScreenToScrollback bool
	TextSizing                 bool
	// SynchronizedOutput is the static DEC 2026 default; probes may flip it.
	SynchronizedOutput bool
	// Multiplexer is true when the session is inside tmux/screen/zellij/cmux.
	Multiplexer bool
}

// ResolveOptions controls Snapshot construction.
type ResolveOptions struct {
	// Env is the environment map used for all policy decisions.
	Env Env
	// IsTTY gates image-protocol fallback (stdout is a TTY).
	IsTTY bool
	// ForceDisableDECCARA forces DECCARA off after detection (test runtimes).
	ForceDisableDECCARA bool
	// TerminalID overrides DetectTerminalID when non-empty.
	TerminalID TerminalID
}

// Resolve builds a Snapshot from options. Pure: no process I/O.
func Resolve(opts ResolveOptions) *Snapshot {
	env := opts.Env
	id := opts.TerminalID
	if id == "" {
		id = DetectTerminalID(env)
	}
	base := LookupTerminalInfo(id)

	s := &Snapshot{
		ID:                         base.ID,
		ImageProtocol:              base.ImageProtocol,
		TrueColor:                  base.TrueColor,
		Hyperlinks:                 base.Hyperlinks,
		NotifyProtocol:             base.NotifyProtocol,
		DECCARA:                    base.DECCARA,
		SupportsScreenToScrollback: base.SupportsScreenToScrollback,
		TextSizing:                 base.TextSizing,
		Multiplexer:                IsMultiplexerSession(env),
	}

	s.ImageProtocol = ResolveImageProtocol(id, env, opts.IsTTY)
	s.Hyperlinks = ShouldEnableHyperlinksByDefault(env, id)
	s.DECCARA = DetectRectangularSGRSupport(id, env)
	if opts.ForceDisableDECCARA {
		s.DECCARA = false
	}
	s.SynchronizedOutput = ShouldEnableSynchronizedOutputByDefault(env, id)
	// TextSizing stays at the static table value until a runtime setting flips it.
	return s
}

// Info returns a TerminalInfo copy of the current snapshot fields.
func (s *Snapshot) Info() TerminalInfo {
	return TerminalInfo{
		ID:                         s.ID,
		ImageProtocol:              s.ImageProtocol,
		TrueColor:                  s.TrueColor,
		Hyperlinks:                 s.Hyperlinks,
		NotifyProtocol:             s.NotifyProtocol,
		DECCARA:                    s.DECCARA,
		SupportsScreenToScrollback: s.SupportsScreenToScrollback,
		TextSizing:                 s.TextSizing,
	}
}

// SetImageProtocol overrides the image protocol after capability probes.
func (s *Snapshot) SetImageProtocol(p ImageProtocol) {
	s.ImageProtocol = p
}

// SetDECCARA overrides rectangular-SGR capability at runtime.
func (s *Snapshot) SetDECCARA(enabled bool) {
	s.DECCARA = enabled
}

// SetScreenToScrollback overrides screen-to-scrollback clear support.
func (s *Snapshot) SetScreenToScrollback(enabled bool) {
	s.SupportsScreenToScrollback = enabled
}

// SetTextSizing enables/disables OSC 66 text-sizing at runtime.
func (s *Snapshot) SetTextSizing(enabled bool) {
	s.TextSizing = enabled
}

// SetHyperlinks overrides OSC 8 hyperlink emission at runtime.
func (s *Snapshot) SetHyperlinks(enabled bool) {
	s.Hyperlinks = enabled
}

// SetSynchronizedOutput overrides DEC 2026 synchronized-output emission.
func (s *Snapshot) SetSynchronizedOutput(enabled bool) {
	s.SynchronizedOutput = enabled
}

// IsImageLine reports whether line carries the snapshot's current image protocol.
func (s *Snapshot) IsImageLine(line string) bool {
	return IsImageLine(line, s.ImageProtocol)
}

// CellSize holds mutable cell dimensions (updated by probes).
type CellSize struct {
	mu   sync.RWMutex
	dims CellDimensions
}

// NewCellSize starts at DefaultCellDimensions.
func NewCellSize() *CellSize {
	return &CellSize{dims: DefaultCellDimensions}
}

// Get returns the current cell dimensions.
func (c *CellSize) Get() CellDimensions {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dims
}

// Set replaces the cell dimensions.
func (c *CellSize) Set(dims CellDimensions) {
	c.mu.Lock()
	c.dims = dims
	c.mu.Unlock()
}

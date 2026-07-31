package termcaps

// TerminalID is a recognized terminal family used for static capability tables.
type TerminalID string

const (
	TerminalKitty     TerminalID = "kitty"
	TerminalGhostty   TerminalID = "ghostty"
	TerminalWezTerm   TerminalID = "wezterm"
	TerminalITerm2    TerminalID = "iterm2"
	TerminalVSCode    TerminalID = "vscode"
	TerminalAlacritty TerminalID = "alacritty"
	TerminalBase      TerminalID = "base"
	TerminalTrueColor TerminalID = "trueColor"
)

// ImageProtocol is the lead sequence for an inline image protocol.
// The zero value means no image protocol.
type ImageProtocol string

const (
	ImageProtocolNone   ImageProtocol = ""
	ImageProtocolKitty  ImageProtocol = "\x1b_G"
	ImageProtocolITerm2 ImageProtocol = "\x1b]1337;File="
	ImageProtocolSixel  ImageProtocol = "\x1bPq"
)

// NotifyProtocol is the lead sequence for desktop / bell notifications.
type NotifyProtocol string

const (
	NotifyProtocolBell  NotifyProtocol = "\x07"
	NotifyProtocolOSC99 NotifyProtocol = "\x1b]99;;"
	NotifyProtocolOSC9  NotifyProtocol = "\x1b]9;"
)

// KittyPlaceholder is the Kitty Unicode placeholder base (U+10EEEE, Plane 16 PUA).
// Used only to classify image lines that carry placeholder cells.
const KittyPlaceholder = "\U0010eeee"

// Env is an explicit environment map for deterministic capability decisions.
// Missing keys are treated as unset (empty string).
type Env map[string]string

// Get returns the value for key, or "" when unset/nil.
func (e Env) Get(key string) string {
	if e == nil {
		return ""
	}
	return e[key]
}

// Has reports whether key is present and non-empty.
func (e Env) Has(key string) bool {
	return e.Get(key) != ""
}

// TerminalInfo is the static capability profile for a terminal family.
// Mutable runtime overrides live on Snapshot, not here.
type TerminalInfo struct {
	ID                         TerminalID
	ImageProtocol              ImageProtocol
	TrueColor                  bool
	Hyperlinks                 bool
	NotifyProtocol             NotifyProtocol
	DECCARA                    bool
	SupportsScreenToScrollback bool
	// TextSizing is Kitty OSC 66 scaled-span support (static table; runtime gate is separate).
	TextSizing bool
}

// Clone returns a copy suitable for building a mutable Snapshot.
func (t TerminalInfo) Clone() TerminalInfo {
	return t
}

// CellDimensions is the pixel size of one terminal cell.
type CellDimensions struct {
	WidthPx  int
	HeightPx int
}

// ImageDimensions is the pixel size of a source image.
type ImageDimensions struct {
	WidthPx  int
	HeightPx int
}

// ImageFitOptions bounds how an image is fitted into the cell grid.
// Zero Max* means unlimited on that axis.
type ImageFitOptions struct {
	MaxWidthCells  int
	MaxHeightCells int
}

// ImageFit is the cell footprint of a fitted image.
type ImageFit struct {
	Columns int
	Rows    int
}

// Version is a major.minor pair parsed from TERM_PROGRAM_VERSION-style strings.
type Version struct {
	Major int
	Minor int
}

// DefaultCellDimensions matches the OMP TUI default (9×18) before a cell-size probe.
var DefaultCellDimensions = CellDimensions{WidthPx: 9, HeightPx: 18}

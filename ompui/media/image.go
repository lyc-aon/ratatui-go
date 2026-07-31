package media

import (
	"strconv"

	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/termcaps"
)

// ReservedImageRow is a non-plain zero-width row used to reserve height for
// direct placements. Keeps transcript blank-edge trimming from collapsing
// image-only blocks. Matches OMP RESERVED_IMAGE_ROW = "\x1b[0m".
const ReservedImageRow = "\x1b[0m"

const (
	saveCursor    = "\x1b7"
	restoreCursor = "\x1b8"
)

// Theme colors the text fallback. FallbackColor receives the plain fallback
// string and returns the styled form (may be identity).
type Theme struct {
	FallbackColor func(string) string
}

// Options configures an Image component.
type Options struct {
	MaxWidthCells  int
	MaxHeightCells int
	Filename       string
	// Shared budget that caps how many inline images render as live graphics.
	Budget *ImageBudget
	// Stable identity for the underlying image (e.g. toolCallId:index). Lets
	// the budget hand back the same graphics id across component re-creations.
	ImageKey string
	// Protocol overrides the process-default image protocol when non-nil.
	// nil → termcaps.DefaultTerminal().ImageProtocol each render.
	Protocol *termcaps.ImageProtocol
	// Cell overrides process-default cell dimensions when non-zero fields set.
	Cell termcaps.CellDimensions
	// UnicodePlaceholders overrides process detection when non-nil.
	UnicodePlaceholders *bool
}

// Image is a component.Component that renders one inline terminal image with
// text fallback, height preservation, protocol/cell/width cache invalidation,
// reserved-row direct placement, placeholder mode, and transmit/purge extraction
// via its ImageBudget.
type Image struct {
	base64Data string
	mimeType   string
	dims       Dimensions
	theme      Theme
	opts       Options
	budget     *ImageBudget
	imageID    uint32
	hasID      bool

	cachedLines                   []string
	cachedWidth                   int
	cachedSuppressed              bool
	cachedProtocol                termcaps.ImageProtocol
	cachedCellW                   int
	cachedCellH                   int
	cachedKittyUnicodePlaceholders bool
	hasCache                      bool

	// Tallest graphic placement this image has rendered. Text fallback pads
	// to this height so a budget demotion never shrinks the block (rows may
	// already be committed to native scrollback).
	renderedGraphicRows int

	gen component.Gen
}

// NewImage constructs an Image. dimensions may be nil to sniff from base64+mime;
// on sniff failure a default 800×600 is used (matches OMP).
func NewImage(base64Data, mimeType string, theme Theme, opts Options, dimensions *Dimensions) *Image {
	if theme.FallbackColor == nil {
		theme.FallbackColor = func(s string) string { return s }
	}
	img := &Image{
		base64Data: base64Data,
		mimeType:   mimeType,
		theme:      theme,
		opts:       opts,
		budget:     opts.Budget,
	}
	if dimensions != nil {
		img.dims = *dimensions
	} else if d, ok := GetImageDimensions(base64Data, mimeType); ok {
		img.dims = d
	} else {
		img.dims = Dimensions{WidthPx: 800, HeightPx: 600}
	}
	if opts.Budget != nil {
		img.imageID = opts.Budget.AcquireID(opts.ImageKey)
		img.hasID = true
	}
	// Sanitize filename once.
	if opts.Filename != "" {
		img.opts.Filename = SanitizeName(opts.Filename)
	}
	return img
}

// ImageID returns the stable Kitty id when a budget is attached, else 0.
func (im *Image) ImageID() uint32 {
	if !im.hasID {
		return 0
	}
	return im.imageID
}

// Dimensions returns the source pixel size.
func (im *Image) Dimensions() Dimensions { return im.dims }

// MimeType returns the configured MIME type.
func (im *Image) MimeType() string { return im.mimeType }

// Invalidate drops cached rendering state so the next Render rebuilds.
func (im *Image) Invalidate() {
	im.hasCache = false
	im.cachedLines = nil
	im.cachedWidth = 0
}

// Dispose is a no-op (no retained resources beyond cache).
func (im *Image) Dispose() {}

// Render implements component.Component.
func (im *Image) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	protocol := im.protocol()
	hasProtocol := protocol != termcaps.ImageProtocolNone && protocol != ""
	cell := im.cellDims()
	kittyPH := im.unicodePlaceholders()

	// observe() must run on every pass — even a cache hit — so the image keeps
	// its display-order slot in the budget. Only graphics-capable frames count.
	suppressed := false
	if hasProtocol && im.budget != nil {
		suppressed = im.budget.Observe(im.imageID)
	}

	if im.hasCache &&
		im.cachedWidth == width &&
		im.cachedSuppressed == suppressed &&
		im.cachedProtocol == protocol &&
		im.cachedCellW == cell.WidthPx &&
		im.cachedCellH == cell.HeightPx &&
		im.cachedKittyUnicodePlaceholders == kittyPH {
		return component.NewFrame(im.cachedLines, im.gen.Current())
	}

	capW := im.opts.MaxWidthCells
	maxWidth := width - 2
	if maxWidth < 1 {
		maxWidth = 1
	}
	if capW > 0 && capW < maxWidth {
		maxWidth = capW
	}

	var lines []string
	if hasProtocol && !suppressed {
		needsTransmit := im.hasID && im.budget != nil && im.budget.ShouldTransmit(im.imageID)
		var imageID uint32
		if im.hasID {
			imageID = im.imageID
		}
		result, ok := RenderImage(im.base64Data, im.dims, protocol, RenderOptions{
			MaxWidthCells:       maxWidth,
			MaxHeightCells:      im.opts.MaxHeightCells,
			ImageID:             imageID,
			IncludeTransmit:     needsTransmit,
			UnicodePlaceholders: kittyPH,
			Cell:                cell,
			Filename:            im.opts.Filename,
		})
		if ok && result.Transmit != "" && im.hasID && im.budget != nil {
			im.budget.EnqueueTransmit(im.imageID, result.Transmit)
		}
		if ok && len(result.Lines) > 0 {
			// Unicode placeholders: real text-cell lines (line 0 has virtual APC).
			lines = result.Lines
		} else if ok {
			// Direct placement: reserve rows-1, last line does save/CUU/seq/restore.
			lines = directPlacementLines(result.Sequence, result.Rows)
		} else {
			lines = im.fallbackLines()
		}
		if len(lines) > im.renderedGraphicRows {
			im.renderedGraphicRows = len(lines)
		}
	} else {
		lines = im.fallbackLines()
	}

	im.cachedLines = lines
	im.cachedWidth = width
	im.cachedSuppressed = suppressed
	im.cachedProtocol = protocol
	im.cachedCellW = cell.WidthPx
	im.cachedCellH = cell.HeightPx
	im.cachedKittyUnicodePlaceholders = kittyPH
	im.hasCache = true
	gen := im.gen.Next()
	return component.NewFrame(lines, gen)
}

func (im *Image) protocol() termcaps.ImageProtocol {
	if im.opts.Protocol != nil {
		return *im.opts.Protocol
	}
	return termcaps.DefaultTerminal().ImageProtocol
}

func (im *Image) cellDims() termcaps.CellDimensions {
	c := im.opts.Cell
	if c.WidthPx > 0 && c.HeightPx > 0 {
		return c
	}
	return termcaps.GetCellDimensions()
}

func (im *Image) unicodePlaceholders() bool {
	if im.opts.UnicodePlaceholders != nil {
		return *im.opts.UnicodePlaceholders
	}
	// Default off unless the host opts in via Options — matches OMP's
	// getKittyGraphics() which starts false until seeded. Callers that want
	// auto-detect should set UnicodePlaceholders from DetectKittyUnicodePlaceholdersSupport.
	return false
}

// fallbackLines returns the text fallback, height-preserving once a graphic
// has rendered so demotion never shrinks committed scrollback rows.
func (im *Image) fallbackLines() []string {
	d := im.dims
	fb := im.theme.FallbackColor(ImageFallback(im.mimeType, &d, im.opts.Filename))
	if im.renderedGraphicRows <= 1 {
		return []string{fb}
	}
	lines := make([]string, im.renderedGraphicRows)
	for i := range im.renderedGraphicRows - 1 {
		lines[i] = ReservedImageRow
	}
	lines[im.renderedGraphicRows-1] = fb
	return lines
}

// directPlacementLines builds reserved-row + save/CUU/seq/restore footprint.
func directPlacementLines(sequence string, rows int) []string {
	if rows < 1 {
		rows = 1
	}
	lines := make([]string, rows)
	for i := range rows - 1 {
		lines[i] = ReservedImageRow
	}
	cursorRows := rows - 1
	placement := sequence
	if cursorRows > 0 {
		moveUp := "\x1b[" + strconv.Itoa(cursorRows) + "A"
		placement = saveCursor + moveUp + sequence + restoreCursor
	}
	lines[rows-1] = placement
	return lines
}

// Compile-time interface checks.
var (
	_ component.Component   = (*Image)(nil)
	_ component.Invalidator = (*Image)(nil)
	_ component.Disposable  = (*Image)(nil)
)

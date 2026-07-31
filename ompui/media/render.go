package media

import (
	"github.com/michaelkelly/ratatui-go/ompui/termcaps"
)

// RenderOptions configures RenderImage.
type RenderOptions struct {
	MaxWidthCells  int
	MaxHeightCells int
	// PreserveAspectRatio defaults true for iTerm2 when unset (nil).
	PreserveAspectRatio *bool
	// ImageID is the stable Kitty image id (i=). 0 means no stable id —
	// Kitty uses self-contained a=T.
	ImageID uint32
	// PlacementID is the stable Kitty placement id (p=); 0 defaults to ImageID
	// when ImageID != 0.
	PlacementID uint32
	// IncludeTransmit when true (Kitty + ImageID) also returns the one-time a=t.
	IncludeTransmit bool
	// UnicodePlaceholders enables U=1 placeholder grids when the fit fits the
	// diacritic table. Ignored for non-Kitty protocols.
	UnicodePlaceholders bool
	// Cell overrides process-default cell dimensions when non-zero.
	Cell termcaps.CellDimensions
	// Filename is optional iTerm2 name metadata (sanitized).
	Filename string
}

// RenderResult is the protocol output of RenderImage.
//
// Exactly one of Sequence (direct placement / sixel / iterm) or Lines
// (placeholder grid) is set on success. Rows is always the cell height.
// Transmit is the optional one-time Kitty a=t payload (never embedded in Lines).
type RenderResult struct {
	Sequence string
	Lines    []string
	Rows     int
	Columns  int
	Transmit string
}

// RenderImage builds the protocol sequence/lines for one image under the given
// protocol. Returns ok=false when protocol is none or encoding fails (Sixel).
// Never panics; never writes to a terminal.
func RenderImage(
	base64Data string,
	dims Dimensions,
	protocol termcaps.ImageProtocol,
	opt RenderOptions,
) (RenderResult, bool) {
	if protocol == termcaps.ImageProtocolNone || protocol == "" {
		return RenderResult{}, false
	}
	cell := opt.Cell
	if cell.WidthPx <= 0 || cell.HeightPx <= 0 {
		cell = termcaps.GetCellDimensions()
	}
	if cell.WidthPx <= 0 {
		cell.WidthPx = termcaps.DefaultCellDimensions.WidthPx
	}
	if cell.HeightPx <= 0 {
		cell.HeightPx = termcaps.DefaultCellDimensions.HeightPx
	}

	fit := termcaps.CalculateImageFit(dims, termcaps.ImageFitOptions{
		MaxWidthCells:  opt.MaxWidthCells,
		MaxHeightCells: opt.MaxHeightCells,
	}, cell)

	switch protocol {
	case termcaps.ImageProtocolKitty:
		return renderKitty(base64Data, fit, opt)
	case termcaps.ImageProtocolSixel:
		return renderSixel(base64Data, fit, cell)
	case termcaps.ImageProtocolITerm2:
		return renderITerm2(base64Data, fit, opt)
	default:
		return RenderResult{}, false
	}
}

func renderKitty(base64Data string, fit termcaps.ImageFit, opt RenderOptions) (RenderResult, bool) {
	if opt.ImageID != 0 {
		placementID := opt.PlacementID
		if placementID == 0 {
			placementID = opt.ImageID
		}
		var transmit string
		if opt.IncludeTransmit {
			transmit = EncodeKittyTransmit(base64Data, opt.ImageID)
		}
		if opt.UnicodePlaceholders && KittyPlaceholdersFit(fit.Columns, fit.Rows) {
			lines := RenderKittyPlaceholderLines(KittyVirtualPlacementOptions{
				ImageID:     opt.ImageID,
				PlacementID: placementID,
				Columns:     fit.Columns,
				Rows:        fit.Rows,
			})
			return RenderResult{Lines: lines, Rows: fit.Rows, Columns: fit.Columns, Transmit: transmit}, true
		}
		seq := EncodeKittyPlacement(KittyPlacementOptions{
			ImageID:     opt.ImageID,
			PlacementID: placementID,
			Columns:     fit.Columns,
			Rows:        fit.Rows,
		})
		return RenderResult{Sequence: seq, Rows: fit.Rows, Columns: fit.Columns, Transmit: transmit}, true
	}
	// No stable id: self-contained transmit-and-display.
	seq := EncodeKittyDirect(base64Data, KittyDirectOptions{
		Columns: fit.Columns,
		Rows:    fit.Rows,
	})
	return RenderResult{Sequence: seq, Rows: fit.Rows, Columns: fit.Columns}, true
}

func renderSixel(base64Data string, fit termcaps.ImageFit, cell termcaps.CellDimensions) (RenderResult, bool) {
	tw := fit.Columns * cell.WidthPx
	th := fit.Rows * cell.HeightPx
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	seq, err := EncodeSixelBase64(base64Data, tw, th)
	if err != nil {
		return RenderResult{}, false
	}
	return RenderResult{Sequence: seq, Rows: fit.Rows, Columns: fit.Columns}, true
}

func renderITerm2(base64Data string, fit termcaps.ImageFit, opt RenderOptions) (RenderResult, bool) {
	preserve := true
	if opt.PreserveAspectRatio != nil {
		preserve = *opt.PreserveAspectRatio
	}
	par := preserve
	seq := EncodeITerm2(base64Data, ITerm2Options{
		Width:               fit.Columns,
		Height:              "auto",
		Name:                opt.Filename,
		PreserveAspectRatio: &par,
	})
	return RenderResult{Sequence: seq, Rows: fit.Rows, Columns: fit.Columns}, true
}

// ImageFallback builds the text fallback line matching OMP imageFallback.
func ImageFallback(mimeType string, dims *Dimensions, filename string) string {
	parts := make([]string, 0, 3)
	if filename != "" {
		parts = append(parts, SanitizeName(filename))
	}
	parts = append(parts, "["+mimeType+"]")
	if dims != nil {
		parts = append(parts,
			itoa(dims.WidthPx)+"x"+itoa(dims.HeightPx))
	}
	out := "[Image: "
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out + "]"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

package termcaps

import (
	"math"
	"strings"
)

// ForcedImageProtocol resolves PI_FORCE_IMAGE_PROTOCOL.
// ok=false means the env var is unset/blank (no override).
// ok=true with protocol=ImageProtocolNone means the user forced images off.
func ForcedImageProtocol(env Env) (protocol ImageProtocol, ok bool) {
	raw := strings.ToLower(strings.TrimSpace(env.Get("PI_FORCE_IMAGE_PROTOCOL")))
	if raw == "" {
		return ImageProtocolNone, false
	}
	switch raw {
	case "kitty":
		return ImageProtocolKitty, true
	case "iterm2", "iterm":
		return ImageProtocolITerm2, true
	case "sixel":
		return ImageProtocolSixel, true
	case "off", "none", "0", "false":
		return ImageProtocolNone, true
	default:
		// Unknown values force images off (matches OMP).
		return ImageProtocolNone, true
	}
}

// FallbackImageProtocol picks a Kitty fallback when the static table has no
// image protocol. Requires isTTY. vscode/alacritty never fall back. screen/tmux/
// ghostty TERM substrings select Kitty; otherwise none.
func FallbackImageProtocol(terminalID TerminalID, env Env, isTTY bool) ImageProtocol {
	if !isTTY {
		return ImageProtocolNone
	}
	if terminalID == TerminalVSCode || terminalID == TerminalAlacritty {
		return ImageProtocolNone
	}
	term := strings.ToLower(env.Get("TERM"))
	if strings.Contains(term, "screen") || strings.Contains(term, "tmux") || strings.Contains(term, "ghostty") {
		return ImageProtocolKitty
	}
	return ImageProtocolNone
}

// ResolveImageProtocol applies force override, then static table, then fallback.
func ResolveImageProtocol(terminalID TerminalID, env Env, isTTY bool) ImageProtocol {
	if forced, ok := ForcedImageProtocol(env); ok {
		return forced
	}
	info := LookupTerminalInfo(terminalID)
	if info.ImageProtocol != ImageProtocolNone {
		return info.ImageProtocol
	}
	return FallbackImageProtocol(terminalID, env, isTTY)
}

// IsWindowsTerminalPreviewSixelSupported is true on Windows Terminal
// with TERM_PROGRAM_VERSION >= 1.22 (Sixel support introduced in preview 1.22).
// platform is a GOOS string ("windows", "linux", "darwin", …).
func IsWindowsTerminalPreviewSixelSupported(env Env, platform string) bool {
	if platform != "windows" {
		return false
	}
	if !env.Has("WT_SESSION") {
		return false
	}
	if tp := env.Get("TERM_PROGRAM"); tp != "" && !strings.EqualFold(tp, "windows_terminal") {
		return false
	}
	v, ok := ParseMajorMinorVersion(env.Get("TERM_PROGRAM_VERSION"))
	if !ok {
		return false
	}
	return v.AtLeast(1, 22)
}

// CalculateImageRows returns how many cell rows an image needs when scaled to
// targetWidthCells wide.
func CalculateImageRows(image ImageDimensions, targetWidthCells int, cell CellDimensions) int {
	if cell.WidthPx <= 0 {
		cell.WidthPx = DefaultCellDimensions.WidthPx
	}
	if cell.HeightPx <= 0 {
		cell.HeightPx = DefaultCellDimensions.HeightPx
	}
	if image.WidthPx <= 0 || targetWidthCells <= 0 {
		return 1
	}
	targetWidthPx := float64(targetWidthCells * cell.WidthPx)
	scale := targetWidthPx / float64(image.WidthPx)
	scaledHeightPx := float64(image.HeightPx) * scale
	rows := int(math.Ceil(scaledHeightPx / float64(cell.HeightPx)))
	if rows < 1 {
		return 1
	}
	return rows
}

// CalculateImageFit returns the cell columns/rows for an image under options.
// Zero MaxWidthCells / MaxHeightCells means unlimited on that axis.
func CalculateImageFit(image ImageDimensions, options ImageFitOptions, cell CellDimensions) ImageFit {
	if cell.WidthPx <= 0 {
		cell.WidthPx = DefaultCellDimensions.WidthPx
	}
	if cell.HeightPx <= 0 {
		cell.HeightPx = DefaultCellDimensions.HeightPx
	}

	var maxColumns, maxRows int
	hasMaxCols := options.MaxWidthCells > 0
	hasMaxRows := options.MaxHeightCells > 0
	if hasMaxCols {
		maxColumns = options.MaxWidthCells
		if maxColumns < 1 {
			maxColumns = 1
		}
	}
	if hasMaxRows {
		maxRows = options.MaxHeightCells
		if maxRows < 1 {
			maxRows = 1
		}
	}

	if !hasMaxCols && !hasMaxRows {
		cols := int(math.Ceil(float64(image.WidthPx) / float64(cell.WidthPx)))
		rows := int(math.Ceil(float64(image.HeightPx) / float64(cell.HeightPx)))
		if cols < 1 {
			cols = 1
		}
		if rows < 1 {
			rows = 1
		}
		return ImageFit{Columns: cols, Rows: rows}
	}

	maxWidthPx := math.Inf(1)
	maxHeightPx := math.Inf(1)
	if hasMaxCols {
		maxWidthPx = float64(maxColumns * cell.WidthPx)
	}
	if hasMaxRows {
		maxHeightPx = float64(maxRows * cell.HeightPx)
	}

	if image.WidthPx <= 0 || image.HeightPx <= 0 {
		return ImageFit{Columns: 1, Rows: 1}
	}

	scale := math.Min(maxWidthPx/float64(image.WidthPx), maxHeightPx/float64(image.HeightPx))
	fittedWidthPx := float64(image.WidthPx) * scale
	fittedHeightPx := float64(image.HeightPx) * scale

	columns := int(math.Floor(fittedWidthPx / float64(cell.WidthPx)))
	rows := int(math.Ceil(fittedHeightPx / float64(cell.HeightPx)))
	if columns < 1 {
		columns = 1
	}
	if rows < 1 {
		rows = 1
	}
	if hasMaxCols && columns > maxColumns {
		columns = maxColumns
	}
	if hasMaxRows && rows > maxRows {
		rows = maxRows
	}
	return ImageFit{Columns: columns, Rows: rows}
}

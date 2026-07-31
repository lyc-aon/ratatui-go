package widgets

import (
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/symbols"
)

// layer is one saved canvas layer (row-major terminal cells).
type layer struct {
	contents []layerCell
}

// layerCell holds optional symbol/fg/bg contributions for one terminal cell.
// Absent fields (has* false) let lower layers show through during compositing.
type layerCell struct {
	symbol    rune
	hasSymbol bool
	fg        style.Color
	hasFG     bool
	bg        style.Color
	hasBG     bool
}

// grid is the internal paint surface behind a Context marker.
type grid interface {
	resolution() (float64, float64)
	paint(x, y int, color style.Color)
	save() layer
	reset()
}

// patternCell is one WxH pattern terminal cell.
type patternCell struct {
	// pattern bits are row-major within the cell:
	//
	//	| 0 1 |
	//	| 2 3 |
	//	| 4 5 |
	//	| 6 7 |   (for 2x4)
	pattern  uint8
	color    style.Color
	hasColor bool
}

// patternGrid paints WxH sub-pixel patterns (braille / quadrant / sextant / octant).
type patternGrid struct {
	width     int
	height    int
	cellW     int
	cellH     int
	cells     []patternCell
	charTable []rune
}

func newPatternGrid(width, height, cellW, cellH int, charTable []rune) *patternGrid {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	n := width * height
	return &patternGrid{
		width:     width,
		height:    height,
		cellW:     cellW,
		cellH:     cellH,
		cells:     make([]patternCell, n),
		charTable: charTable,
	}
}

func (g *patternGrid) resolution() (float64, float64) {
	return float64(g.width) * float64(g.cellW), float64(g.height) * float64(g.cellH)
}

func (g *patternGrid) paint(x, y int, color style.Color) {
	if g.cellW <= 0 || g.cellH <= 0 || g.width <= 0 {
		return
	}
	index := (y/g.cellH)*g.width + (x / g.cellW)
	if index < 0 || index >= len(g.cells) {
		return
	}
	bit := uint8((x % g.cellW) + g.cellW*(y%g.cellH))
	cell := &g.cells[index]
	cell.pattern |= 1 << bit
	cell.color = color
	cell.hasColor = true
}

func (g *patternGrid) save() layer {
	contents := make([]layerCell, len(g.cells))
	for i, cell := range g.cells {
		lc := layerCell{}
		// pattern 0 stays blank so lower layers show through
		if cell.pattern != 0 && int(cell.pattern) < len(g.charTable) {
			lc.symbol = g.charTable[cell.pattern]
			lc.hasSymbol = true
		}
		if cell.hasColor {
			lc.fg = cell.color
			lc.hasFG = true
		}
		contents[i] = lc
	}
	return layer{contents: contents}
}

func (g *patternGrid) reset() {
	clear(g.cells)
}

// charCell is one optional painted color in a CharGrid.
type charCell struct {
	color    style.Color
	hasColor bool
}

// charGrid paints one character per terminal cell (dot/block/bar/custom).
type charGrid struct {
	width          int
	height         int
	cells          []charCell
	cellChar       rune
	applyColorToBG bool
}

func newCharGrid(width, height int, cellChar rune) *charGrid {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return &charGrid{
		width:    width,
		height:   height,
		cells:    make([]charCell, width*height),
		cellChar: cellChar,
	}
}

func (g *charGrid) withColorToBG() *charGrid {
	g.applyColorToBG = true
	return g
}

func (g *charGrid) resolution() (float64, float64) {
	return float64(g.width), float64(g.height)
}

func (g *charGrid) paint(x, y int, color style.Color) {
	if g.width <= 0 {
		return
	}
	index := y*g.width + x
	if index < 0 || index >= len(g.cells) {
		return
	}
	g.cells[index] = charCell{color: color, hasColor: true}
}

func (g *charGrid) save() layer {
	contents := make([]layerCell, len(g.cells))
	for i, cell := range g.cells {
		if !cell.hasColor {
			contents[i] = layerCell{}
			continue
		}
		lc := layerCell{
			symbol:    g.cellChar,
			hasSymbol: true,
			fg:        cell.color,
			hasFG:     true,
		}
		if g.applyColorToBG {
			lc.bg = cell.color
			lc.hasBG = true
		}
		contents[i] = lc
	}
	return layer{contents: contents}
}

func (g *charGrid) reset() {
	clear(g.cells)
}

// halfPixel is one optional color in a half-block sub-row.
type halfPixel struct {
	color style.Color
	set   bool
}

// halfBlockGrid paints 1x2 vertical half-block pixels per terminal cell.
type halfBlockGrid struct {
	width  int
	height int
	// pixels is row-major half-rows: height*2 rows, each width wide.
	pixels [][]halfPixel
}

func newHalfBlockGrid(width, height int) *halfBlockGrid {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	rows := height * 2
	pixels := make([][]halfPixel, rows)
	for i := range pixels {
		pixels[i] = make([]halfPixel, width)
	}
	return &halfBlockGrid{
		width:  width,
		height: height,
		pixels: pixels,
	}
}

func (g *halfBlockGrid) resolution() (float64, float64) {
	return float64(g.width), float64(g.height) * 2
}

func (g *halfBlockGrid) paint(x, y int, color style.Color) {
	if y < 0 || y >= len(g.pixels) {
		return
	}
	row := g.pixels[y]
	if x < 0 || x >= len(row) {
		return
	}
	row[x] = halfPixel{color: color, set: true}
}

func (g *halfBlockGrid) save() layer {
	// Pair vertical half-rows into terminal cells:
	// 1. upper none, lower none  => blank
	// 2. upper none, lower color => lower half, fg=lower
	// 3. upper color, lower none => upper half, fg=upper
	// 4. same colors             => full block, fg=bg=color
	// 5. different colors        => upper half, fg=upper, bg=lower
	n := g.width * g.height
	contents := make([]layerCell, 0, n)
	for row := 0; row+1 < len(g.pixels); row += 2 {
		upperRow := g.pixels[row]
		lowerRow := g.pixels[row+1]
		for col := range g.width {
			var upper, lower halfPixel
			if col < len(upperRow) {
				upper = upperRow[col]
			}
			if col < len(lowerRow) {
				lower = lowerRow[col]
			}
			contents = append(contents, halfBlockLayerCell(upper, lower))
		}
	}
	// If height is odd in half-rows somehow, pad remaining as blank.
	for len(contents) < n {
		contents = append(contents, layerCell{})
	}
	return layer{contents: contents}
}

func halfBlockLayerCell(upper, lower halfPixel) layerCell {
	switch {
	case !upper.set && !lower.set:
		return layerCell{}
	case !upper.set && lower.set:
		return layerCell{
			symbol:    symbols.HalfBlockLower,
			hasSymbol: true,
			fg:        lower.color,
			hasFG:     true,
		}
	case upper.set && !lower.set:
		return layerCell{
			symbol:    symbols.HalfBlockUpper,
			hasSymbol: true,
			fg:        upper.color,
			hasFG:     true,
		}
	case upper.set && lower.set && upper.color == lower.color:
		return layerCell{
			symbol:    symbols.HalfBlockFull,
			hasSymbol: true,
			fg:        upper.color,
			hasFG:     true,
			bg:        lower.color,
			hasBG:     true,
		}
	default:
		return layerCell{
			symbol:    symbols.HalfBlockUpper,
			hasSymbol: true,
			fg:        upper.color,
			hasFG:     true,
			bg:        lower.color,
			hasBG:     true,
		}
	}
}

func (g *halfBlockGrid) reset() {
	for i := range g.pixels {
		clear(g.pixels[i])
	}
}

func markerToGrid(width, height int, marker symbols.Marker) grid {
	dot := firstRune(symbols.Dot, '•')
	block := firstRune(symbols.BlockFull, '█')
	bar := firstRune(symbols.BarHalf, '▄')

	switch marker.Kind {
	case symbols.MarkerBlock:
		return newCharGrid(width, height, block).withColorToBG()
	case symbols.MarkerBar:
		return newCharGrid(width, height, bar)
	case symbols.MarkerBraille:
		return newPatternGrid(width, height, 2, 4, symbols.Braille[:])
	case symbols.MarkerHalfBlock:
		return newHalfBlockGrid(width, height)
	case symbols.MarkerQuadrant:
		return newPatternGrid(width, height, 2, 2, symbols.Quadrants[:])
	case symbols.MarkerSextant:
		return newPatternGrid(width, height, 2, 3, symbols.Sextants[:])
	case symbols.MarkerOctant:
		return newPatternGrid(width, height, 2, 4, symbols.Octants[:])
	case symbols.MarkerCustom:
		return newCharGrid(width, height, marker.Rune)
	case symbols.MarkerDot:
		fallthrough
	default:
		return newCharGrid(width, height, dot)
	}
}

func firstRune(s string, fallback rune) rune {
	for _, r := range s {
		return r
	}
	return fallback
}

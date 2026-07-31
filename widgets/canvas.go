package widgets

import (
	"math"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
	"github.com/lyc-aon/ratatui-go/text"
)

// Shape draws itself onto a canvas Painter.
type Shape interface {
	Draw(p *Painter)
}

// label is text drawn on top of canvas layers in world coordinates.
type label struct {
	x, y float64
	line text.Line
}

// Painter maps world coordinates onto the current grid and paints dots.
type Painter struct {
	context    *Context
	resolution [2]float64
}

// GetPoint maps world (x, y) onto grid-dot coordinates.
//
// Origin of the world system is the lower-left of the canvas bounds. Grid origin
// is the upper-left. Points outside bounds (or with non-positive span) return ok=false.
// Values land on the nearest grid cell; exact midpoints round away from -inf (Go Round).
func (p *Painter) GetPoint(x, y float64) (px, py int, ok bool) {
	if p == nil || p.context == nil {
		return 0, 0, false
	}
	left, right := p.context.xBounds[0], p.context.xBounds[1]
	bottom, top := p.context.yBounds[0], p.context.yBounds[1]
	if x < left || x > right || y < bottom || y > top {
		return 0, 0, false
	}
	width := right - left
	height := top - bottom
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	px = int(math.Round((x - left) * (p.resolution[0] - 1) / width))
	py = int(math.Round((top - y) * (p.resolution[1] - 1) / height))
	if px < 0 {
		px = 0
	}
	if py < 0 {
		py = 0
	}
	return px, py, true
}

// Paint sets the color of grid-dot (px, py) on the current layer.
func (p *Painter) Paint(px, py int, color style.Color) {
	if p == nil || p.context == nil || p.context.grid == nil {
		return
	}
	p.context.grid.paint(px, py, color)
}

// Bounds returns the canvas world x and y bounds ([left,right], [bottom,top]).
func (p *Painter) Bounds() (xBounds, yBounds [2]float64) {
	if p == nil || p.context == nil {
		return [2]float64{}, [2]float64{}
	}
	return p.context.xBounds, p.context.yBounds
}

func newPainter(ctx *Context) Painter {
	var res [2]float64
	if ctx != nil && ctx.grid != nil {
		rx, ry := ctx.grid.resolution()
		res = [2]float64{rx, ry}
	}
	return Painter{context: ctx, resolution: res}
}

// Context holds paint state while a Canvas paint func runs.
type Context struct {
	width   int
	height  int
	xBounds [2]float64
	yBounds [2]float64
	grid    grid
	dirty   bool
	layers  []layer
	labels  []label
}

// NewContext creates a paint context for width×height terminal cells.
//
// xBounds is [left, right], yBounds is [bottom, top]. marker selects the grid
// resolution and glyph set (braille default at the Canvas level).
func NewContext(width, height int, xBounds, yBounds [2]float64, marker symbols.Marker) *Context {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return &Context{
		width:   width,
		height:  height,
		xBounds: xBounds,
		yBounds: yBounds,
		grid:    markerToGrid(width, height, marker),
		layers:  nil,
		labels:  nil,
	}
}

// SetMarker swaps the active marker, saving the current dirty layer first.
func (c *Context) SetMarker(marker symbols.Marker) {
	if c == nil {
		return
	}
	c.finish()
	c.grid = markerToGrid(c.width, c.height, marker)
}

// Draw paints shape onto the current layer.
func (c *Context) Draw(shape Shape) {
	if c == nil || shape == nil {
		return
	}
	c.dirty = true
	p := newPainter(c)
	shape.Draw(&p)
}

// Layer saves the current grid as a layer and clears the grid for the next one.
func (c *Context) Layer() {
	if c == nil || c.grid == nil {
		return
	}
	c.layers = append(c.layers, c.grid.save())
	c.grid.reset()
	c.dirty = false
}

// Print queues a text label at world (x, y). Labels draw above all layers.
func (c *Context) Print(x, y float64, line text.Line) {
	if c == nil {
		return
	}
	c.labels = append(c.labels, label{
		x:    x,
		y:    y,
		line: copyLine(line),
	})
}

func (c *Context) finish() {
	if c == nil {
		return
	}
	if c.dirty {
		c.Layer()
	}
}

// Canvas is a drawable world-coordinate surface rendered into terminal cells.
//
// Use Paint to supply a closure that draws via Context. Default marker is braille.
type Canvas struct {
	block           *Block
	xBounds         [2]float64
	yBounds         [2]float64
	paintFn         func(*Context)
	backgroundColor style.Color
	marker          symbols.Marker
}

// NewCanvas creates a canvas with zero bounds, reset background, and braille marker.
func NewCanvas() Canvas {
	return Canvas{
		backgroundColor: style.Reset,
		marker:          symbols.Marker{Kind: symbols.MarkerBraille},
	}
}

// Block wraps the canvas with a block.
func (c Canvas) Block(b Block) Canvas {
	cp := b
	c.block = &cp
	return c
}

// XBounds sets the world X viewport [left, right].
func (c Canvas) XBounds(bounds [2]float64) Canvas {
	c.xBounds = bounds
	return c
}

// YBounds sets the world Y viewport [bottom, top].
func (c Canvas) YBounds(bounds [2]float64) Canvas {
	c.yBounds = bounds
	return c
}

// Paint stores the paint function invoked during Render.
func (c Canvas) Paint(fn func(*Context)) Canvas {
	c.paintFn = fn
	return c
}

// BackgroundColor sets the canvas area background before layers blit.
func (c Canvas) BackgroundColor(color style.Color) Canvas {
	c.backgroundColor = color
	return c
}

// Marker selects the grid marker used for drawing.
func (c Canvas) Marker(m symbols.Marker) Canvas {
	c.marker = m
	return c
}

// Render draws the canvas into buf within area.
//
// Intersects with buf.Area and returns on empty. Order: optional block → background
// style on inner → paint closure → layer composite → labels.
func (c Canvas) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}

	inner := InnerIfSome(c.block, area, buf)
	if inner.IsEmpty() {
		return
	}

	buf.SetStyle(inner, style.New().WithBG(c.backgroundColor))

	if c.paintFn == nil {
		return
	}

	ctx := NewContext(inner.Width, inner.Height, c.xBounds, c.yBounds, c.marker)
	c.paintFn(ctx)
	ctx.finish()

	width := inner.Width
	if width <= 0 {
		return
	}
	for _, lyr := range ctx.layers {
		for index, lc := range lyr.contents {
			x := (index % width) + inner.X
			y := (index / width) + inner.Y
			cell := buf.GetMut(x, y)
			if cell == nil {
				continue
			}
			if lc.hasSymbol {
				cell.SetSymbol(string(lc.symbol))
			}
			if lc.hasFG || lc.hasBG {
				st := style.New()
				if lc.hasFG {
					st = st.WithFG(lc.fg)
				}
				if lc.hasBG {
					st = st.WithBG(lc.bg)
				}
				cell.SetStyle(st)
			}
		}
	}

	// Labels use cell (not sub-pixel) resolution and absolute bounds span.
	// Rust casts NaN (zero-span divide) to u16 → 0; map zero x/y span to offset 0
	// instead of returning early. GetPoint keeps its own zero-span guard.
	left := c.xBounds[0]
	right := c.xBounds[1]
	top := c.yBounds[1]
	bottom := c.yBounds[0]
	spanW := math.Abs(right - left)
	spanH := math.Abs(top - bottom)
	if inner.Width == 0 || inner.Height == 0 {
		return
	}
	resX := float64(inner.Width - 1)
	resY := float64(inner.Height - 1)
	if inner.Width == 1 {
		resX = 0
	}
	if inner.Height == 1 {
		resY = 0
	}
	for _, lbl := range ctx.labels {
		if lbl.x < left || lbl.x > right || lbl.y > top || lbl.y < bottom {
			continue
		}
		xOff := 0
		yOff := 0
		if spanW != 0 {
			xOff = int((lbl.x - left) * resX / spanW)
		}
		if spanH != 0 {
			yOff = int((top - lbl.y) * resY / spanH)
		}
		x := xOff + inner.X
		y := yOff + inner.Y
		maxWidth := inner.Right() - x
		if maxWidth <= 0 {
			continue
		}
		buf.SetLine(x, y, lbl.line, maxWidth)
	}
}

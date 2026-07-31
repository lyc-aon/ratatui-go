package widgets

import (
	"math"

	"github.com/michaelkelly/ratatui-go/style"
)

// Points is a scatter of world-coordinate dots.
type Points struct {
	Coords [][2]float64
	Color  style.Color
}

// NewPoints builds a Points shape, copying coords.
func NewPoints(coords [][2]float64, color style.Color) Points {
	return Points{Coords: copyCoords(coords), Color: color}
}

// Draw paints each in-bounds coordinate.
func (p Points) Draw(painter *Painter) {
	if painter == nil {
		return
	}
	for _, c := range p.Coords {
		if x, y, ok := painter.GetPoint(c[0], c[1]); ok {
			painter.Paint(x, y, p.Color)
		}
	}
}

// Line is a straight segment from (X1,Y1) to (X2,Y2).
type Line struct {
	X1, Y1 float64
	X2, Y2 float64
	Color  style.Color
}

// NewLine builds a line shape.
func NewLine(x1, y1, x2, y2 float64, color style.Color) Line {
	return Line{X1: x1, Y1: y1, X2: x2, Y2: y2, Color: color}
}

// Draw clips to painter bounds then rasterizes with Bresenham.
func (l Line) Draw(painter *Painter) {
	if painter == nil {
		return
	}
	xBounds, yBounds := painter.Bounds()
	wx1, wy1, wx2, wy2, ok := clipLine(xBounds[0], xBounds[1], yBounds[0], yBounds[1], l.X1, l.Y1, l.X2, l.Y2)
	if !ok {
		return
	}
	x1, y1, ok1 := painter.GetPoint(wx1, wy1)
	x2, y2, ok2 := painter.GetPoint(wx2, wy2)
	if !ok1 || !ok2 {
		return
	}
	drawLine(painter, x1, y1, x2, y2, l.Color)
}

// FilledLine is a line that also fills vertically to FillToY.
//
// Useful for area charts: every column on the line segment is filled between
// the line's y and FillToY (clamped into the painter y bounds).
type FilledLine struct {
	X1, Y1  float64
	X2, Y2  float64
	FillToY float64
	Color   style.Color
}

// NewFilledLine builds a filled line shape.
func NewFilledLine(x1, y1, x2, y2, fillToY float64, color style.Color) FilledLine {
	return FilledLine{X1: x1, Y1: y1, X2: x2, Y2: y2, FillToY: fillToY, Color: color}
}

// Draw clips the line, maps FillToY, and paints vertical spans under the stroke.
func (l FilledLine) Draw(painter *Painter) {
	if painter == nil {
		return
	}
	xBounds, yBounds := painter.Bounds()
	wx1, wy1, wx2, wy2, ok := clipLine(xBounds[0], xBounds[1], yBounds[0], yBounds[1], l.X1, l.Y1, l.X2, l.Y2)
	if !ok {
		return
	}
	x1, y1, ok1 := painter.GetPoint(wx1, wy1)
	x2, y2, ok2 := painter.GetPoint(wx2, wy2)
	if !ok1 || !ok2 {
		return
	}
	yFillWorld := clampFloat(l.FillToY, yBounds[0], yBounds[1])
	_, yFill, okFill := painter.GetPoint(wx1, yFillWorld)
	if !okFill {
		return
	}
	forEachLinePoint(x1, y1, x2, y2, func(x, y int) {
		start := y
		end := yFill
		if start > end {
			start, end = end, start
		}
		for yy := start; yy <= end; yy++ {
			painter.Paint(x, yy, l.Color)
		}
	})
}

// Circle is an outline circle approximated by a 360° angle sweep.
type Circle struct {
	X, Y   float64
	Radius float64
	Color  style.Color
}

// NewCircle builds a circle shape.
func NewCircle(x, y, radius float64, color style.Color) Circle {
	return Circle{X: x, Y: y, Radius: radius, Color: color}
}

// Draw paints one sample per degree around the circumference.
func (c Circle) Draw(painter *Painter) {
	if painter == nil {
		return
	}
	for angle := range 360 {
		radians := float64(angle) * math.Pi / 180
		cx := c.Radius*math.Cos(radians) + c.X
		cy := c.Radius*math.Sin(radians) + c.Y
		if x, y, ok := painter.GetPoint(cx, cy); ok {
			painter.Paint(x, y, c.Color)
		}
	}
}

// Rectangle is an axis-aligned outline from bottom-left (X,Y) with Width×Height.
type Rectangle struct {
	X, Y          float64
	Width, Height float64
	Color         style.Color
}

// NewRectangle builds a rectangle shape.
func NewRectangle(x, y, width, height float64, color style.Color) Rectangle {
	return Rectangle{X: x, Y: y, Width: width, Height: height, Color: color}
}

// Draw paints the four edges as lines.
func (r Rectangle) Draw(painter *Painter) {
	if painter == nil {
		return
	}
	lines := [4]Line{
		{X1: r.X, Y1: r.Y, X2: r.X, Y2: r.Y + r.Height, Color: r.Color},
		{X1: r.X, Y1: r.Y + r.Height, X2: r.X + r.Width, Y2: r.Y + r.Height, Color: r.Color},
		{X1: r.X + r.Width, Y1: r.Y, X2: r.X + r.Width, Y2: r.Y + r.Height, Color: r.Color},
		{X1: r.X, Y1: r.Y, X2: r.X + r.Width, Y2: r.Y, Color: r.Color},
	}
	for i := range lines {
		lines[i].Draw(painter)
	}
}

func drawLine(painter *Painter, x1, y1, x2, y2 int, color style.Color) {
	forEachLinePoint(x1, y1, x2, y2, func(x, y int) {
		painter.Paint(x, y, color)
	})
}

// forEachLinePoint walks Bresenham pixels from (x1,y1) to (x2,y2).
func forEachLinePoint(x1, y1, x2, y2 int, f func(x, y int)) {
	dx := absDiffInt(x1, x2)
	dy := absDiffInt(y1, y2)

	if dx == 0 {
		lo, hi := y1, y2
		if lo > hi {
			lo, hi = hi, lo
		}
		for y := lo; y <= hi; y++ {
			f(x1, y)
		}
		return
	}
	if dy == 0 {
		lo, hi := x1, x2
		if lo > hi {
			lo, hi = hi, lo
		}
		for x := lo; x <= hi; x++ {
			f(x, y1)
		}
		return
	}
	if dy < dx {
		if x1 > x2 {
			forEachLinePointLow(x2, y2, x1, y1, f)
		} else {
			forEachLinePointLow(x1, y1, x2, y2, f)
		}
		return
	}
	if y1 > y2 {
		forEachLinePointHigh(x2, y2, x1, y1, f)
	} else {
		forEachLinePointHigh(x1, y1, x2, y2, f)
	}
}

func forEachLinePointLow(x1, y1, x2, y2 int, f func(x, y int)) {
	dx := x2 - x1
	dy := absDiffInt(y1, y2)
	d := 2*dy - dx
	y := y1
	for x := x1; x <= x2; x++ {
		f(x, y)
		if d > 0 {
			if y1 > y2 {
				if y > 0 {
					y--
				}
			} else {
				y++
			}
			d -= 2 * dx
		}
		d += 2 * dy
	}
}

func forEachLinePointHigh(x1, y1, x2, y2 int, f func(x, y int)) {
	dx := absDiffInt(x1, x2)
	dy := y2 - y1
	d := 2*dx - dy
	x := x1
	for y := y1; y <= y2; y++ {
		f(x, y)
		if d > 0 {
			if x1 > x2 {
				if x > 0 {
					x--
				}
			} else {
				x++
			}
			d -= 2 * dy
		}
		d += 2 * dx
	}
}

// clipLine runs Cohen-Sutherland clipping against [xmin,xmax]x[ymin,ymax].
func clipLine(xmin, xmax, ymin, ymax, x1, y1, x2, y2 float64) (cx1, cy1, cx2, cy2 float64, ok bool) {
	const (
		codeInside = 0
		codeLeft   = 1
		codeRight  = 2
		codeBottom = 4
		codeTop    = 8
	)
	outCode := func(x, y float64) int {
		code := codeInside
		if x < xmin {
			code |= codeLeft
		} else if x > xmax {
			code |= codeRight
		}
		if y < ymin {
			code |= codeBottom
		} else if y > ymax {
			code |= codeTop
		}
		return code
	}

	code1 := outCode(x1, y1)
	code2 := outCode(x2, y2)
	for {
		if code1|code2 == 0 {
			return x1, y1, x2, y2, true
		}
		if code1&code2 != 0 {
			return 0, 0, 0, 0, false
		}
		codeOut := code1
		if codeOut == 0 {
			codeOut = code2
		}
		var x, y float64
		switch {
		case codeOut&codeTop != 0:
			if y2 != y1 {
				x = x1 + (x2-x1)*(ymax-y1)/(y2-y1)
			} else {
				x = x1
			}
			y = ymax
		case codeOut&codeBottom != 0:
			if y2 != y1 {
				x = x1 + (x2-x1)*(ymin-y1)/(y2-y1)
			} else {
				x = x1
			}
			y = ymin
		case codeOut&codeRight != 0:
			if x2 != x1 {
				y = y1 + (y2-y1)*(xmax-x1)/(x2-x1)
			} else {
				y = y1
			}
			x = xmax
		default: // left
			if x2 != x1 {
				y = y1 + (y2-y1)*(xmin-x1)/(x2-x1)
			} else {
				y = y1
			}
			x = xmin
		}
		if codeOut == code1 {
			x1, y1 = x, y
			code1 = outCode(x1, y1)
		} else {
			x2, y2 = x, y
			code2 = outCode(x2, y2)
		}
	}
}

func absDiffInt(a, b int) int {
	if a >= b {
		return a - b
	}
	return b - a
}

func clampFloat(v, lo, hi float64) float64 {
	if lo > hi {
		lo, hi = hi, lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func copyCoords(in [][2]float64) [][2]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make([][2]float64, len(in))
	copy(out, in)
	return out
}

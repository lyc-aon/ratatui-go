package layout

// Rect is a rectangular region in terminal cell coordinates.
//
// The half-open ranges covered are X..Right() by Y..Bottom().
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// NewRect creates a Rect at (x, y) with the given size.
// Width and height are clamped so sizes are never negative.
func NewRect(x, y, width, height int) Rect {
	return Rect{
		X:      clampNonNeg(x),
		Y:      clampNonNeg(y),
		Width:  clampNonNeg(width),
		Height: clampNonNeg(height),
	}
}

// Right returns the first X coordinate outside the rect (X + Width).
func (r Rect) Right() int {
	return clampNonNeg(r.X + r.Width)
}

// Bottom returns the first Y coordinate outside the rect (Y + Height).
func (r Rect) Bottom() int {
	return clampNonNeg(r.Y + r.Height)
}

// Left returns the left edge X coordinate.
func (r Rect) Left() int { return r.X }

// Top returns the top edge Y coordinate.
func (r Rect) Top() int { return r.Y }

// IsEmpty reports whether the rect has zero area.
func (r Rect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

// Area returns Width * Height, or 0 when empty.
func (r Rect) Area() int {
	if r.IsEmpty() {
		return 0
	}
	return r.Width * r.Height
}

// Contains reports whether position is inside the half-open rect.
func (r Rect) Contains(p Position) bool {
	return p.X >= r.X && p.X < r.Right() && p.Y >= r.Y && p.Y < r.Bottom()
}

// Intersects reports whether the two rects overlap.
func (r Rect) Intersects(other Rect) bool {
	return r.X < other.Right() && r.Right() > other.X && r.Y < other.Bottom() && r.Bottom() > other.Y
}

// Intersection returns the overlapping region, or a zero-area rect if they do not overlap.
func (r Rect) Intersection(other Rect) Rect {
	x1 := maxInt(r.X, other.X)
	y1 := maxInt(r.Y, other.Y)
	x2 := minInt(r.Right(), other.Right())
	y2 := minInt(r.Bottom(), other.Bottom())
	return NewRect(x1, y1, clampNonNeg(x2-x1), clampNonNeg(y2-y1))
}

// Union returns the smallest rect covering both receivers.
func (r Rect) Union(other Rect) Rect {
	x1 := minInt(r.X, other.X)
	y1 := minInt(r.Y, other.Y)
	x2 := maxInt(r.Right(), other.Right())
	y2 := maxInt(r.Bottom(), other.Bottom())
	return NewRect(x1, y1, clampNonNeg(x2-x1), clampNonNeg(y2-y1))
}

// Inner returns the rect shrunk by margin on each side.
// If the margin does not fit, a zero rect is returned.
func (r Rect) Inner(m Margin) Rect {
	h := clampNonNeg(m.Horizontal)
	v := clampNonNeg(m.Vertical)
	dh := h * 2
	dv := v * 2
	if r.Width < dh || r.Height < dv {
		return Rect{}
	}
	return Rect{
		X:      r.X + h,
		Y:      r.Y + v,
		Width:  r.Width - dh,
		Height: r.Height - dv,
	}
}

// Outer returns the rect expanded by margin on each side, clamped at zero origin.
func (r Rect) Outer(m Margin) Rect {
	h := clampNonNeg(m.Horizontal)
	v := clampNonNeg(m.Vertical)
	x := clampNonNeg(r.X - h)
	y := clampNonNeg(r.Y - v)
	right := r.Right() + h
	bottom := r.Bottom() + v
	return NewRect(x, y, clampNonNeg(right-x), clampNonNeg(bottom-y))
}

// Offset moves the rect by the given offset without changing its size.
// Coordinates are clamped so the result stays in non-negative space.
func (r Rect) Offset(o Offset) Rect {
	return Rect{
		X:      clampNonNeg(r.X + o.X),
		Y:      clampNonNeg(r.Y + o.Y),
		Width:  r.Width,
		Height: r.Height,
	}
}

// Resize returns a copy with the given size, keeping the top-left corner.
func (r Rect) Resize(s Size) Rect {
	return NewRect(r.X, r.Y, s.Width, s.Height)
}

// Clamp moves and shrinks r so it fits entirely inside other.
func (r Rect) Clamp(other Rect) Rect {
	width := minInt(r.Width, other.Width)
	height := minInt(r.Height, other.Height)
	maxX := other.Right() - width
	maxY := other.Bottom() - height
	if maxX < other.X {
		maxX = other.X
	}
	if maxY < other.Y {
		maxY = other.Y
	}
	x := clampInt(r.X, other.X, maxX)
	y := clampInt(r.Y, other.Y, maxY)
	return NewRect(x, y, width, height)
}

// AsPosition returns the top-left corner as a Position.
func (r Rect) AsPosition() Position {
	return Position{X: r.X, Y: r.Y}
}

// AsSize returns the dimensions as a Size.
func (r Rect) AsSize() Size {
	return Size{Width: r.Width, Height: r.Height}
}

// Rows returns one-cell-tall sub-rects covering each row of r.
func (r Rect) Rows() []Rect {
	if r.Height <= 0 {
		return nil
	}
	out := make([]Rect, r.Height)
	for i := range r.Height {
		out[i] = Rect{X: r.X, Y: r.Y + i, Width: clampNonNeg(r.Width), Height: 1}
	}
	return out
}

// Columns returns one-cell-wide sub-rects covering each column of r.
func (r Rect) Columns() []Rect {
	if r.Width <= 0 {
		return nil
	}
	out := make([]Rect, r.Width)
	for i := range r.Width {
		out[i] = Rect{X: r.X + i, Y: r.Y, Width: 1, Height: clampNonNeg(r.Height)}
	}
	return out
}

// Positions returns every cell position inside r in row-major order.
func (r Rect) Positions() []Position {
	if r.IsEmpty() {
		return nil
	}
	out := make([]Position, 0, r.Area())
	for y := r.Y; y < r.Bottom(); y++ {
		for x := r.X; x < r.Right(); x++ {
			out = append(out, Position{X: x, Y: y})
		}
	}
	return out
}

// CenteredHorizontally returns a horizontally centered sub-rect of the given constraint width.
func (r Rect) CenteredHorizontally(c Constraint) Rect {
	parts := Horizontal(c).Flex(FlexCenter).Split(r)
	if len(parts) == 0 {
		return Rect{X: r.X, Y: r.Y, Width: 0, Height: r.Height}
	}
	return parts[0]
}

// CenteredVertically returns a vertically centered sub-rect of the given constraint height.
func (r Rect) CenteredVertically(c Constraint) Rect {
	parts := Vertical(c).Flex(FlexCenter).Split(r)
	if len(parts) == 0 {
		return Rect{X: r.X, Y: r.Y, Width: r.Width, Height: 0}
	}
	return parts[0]
}

// Centered returns a sub-rect centered on both axes.
func (r Rect) Centered(horizontal, vertical Constraint) Rect {
	return r.CenteredHorizontally(horizontal).CenteredVertically(vertical)
}

package layout

// Position is a terminal cell coordinate relative to the top-left origin (0, 0).
type Position struct {
	X int
	Y int
}

// Origin is the top-left position (0, 0).
var Origin = Position{X: 0, Y: 0}

// NewPosition returns a Position at (x, y).
func NewPosition(x, y int) Position {
	return Position{X: x, Y: y}
}

// Offset moves the position by the given delta, clamping coordinates at zero.
func (p Position) Offset(dx, dy int) Position {
	return Position{
		X: clampNonNeg(p.X + dx),
		Y: clampNonNeg(p.Y + dy),
	}
}

// Size is a width/height pair in terminal cells.
type Size struct {
	Width  int
	Height int
}

// NewSize returns a Size with the given dimensions, clamped at zero.
func NewSize(width, height int) Size {
	return Size{
		Width:  clampNonNeg(width),
		Height: clampNonNeg(height),
	}
}

// Area returns width * height, or 0 when empty.
func (s Size) Area() int {
	if s.Width <= 0 || s.Height <= 0 {
		return 0
	}
	return s.Width * s.Height
}

// IsEmpty reports whether either dimension is zero.
func (s Size) IsEmpty() bool {
	return s.Width == 0 || s.Height == 0
}

// Margin is horizontal and vertical inset applied on both sides of a rectangle.
type Margin struct {
	Horizontal int
	Vertical   int
}

// NewMargin returns a Margin with the given horizontal and vertical insets, clamped at zero.
func NewMargin(horizontal, vertical int) Margin {
	return Margin{
		Horizontal: clampNonNeg(horizontal),
		Vertical:   clampNonNeg(vertical),
	}
}

// UniformMargin returns a Margin with the same inset on both axes.
func UniformMargin(n int) Margin {
	n = clampNonNeg(n)
	return Margin{Horizontal: n, Vertical: n}
}

// Offset is a signed displacement used to move rectangles and positions.
type Offset struct {
	X int
	Y int
}

// NewOffset returns an Offset with the given components.
func NewOffset(x, y int) Offset {
	return Offset{X: x, Y: y}
}

// Neg returns the negated offset.
func (o Offset) Neg() Offset {
	return Offset{X: -o.X, Y: -o.Y}
}

func clampNonNeg(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

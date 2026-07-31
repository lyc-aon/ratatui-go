package widgets

// Padding is the inner inset of a Block, in terminal cells.
//
// Terminal cells are often taller than wide; doubling horizontal padding is a
// common way to make padding look even (see PaddingProportional).
type Padding struct {
	Left   int
	Right  int
	Top    int
	Bottom int
}

// PaddingZero is Padding with every side 0.
var PaddingZero = Padding{}

// NewPadding creates a Padding by specifying every side.
// Order is left, right, top, bottom (matches upstream, not CSS order).
// Negative values clamp to 0.
func NewPadding(left, right, top, bottom int) Padding {
	return Padding{
		Left:   clampNonNeg(left),
		Right:  clampNonNeg(right),
		Top:    clampNonNeg(top),
		Bottom: clampNonNeg(bottom),
	}
}

// PaddingUniform sets every side to value.
func PaddingUniform(value int) Padding {
	value = clampNonNeg(value)
	return Padding{Left: value, Right: value, Top: value, Bottom: value}
}

// PaddingHorizontal sets left and right to value.
func PaddingHorizontal(value int) Padding {
	value = clampNonNeg(value)
	return Padding{Left: value, Right: value}
}

// PaddingVertical sets top and bottom to value.
func PaddingVertical(value int) Padding {
	value = clampNonNeg(value)
	return Padding{Top: value, Bottom: value}
}

// PaddingSymmetric sets left/right to x and top/bottom to y.
func PaddingSymmetric(x, y int) Padding {
	x = clampNonNeg(x)
	y = clampNonNeg(y)
	return Padding{Left: x, Right: x, Top: y, Bottom: y}
}

// PaddingProportional sets left/right to 2*value and top/bottom to value.
func PaddingProportional(value int) Padding {
	value = clampNonNeg(value)
	return Padding{
		Left:   2 * value,
		Right:  2 * value,
		Top:    value,
		Bottom: value,
	}
}

// PaddingLeft sets only the left side.
func PaddingLeft(value int) Padding {
	return Padding{Left: clampNonNeg(value)}
}

// PaddingRight sets only the right side.
func PaddingRight(value int) Padding {
	return Padding{Right: clampNonNeg(value)}
}

// PaddingTop sets only the top side.
func PaddingTop(value int) Padding {
	return Padding{Top: clampNonNeg(value)}
}

// PaddingBottom sets only the bottom side.
func PaddingBottom(value int) Padding {
	return Padding{Bottom: clampNonNeg(value)}
}

func clampNonNeg(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

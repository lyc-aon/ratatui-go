package layout

import "fmt"

// ConstraintKind identifies which constraint variant is active.
type ConstraintKind int

const (
	// ConstraintMin is a minimum size constraint.
	ConstraintMin ConstraintKind = iota
	// ConstraintMax is a maximum size constraint.
	ConstraintMax
	// ConstraintLength is a fixed length constraint.
	ConstraintLength
	// ConstraintPercentage is a percentage of the full split area.
	ConstraintPercentage
	// ConstraintRatio is a ratio of the full split area.
	ConstraintRatio
	// ConstraintFill is a proportional fill of remaining space.
	ConstraintFill
)

// Constraint defines how much space a layout segment should take.
//
// Relative constraints (Percentage, Ratio) are calculated against the entire
// space being divided, not the residual after fixed constraints.
//
// Priority order when space is contested (highest first):
// Min, Max, Length, Percentage, Ratio, Fill.
type Constraint struct {
	kind ConstraintKind
	a    int // length/min/max/percentage/fill weight, or ratio numerator
	b    int // ratio denominator (Ratio only)
}

// Min returns a minimum size constraint.
func Min(n int) Constraint {
	return Constraint{kind: ConstraintMin, a: clampNonNeg(n)}
}

// Max returns a maximum size constraint.
func Max(n int) Constraint {
	return Constraint{kind: ConstraintMax, a: clampNonNeg(n)}
}

// Length returns a fixed length constraint.
func Length(n int) Constraint {
	return Constraint{kind: ConstraintLength, a: clampNonNeg(n)}
}

// Percentage returns a percentage-of-area constraint (0–100 typical; larger values allowed).
func Percentage(n int) Constraint {
	return Constraint{kind: ConstraintPercentage, a: clampNonNeg(n)}
}

// Ratio returns a num/den fraction-of-area constraint. A zero denominator is treated as 1.
func Ratio(num, den int) Constraint {
	if num < 0 {
		num = 0
	}
	if den < 0 {
		den = 0
	}
	return Constraint{kind: ConstraintRatio, a: num, b: den}
}

// Fill returns a proportional fill constraint.
func Fill(weight int) Constraint {
	return Constraint{kind: ConstraintFill, a: clampNonNeg(weight)}
}

// Kind returns the active constraint variant.
func (c Constraint) Kind() ConstraintKind { return c.kind }

// Value returns the primary numeric parameter (length/min/max/percentage/fill/ratio numerator).
func (c Constraint) Value() int { return c.a }

// Denominator returns the ratio denominator, or 0 for non-ratio constraints.
func (c Constraint) Denominator() int {
	if c.kind == ConstraintRatio {
		return c.b
	}
	return 0
}

// IsMin reports whether c is a Min constraint.
func (c Constraint) IsMin() bool { return c.kind == ConstraintMin }

// IsMax reports whether c is a Max constraint.
func (c Constraint) IsMax() bool { return c.kind == ConstraintMax }

// IsLength reports whether c is a Length constraint.
func (c Constraint) IsLength() bool { return c.kind == ConstraintLength }

// IsPercentage reports whether c is a Percentage constraint.
func (c Constraint) IsPercentage() bool { return c.kind == ConstraintPercentage }

// IsRatio reports whether c is a Ratio constraint.
func (c Constraint) IsRatio() bool { return c.kind == ConstraintRatio }

// IsFill reports whether c is a Fill constraint.
func (c Constraint) IsFill() bool { return c.kind == ConstraintFill }

// Apply returns the size obtained by applying c to length (legacy helper).
func (c Constraint) Apply(length int) int {
	length = clampNonNeg(length)
	switch c.kind {
	case ConstraintPercentage:
		v := int(float64(c.a) / 100.0 * float64(length))
		if v > length {
			return length
		}
		return clampNonNeg(v)
	case ConstraintRatio:
		den := c.b
		if den <= 0 {
			den = 1
		}
		v := int(float64(c.a) / float64(den) * float64(length))
		if v > length {
			return length
		}
		return clampNonNeg(v)
	case ConstraintLength, ConstraintFill:
		return minInt(length, c.a)
	case ConstraintMax:
		return minInt(length, c.a)
	case ConstraintMin:
		return maxInt(length, c.a)
	default:
		return length
	}
}

// String returns a stable debug representation.
func (c Constraint) String() string {
	switch c.kind {
	case ConstraintMin:
		return fmt.Sprintf("Min(%d)", c.a)
	case ConstraintMax:
		return fmt.Sprintf("Max(%d)", c.a)
	case ConstraintLength:
		return fmt.Sprintf("Length(%d)", c.a)
	case ConstraintPercentage:
		return fmt.Sprintf("Percentage(%d)", c.a)
	case ConstraintRatio:
		return fmt.Sprintf("Ratio(%d, %d)", c.a, c.b)
	case ConstraintFill:
		return fmt.Sprintf("Fill(%d)", c.a)
	default:
		return "Constraint(?)"
	}
}

// FromLengths builds Length constraints.
func FromLengths(lengths ...int) []Constraint {
	out := make([]Constraint, len(lengths))
	for i, n := range lengths {
		out[i] = Length(n)
	}
	return out
}

// FromMins builds Min constraints.
func FromMins(mins ...int) []Constraint {
	out := make([]Constraint, len(mins))
	for i, n := range mins {
		out[i] = Min(n)
	}
	return out
}

// FromMaxes builds Max constraints.
func FromMaxes(maxes ...int) []Constraint {
	out := make([]Constraint, len(maxes))
	for i, n := range maxes {
		out[i] = Max(n)
	}
	return out
}

// FromPercentages builds Percentage constraints.
func FromPercentages(percentages ...int) []Constraint {
	out := make([]Constraint, len(percentages))
	for i, n := range percentages {
		out[i] = Percentage(n)
	}
	return out
}

// FromFills builds Fill constraints.
func FromFills(weights ...int) []Constraint {
	out := make([]Constraint, len(weights))
	for i, n := range weights {
		out[i] = Fill(n)
	}
	return out
}

// RatioPair is a (numerator, denominator) pair for FromRatios.
type RatioPair struct {
	Num int
	Den int
}

// FromRatios builds Ratio constraints.
func FromRatios(ratios ...RatioPair) []Constraint {
	out := make([]Constraint, len(ratios))
	for i, r := range ratios {
		out[i] = Ratio(r.Num, r.Den)
	}
	return out
}

// priorityRank returns lower numbers for higher-priority constraints.
func (c Constraint) priorityRank() int {
	switch c.kind {
	case ConstraintMin:
		return 0
	case ConstraintMax:
		return 1
	case ConstraintLength:
		return 2
	case ConstraintPercentage:
		return 3
	case ConstraintRatio:
		return 4
	case ConstraintFill:
		return 5
	default:
		return 6
	}
}

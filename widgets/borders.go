package widgets

import "github.com/michaelkelly/ratatui-go/symbols"

// Borders is a bitset of which block sides draw a border.
type Borders uint8

// Border side flags. Combine with | .
const (
	BorderNone   Borders = 0
	BorderTop    Borders = 1 << 0 // 0b0001
	BorderRight  Borders = 1 << 1 // 0b0010
	BorderBottom Borders = 1 << 2 // 0b0100
	BorderLeft   Borders = 1 << 3 // 0b1000
	BorderAll            = BorderTop | BorderRight | BorderBottom | BorderLeft
)

// Contains reports whether b includes every flag in other.
func (b Borders) Contains(other Borders) bool {
	return b&other == other
}

// Intersects reports whether b shares any flag with other.
func (b Borders) Intersects(other Borders) bool {
	return b&other != 0
}

// IsEmpty reports whether no border sides are set.
func (b Borders) IsEmpty() bool {
	return b == BorderNone
}

// IsAll reports whether every border side is set.
func (b Borders) IsAll() bool {
	return b&BorderAll == BorderAll
}

// String renders Borders like Rust Debug: NONE, ALL, or pipe-separated names.
func (b Borders) String() string {
	if b.IsEmpty() {
		return "NONE"
	}
	if b.IsAll() {
		return "ALL"
	}
	var parts []string
	if b&BorderTop != 0 {
		parts = append(parts, "TOP")
	}
	if b&BorderRight != 0 {
		parts = append(parts, "RIGHT")
	}
	if b&BorderBottom != 0 {
		parts = append(parts, "BOTTOM")
	}
	if b&BorderLeft != 0 {
		parts = append(parts, "LEFT")
	}
	if len(parts) == 0 {
		return "NONE"
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += " | " + parts[i]
	}
	return out
}

// BorderType selects a preset border symbol set.
type BorderType int

const (
	BorderTypePlain BorderType = iota
	BorderTypeRounded
	BorderTypeDouble
	BorderTypeThick
	BorderTypeLightDoubleDashed
	BorderTypeHeavyDoubleDashed
	BorderTypeLightTripleDashed
	BorderTypeHeavyTripleDashed
	BorderTypeLightQuadrupleDashed
	BorderTypeHeavyQuadrupleDashed
	BorderTypeQuadrantInside
	BorderTypeQuadrantOutside
)

// String returns the BorderType name.
func (t BorderType) String() string {
	switch t {
	case BorderTypePlain:
		return "Plain"
	case BorderTypeRounded:
		return "Rounded"
	case BorderTypeDouble:
		return "Double"
	case BorderTypeThick:
		return "Thick"
	case BorderTypeLightDoubleDashed:
		return "LightDoubleDashed"
	case BorderTypeHeavyDoubleDashed:
		return "HeavyDoubleDashed"
	case BorderTypeLightTripleDashed:
		return "LightTripleDashed"
	case BorderTypeHeavyTripleDashed:
		return "HeavyTripleDashed"
	case BorderTypeLightQuadrupleDashed:
		return "LightQuadrupleDashed"
	case BorderTypeHeavyQuadrupleDashed:
		return "HeavyQuadrupleDashed"
	case BorderTypeQuadrantInside:
		return "QuadrantInside"
	case BorderTypeQuadrantOutside:
		return "QuadrantOutside"
	default:
		return "Plain"
	}
}

// ToBorderSet maps the border type to its symbols.BorderSet.
func (t BorderType) ToBorderSet() symbols.BorderSet {
	return BorderTypeSymbols(t)
}

// BorderTypeSymbols maps a BorderType to the corresponding symbols.BorderSet.
func BorderTypeSymbols(t BorderType) symbols.BorderSet {
	switch t {
	case BorderTypePlain:
		return symbols.BorderPlain
	case BorderTypeRounded:
		return symbols.BorderRounded
	case BorderTypeDouble:
		return symbols.BorderDouble
	case BorderTypeThick:
		return symbols.BorderThick
	case BorderTypeLightDoubleDashed:
		return symbols.BorderLightDoubleDashed
	case BorderTypeHeavyDoubleDashed:
		return symbols.BorderHeavyDoubleDashed
	case BorderTypeLightTripleDashed:
		return symbols.BorderLightTripleDashed
	case BorderTypeHeavyTripleDashed:
		return symbols.BorderHeavyTripleDashed
	case BorderTypeLightQuadrupleDashed:
		return symbols.BorderLightQuadrupleDashed
	case BorderTypeHeavyQuadrupleDashed:
		return symbols.BorderHeavyQuadrupleDashed
	case BorderTypeQuadrantInside:
		return symbols.BorderQuadrantInside
	case BorderTypeQuadrantOutside:
		return symbols.BorderQuadrantOutside
	default:
		return symbols.BorderPlain
	}
}

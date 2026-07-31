package symbols

import "fmt"

const Dot = "•"

type MarkerKind int

const (
	MarkerDot MarkerKind = iota
	MarkerBlock
	MarkerBar
	MarkerBraille
	MarkerHalfBlock
	MarkerQuadrant
	MarkerSextant
	MarkerOctant
	MarkerCustom
)

type Marker struct {
	Kind MarkerKind
	Rune rune
}

func (m Marker) String() string {
	switch m.Kind {
	case MarkerDot:
		return "Dot"
	case MarkerBlock:
		return "Block"
	case MarkerBar:
		return "Bar"
	case MarkerBraille:
		return "Braille"
	case MarkerHalfBlock:
		return "HalfBlock"
	case MarkerQuadrant:
		return "Quadrant"
	case MarkerSextant:
		return "Sextant"
	case MarkerOctant:
		return "Octant"
	case MarkerCustom:
		return "Custom"
	default:
		return ""
	}
}

func ParseMarker(s string) (Marker, error) {
	switch s {
	case "Dot":
		return Marker{Kind: MarkerDot}, nil
	case "Block":
		return Marker{Kind: MarkerBlock}, nil
	case "Bar":
		return Marker{Kind: MarkerBar}, nil
	case "Braille":
		return Marker{Kind: MarkerBraille}, nil
	case "HalfBlock":
		return Marker{Kind: MarkerHalfBlock}, nil
	case "Quadrant":
		return Marker{Kind: MarkerQuadrant}, nil
	case "Sextant":
		return Marker{Kind: MarkerSextant}, nil
	case "Octant":
		return Marker{Kind: MarkerOctant}, nil
	default:
		return Marker{}, fmt.Errorf("invalid marker string: %q", s)
	}
}

func NewCustomMarker(r rune) Marker {
	return Marker{Kind: MarkerCustom, Rune: r}
}

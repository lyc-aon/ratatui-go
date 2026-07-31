package widgets

import "github.com/lyc-aon/ratatui-go/style"

// MapResolution selects coastline point density for Map.
type MapResolution int

const (
	// MapLow is ~1166 coastline points (default).
	MapLow MapResolution = iota
	// MapHigh is ~5125 coastline points; pair with braille markers.
	MapHigh
)

// String returns a stable name for the resolution.
func (r MapResolution) String() string {
	switch r {
	case MapHigh:
		return "High"
	default:
		return "Low"
	}
}

func (r MapResolution) data() [][2]float64 {
	switch r {
	case MapHigh:
		return WorldHighResolution[:]
	default:
		return WorldLowResolution[:]
	}
}

// Map draws a world coastline in EPSG:4326 coordinates.
type Map struct {
	Resolution MapResolution
	Color      style.Color
}

// NewMap builds a map shape with the given resolution and color.
func NewMap(resolution MapResolution, color style.Color) Map {
	return Map{Resolution: resolution, Color: color}
}

// Draw paints every coastline point that falls inside the painter bounds.
func (m Map) Draw(painter *Painter) {
	if painter == nil {
		return
	}
	for _, pt := range m.Resolution.data() {
		if x, y, ok := painter.GetPoint(pt[0], pt[1]); ok {
			painter.Paint(x, y, m.Color)
		}
	}
}

package layout

// Alignment is horizontal content alignment within a layout area.
type Alignment int

const (
	// AlignLeft aligns content to the left edge.
	AlignLeft Alignment = iota
	// AlignCenter centers content horizontally.
	AlignCenter
	// AlignRight aligns content to the right edge.
	AlignRight
)

// String returns a stable name for the alignment.
func (a Alignment) String() string {
	switch a {
	case AlignLeft:
		return "Left"
	case AlignCenter:
		return "Center"
	case AlignRight:
		return "Right"
	default:
		return "Left"
	}
}

// VerticalAlignment is vertical content alignment within a layout area.
type VerticalAlignment int

const (
	// AlignTop aligns content to the top edge.
	AlignTop VerticalAlignment = iota
	// AlignMiddle centers content vertically.
	AlignMiddle
	// AlignBottom aligns content to the bottom edge.
	AlignBottom
)

// String returns a stable name for the vertical alignment.
func (a VerticalAlignment) String() string {
	switch a {
	case AlignTop:
		return "Top"
	case AlignMiddle:
		return "Center"
	case AlignBottom:
		return "Bottom"
	default:
		return "Top"
	}
}

// Direction is the axis along which a Layout splits space.
type Direction int

const (
	// VerticalDir arranges segments top to bottom (default).
	VerticalDir Direction = iota
	// HorizontalDir arranges segments left to right.
	HorizontalDir
)

// String returns a stable name for the direction.
func (d Direction) String() string {
	switch d {
	case HorizontalDir:
		return "Horizontal"
	case VerticalDir:
		return "Vertical"
	default:
		return "Vertical"
	}
}

// Perpendicular returns the opposite axis.
func (d Direction) Perpendicular() Direction {
	if d == HorizontalDir {
		return VerticalDir
	}
	return HorizontalDir
}

// Flex controls how excess space is distributed once constraints are satisfied.
type Flex int

const (
	// FlexStart aligns segments to the start of the container (default).
	FlexStart Flex = iota
	// FlexLegacy puts excess space into the last segment (historical tui/ratatui default).
	FlexLegacy
	// FlexEnd aligns segments to the end of the container.
	FlexEnd
	// FlexCenter centers segments within the container.
	FlexCenter
	// FlexSpaceBetween puts excess space between segments (none before first / after last).
	FlexSpaceBetween
	// FlexSpaceAround puts excess space around each segment (half-size ends).
	FlexSpaceAround
	// FlexSpaceEvenly distributes excess space evenly including before first and after last.
	FlexSpaceEvenly
)

// String returns a stable name for the flex mode.
func (f Flex) String() string {
	switch f {
	case FlexLegacy:
		return "Legacy"
	case FlexStart:
		return "Start"
	case FlexEnd:
		return "End"
	case FlexCenter:
		return "Center"
	case FlexSpaceBetween:
		return "SpaceBetween"
	case FlexSpaceAround:
		return "SpaceAround"
	case FlexSpaceEvenly:
		return "SpaceEvenly"
	default:
		return "Start"
	}
}

// IsLegacy reports whether f is FlexLegacy.
func (f Flex) IsLegacy() bool { return f == FlexLegacy }

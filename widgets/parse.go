package widgets

import "fmt"

// ParseBorderType parses a BorderType from its canonical String name.
func ParseBorderType(s string) (BorderType, error) {
	switch s {
	case "Plain":
		return BorderTypePlain, nil
	case "Rounded":
		return BorderTypeRounded, nil
	case "Double":
		return BorderTypeDouble, nil
	case "Thick":
		return BorderTypeThick, nil
	case "LightDoubleDashed":
		return BorderTypeLightDoubleDashed, nil
	case "HeavyDoubleDashed":
		return BorderTypeHeavyDoubleDashed, nil
	case "LightTripleDashed":
		return BorderTypeLightTripleDashed, nil
	case "HeavyTripleDashed":
		return BorderTypeHeavyTripleDashed, nil
	case "LightQuadrupleDashed":
		return BorderTypeLightQuadrupleDashed, nil
	case "HeavyQuadrupleDashed":
		return BorderTypeHeavyQuadrupleDashed, nil
	case "QuadrantInside":
		return BorderTypeQuadrantInside, nil
	case "QuadrantOutside":
		return BorderTypeQuadrantOutside, nil
	default:
		return BorderTypePlain, fmt.Errorf("invalid BorderType string: %q", s)
	}
}

// ParseTitlePosition parses a TitlePosition from its canonical String name.
func ParseTitlePosition(s string) (TitlePosition, error) {
	switch s {
	case "Top":
		return TitlePositionTop, nil
	case "Bottom":
		return TitlePositionBottom, nil
	default:
		return TitlePositionTop, fmt.Errorf("invalid TitlePosition string: %q", s)
	}
}

// ParseScrollbarOrientation parses a ScrollbarOrientation from its canonical String name.
func ParseScrollbarOrientation(s string) (ScrollbarOrientation, error) {
	switch s {
	case "VerticalRight":
		return ScrollbarVerticalRight, nil
	case "VerticalLeft":
		return ScrollbarVerticalLeft, nil
	case "HorizontalBottom":
		return ScrollbarHorizontalBottom, nil
	case "HorizontalTop":
		return ScrollbarHorizontalTop, nil
	default:
		return ScrollbarVerticalRight, fmt.Errorf("invalid ScrollbarOrientation string: %q", s)
	}
}

// ParseScrollDirection parses a ScrollDirection from its canonical String name.
func ParseScrollDirection(s string) (ScrollDirection, error) {
	switch s {
	case "Forward":
		return ScrollForward, nil
	case "Backward":
		return ScrollBackward, nil
	default:
		return ScrollForward, fmt.Errorf("invalid ScrollDirection string: %q", s)
	}
}

// ParseRenderDirection parses a RenderDirection from its canonical String name.
func ParseRenderDirection(s string) (RenderDirection, error) {
	switch s {
	case "LeftToRight":
		return RenderLeftToRight, nil
	case "RightToLeft":
		return RenderRightToLeft, nil
	default:
		return RenderLeftToRight, fmt.Errorf("invalid RenderDirection string: %q", s)
	}
}

// ParseListDirection parses a ListDirection from its canonical String name.
func ParseListDirection(s string) (ListDirection, error) {
	switch s {
	case "TopToBottom":
		return ListTopToBottom, nil
	case "BottomToTop":
		return ListBottomToTop, nil
	default:
		return ListTopToBottom, fmt.Errorf("invalid ListDirection string: %q", s)
	}
}

// ParseGraphType parses a GraphType from its canonical String name.
func ParseGraphType(s string) (GraphType, error) {
	switch s {
	case "Scatter":
		return GraphScatter, nil
	case "Line":
		return GraphLine, nil
	case "Bar":
		return GraphBar, nil
	case "Area":
		return GraphArea, nil
	default:
		return GraphScatter, fmt.Errorf("invalid GraphType string: %q", s)
	}
}

// ParseMapResolution parses a MapResolution from its canonical String name.
func ParseMapResolution(s string) (MapResolution, error) {
	switch s {
	case "Low":
		return MapLow, nil
	case "High":
		return MapHigh, nil
	default:
		return MapLow, fmt.Errorf("invalid MapResolution string: %q", s)
	}
}

// ParseLegendPosition parses a LegendPosition from its canonical String name.
// LegendPosition has no EnumString upstream, but String names are stable and
// useful for config round-trips.
func ParseLegendPosition(s string) (LegendPosition, error) {
	switch s {
	case "Top":
		return LegendTop, nil
	case "TopRight":
		return LegendTopRight, nil
	case "TopLeft":
		return LegendTopLeft, nil
	case "Left":
		return LegendLeft, nil
	case "Right":
		return LegendRight, nil
	case "Bottom":
		return LegendBottom, nil
	case "BottomRight":
		return LegendBottomRight, nil
	case "BottomLeft":
		return LegendBottomLeft, nil
	default:
		return LegendTopRight, fmt.Errorf("invalid LegendPosition string: %q", s)
	}
}

// ParseHighlightSpacing parses a HighlightSpacing from its canonical String name.
func ParseHighlightSpacing(s string) (HighlightSpacing, error) {
	switch s {
	case "Always":
		return HighlightAlways, nil
	case "WhenSelected":
		return HighlightWhenSelected, nil
	case "Never":
		return HighlightNever, nil
	default:
		return HighlightWhenSelected, fmt.Errorf("invalid HighlightSpacing string: %q", s)
	}
}

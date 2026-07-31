package widgets

// HighlightSpacing controls when the selection-symbol column is reserved.
//
// Shared by List and Table. Zero value is HighlightWhenSelected (Rust default).
type HighlightSpacing int

const (
	// HighlightWhenSelected reserves the selection column only when a row is selected.
	// This is the zero value / default.
	HighlightWhenSelected HighlightSpacing = iota
	// HighlightAlways always reserves the selection column width.
	HighlightAlways
	// HighlightNever never reserves the selection column (symbol never drawn).
	HighlightNever
)

// String returns a stable name for the spacing mode.
func (h HighlightSpacing) String() string {
	switch h {
	case HighlightAlways:
		return "Always"
	case HighlightWhenSelected:
		return "WhenSelected"
	case HighlightNever:
		return "Never"
	default:
		return "WhenSelected"
	}
}

// ShouldAdd reports whether the selection column should be allocated.
//
// hasSelection is true when a row is currently selected.
func (h HighlightSpacing) ShouldAdd(hasSelection bool) bool {
	switch h {
	case HighlightAlways:
		return true
	case HighlightWhenSelected:
		return hasSelection
	case HighlightNever:
		return false
	default:
		return hasSelection
	}
}

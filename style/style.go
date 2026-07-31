package style

// Style describes an incremental style change applied to terminal cells.
//
// Color presence is tracked by HasFG/HasBG/HasUnderlineColor:
//   - false: unset (leave the previous value alone when patched)
//   - true with Color Reset: explicitly reset that color
//
// Zero-value Style is safe and means "change nothing".
type Style struct {
	FG             Color
	BG             Color
	UnderlineColor Color

	HasFG             bool
	HasBG             bool
	HasUnderlineColor bool

	AddModifier Modifier
	SubModifier Modifier
}

// New returns a Style that changes nothing.
func New() Style {
	return Style{}
}

// ResetStyle returns a Style that clears colors and removes every modifier.
//
// Distinct from New()/zero value: ResetStyle forces colors to Reset and
// subtracts ModAll.
//
// Named ResetStyle (not Reset) because Reset is the Color package var.
func ResetStyle() Style {
	return Style{
		FG:                Reset,
		BG:                Reset,
		UnderlineColor:    Reset,
		HasFG:             true,
		HasBG:             true,
		HasUnderlineColor: true,
		AddModifier:       0,
		SubModifier:       ModAll,
	}
}

// Foreground returns the foreground color and whether it is set.
func (s Style) Foreground() (Color, bool) {
	return s.FG, s.HasFG
}

// Background returns the background color and whether it is set.
func (s Style) Background() (Color, bool) {
	return s.BG, s.HasBG
}

// Underline returns the underline color and whether it is set.
func (s Style) Underline() (Color, bool) {
	return s.UnderlineColor, s.HasUnderlineColor
}

// Modifiers returns the effective modifiers this style adds after removals.
// Equivalent to AddModifier with SubModifier bits cleared.
func (s Style) Modifiers() Modifier {
	return s.AddModifier.Difference(s.SubModifier)
}

// WithFG sets the foreground color (value builder; does not mutate receiver).
func (s Style) WithFG(c Color) Style {
	s.FG = c
	s.HasFG = true
	return s
}

// WithBG sets the background color.
func (s Style) WithBG(c Color) Style {
	s.BG = c
	s.HasBG = true
	return s
}

// WithUnderlineColor sets the underline color.
func (s Style) WithUnderlineColor(c Color) Style {
	s.UnderlineColor = c
	s.HasUnderlineColor = true
	return s
}

// WithAddModifier adds emphasis flags. Also clears those flags from SubModifier.
func (s Style) WithAddModifier(m Modifier) Style {
	s.SubModifier = s.SubModifier.Difference(m)
	s.AddModifier = s.AddModifier.Union(m)
	return s
}

// WithRemoveModifier removes emphasis flags. Also clears those flags from AddModifier.
func (s Style) WithRemoveModifier(m Modifier) Style {
	s.AddModifier = s.AddModifier.Difference(m)
	s.SubModifier = s.SubModifier.Union(m)
	return s
}

// HasModifier reports whether the style effectively enables every flag in m
// (add contains all bits of m, and sub does not contain all bits of m).
func (s Style) HasModifier(m Modifier) bool {
	return s.AddModifier.Contains(m) && !s.SubModifier.Contains(m)
}

// Patch merges other onto s, matching Ratatui Style::patch semantics.
//
// Color fields: other's present value wins; unset leaves s unchanged.
// Modifiers:
//
//	add = (s.add - other.sub) | other.add
//	sub = (s.sub - other.add) | other.sub
func (s Style) Patch(other Style) Style {
	if other.HasFG {
		s.FG = other.FG
		s.HasFG = true
	}
	if other.HasBG {
		s.BG = other.BG
		s.HasBG = true
	}
	if other.HasUnderlineColor {
		s.UnderlineColor = other.UnderlineColor
		s.HasUnderlineColor = true
	}

	s.AddModifier = s.AddModifier.Remove(other.SubModifier).Insert(other.AddModifier)
	s.SubModifier = s.SubModifier.Remove(other.AddModifier).Insert(other.SubModifier)
	return s
}

// ApplyModifiers returns the modifier bitset after applying this style's
// add/sub modifiers to base.
//
// Order matches Ratatui / Cell.SetStyle: insert AddModifier, then remove SubModifier.
func (s Style) ApplyModifiers(base Modifier) Modifier {
	return base.Insert(s.AddModifier).Remove(s.SubModifier)
}

// FGOr returns the foreground if set, otherwise fallback.
func (s Style) FGOr(fallback Color) Color {
	if s.HasFG {
		return s.FG
	}
	return fallback
}

// BGOr returns the background if set, otherwise fallback.
func (s Style) BGOr(fallback Color) Color {
	if s.HasBG {
		return s.BG
	}
	return fallback
}

// UnderlineColorOr returns the underline color if set, otherwise fallback.
func (s Style) UnderlineColorOr(fallback Color) Color {
	if s.HasUnderlineColor {
		return s.UnderlineColor
	}
	return fallback
}

// FromColor builds a Style with only the foreground set.
func FromColor(c Color) Style {
	return New().WithFG(c)
}

// FromColors builds a Style with foreground and background set.
func FromColors(fg, bg Color) Style {
	return New().WithFG(fg).WithBG(bg)
}

// FromModifier builds a Style that adds the given modifiers.
func FromModifier(m Modifier) Style {
	return New().WithAddModifier(m)
}

// FromModifiers builds a Style that adds and removes the given modifiers.
func FromModifiers(add, sub Modifier) Style {
	return New().WithAddModifier(add).WithRemoveModifier(sub)
}

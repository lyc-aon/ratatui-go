package style

import "strings"

// Modifier is a bitset of text emphasis flags.
type Modifier uint16

// Modifier flags matching Ratatui's bit layout.
const (
	ModBold Modifier = 1 << iota
	ModDim
	ModItalic
	ModUnderlined
	ModSlowBlink
	ModRapidBlink
	ModReversed
	ModHidden
	ModCrossedOut
)

// ModAll is the set of every known modifier flag.
const ModAll = ModBold | ModDim | ModItalic | ModUnderlined |
	ModSlowBlink | ModRapidBlink | ModReversed | ModHidden | ModCrossedOut

// Contains reports whether m includes every flag in other.
func (m Modifier) Contains(other Modifier) bool {
	return m&other == other
}

// Intersects reports whether m shares any flag with other.
func (m Modifier) Intersects(other Modifier) bool {
	return m&other != 0
}

// Union returns the bitwise OR of m and other.
func (m Modifier) Union(other Modifier) Modifier {
	return m | other
}

// Difference returns flags in m that are not in other.
func (m Modifier) Difference(other Modifier) Modifier {
	return m &^ other
}

// Insert returns m with other flags set.
func (m Modifier) Insert(other Modifier) Modifier {
	return m | other
}

// Remove returns m with other flags cleared.
func (m Modifier) Remove(other Modifier) Modifier {
	return m &^ other
}

// IsEmpty reports whether no flags are set.
func (m Modifier) IsEmpty() bool {
	return m == 0
}

// String renders flags as "NONE" or a pipe-separated list.
func (m Modifier) String() string {
	if m == 0 {
		return "NONE"
	}
	var parts []string
	for _, item := range []struct {
		flag Modifier
		name string
	}{
		{ModBold, "BOLD"},
		{ModDim, "DIM"},
		{ModItalic, "ITALIC"},
		{ModUnderlined, "UNDERLINED"},
		{ModSlowBlink, "SLOW_BLINK"},
		{ModRapidBlink, "RAPID_BLINK"},
		{ModReversed, "REVERSED"},
		{ModHidden, "HIDDEN"},
		{ModCrossedOut, "CROSSED_OUT"},
	} {
		if m&item.flag != 0 {
			parts = append(parts, item.name)
		}
	}
	if unknown := m &^ ModAll; unknown != 0 {
		parts = append(parts, "UNKNOWN")
	}
	return strings.Join(parts, " | ")
}

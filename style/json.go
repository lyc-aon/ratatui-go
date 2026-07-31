package style

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// MarshalText serializes a color using its stable Ratatui display form.
func (c Color) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

// UnmarshalText parses named, indexed, and #RRGGBB colors.
func (c *Color) UnmarshalText(data []byte) error {
	parsed, err := ParseColor(string(data))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// MarshalJSON serializes colors as strings, matching Ratatui's serde feature.
func (c Color) MarshalJSON() ([]byte, error) { return json.Marshal(c.String()) }

// UnmarshalJSON accepts the current string representation and Ratatui's legacy
// {"Rgb":[r,g,b]} / {"Indexed":n} representations.
func (c *Color) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		return c.UnmarshalText([]byte(value))
	}
	var legacy map[string]json.RawMessage
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("style: invalid color JSON: %w", err)
	}
	if raw, ok := legacy["Rgb"]; ok && len(legacy) == 1 {
		var channels [3]uint8
		if err := json.Unmarshal(raw, &channels); err != nil {
			return fmt.Errorf("style: invalid legacy RGB color: %w", err)
		}
		*c = RGB(channels[0], channels[1], channels[2])
		return nil
	}
	if raw, ok := legacy["Indexed"]; ok && len(legacy) == 1 {
		var index uint8
		if err := json.Unmarshal(raw, &index); err != nil {
			return fmt.Errorf("style: invalid legacy indexed color: %w", err)
		}
		*c = Indexed(index)
		return nil
	}
	return fmt.Errorf("style: invalid color JSON %s", data)
}

// ParseModifier parses NONE or a pipe-separated set of modifier names.
func ParseModifier(value string) (Modifier, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "NONE" {
		return 0, nil
	}
	var result Modifier
	for _, part := range strings.Split(value, "|") {
		name := strings.TrimSpace(part)
		var flag Modifier
		switch name {
		case "BOLD":
			flag = ModBold
		case "DIM":
			flag = ModDim
		case "ITALIC":
			flag = ModItalic
		case "UNDERLINED":
			flag = ModUnderlined
		case "SLOW_BLINK":
			flag = ModSlowBlink
		case "RAPID_BLINK":
			flag = ModRapidBlink
		case "REVERSED":
			flag = ModReversed
		case "HIDDEN":
			flag = ModHidden
		case "CROSSED_OUT":
			flag = ModCrossedOut
		default:
			return 0, fmt.Errorf("style: invalid modifier %q", name)
		}
		result |= flag
	}
	return result, nil
}

// MarshalText serializes modifier flags by stable names.
func (m Modifier) MarshalText() ([]byte, error) { return []byte(m.String()), nil }

// UnmarshalText parses modifier names.
func (m *Modifier) UnmarshalText(data []byte) error {
	parsed, err := ParseModifier(string(data))
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// MarshalJSON serializes modifiers as a pipe-separated string.
func (m Modifier) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

// UnmarshalJSON accepts a modifier string or null (treated as empty).
func (m *Modifier) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*m = 0
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("style: invalid modifier JSON: %w", err)
	}
	return m.UnmarshalText([]byte(value))
}

type styleJSON struct {
	FG             *Color   `json:"fg,omitempty"`
	BG             *Color   `json:"bg,omitempty"`
	UnderlineColor *Color   `json:"underline_color,omitempty"`
	AddModifier    Modifier `json:"add_modifier,omitempty"`
	SubModifier    Modifier `json:"sub_modifier,omitempty"`
}

// MarshalJSON emits only properties explicitly set on the incremental style.
func (s Style) MarshalJSON() ([]byte, error) {
	wire := styleJSON{AddModifier: s.AddModifier, SubModifier: s.SubModifier}
	if s.HasFG {
		wire.FG = &s.FG
	}
	if s.HasBG {
		wire.BG = &s.BG
	}
	if s.HasUnderlineColor {
		wire.UnderlineColor = &s.UnderlineColor
	}
	return json.Marshal(wire)
}

// UnmarshalJSON restores color presence and defaults absent/null modifiers to empty.
func (s *Style) UnmarshalJSON(data []byte) error {
	var wire styleJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*s = Style{AddModifier: wire.AddModifier, SubModifier: wire.SubModifier}
	if wire.FG != nil {
		s.FG, s.HasFG = *wire.FG, true
	}
	if wire.BG != nil {
		s.BG, s.HasBG = *wire.BG, true
	}
	if wire.UnderlineColor != nil {
		s.UnderlineColor, s.HasUnderlineColor = *wire.UnderlineColor, true
	}
	return nil
}

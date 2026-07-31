package widgets

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
)

// CellEffect modifies cells covered by a Shadow after the shadow style is applied.
type CellEffect interface {
	Apply(shadowArea, baseArea layout.Rect, buf *buffer.Buffer)
}

// Shadow is drawn behind a Block in an offset area.
//
// Style is applied first to cells outside the base area; then the effect runs.
type Shadow struct {
	effect cellEffectKind
	custom CellEffect
	symbol string
	style  style.Style
	offset layout.Offset
}

type cellEffectKind int

const (
	effectOverlay cellEffectKind = iota
	effectSymbol
	effectCustom
)

// ShadowOverlay creates a shadow that only applies style (keeps existing symbols).
func ShadowOverlay() Shadow {
	return Shadow{
		effect: effectOverlay,
		offset: layout.NewOffset(1, 1),
	}
}

// ShadowBlock fills the shadow area with full-block symbols.
func ShadowBlock() Shadow {
	return ShadowSymbol(symbols.ShadeFull)
}

// ShadowLightShade fills with light shade symbols.
func ShadowLightShade() Shadow {
	return ShadowSymbol(symbols.ShadeLight)
}

// ShadowMediumShade fills with medium shade symbols.
func ShadowMediumShade() Shadow {
	return ShadowSymbol(symbols.ShadeMedium)
}

// ShadowDarkShade fills with dark shade symbols.
func ShadowDarkShade() Shadow {
	return ShadowSymbol(symbols.ShadeDark)
}

// ShadowSymbol fills the shadow area with the given symbol.
func ShadowSymbol(symbol string) Shadow {
	return Shadow{
		effect: effectSymbol,
		symbol: symbol,
		offset: layout.NewOffset(1, 1),
	}
}

// ShadowCustom creates a shadow from a custom CellEffect.
func ShadowCustom(effect CellEffect) Shadow {
	return Shadow{
		effect: effectCustom,
		custom: effect,
		offset: layout.NewOffset(1, 1),
	}
}

// NewShadow is an alias for ShadowCustom.
func NewShadow(effect CellEffect) Shadow {
	return ShadowCustom(effect)
}

// Style sets the style applied to the shadow area.
func (s Shadow) Style(st style.Style) Shadow {
	s.style = st
	return s
}

// Offset sets the shadow offset relative to the block area.
// Positive X moves right; positive Y moves down.
func (s Shadow) Offset(o layout.Offset) Shadow {
	s.offset = o
	return s
}

// Render draws the shadow for baseArea into buf.
func (s Shadow) Render(baseArea layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	shadowArea := baseArea.Offset(s.offset).Intersection(buf.Area)
	if shadowArea.IsEmpty() {
		return
	}

	for y := shadowArea.Y; y < shadowArea.Bottom(); y++ {
		for x := shadowArea.X; x < shadowArea.Right(); x++ {
			if baseArea.Contains(layout.Position{X: x, Y: y}) {
				continue
			}
			if cell := buf.GetMut(x, y); cell != nil {
				cell.SetStyle(s.style)
			}
		}
	}

	s.applyEffect(shadowArea, baseArea, buf)
}

func (s Shadow) applyEffect(shadowArea, baseArea layout.Rect, buf *buffer.Buffer) {
	switch s.effect {
	case effectOverlay:
		// style only
	case effectSymbol:
		forEachShadowCell(shadowArea, baseArea, buf, func(cell *buffer.Cell) {
			cell.SetSymbol(s.symbol)
		})
	case effectCustom:
		if s.custom != nil {
			s.custom.Apply(shadowArea, baseArea, buf)
		}
	}
}

// Dimmed dims shadow cells: sets ModDim; halves RGB backgrounds or forces Black.
type Dimmed struct{}

// DimmedEffect returns a Dimmed CellEffect value.
func DimmedEffect() Dimmed {
	return Dimmed{}
}

// Apply implements CellEffect.
func (Dimmed) Apply(shadowArea, baseArea layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	forEachShadowCell(shadowArea, baseArea, buf, func(cell *buffer.Cell) {
		bg := style.Black
		if r, g, b, ok := cell.Style.BG.RGB(); ok {
			bg = style.RGB(r/2, g/2, b/2)
		}
		cell.SetStyle(style.New().WithAddModifier(style.ModDim).WithBG(bg))
	})
}

func forEachShadowCell(shadowArea, baseArea layout.Rect, buf *buffer.Buffer, f func(*buffer.Cell)) {
	if buf == nil || f == nil {
		return
	}
	for y := shadowArea.Y; y < shadowArea.Bottom(); y++ {
		for x := shadowArea.X; x < shadowArea.Right(); x++ {
			if baseArea.Contains(layout.Position{X: x, Y: y}) {
				continue
			}
			if cell := buf.GetMut(x, y); cell != nil {
				f(cell)
			}
		}
	}
}

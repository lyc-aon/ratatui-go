package view

import "github.com/lyc-aon/ratatui-go/ompui/component"

// widget is the shared plumbing behind every chrome component in this package:
// a bound renderer plus a per-width row cache. Components build their rows on
// every Render and hand them to the cache, which keeps the slice reference and
// frame generation stable while the content is unchanged. Building unconditionally
// costs a few string joins and removes a whole class of staleness bug.
type widget struct {
	r     Renderer
	cache frameCache
}

func newWidget(theme Theme, opts Options) widget {
	return widget{r: NewRenderer(theme, opts)}
}

// SetTheme rebinds the theme and drops cached rows.
func (w *widget) SetTheme(theme Theme) {
	w.r = NewRenderer(theme, w.r.opts)
	w.cache.invalidate()
}

// SetOptions rebinds render options and drops cached rows.
func (w *widget) SetOptions(opts Options) {
	w.r = NewRenderer(w.r.theme, opts)
	w.cache.invalidate()
}

// SetIgnoreTight implements component.TightLayoutAware. The flag is inverted at
// the boundary: hosts propagate "ignore tight", options carry "tight".
func (w *widget) SetIgnoreTight(ignore bool) {
	opts := w.r.opts
	opts.Tight = !ignore
	w.SetOptions(opts)
}

// Invalidate implements component.Invalidator.
func (w *widget) Invalidate() { w.cache.invalidate() }

// Theme returns the bound theme.
func (w *widget) Theme() Theme { return w.r.theme }

// Options returns the bound options.
func (w *widget) Options() Options { return w.r.opts }

// frame stores rows and returns the resulting component frame.
func (w *widget) frame(width int, lines []string) component.Frame {
	if len(lines) == 0 {
		return component.EmptyFrame(w.cache.gen.Current())
	}
	return w.cache.store(width, lines)
}

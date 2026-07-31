// Package view renders OMP semantic state ([model.Snapshot]) into immutable
// line frames ([component.Frame]).
//
// Design contract
//
//   - Views are pure functions of (snapshot, width, theme, options). No global
//     state, no terminal writes, no goroutines, no timers. Anything that would
//     animate is driven by an explicit frame index the host advances; the
//     default frame source is the snapshot generation, so motion follows real
//     state changes and a still snapshot renders identical rows forever.
//   - Transcript prose is never padded to the render width. Rows carry only the
//     cells they need so a terminal selection copies clean text. Full-width
//     paint is opt-in through [Theme.UserBackground] and is used by chrome
//     rules only.
//   - Color arrives exclusively through injected [StyleFunc] values. The
//     package emits SGR attribute codes (bold/italic/underline) directly but
//     never hardcodes a color; [NewTheme] resolves a palette into truecolor,
//     256-color, or attribute-only styling.
//   - Every styled span closes with a scoped reset (39m/22m/23m/24m) so styles
//     cannot bleed across a row or into the renderer's own line terminator.
//
// Visual register: this is operational tooling, not a marketing surface. The
// transcript is a reading column — user turns sit behind a chevron gutter,
// assistant prose runs flush so it copies verbatim, reasoning hides behind a
// quiet vertical rule, and tool activity collapses into a single scannable
// header with a bounded body. Nothing here draws a dashboard.
package view

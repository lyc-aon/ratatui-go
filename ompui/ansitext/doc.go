// Package ansitext provides OMP-compatible ANSI/OSC-aware terminal text
// measurement, slicing, truncation, normalization, and SGR coalescing.
//
// Behavior matches the Oh My Pi TUI helpers (visibleWidth, sliceByColumn,
// truncateToWidth, normalizeTerminalOutput, coalesceAdjacentSgr) and the
// pi-natives text engine they wrap. Widths are terminal cells, not runes or
// bytes. SGR and OSC 8 sequences are zero-width; OSC 66 text-sizing spans
// contribute scale × (explicit w or payload width). Tabs expand to
// DefaultTabWidth cells. Grapheme clusters are never split.
//
// Depends only on github.com/michaelkelly/ratatui-go/text for grapheme cell
// widths. This package is a leaf in the ompui graph.
package ansitext

// Package media implements terminal rich-image support for the OMP Go frontend.
//
// It covers source-image dimension sniffing (PNG/JPEG/GIF/WebP), cell fit (via
// termcaps), Kitty graphics encoders (direct a=T, transmit a=t, placement a=p,
// delete d=I, 4096-base64 chunks), iTerm2 OSC 1337, a pure-Go Sixel encoder for
// decoded PNG/JPEG/GIF/WebP, Kitty Unicode placeholder grids (U+10EEEE + the 297
// row/column diacritic table with 24-bit image/placement IDs), ImageBudget pass
// state, and an Image component.Component with text fallback and height
// preservation.
//
// Imports: component, termcaps, the Go standard library, and the official
// golang.org/x/image WebP decoder. No terminal I/O, cgo, or external image
// binaries. Kitty/iTerm paths stream base64 as received; Sixel decodes then
// re-encodes with a bounded palette and size.
//
// Unsupported media formats (honest fallback / error, never panic): AVIF, HEIC,
// HEIF, BMP, TIFF, SVG, ICO, PDF, raw camera formats, and animated-frame
// selection beyond the first GIF/WebP frame. Static lossy and lossless WebP are
// supported for Sixel.
package media

// Package richtext is the OMP line-string rich-text core: GFM markdown,
// terminal LaTeX, syntax-highlighted code, Mermaid diagrams, plain Text, and
// TruncatedText.
//
// Output is []string terminal rows with ANSI/OSC embedded. It does not import
// ompui/component; adapters wrap these types. Parsing uses Goldmark with GFM
// and math extensions. Theme hooks provide syntax highlighting and Mermaid
// rendering; absent hooks degrade to fenced source rather than fake output.
//
// Depends on ompui/ansitext and ompui/latex for width-safe rendering. Theme
// values are caller-owned; this package holds no global mutable theme.
package richtext

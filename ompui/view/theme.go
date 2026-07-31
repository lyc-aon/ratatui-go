package view

import (
	"strconv"
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/highlight"
	"github.com/michaelkelly/ratatui-go/ompui/mermaid"
	"github.com/michaelkelly/ratatui-go/ompui/richtext"
)

// StyleFunc styles a fragment with ANSI and returns the result. A nil StyleFunc
// is identity everywhere in this package. Aliased to the richtext type so a
// Theme's hooks can be handed straight to the markdown renderer.
type StyleFunc = richtext.StyleFunc

// Appearance selects the palette tuned for the terminal's background.
type Appearance uint8

const (
	// AppearanceDark targets a dark terminal background (~#1e1e1e or darker).
	AppearanceDark Appearance = iota
	// AppearanceLight targets a light terminal background (~#ffffff).
	AppearanceLight
)

// ColorMode selects how palette entries are encoded.
type ColorMode uint8

const (
	// ColorNone emits no color at all: hierarchy is carried by SGR attributes
	// (bold/dim/italic/underline) only. Correct for NO_COLOR and dumb terminals.
	ColorNone ColorMode = iota
	// Color256 encodes each palette entry as the nearest xterm-256 index.
	Color256
	// ColorTrue encodes each palette entry as a 24-bit SGR sequence.
	ColorTrue
)

// Palette is the set of hex colors a Theme is built from. Every entry is
// "#rrggbb". Values are chosen so body-weight text clears 4.5:1 against the
// appearance's reference background and meta-weight text clears it too — there
// is no "elegant light gray" tier that stops being readable in daylight.
type Palette struct {
	Text     string
	Muted    string
	Dim      string
	Accent   string
	Success  string
	Error    string
	Warning  string
	Thinking string
	Code     string
	Border   string
	User     string
}

// DarkPalette is the default dark-terminal palette.
//
// Contrast against #1e1e1e: Text 11.4:1, Muted 6.4:1, Dim 5.0:1, Accent 7.9:1,
// Success 8.6:1, Error 7.0:1, Warning 8.7:1, Thinking 6.8:1.
func DarkPalette() Palette {
	return Palette{
		Text:     "#d4d8de",
		Muted:    "#9aa3ad",
		Dim:      "#868f9a",
		Accent:   "#7fb2ff",
		Success:  "#79c98a",
		Error:    "#ff8080",
		Warning:  "#e0b25a",
		Thinking: "#a99bd6",
		Code:     "#c8b6e2",
		Border:   "#4b535d",
		User:     "#e6e9ee",
	}
}

// LightPalette is the default light-terminal palette. Saturated mid-tones
// replace the dark theme's pastels; a pastel on white is the classic
// unreadable-contrast failure.
//
// Contrast against #ffffff: Text 15.3:1, Muted 8.5:1, Dim 5.6:1, Accent 5.9:1,
// Success 5.4:1, Error 6.1:1, Warning 6.4:1, Thinking 7.2:1.
func LightPalette() Palette {
	return Palette{
		Text:     "#1f2328",
		Muted:    "#4a5058",
		Dim:      "#626a73",
		Accent:   "#1a5fd0",
		Success:  "#1a7f37",
		Error:    "#b3261e",
		Warning:  "#8a5a00",
		Thinking: "#5b4b8a",
		Code:     "#6f42c1",
		Border:   "#b6bcc4",
		User:     "#101418",
	}
}

// Symbols is the glyph vocabulary. Two presets ship: [UnicodeSymbols] and
// [ASCIISymbols]. Emoji are deliberately absent — their cell width varies
// between terminals, and a dense status row whose width shifts per terminal
// cannot keep its columns aligned.
type Symbols struct {
	Success string
	Error   string
	Warning string
	Info    string
	Pending string
	Running string
	Aborted string
	Done    string

	// UserCursor marks an ordinary user turn, SteerCursor a mid-turn
	// interjection, SyntheticCursor a message the harness injected.
	UserCursor      string
	SteerCursor     string
	SyntheticCursor string

	TreeBranch   string
	TreeLast     string
	TreeVertical string

	// QuoteBar is the reasoning/quote gutter rule.
	QuoteBar string
	// Rule fills horizontal separators.
	Rule string

	Bullet       string
	Ellipsis     string
	BracketLeft  string
	BracketRight string
	Sep          string

	CheckDone      string
	CheckActive    string
	CheckPending   string
	CheckAbandoned string

	IconModel   string
	IconContext string
	IconAgents  string
	IconTokens  string
	IconCost    string
	IconGit     string
	IconFolder  string
	IconFile    string
	IconPackage string

	// Spinner frames the working indicator cycles through.
	Spinner []string
	// ThinkingPulse frames the hidden-reasoning indicator cycles through.
	ThinkingPulse []string
}

// UnicodeSymbols is the default preset: narrow, non-emoji glyphs.
func UnicodeSymbols() Symbols {
	return Symbols{
		Success: "✔", Error: "✘", Warning: "⚠", Info: "ⓘ",
		Pending: "◌", Running: "⟳", Aborted: "⏹", Done: "•",
		UserCursor: "❯", SteerCursor: "⇢", SyntheticCursor: "·",
		TreeBranch: "├─", TreeLast: "└─", TreeVertical: "│",
		QuoteBar: "▏", Rule: "─",
		Bullet: "•", Ellipsis: "…", BracketLeft: "⟨", BracketRight: "⟩", Sep: " · ",
		CheckDone: "☑", CheckActive: "◐", CheckPending: "☐", CheckAbandoned: "⦸",
		IconModel: "⬢", IconContext: "◫", IconAgents: "⧉", IconTokens: "∑",
		IconCost: "$", IconGit: "⑂", IconFolder: "▸", IconFile: "▫", IconPackage: "▤",
		Spinner:       []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		ThinkingPulse: []string{"✻", "✼", "❉", "❊", "✺", "✹", "✸", "✶"},
	}
}

// ASCIISymbols is the 7-bit preset for terminals and fonts that cannot be
// trusted with box drawing.
func ASCIISymbols() Symbols {
	return Symbols{
		Success: "ok", Error: "!!", Warning: "!", Info: "i",
		Pending: ".", Running: "~", Aborted: "x", Done: "*",
		UserCursor: ">", SteerCursor: "->", SyntheticCursor: "-",
		TreeBranch: "|-", TreeLast: "`-", TreeVertical: "|",
		QuoteBar: "|", Rule: "-",
		Bullet: "*", Ellipsis: "...", BracketLeft: "[", BracketRight: "]", Sep: " - ",
		CheckDone: "[x]", CheckActive: "[~]", CheckPending: "[ ]", CheckAbandoned: "[/]",
		IconModel: "M", IconContext: "ctx", IconAgents: "ag", IconTokens: "tok",
		IconCost: "$", IconGit: "git", IconFolder: "d", IconFile: "f", IconPackage: "p",
		Spinner:       []string{"|", "/", "-", "\\"},
		ThinkingPulse: []string{"*", "+", "x", "+"},
	}
}

// Theme is the complete style contract the views render through. Every field is
// injectable; the zero Theme renders unstyled but structurally correct rows.
type Theme struct {
	Text    StyleFunc
	Muted   StyleFunc
	Dim     StyleFunc
	Accent  StyleFunc
	Success StyleFunc
	Error   StyleFunc
	Warning StyleFunc

	// UserText styles an ordinary user turn's prose; SyntheticText styles a
	// harness-injected turn. The *Gutter hooks style the leading marker column.
	UserText        StyleFunc
	SyntheticText   StyleFunc
	UserGutter      StyleFunc
	SteerGutter     StyleFunc
	SyntheticGutter StyleFunc
	ThinkingText    StyleFunc
	ThinkingGutter  StyleFunc

	ToolTitle  StyleFunc
	ToolOutput StyleFunc

	Border      StyleFunc
	BorderMuted StyleFunc

	StatusModel    StyleFunc
	StatusPath     StyleFunc
	StatusContext  StyleFunc
	StatusGitClean StyleFunc
	StatusGitDirty StyleFunc
	StatusCost     StyleFunc

	Bold      StyleFunc
	Italic    StyleFunc
	Underline StyleFunc

	// UserBackground optionally paints user-message rows to the full render
	// width, receiving (line, width). Nil — the default — leaves rows unpadded
	// so a mouse selection copies the prose without trailing whitespace.
	UserBackground func(line string, width int) string

	Symbols Symbols

	// Markdown is handed to the richtext renderer for every prose block.
	Markdown richtext.Theme

	// Hyperlinks enables OSC 8 wrappers in markdown links.
	Hyperlinks bool
}

// ThemeOptions configures [NewTheme].
type ThemeOptions struct {
	Appearance Appearance
	ColorMode  ColorMode
	// Palette overrides the appearance default when non-nil.
	Palette *Palette
	// Symbols overrides [UnicodeSymbols] when non-nil.
	Symbols *Symbols
	// Hyperlinks enables OSC 8 in markdown links.
	Hyperlinks bool
	// TextSizing enables OSC 66 double-height H1 in markdown.
	TextSizing bool
	// HighlightCode overrides the built-in Chroma highlighter. Leave nil to get
	// syntax highlighting wired automatically from [ThemeOptions.SyntaxTheme]
	// and ColorMode; set [ThemeOptions.DisableHighlight] to render code plain.
	HighlightCode func(code, lang string) []string
	// SyntaxTheme names the Chroma style for fenced code. Empty picks a style
	// matched to Appearance.
	SyntaxTheme highlight.Theme
	// DisableHighlight renders fenced code in the plain code-block style.
	DisableHighlight bool
	// ResolveMermaidASCII overrides the built-in mermaid renderer.
	ResolveMermaidASCII func(source string, maxWidth int) (string, bool)
	// DisableMermaid renders ```mermaid fences as ordinary code blocks.
	DisableMermaid bool
}

// DefaultTheme is the dark, truecolor, Unicode theme — the constructor to reach
// for before terminal capabilities are known.
func DefaultTheme() Theme {
	return NewTheme(ThemeOptions{Appearance: AppearanceDark, ColorMode: ColorTrue})
}

// DarkTheme returns the dark palette at the given color mode.
func DarkTheme(mode ColorMode) Theme {
	return NewTheme(ThemeOptions{Appearance: AppearanceDark, ColorMode: mode})
}

// LightTheme returns the light palette at the given color mode.
func LightTheme(mode ColorMode) Theme {
	return NewTheme(ThemeOptions{Appearance: AppearanceLight, ColorMode: mode})
}

// MonoTheme returns an attribute-only theme (no color) with ASCII symbols.
// Correct for NO_COLOR, pipes, and terminals with an unknown background.
func MonoTheme() Theme {
	symbols := ASCIISymbols()
	return NewTheme(ThemeOptions{ColorMode: ColorNone, Symbols: &symbols})
}

// NewTheme resolves opts into a complete Theme.
func NewTheme(opts ThemeOptions) Theme {
	palette := DarkPalette()
	if opts.Appearance == AppearanceLight {
		palette = LightPalette()
	}
	if opts.Palette != nil {
		palette = *opts.Palette
	}
	symbols := UnicodeSymbols()
	if opts.Symbols != nil {
		symbols = *opts.Symbols
	}

	fg := func(hex string) StyleFunc { return colorStyle(opts.ColorMode, hex) }
	bold := attrStyle("\x1b[1m", "\x1b[22m")
	italic := attrStyle("\x1b[3m", "\x1b[23m")
	underline := attrStyle("\x1b[4m", "\x1b[24m")

	text := fg(palette.Text)
	muted := fg(palette.Muted)
	dim := fg(palette.Dim)
	accent := fg(palette.Accent)
	success := fg(palette.Success)
	errStyle := fg(palette.Error)
	warning := fg(palette.Warning)
	thinking := fg(palette.Thinking)
	code := fg(palette.Code)
	border := fg(palette.Border)
	user := fg(palette.User)

	if opts.ColorMode == ColorNone {
		// Without color the hierarchy comes from attributes: faint carries
		// de-emphasis, bold carries emphasis, everything else stays plain so
		// rows never become a smear of overlapping attributes.
		faint := attrStyle("\x1b[2m", "\x1b[22m")
		text = nil
		muted = faint
		dim = faint
		accent = bold
		success = nil
		errStyle = bold
		warning = bold
		thinking = italic
		code = nil
		border = faint
		user = bold
	}

	theme := Theme{
		Text:    text,
		Muted:   muted,
		Dim:     dim,
		Accent:  accent,
		Success: success,
		Error:   errStyle,
		Warning: warning,

		UserText:        user,
		SyntheticText:   dim,
		UserGutter:      accent,
		SteerGutter:     warning,
		SyntheticGutter: dim,
		ThinkingText:    thinking,
		ThinkingGutter:  dim,

		ToolTitle:  accent,
		ToolOutput: muted,

		Border:      border,
		BorderMuted: dim,

		StatusModel:    muted,
		StatusPath:     dim,
		StatusContext:  muted,
		StatusGitClean: success,
		StatusGitDirty: warning,
		StatusCost:     muted,

		Bold:      bold,
		Italic:    italic,
		Underline: underline,

		Symbols:    symbols,
		Hyperlinks: opts.Hyperlinks,
	}

	strike := attrStyle("\x1b[9m", "\x1b[29m")
	theme.Markdown = richtext.Theme{
		Heading1:        compose(bold, accent),
		Heading2:        accent,
		Heading3:        compose(bold, text),
		Bold:            bold,
		Italic:          italic,
		Strikethrough:   strike,
		Code:            code,
		CodeBlock:       text,
		CodeBlockBorder: border,
		Quote:           muted,
		QuoteBorder:     dim,
		HR:              border,
		ListBullet:      accent,
		Link:            compose(underline, accent),
		LinkURL:         dim,
		TableHeader:     bold,
		HighlightCode:   resolveHighlighter(opts),
		Hyperlinks:      opts.Hyperlinks,
		TextSizing:      opts.TextSizing,
		QuoteBorderChar: symbols.QuoteBar,
		HRChar:          symbols.Rule,
	}
	if symbols.Rule == "-" {
		// ASCII preset: keep tables and swatches inside 7-bit too.
		theme.Markdown.Table = richtext.TableSymbols{
			TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
			Horizontal: "-", Vertical: "|",
			TeeDown: "+", TeeUp: "+", TeeLeft: "+", TeeRight: "+", Cross: "+",
		}
	} else {
		theme.Markdown.ColorSwatch = "■"
	}
	theme.Markdown.ResolveMermaidASCII = resolveMermaid(opts, symbols, border)
	return theme
}

// resolveHighlighter wires Chroma syntax highlighting for fenced code. A
// capable terminal never renders code as an undifferentiated gray block.
func resolveHighlighter(opts ThemeOptions) func(code, lang string) []string {
	if opts.HighlightCode != nil {
		return opts.HighlightCode
	}
	if opts.DisableHighlight || opts.ColorMode == ColorNone {
		return nil
	}
	style := opts.SyntaxTheme
	if style == "" {
		// Chroma styles are background-specific: a dark style's comment gray on
		// a white terminal is the exact low-contrast failure to avoid.
		if opts.Appearance == AppearanceLight {
			style = "github"
		} else {
			style = highlight.DefaultTheme
		}
	}
	mode := highlight.ColorANSI256
	if opts.ColorMode == ColorTrue {
		mode = highlight.ColorTrueColor
	}
	return highlight.New(highlight.Options{Theme: style, ColorMode: mode}).Func()
}

// resolveMermaid wires the mermaid→ASCII adapter. The finished diagram is
// painted per row in the border color: it is structure, not prose, and a
// whole-string wrap would leave interior rows unstyled and the last row
// carrying a stray reset.
func resolveMermaid(opts ThemeOptions, symbols Symbols, border StyleFunc) func(string, int) (string, bool) {
	if opts.ResolveMermaidASCII != nil {
		return opts.ResolveMermaidASCII
	}
	if opts.DisableMermaid {
		return nil
	}
	mode := mermaid.ColorNone
	switch opts.ColorMode {
	case ColorTrue:
		mode = mermaid.ColorTrueColor
	case Color256:
		mode = mermaid.ColorANSI256
	}
	mopts := mermaid.Options{UseASCII: symbols.Rule == "-", ColorMode: mode}
	if border != nil && mode != mermaid.ColorNone {
		mopts.Colorize = func(plain string, _ mermaid.ColorMode) string {
			rows := strings.Split(plain, "\n")
			for i, row := range rows {
				rows[i] = apply(border, row)
			}
			return strings.Join(rows, "\n")
		}
	}
	renderer := mermaid.New(mopts)
	return renderer.ResolveMermaidASCII
}

// apply runs fn when non-nil. Empty input is never styled: wrapping "" in SGR
// yields a row that looks blank but carries escapes, which defeats the
// renderer's plain-blank detection.
func apply(fn StyleFunc, s string) string {
	if fn == nil || s == "" {
		return s
	}
	return fn(s)
}

func compose(outer, inner StyleFunc) StyleFunc {
	if outer == nil {
		return inner
	}
	if inner == nil {
		return outer
	}
	return func(s string) string { return outer(inner(s)) }
}

func attrStyle(open, close string) StyleFunc {
	return func(s string) string {
		if s == "" {
			return s
		}
		return open + s + close
	}
}

// colorStyle builds a foreground style for hex at the given mode. The span
// closes with SGR 39 (default foreground) rather than a full reset so an outer
// bold/italic survives a nested colored fragment.
func colorStyle(mode ColorMode, hex string) StyleFunc {
	if mode == ColorNone {
		return nil
	}
	r, g, b, ok := parseHexColor(hex)
	if !ok {
		return nil
	}
	var open string
	switch mode {
	case ColorTrue:
		open = "\x1b[38;2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b) + "m"
	default:
		open = "\x1b[38;5;" + strconv.Itoa(ansi256Index(r, g, b)) + "m"
	}
	return func(s string) string {
		if s == "" {
			return s
		}
		return open + s + "\x1b[39m"
	}
}

func parseHexColor(hex string) (r, g, b int, ok bool) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(value>>16) & 0xff, int(value>>8) & 0xff, int(value) & 0xff, true
}

// cubeLevels are the xterm 6x6x6 color-cube component values.
var cubeLevels = [6]int{0, 95, 135, 175, 215, 255}

func cubeIndex(v int) int {
	best, bestDelta := 0, 1<<30
	for i, level := range cubeLevels {
		delta := level - v
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			best, bestDelta = i, delta
		}
	}
	return best
}

// ansi256Index maps an RGB triple onto the nearest xterm-256 index, choosing
// between the color cube and the 24-step gray ramp by squared distance.
func ansi256Index(r, g, b int) int {
	ri, gi, bi := cubeIndex(r), cubeIndex(g), cubeIndex(b)
	cubeDist := squaredDistance(r, g, b, cubeLevels[ri], cubeLevels[gi], cubeLevels[bi])

	// Gray ramp 232..255 runs 8, 18, ... 238.
	gray := (r*299 + g*587 + b*114) / 1000
	step := (gray - 8 + 5) / 10
	if step < 0 {
		step = 0
	}
	if step > 23 {
		step = 23
	}
	grayValue := 8 + step*10
	if squaredDistance(r, g, b, grayValue, grayValue, grayValue) < cubeDist {
		return 232 + step
	}
	return 16 + 36*ri + 6*gi + bi
}

func squaredDistance(r1, g1, b1, r2, g2, b2 int) int {
	dr, dg, db := r1-r2, g1-g2, b1-b2
	return dr*dr + dg*dg + db*db
}

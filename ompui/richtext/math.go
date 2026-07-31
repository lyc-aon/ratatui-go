package richtext

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"

	"github.com/michaelkelly/ratatui-go/ompui/latex"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindMath is an inline math node produced by the math extension.
var KindMath = ast.NewNodeKind("Math")

// KindMathBlock is a block-level display math node.
var KindMathBlock = ast.NewNodeKind("MathBlock")

// Math is an inline LaTeX math fragment ($…$, \(…\), or single-line $$ / \[).
type Math struct {
	ast.BaseInline
	Latex   string
	Display bool
}

// Dump implements ast.Node.
func (n *Math) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Latex":   n.Latex,
		"Display": boolStr(n.Display),
	}, nil)
}

// Kind implements ast.Node.
func (n *Math) Kind() ast.NodeKind { return KindMath }

// MathBlock is a block-level display math node (own-line $$ / \[ / bare env).
type MathBlock struct {
	ast.BaseBlock
	Latex string
}

// Dump implements ast.Node.
func (n *MathBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Latex": n.Latex}, nil)
}

// Kind implements ast.Node.
func (n *MathBlock) Kind() ast.NodeKind { return KindMathBlock }

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func trimLineEOL(line []byte) []byte {
	return bytes.TrimRightFunc(line, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// ---------------------------------------------------------------------------
// Inline parser: $…$, $$…$$, \(…\), \[…\]
// ---------------------------------------------------------------------------

type mathInlineParser struct{}

func (p *mathInlineParser) Trigger() []byte { return []byte{'$', '\\'} }

func (p *mathInlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) == 0 {
		return nil
	}

	// $$…$$ on one line (multi-line form is a block).
	if len(line) >= 2 && line[0] == '$' && line[1] == '$' {
		rest := line[2:]
		closeRel := bytes.Index(rest, []byte("$$"))
		if closeRel < 0 {
			return nil
		}
		inner := rest[:closeRel]
		if len(bytes.TrimSpace(inner)) == 0 || bytes.IndexByte(inner, '\n') >= 0 {
			return nil
		}
		block.Advance(2 + closeRel + 2)
		return &Math{Latex: string(inner), Display: true}
	}

	// \[…\] or \(…\)
	if line[0] == '\\' && len(line) >= 2 {
		var closer string
		var display bool
		switch line[1] {
		case '[':
			closer = `\]`
			display = true
		case '(':
			closer = `\)`
			display = false
		default:
			return nil
		}
		rest := line[2:]
		closeRel := bytes.Index(rest, []byte(closer))
		if closeRel < 0 {
			return nil
		}
		if display && bytes.IndexByte(rest[:closeRel], '\n') >= 0 {
			return nil // multi-line \[ is a block
		}
		inner := string(rest[:closeRel])
		block.Advance(2 + closeRel + len(closer))
		return &Math{Latex: inner, Display: display}
	}

	// $…$ with pandoc anti-currency rules (span may not cross newline).
	if line[0] == '$' {
		s := string(line)
		end := latex.InlineMathSpanEnd(s, 0)
		if end < 0 {
			return nil
		}
		block.Advance(end + 1)
		return &Math{Latex: s[1:end], Display: false}
	}
	return nil
}

func (p *mathInlineParser) CloseBlock(parent ast.Node, pc parser.Context) {}

// ---------------------------------------------------------------------------
// Block parser: own-line $$…$$ and \[…\]  (Open/Continue like fenced code)
// ---------------------------------------------------------------------------

type mathFenceData struct {
	close []byte
	node  *MathBlock
	body  strings.Builder
}

var mathFenceKey = parser.NewContextKey()

type mathBlockParser struct{}

func (b *mathBlockParser) Trigger() []byte { return []byte{'$', '\\'} }

func (b *mathBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || pos >= len(line) {
		return nil, parser.NoChildren
	}
	open := trimLineEOL(line[pos:])
	var closeTok []byte
	switch {
	case bytes.Equal(open, []byte("$$")):
		closeTok = []byte("$$")
	case bytes.Equal(open, []byte(`\[`)):
		closeTok = []byte(`\]`)
	default:
		return nil, parser.NoChildren
	}
	node := &MathBlock{}
	pc.Set(mathFenceKey, &mathFenceData{close: closeTok, node: node})
	// Opener line is consumed by goldmark after Open returns Continue.
	return node, parser.NoChildren
}

func (b *mathBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	data, _ := pc.Get(mathFenceKey).(*mathFenceData)
	if data == nil || data.node != node {
		return parser.Close
	}
	line, segment := reader.PeekLine()
	if line == nil {
		return parser.Close
	}
	// Closing line: ≤3 leading spaces + exact close token.
	w, pos := util.IndentWidth(line, reader.LineOffset())
	if w < 4 && pos >= 0 {
		cand := trimLineEOL(line[pos:])
		if bytes.Equal(cand, data.close) {
			reader.AdvanceToEOL()
			return parser.Close
		}
	}
	// Body line.
	data.body.Write(segment.Value(reader.Source()))
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

func (b *mathBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	data, _ := pc.Get(mathFenceKey).(*mathFenceData)
	if data == nil || data.node != node {
		return
	}
	mb := node.(*MathBlock)
	mb.Latex = strings.TrimSpace(data.body.String())
	pc.Set(mathFenceKey, nil)
}

func (b *mathBlockParser) CanInterruptParagraph() bool { return true }
func (b *mathBlockParser) CanAcceptIndentedLine() bool { return false }

// ---------------------------------------------------------------------------
// Block parser: bare \begin{mathenv}…\end{…}
// ---------------------------------------------------------------------------

type mathEnvData struct {
	env  string
	end  []byte
	node *MathBlock
	body strings.Builder
}

var mathEnvKey = parser.NewContextKey()

var beginEnvRe = regexp.MustCompile(`^\\begin\{([A-Za-z]+\*?)\}`)

type mathEnvBlockParser struct{}

func (b *mathEnvBlockParser) Trigger() []byte { return []byte{'\\'} }

func (b *mathEnvBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || pos >= len(line) {
		return nil, parser.NoChildren
	}
	rest := trimLineEOL(line[pos:])
	m := beginEnvRe.FindSubmatch(rest)
	if m == nil {
		return nil, parser.NoChildren
	}
	env := string(m[1])
	if !latex.IsBareMathEnvironment(env) {
		return nil, parser.NoChildren
	}
	endTok := []byte(`\end{` + env + `}`)
	node := &MathBlock{}
	data := &mathEnvData{env: env, end: endTok, node: node}

	// Include the opening line content from the begin token.
	// segment covers the full line; write from pos.
	lineVal := segment.Value(reader.Source())
	if pos < len(lineVal) {
		data.body.Write(lineVal[pos:])
	} else {
		data.body.Write(rest)
		data.body.WriteByte('\n')
	}

	// Same-line end?
	if bytes.Contains(rest, endTok) {
		raw := strings.TrimSpace(data.body.String())
		raw = regexp.MustCompile(`\n[ \t]*$`).ReplaceAllString(raw, "")
		node.Latex = raw
		// Open returns the finished node; goldmark will still call Close.
		pc.Set(mathEnvKey, data)
		return node, parser.NoChildren
	}

	pc.Set(mathEnvKey, data)
	return node, parser.NoChildren
}

func (b *mathEnvBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	data, _ := pc.Get(mathEnvKey).(*mathEnvData)
	if data == nil || data.node != node {
		return parser.Close
	}
	// Already closed on open line (single-line env).
	if node.(*MathBlock).Latex != "" {
		return parser.Close
	}
	line, segment := reader.PeekLine()
	if line == nil {
		return parser.Close
	}
	// Blank line ends bare-env eligibility (OMP rejects blank lines inside).
	if util.IsBlank(line) {
		// Abort: leave Latex empty so Close can drop? Better: keep what we have and close.
		// OMP returns null (not a math block). We cannot unread; close with empty and
		// render path should fall back. Set a poison marker.
		data.body.Reset()
		reader.AdvanceToEOL()
		return parser.Close
	}
	val := segment.Value(reader.Source())
	data.body.Write(val)
	reader.AdvanceToEOL()
	if bytes.Contains(val, data.end) {
		return parser.Close
	}
	return parser.Continue | parser.NoChildren
}

func (b *mathEnvBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	data, _ := pc.Get(mathEnvKey).(*mathEnvData)
	if data == nil || data.node != node {
		return
	}
	mb := node.(*MathBlock)
	if mb.Latex == "" {
		raw := data.body.String()
		if raw != "" && bytes.Contains([]byte(raw), data.end) {
			raw = regexp.MustCompile(`\n[ \t]*$`).ReplaceAllString(raw, "")
			mb.Latex = strings.TrimRightFunc(raw, func(r rune) bool {
				return r == ' ' || r == '\t' || r == '\n' || r == '\r'
			})
			// OMP: text = raw.replace(/\n[ \t]*$/, "") — already
			mb.Latex = regexp.MustCompile(`\n[ \t]*$`).ReplaceAllString(mb.Latex, "")
		}
	}
	pc.Set(mathEnvKey, nil)
}

func (b *mathEnvBlockParser) CanInterruptParagraph() bool { return true }
func (b *mathEnvBlockParser) CanAcceptIndentedLine() bool { return false }

// ---------------------------------------------------------------------------
// Extension
// ---------------------------------------------------------------------------

type mathExtension struct{}

// MathExtension registers inline and block math parsers with Goldmark.
var MathExtension goldmark.Extender = &mathExtension{}

func (e *mathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(&mathBlockParser{}, 500),
			util.Prioritized(&mathEnvBlockParser{}, 499),
		),
		parser.WithInlineParsers(
			// After code span (100) so backticks win; before emphasis (500).
			util.Prioritized(&mathInlineParser{}, 150),
		),
	)
}

// soleDisplayMath returns the single display Math node when parent contains only
// that (plus empty/whitespace text), matching OMP's soleDisplayMath helper.
func soleDisplayMath(parent ast.Node, source []byte) *Math {
	var found *Math
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *Math:
			if !n.Display {
				return nil
			}
			if found != nil {
				return nil
			}
			found = n
		case *ast.Text:
			if strings.TrimSpace(string(n.Segment.Value(source))) != "" {
				return nil
			}
		case *ast.String:
			if strings.TrimSpace(string(n.Value)) != "" {
				return nil
			}
		default:
			return nil
		}
	}
	return found
}

func renderMathInline(src string) string {
	u := latex.ToUnicode(src)
	var b strings.Builder
	prevSpace := false
	for _, r := range u {
		if r == '\n' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = r == ' '
		b.WriteRune(r)
	}
	return b.String()
}

func renderMathDisplayLines(src string) []string {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	return latex.ToBlock(src)
}

func isSpaceOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

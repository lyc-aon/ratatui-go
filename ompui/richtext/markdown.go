package richtext

import (
	"bytes"
	"strings"
	"sync"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// parser is package-level Goldmark GFM instance. Safe for concurrent Parse.
var (
	mdOnce   sync.Once
	mdParser goldmark.Markdown
)

func mdEngine() goldmark.Markdown {
	mdOnce.Do(func() {
		mdParser = goldmark.New(
			goldmark.WithExtensions(extension.GFM, MathExtension),
		)
	})
	return mdParser
}

// Markdown renders GFM markdown to terminal line strings.
type Markdown struct {
	text        string
	opts        MarkdownOptions
	theme       Theme
	cachedText  string
	cachedWidth int
	cachedLines []string
	hasCache    bool
}

// NewMarkdown constructs a markdown renderer.
// CodeBlockIndent defaults to 2 when opts.CodeBlockIndent is 0.
// Pass a negative CodeBlockIndent to force zero indent.
func NewMarkdown(src string, theme Theme, opts MarkdownOptions) *Markdown {
	if opts.CodeBlockIndent == 0 {
		opts.CodeBlockIndent = 2
	}
	if opts.CodeBlockIndent < 0 {
		opts.CodeBlockIndent = 0
	}
	return &Markdown{
		text:  src,
		opts:  opts,
		theme: theme,
	}
}

// SetText replaces markdown source and invalidates caches.
func (m *Markdown) SetText(src string) {
	m.text = src
	m.Invalidate()
}

// Text returns the raw markdown source.
func (m *Markdown) Text() string { return m.text }

// SetTheme replaces the theme and invalidates caches.
func (m *Markdown) SetTheme(theme Theme) {
	m.theme = theme
	m.Invalidate()
}

// SetOptions replaces layout options and invalidates caches.
func (m *Markdown) SetOptions(opts MarkdownOptions) {
	if opts.CodeBlockIndent == 0 {
		opts.CodeBlockIndent = 2
	}
	if opts.CodeBlockIndent < 0 {
		opts.CodeBlockIndent = 0
	}
	m.opts = opts
	m.Invalidate()
}

// Invalidate drops the render cache.
func (m *Markdown) Invalidate() {
	m.hasCache = false
	m.cachedLines = nil
	m.cachedText = ""
	m.cachedWidth = 0
}

// Render produces terminal rows for the given width.
func (m *Markdown) Render(width int) []string {
	if m.hasCache && m.cachedText == m.text && m.cachedWidth == width {
		return m.cachedLines
	}

	padX := m.opts.PaddingX
	if padX < 0 {
		padX = 0
	}
	contentWidth := width - padX*2
	if contentWidth < 1 {
		contentWidth = 1
	}

	if strings.TrimSpace(m.text) == "" {
		out := []string{}
		m.storeCache(width, out)
		return out
	}

	normalized := replaceTabs(m.text)
	source := []byte(normalized)
	doc := mdEngine().Parser().Parse(text.NewReader(source))

	var rendered []string
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		nextType := ""
		if n.NextSibling() != nil {
			nextType = n.NextSibling().Kind().String()
		}
		rendered = append(rendered, m.renderBlock(n, source, contentWidth, nextType, nil)...)
	}

	var wrapped []string
	for _, line := range rendered {
		if isOsc66Line(line) {
			wrapped = append(wrapped, line)
			continue
		}
		wrapped = append(wrapped, WrapTextWithAnsi(line, contentWidth)...)
	}

	left := padding(padX)
	right := padding(padX)
	bg := m.opts.Background
	content := make([]string, 0, len(wrapped))
	prevOsc66 := false
	for _, line := range wrapped {
		if prevOsc66 && line == "" {
			content = append(content, "")
			prevOsc66 = false
			continue
		}
		if isOsc66Line(line) {
			content = append(content, line)
			prevOsc66 = true
			continue
		}
		prevOsc66 = false
		withMargins := left + line + right
		if bg != nil {
			content = append(content, applyBackgroundToLine(withMargins, width, bg))
		} else {
			content = append(content, padLineToWidth(withMargins, width))
		}
	}

	empty := padding(width)
	if bg != nil {
		empty = applyBackgroundToLine(empty, width, bg)
	}
	var vb []string
	for range m.opts.PaddingY {
		vb = append(vb, empty)
	}
	result := make([]string, 0, len(vb)*2+len(content))
	result = append(result, vb...)
	result = append(result, content...)
	result = append(result, vb...)
	if len(result) == 0 {
		result = []string{""}
	}
	m.storeCache(width, result)
	return result
}

func (m *Markdown) storeCache(width int, lines []string) {
	m.cachedText = m.text
	m.cachedWidth = width
	m.cachedLines = lines
	m.hasCache = true
}

type inlineStyleCtx struct {
	applyText   func(string) string
	stylePrefix string
}

func (m *Markdown) defaultInlineCtx() inlineStyleCtx {
	return inlineStyleCtx{
		applyText:   func(s string) string { return s },
		stylePrefix: "",
	}
}

func (m *Markdown) renderBlock(n ast.Node, source []byte, width int, nextType string, style *inlineStyleCtx) []string {
	ctx := m.defaultInlineCtx()
	if style != nil {
		ctx = *style
	}

	switch n.Kind() {
	case KindMathBlock:
		mb := n.(*MathBlock)
		if strings.TrimSpace(mb.Latex) == "" {
			return nil
		}
		var lines []string
		for _, ml := range renderMathDisplayLines(mb.Latex) {
			lines = append(lines, ctx.applyText(ml))
		}
		if nextType != "" && nextType != "Space" {
			lines = append(lines, "")
		}
		return lines
	case ast.KindHeading:
		return m.renderHeading(n.(*ast.Heading), source, width, nextType, ctx)
	case ast.KindParagraph:
		return m.renderParagraph(n, source, width, nextType, ctx)
	case ast.KindFencedCodeBlock:
		return m.renderFencedCode(n.(*ast.FencedCodeBlock), source, width, nextType)
	case ast.KindCodeBlock:
		return m.renderIndentedCode(n, source, nextType)
	case ast.KindList:
		return m.renderList(n.(*ast.List), source, 0, ctx)
	case east.KindTable:
		return m.renderTable(n, source, width, nextType, ctx)
	case ast.KindBlockquote:
		return m.renderBlockquote(n, source, width, nextType)
	case ast.KindThematicBreak:
		return m.renderHR(n, source, width, nextType)
	case ast.KindHTMLBlock:
		return m.renderHTMLBlock(n, source)
	case ast.KindTextBlock:
		if dm := soleDisplayMath(n, source); dm != nil {
			var lines []string
			for _, ml := range renderMathDisplayLines(dm.Latex) {
				lines = append(lines, ctx.applyText(ml))
			}
			return lines
		}
		t := m.renderInlines(n, source, ctx)
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		if n.HasChildren() {
			var lines []string
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				nt := ""
				if c.NextSibling() != nil {
					nt = c.NextSibling().Kind().String()
				}
				lines = append(lines, m.renderBlock(c, source, width, nt, style)...)
			}
			return lines
		}
		return nil
	}
}

func (m *Markdown) renderHeading(h *ast.Heading, source []byte, width int, nextType string, ctx inlineStyleCtx) []string {
	level := h.Level
	headingText := m.renderInlines(h, source, ctx)
	plain := plainInlines(h, source)
	prefix := strings.Repeat("#", level) + " "

	var lines []string
	if level == 1 && m.theme.TextSizing {
		pw := ansitext.VisibleWidth(plain)
		if pw > 0 && 2*pw <= width {
			sized := encodeTextSizedHeading(plain, 2)
			styled := m.theme.apply(m.theme.Heading1, m.theme.apply(m.theme.Bold, underline(sized)))
			lines = append(lines, styled, "")
			if nextType != "" {
				lines = append(lines, "")
			}
			return lines
		}
	}

	var styled string
	switch {
	case level == 1:
		styled = m.theme.apply(m.theme.Heading1, m.theme.apply(m.theme.Bold, underline(headingText)))
	case level == 2:
		styled = m.theme.apply(m.theme.Heading2, m.theme.apply(m.theme.Bold, headingText))
	default:
		body := prefix + headingText
		styled = m.theme.apply(m.theme.headingStyle(level), m.theme.apply(m.theme.Bold, body))
	}
	lines = append(lines, styled)
	if nextType != "" {
		lines = append(lines, "")
	}
	return lines
}
func (m *Markdown) renderParagraph(n ast.Node, source []byte, width int, nextType string, ctx inlineStyleCtx) []string {
	// Custom Unicode/equals HR lines that goldmark leaves as paragraphs.
	if ch, ok := customHRFill(plainInlines(n, source)); ok {
		return m.renderHRFill(ch, width, nextType)
	}
	// Sole display math ($$…$$ / \[…\] captured inline) stacks multi-line.
	if dm := soleDisplayMath(n, source); dm != nil {
		var lines []string
		for _, ml := range renderMathDisplayLines(dm.Latex) {
			lines = append(lines, ctx.applyText(ml))
		}
		if nextType != "" && nextType != "List" {
			lines = append(lines, "")
		}
		return lines
	}
	t := m.renderInlines(n, source, ctx)
	lines := []string{t}
	if nextType != "" && nextType != "List" {
		lines = append(lines, "")
	}
	return lines
}

// customHRFill reports when plain text is a pure HR run of the same character
// (at least 3), matching OMP's custom HR tokenizer for = ─ ━ ═ – — and -_*.
func customHRFill(plain string) (sourceChar rune, ok bool) {
	s := strings.TrimSpace(plain)
	if s == "" {
		return 0, false
	}
	// Allow spaces between markers like "- - -"
	var runes []rune
	for _, r := range s {
		if r == ' ' || r == '\t' {
			continue
		}
		runes = append(runes, r)
	}
	if len(runes) < 3 {
		return 0, false
	}
	first := runes[0]
	switch first {
	case '-', '*', '_', '=', '─', '━', '═', '–', '—':
	default:
		return 0, false
	}
	for _, r := range runes[1:] {
		if r != first {
			return 0, false
		}
	}
	return first, true
}

func (m *Markdown) renderHRFill(sourceChar rune, width int, nextType string) []string {
	fill := getHrFillChar(sourceChar, m.theme.hrChar())
	nFill := width
	if nFill > 80 {
		nFill = 80
	}
	if nFill < 1 {
		nFill = 1
	}
	line := m.theme.apply(m.theme.HR, strings.Repeat(fill, nFill))
	out := []string{line}
	if nextType != "" {
		out = append(out, "")
	}
	return out
}

func (m *Markdown) codeLines(n ast.Node, source []byte) (lang string, body string) {
	if fc, ok := n.(*ast.FencedCodeBlock); ok {
		if l := fc.Language(source); len(l) > 0 {
			lang = string(l)
		}
	}
	var b strings.Builder
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
	body = strings.TrimSuffix(b.String(), "\n")
	return lang, body
}

func (m *Markdown) renderFencedCode(n *ast.FencedCodeBlock, source []byte, width int, nextType string) []string {
	lang, body := m.codeLines(n, source)

	if strings.EqualFold(lang, "mermaid") && m.theme.ResolveMermaidASCII != nil {
		if ascii, ok := m.theme.ResolveMermaidASCII(body, width); ok && ascii != "" {
			var lines []string
			for _, al := range splitTerminalLines(ascii) {
				if ansitext.VisibleWidth(al) > width {
					lines = append(lines, ansitext.TruncateToWidth(al, width, ""))
				} else {
					lines = append(lines, al)
				}
			}
			if nextType != "" {
				lines = append(lines, "")
			}
			return lines
		}
	}

	indent := padding(m.opts.CodeBlockIndent)
	var lines []string
	lines = append(lines, m.theme.apply(m.theme.CodeBlockBorder, "```"+lang))
	if m.theme.HighlightCode != nil {
		for _, hl := range m.theme.HighlightCode(body, lang) {
			lines = append(lines, indent+hl)
		}
	} else {
		codeLines := splitTerminalLines(body)
		if len(codeLines) == 0 {
			codeLines = []string{""}
		}
		for _, cl := range codeLines {
			lines = append(lines, indent+m.theme.apply(m.theme.CodeBlock, cl))
		}
	}
	lines = append(lines, m.theme.apply(m.theme.CodeBlockBorder, "```"))
	if nextType != "" {
		lines = append(lines, "")
	}
	return lines
}

func (m *Markdown) renderIndentedCode(n ast.Node, source []byte, nextType string) []string {
	_, body := m.codeLines(n, source)
	indent := padding(m.opts.CodeBlockIndent)
	var lines []string
	lines = append(lines, m.theme.apply(m.theme.CodeBlockBorder, "```"))
	codeLines := splitTerminalLines(body)
	if len(codeLines) == 0 {
		codeLines = []string{""}
	}
	for _, cl := range codeLines {
		lines = append(lines, indent+m.theme.apply(m.theme.CodeBlock, cl))
	}
	lines = append(lines, m.theme.apply(m.theme.CodeBlockBorder, "```"))
	if nextType != "" {
		lines = append(lines, "")
	}
	return lines
}

func (m *Markdown) renderBlockquote(n ast.Node, source []byte, width int, nextType string) []string {
	quoteStyle := func(s string) string {
		return m.theme.apply(m.theme.Quote, m.theme.apply(m.theme.Italic, s))
	}
	prefix := stylePrefixOf(quoteStyle)
	applyQuote := func(line string) string {
		if prefix == "" {
			return quoteStyle(line)
		}
		reopened := strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+prefix)
		return quoteStyle(reopened)
	}

	innerCtx := inlineStyleCtx{
		applyText:   func(s string) string { return s },
		stylePrefix: "",
	}
	quoteWidth := width - 2
	if quoteWidth < 1 {
		quoteWidth = 1
	}

	var rendered []string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		nt := ""
		if c.NextSibling() != nil {
			nt = c.NextSibling().Kind().String()
		}
		rendered = append(rendered, m.renderBlock(c, source, quoteWidth, nt, &innerCtx)...)
	}
	for len(rendered) > 0 && rendered[len(rendered)-1] == "" {
		rendered = rendered[:len(rendered)-1]
	}

	border := m.theme.quoteBorderChar() + " "
	var lines []string
	for _, ql := range rendered {
		styled := applyQuote(ql)
		for _, wl := range WrapTextWithAnsi(styled, quoteWidth) {
			lines = append(lines, m.theme.apply(m.theme.QuoteBorder, border)+wl)
		}
	}
	if nextType != "" {
		lines = append(lines, "")
	}
	return lines
}

func stylePrefixOf(fn func(string) string) string {
	const sentinel = "\x00"
	styled := fn(sentinel)
	i := strings.Index(styled, sentinel)
	if i <= 0 {
		return ""
	}
	return styled[:i]
}

func (m *Markdown) renderHR(n ast.Node, source []byte, width int, nextType string) []string {
	char := rune(0)
	if lines := n.Lines(); lines != nil && lines.Len() > 0 {
		seg := lines.At(0)
		raw := strings.TrimSpace(string(seg.Value(source)))
		if raw != "" {
			for _, r := range raw {
				char = r
				break
			}
		}
	}
	if char == 0 {
		pos := n.Pos()
		if pos >= 0 && pos < len(source) {
			start := pos
			for start > 0 && source[start-1] != '\n' {
				start--
			}
			end := pos
			for end < len(source) && source[end] != '\n' {
				end++
			}
			raw := strings.TrimSpace(string(source[start:end]))
			if raw != "" {
				for _, r := range raw {
					char = r
					break
				}
			}
		}
	}
	fill := getHrFillChar(char, m.theme.hrChar())
	nFill := width
	if nFill > 80 {
		nFill = 80
	}
	if nFill < 1 {
		nFill = 1
	}
	line := m.theme.apply(m.theme.HR, strings.Repeat(fill, nFill))
	out := []string{line}
	if nextType != "" {
		out = append(out, "")
	}
	return out
}

func (m *Markdown) renderHTMLBlock(n ast.Node, source []byte) []string {
	var b strings.Builder
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
	cleaned := normalizeHtmlForTerminal(b.String(), newHTMLNormState())
	var out []string
	for _, line := range splitTerminalLines(cleaned) {
		trimmed := trimRightSpaceRunes(line)
		if strings.TrimSpace(trimmed) == "" {
			out = append(out, "")
		} else {
			out = append(out, trimmed)
		}
	}
	return out
}

type listLine struct {
	text   string
	nested bool
}

func (m *Markdown) renderList(list *ast.List, source []byte, depth int, ctx inlineStyleCtx) []string {
	indent := strings.Repeat("  ", depth)
	start := list.Start
	if start == 0 {
		start = 1
	}
	var lines []string
	i := 0
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		li, ok := item.(*ast.ListItem)
		if !ok {
			continue
		}
		var bullet string
		if list.IsOrdered() {
			bullet = itoa(start+i) + ". "
		} else {
			bullet = "- "
		}
		contIndent := indent + padding(len(bullet))
		itemLines := m.renderListItem(li, source, depth, ctx)
		if len(itemLines) == 0 {
			lines = append(lines, indent+m.theme.apply(m.theme.ListBullet, bullet))
		} else {
			first := itemLines[0]
			if first.nested {
				lines = append(lines, first.text)
			} else {
				lines = append(lines, indent+m.theme.apply(m.theme.ListBullet, bullet)+first.text)
			}
			for _, ln := range itemLines[1:] {
				if ln.nested {
					lines = append(lines, ln.text)
				} else {
					lines = append(lines, contIndent+ln.text)
				}
			}
		}
		i++
	}
	return lines
}

func (m *Markdown) renderListItem(li *ast.ListItem, source []byte, parentDepth int, ctx inlineStyleCtx) []listLine {
	var lines []listLine
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindList:
			nested := m.renderList(c.(*ast.List), source, parentDepth+1, ctx)
			for _, nl := range nested {
				lines = append(lines, listLine{text: nl, nested: true})
			}
		case ast.KindParagraph, ast.KindTextBlock:
			if dm := soleDisplayMath(c, source); dm != nil {
				for _, ml := range renderMathDisplayLines(dm.Latex) {
					lines = append(lines, listLine{text: ctx.applyText(ml), nested: false})
				}
			} else {
				t := m.renderInlines(c, source, ctx)
				lines = append(lines, listLine{text: t, nested: false})
			}
		case KindMathBlock:
			mb := c.(*MathBlock)
			for _, ml := range renderMathDisplayLines(mb.Latex) {
				lines = append(lines, listLine{text: ctx.applyText(ml), nested: false})
			}
		case ast.KindFencedCodeBlock:
			indent := padding(m.opts.CodeBlockIndent)
			fc := c.(*ast.FencedCodeBlock)
			lang, body := m.codeLines(fc, source)
			lines = append(lines, listLine{text: m.theme.apply(m.theme.CodeBlockBorder, "```"+lang), nested: false})
			if m.theme.HighlightCode != nil {
				for _, hl := range m.theme.HighlightCode(body, lang) {
					lines = append(lines, listLine{text: indent + hl, nested: false})
				}
			} else {
				for _, cl := range splitTerminalLines(body) {
					lines = append(lines, listLine{text: indent + m.theme.apply(m.theme.CodeBlock, cl), nested: false})
				}
			}
			lines = append(lines, listLine{text: m.theme.apply(m.theme.CodeBlockBorder, "```"), nested: false})
		case east.KindTaskCheckBox:
			cb := c.(*east.TaskCheckBox)
			mark := "[ ] "
			if cb.IsChecked {
				mark = "[x] "
			}
			lines = append(lines, listLine{text: mark, nested: false})
		default:
			if c.Type() == ast.TypeBlock {
				blockLines := m.renderBlock(c, source, 80, "", &ctx)
				for _, bl := range blockLines {
					lines = append(lines, listLine{text: bl, nested: false})
				}
			} else {
				t := m.renderInlines(c, source, ctx)
				if t != "" {
					lines = append(lines, listLine{text: t, nested: false})
				}
			}
		}
	}
	if len(lines) >= 2 && (strings.HasPrefix(lines[0].text, "[ ] ") || strings.HasPrefix(lines[0].text, "[x] ")) && !lines[1].nested {
		lines[1].text = lines[0].text + lines[1].text
		lines = lines[1:]
	}
	return lines
}

func (m *Markdown) renderTable(n ast.Node, source []byte, availableWidth int, nextType string, ctx inlineStyleCtx) []string {
	var headerCells []string
	var rows [][]string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case east.KindTableHeader:
			for cell := c.FirstChild(); cell != nil; cell = cell.NextSibling() {
				if cell.Kind() == east.KindTableCell {
					headerCells = append(headerCells, m.renderInlines(cell, source, ctx))
				}
			}
		case east.KindTableRow:
			var row []string
			for cell := c.FirstChild(); cell != nil; cell = cell.NextSibling() {
				if cell.Kind() == east.KindTableCell {
					row = append(row, m.renderInlines(cell, source, ctx))
				}
			}
			rows = append(rows, row)
		}
	}
	numCols := len(headerCells)
	if numCols == 0 {
		return nil
	}

	borderOverhead := 3*numCols + 1
	availableForCells := availableWidth - borderOverhead
	if availableForCells < numCols {
		raw := tableRaw(n, source)
		fb := WrapTextWithAnsi(raw, availableWidth)
		if nextType != "" {
			fb = append(fb, "")
		}
		return fb
	}

	const maxUnbroken = 30
	natural := make([]int, numCols)
	minWord := make([]int, numCols)
	for i, h := range headerCells {
		natural[i] = maxLineWidth(h)
		minWord[i] = longestWordWidth(h, maxUnbroken)
	}
	for _, row := range rows {
		for i := 0; i < numCols && i < len(row); i++ {
			natural[i] = maxInt(natural[i], maxLineWidth(row[i]))
			minWord[i] = maxInt(minWord[i], longestWordWidth(row[i], maxUnbroken))
		}
	}

	minCol := append([]int(nil), minWord...)
	minCells := sumInts(minCol)
	if minCells > availableForCells {
		minCol = make([]int, numCols)
		for i := range minCol {
			minCol[i] = 1
		}
		remaining := availableForCells - numCols
		if remaining > 0 {
			totalWeight := 0
			for _, w := range minWord {
				totalWeight += maxInt(0, w-1)
			}
			growth := make([]int, numCols)
			for i, w := range minWord {
				weight := maxInt(0, w-1)
				if totalWeight > 0 {
					growth[i] = (weight * remaining) / totalWeight
				}
			}
			for i := range minCol {
				minCol[i] += growth[i]
			}
			allocated := sumInts(growth)
			leftover := remaining - allocated
			for i := 0; leftover > 0 && i < numCols; i++ {
				minCol[i]++
				leftover--
			}
		}
		minCells = sumInts(minCol)
	}

	totalNatural := sumInts(natural) + borderOverhead
	var colW []int
	if totalNatural <= availableWidth {
		colW = make([]int, numCols)
		for i := range colW {
			colW[i] = maxInt(natural[i], minCol[i])
		}
	} else {
		totalGrow := 0
		for i, w := range natural {
			totalGrow += maxInt(0, w-minCol[i])
		}
		extra := maxInt(0, availableForCells-minCells)
		colW = make([]int, numCols)
		for i, minW := range minCol {
			delta := maxInt(0, natural[i]-minW)
			grow := 0
			if totalGrow > 0 {
				grow = (delta * extra) / totalGrow
			}
			colW[i] = minW + grow
		}
		allocated := sumInts(colW)
		remaining := availableForCells - allocated
		for remaining > 0 {
			grew := false
			for i := 0; i < numCols && remaining > 0; i++ {
				if colW[i] < natural[i] {
					colW[i]++
					remaining--
					grew = true
				}
			}
			if !grew {
				break
			}
		}
	}

	ts := m.theme.tableSymbols()
	hch := ts.Horizontal
	vch := ts.Vertical

	var lines []string
	topCells := make([]string, numCols)
	for i, w := range colW {
		topCells[i] = strings.Repeat(hch, w)
	}
	lines = append(lines, ts.TopLeft+hch+strings.Join(topCells, hch+ts.TeeDown+hch)+hch+ts.TopRight)

	headerLines := make([][]string, numCols)
	maxHL := 1
	for i, cell := range headerCells {
		headerLines[i] = wrapCell(cell, colW[i])
		if len(headerLines[i]) > maxHL {
			maxHL = len(headerLines[i])
		}
	}
	for li := range maxHL {
		parts := make([]string, numCols)
		for ci := range numCols {
			txt := ""
			if li < len(headerLines[ci]) {
				txt = headerLines[ci][li]
			}
			pad := colW[ci] - ansitext.VisibleWidth(txt)
			if pad < 0 {
				pad = 0
			}
			padded := txt + padding(pad)
			if m.theme.TableHeader != nil {
				parts[ci] = m.theme.TableHeader(padded)
			} else {
				parts[ci] = m.theme.apply(m.theme.Bold, padded)
			}
		}
		lines = append(lines, vch+" "+strings.Join(parts, " "+vch+" ")+" "+vch)
	}

	sepCells := make([]string, numCols)
	for i, w := range colW {
		sepCells[i] = strings.Repeat(hch, w)
	}
	sep := ts.TeeRight + hch + strings.Join(sepCells, hch+ts.Cross+hch) + hch + ts.TeeLeft
	lines = append(lines, sep)

	for ri, row := range rows {
		cellLines := make([][]string, numCols)
		maxRL := 1
		for i := range numCols {
			txt := ""
			if i < len(row) {
				txt = row[i]
			}
			cellLines[i] = wrapCell(txt, colW[i])
			if len(cellLines[i]) > maxRL {
				maxRL = len(cellLines[i])
			}
		}
		for li := range maxRL {
			parts := make([]string, numCols)
			for ci := range numCols {
				txt := ""
				if li < len(cellLines[ci]) {
					txt = cellLines[ci][li]
				}
				pad := colW[ci] - ansitext.VisibleWidth(txt)
				if pad < 0 {
					pad = 0
				}
				parts[ci] = txt + padding(pad)
			}
			lines = append(lines, vch+" "+strings.Join(parts, " "+vch+" ")+" "+vch)
		}
		if ri < len(rows)-1 {
			lines = append(lines, sep)
		}
	}

	botCells := make([]string, numCols)
	for i, w := range colW {
		botCells[i] = strings.Repeat(hch, w)
	}
	lines = append(lines, ts.BottomLeft+hch+strings.Join(botCells, hch+ts.TeeUp+hch)+hch+ts.BottomRight)

	if nextType != "" {
		lines = append(lines, "")
	}
	return lines
}

func wrapCell(text string, maxWidth int) []string {
	w := maxWidth
	if w < 1 {
		w = 1
	}
	var out []string
	for _, line := range splitTerminalLines(text) {
		out = append(out, WrapTextWithAnsi(line, w)...)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func maxLineWidth(text string) int {
	max := 0
	for _, line := range splitTerminalLines(text) {
		max = maxInt(max, ansitext.VisibleWidth(line))
	}
	return max
}

func sumInts(xs []int) int {
	s := 0
	for _, v := range xs {
		s += v
	}
	return s
}

func tableRaw(n ast.Node, source []byte) string {
	start, end := -1, -1
	_ = ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if lines := node.Lines(); lines != nil && lines.Len() > 0 {
			seg0 := lines.At(0)
			segN := lines.At(lines.Len() - 1)
			if start < 0 || seg0.Start < start {
				start = seg0.Start
			}
			if segN.Stop > end {
				end = segN.Stop
			}
		}
		return ast.WalkContinue, nil
	})
	if start >= 0 && end > start && end <= len(source) {
		return string(source[start:end])
	}
	return ""
}

func (m *Markdown) renderInlines(parent ast.Node, source []byte, ctx inlineStyleCtx) string {
	var b strings.Builder
	htmlState := newHTMLNormState()
	trimLead := false
	swatchGlyph := m.theme.ColorSwatch

	applyNL := func(s string) string {
		parts := strings.Split(s, "\n")
		for i, p := range parts {
			if p != "" {
				parts[i] = ctx.applyText(p)
			}
		}
		return strings.Join(parts, "\n")
	}

	var walk func(ast.Node)
	walk = func(node ast.Node) {
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			switch c.Kind() {
			case ast.KindText:
				t := c.(*ast.Text)
				raw := string(t.Segment.Value(source))
				if trimLead {
					raw = strings.TrimLeft(raw, " \t")
					trimLead = false
				}
				raw = normalizeHtmlEntitiesForTerminal(raw)
				b.WriteString(renderTextWithSwatches(raw, applyNL, swatchGlyph))
				if t.HardLineBreak() {
					b.WriteByte('\n')
					trimLead = true
				} else if t.SoftLineBreak() {
					// CommonMark softbreak → space so wrap sees one flow.
					b.WriteByte(' ')
				}
			case ast.KindString:
				s := c.(*ast.String)
				raw := string(s.Value)
				if trimLead {
					raw = strings.TrimLeft(raw, " \t")
					trimLead = false
				}
				b.WriteString(applyNL(raw))
			case ast.KindEmphasis:
				em := c.(*ast.Emphasis)
				inner := m.renderInlines(em, source, ctx)
				if em.Level >= 2 {
					b.WriteString(m.theme.apply(m.theme.Bold, inner))
				} else {
					b.WriteString(m.theme.apply(m.theme.Italic, inner))
				}
				b.WriteString(ctx.stylePrefix)
			case east.KindStrikethrough:
				inner := m.renderInlines(c, source, ctx)
				b.WriteString(m.theme.apply(m.theme.Strikethrough, inner))
				b.WriteString(ctx.stylePrefix)
			case ast.KindCodeSpan:
				code := codeSpanText(c, source)
				b.WriteString(codespanSwatch(code, swatchGlyph))
				b.WriteString(m.theme.apply(m.theme.Code, code))
				b.WriteString(ctx.stylePrefix)
			case ast.KindLink:
				link := c.(*ast.Link)
				href := string(link.Destination)
				linkText := m.renderInlines(link, source, ctx)
				styled := m.theme.apply(m.theme.Link, underline(linkText))
				clickable := formatHyperlink(styled, href, m.theme.Hyperlinks)
				plainLabel := plainInlines(link, source)
				hrefCmp := href
				if strings.HasPrefix(href, "mailto:") {
					hrefCmp = href[len("mailto:"):]
				}
				if plainLabel == href || plainLabel == hrefCmp {
					b.WriteString(clickable)
				} else {
					urlPart := m.theme.apply(m.theme.LinkURL, " ("+href+")")
					b.WriteString(clickable)
					b.WriteString(formatHyperlink(urlPart, href, m.theme.Hyperlinks))
				}
				b.WriteString(ctx.stylePrefix)
			case ast.KindAutoLink:
				al := c.(*ast.AutoLink)
				label := string(al.Label(source))
				url := string(al.URL(source))
				styled := m.theme.apply(m.theme.Link, underline(ctx.applyText(label)))
				b.WriteString(formatHyperlink(styled, url, m.theme.Hyperlinks))
				b.WriteString(ctx.stylePrefix)
			case ast.KindImage:
				img := c.(*ast.Image)
				alt := plainInlines(img, source)
				dest := string(img.Destination)
				label := "![" + alt + "]"
				if dest != "" {
					label += "(" + dest + ")"
				}
				b.WriteString(ctx.applyText(label))
			case ast.KindRawHTML:
				raw := rawHTMLText(c, source)
				cleaned := normalizeHtmlForTerminal(raw, htmlState)
				b.WriteString(applyNL(cleaned))
				if strings.HasSuffix(cleaned, "\n") {
					trimLead = true
				} else if cleaned != "" {
					trimLead = false
				}
			case east.KindTaskCheckBox:
				cb := c.(*east.TaskCheckBox)
				if cb.IsChecked {
					b.WriteString(ctx.applyText("[x] "))
				} else {
					b.WriteString(ctx.applyText("[ ] "))
				}
			case KindMath:
				mn := c.(*Math)
				b.WriteString(ctx.applyText(renderMathInline(mn.Latex)))
			default:
				if c.HasChildren() {
					walk(c)
				}
			}
		}
	}
	walk(parent)

	result := b.String()
	for ctx.stylePrefix != "" && strings.HasSuffix(result, ctx.stylePrefix) {
		result = result[:len(result)-len(ctx.stylePrefix)]
	}
	return result
}

func codeSpanText(n ast.Node, source []byte) string {
	var b bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		}
	}
	return strings.ReplaceAll(b.String(), "\n", " ")
}

func rawHTMLText(n ast.Node, source []byte) string {
	rn, ok := n.(*ast.RawHTML)
	if !ok {
		return ""
	}
	var b strings.Builder
	for i := range rn.Segments.Len() {
		seg := rn.Segments.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

func plainInlines(parent ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(parent, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		case *ast.CodeSpan:
			b.WriteString(codeSpanText(t, source))
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// RenderInlineMarkdown styles inline markdown (strong/em/code/link/del) to one string.
// Block structure is flattened: paragraphs concatenate; lists become spaced items.
func RenderInlineMarkdown(src string, theme Theme, baseColor func(string) string) string {
	if baseColor == nil {
		baseColor = func(s string) string { return s }
	}
	if src == "" {
		return baseColor("")
	}
	normalized := replaceTabs(src)
	source := []byte(normalized)
	doc := mdEngine().Parser().Parse(text.NewReader(source))

	tmp := &Markdown{theme: theme}
	ctx := inlineStyleCtx{
		applyText:   baseColor,
		stylePrefix: stylePrefixOf(baseColor),
	}

	var b strings.Builder
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch n.Kind() {
		case KindMathBlock:
			mb := n.(*MathBlock)
			b.WriteString(baseColor(renderMathInline(mb.Latex)))
		case ast.KindParagraph, ast.KindTextBlock:
			if dm := soleDisplayMath(n, source); dm != nil {
				b.WriteString(baseColor(renderMathInline(dm.Latex)))
			} else {
				b.WriteString(tmp.renderInlines(n, source, ctx))
			}
		case ast.KindList:
			list := n.(*ast.List)
			start := list.Start
			if start == 0 {
				start = 1
			}
			i := 0
			for item := list.FirstChild(); item != nil; item = item.NextSibling() {
				if i > 0 {
					b.WriteString(baseColor(" "))
				}
				var prefix string
				if list.IsOrdered() {
					prefix = itoa(start+i) + ". "
				} else {
					prefix = "• "
				}
				b.WriteString(baseColor(prefix))
				b.WriteString(tmp.renderInlines(item, source, ctx))
				i++
			}
		default:
			if n.Lines() != nil && n.Lines().Len() > 0 {
				b.WriteString(baseColor(string(n.Lines().Value(source))))
			} else {
				b.WriteString(tmp.renderInlines(n, source, ctx))
			}
		}
	}
	return b.String()
}

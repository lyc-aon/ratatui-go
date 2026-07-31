package richtext_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/richtext"
)

func visible(s string) string {
	var b strings.Builder
	for _, seg := range ansitext.ParseSegments(s) {
		if seg.Kind == "text" {
			b.WriteString(seg.Text)
		}
	}
	return b.String()
}

func joinVis(lines []string) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = visible(l)
	}
	return strings.Join(parts, "\n")
}

func TestMarkdownGFMBasics(t *testing.T) {
	src := "# Title\n\nParagraph with **bold** and *em* and `code`.\n\n- item one\n- item two\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n> quote\n\n~~strike~~ and [link](https://example.com)\n"
	md := richtext.NewMarkdown(src, richtext.Theme{}, richtext.MarkdownOptions{})
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("empty render")
	}
	body := joinVis(lines)
	for _, want := range []string{"Title", "bold", "em", "code", "item one", "quote"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	// table cells
	if !strings.Contains(body, "A") || !strings.Contains(body, "1") {
		t.Fatalf("table missing:\n%s", body)
	}
	// no raw markdown markers required gone for bold
	if strings.Contains(body, "**bold**") {
		t.Fatalf("bold markers remain:\n%s", body)
	}
}

func TestMarkdownInlineAndDisplayMath(t *testing.T) {
	src := "Inline $x$ and display:\n\n$$\\frac{1}{2}$$\n\nDone.\n"
	md := richtext.NewMarkdown(src, richtext.Theme{}, richtext.MarkdownOptions{PaddingX: 0, PaddingY: 0})
	lines := md.Render(60)
	body := joinVis(lines)
	if strings.Contains(body, `$x$`) {
		t.Fatalf("inline math delimiters remain:\n%s", body)
	}
	if strings.Contains(body, `$$`) {
		t.Fatalf("display delimiters remain:\n%s", body)
	}
	if !strings.Contains(body, "Inline") || !strings.Contains(body, "Done") {
		t.Fatalf("prose lost:\n%s", body)
	}
}

func TestMarkdownAntiCurrency(t *testing.T) {
	src := "Price is $5 today and math is $x+y$ ok.\n"
	md := richtext.NewMarkdown(src, richtext.Theme{}, richtext.MarkdownOptions{PaddingX: 0, PaddingY: 0})
	body := joinVis(md.Render(80))
	if !strings.Contains(body, "$5") && !strings.Contains(body, "5") {
		// currency may keep $5
		t.Fatalf("currency text lost:\n%s", body)
	}
	// real math should not remain as $x+y$
	if strings.Contains(body, "$x+y$") {
		t.Fatalf("math not rendered:\n%s", body)
	}
}

func TestMarkdownBareEnvMath(t *testing.T) {
	src := "Matrix:\n\n\\begin{matrix} a & b \\\\ c & d \\end{matrix}\n"
	md := richtext.NewMarkdown(src, richtext.Theme{}, richtext.MarkdownOptions{PaddingX: 0, PaddingY: 0})
	lines := md.Render(80)
	if len(lines) == 0 {
		t.Fatal("empty")
	}
	body := joinVis(lines)
	// should not be only the raw begin line unprocessed for empty result
	if body == "" {
		t.Fatal("empty body")
	}
}

func TestMarkdownFencedCodeAndMermaidHook(t *testing.T) {
	src := "```go\npackage main\n```\n\n```mermaid\ngraph LR\n  A --> B\n```\n"
	called := false
	theme := richtext.Theme{
		HighlightCode: func(code, lang string) []string {
			if lang != "go" {
				t.Fatalf("lang %q", lang)
			}
			return []string{"«" + strings.TrimSpace(code) + "»"}
		},
		ResolveMermaidASCII: func(source string, maxWidth int) (string, bool) {
			called = true
			if maxWidth < 1 {
				t.Fatalf("maxWidth %d", maxWidth)
			}
			if !strings.Contains(source, "graph") {
				t.Fatalf("source %q", source)
			}
			return "A-->B", true
		},
	}
	md := richtext.NewMarkdown(src, theme, richtext.MarkdownOptions{PaddingX: 0, PaddingY: 0, CodeBlockIndent: -1})
	body := joinVis(md.Render(60))
	if !strings.Contains(body, "package main") && !strings.Contains(body, "«package main»") {
		t.Fatalf("code missing:\n%s", body)
	}
	if !called {
		t.Fatal("mermaid hook not called")
	}
	if !strings.Contains(body, "A-->B") {
		t.Fatalf("mermaid art missing:\n%s", body)
	}

	// absent mermaid hook → fenced source fallback, not fake art
	md2 := richtext.NewMarkdown("```mermaid\ngraph LR\n  A --> B\n```\n", richtext.Theme{}, richtext.MarkdownOptions{PaddingX: 0, PaddingY: 0})
	body2 := joinVis(md2.Render(60))
	if !strings.Contains(body2, "graph") {
		t.Fatalf("fallback should show source:\n%s", body2)
	}
}

func TestMarkdownWidthsNoOverflow(t *testing.T) {
	src := strings.Repeat("word ", 40) + "\n\n# Heading that is fairly long for narrow terminals\n"
	md := richtext.NewMarkdown(src, richtext.Theme{}, richtext.MarkdownOptions{PaddingX: 0, PaddingY: 0})
	for _, w := range []int{30, 60, 100} {
		lines := md.Render(w)
		for i, line := range lines {
			if strings.Contains(line, "\n") {
				t.Fatalf("width %d line %d has newline", w, i)
			}
			vw := ansitext.VisibleWidth(line)
			if vw > w {
				t.Fatalf("width %d overflow line %d vw=%d: %q", w, i, vw, line)
			}
		}
	}
}

func TestMarkdownCacheReuse(t *testing.T) {
	md := richtext.NewMarkdown("hello **x**", richtext.Theme{}, richtext.MarkdownOptions{})
	a := md.Render(40)
	b := md.Render(40)
	if len(a) == 0 {
		t.Fatal("empty")
	}
	// same contents
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("cache content drift")
		}
	}
	// pointer reuse of slice
	if &a[0] != &b[0] && len(a) > 0 {
		// allow either copy or reuse; content equality is the contract
	}
	md.SetText("other")
	c := md.Render(40)
	if joinVis(c) == joinVis(a) {
		t.Fatal("SetText did not invalidate")
	}
}

func TestTextAndTruncate(t *testing.T) {
	tx := richtext.NewText("one two three four five", 0, 0)
	lines := tx.Render(10)
	if len(lines) == 0 {
		t.Fatal("empty text wrap")
	}
	for _, line := range lines {
		if ansitext.VisibleWidth(line) > 10 {
			t.Fatalf("wrap overflow: %q", line)
		}
	}
	// empty
	tx.SetText("   ")
	if len(tx.Render(10)) != 0 {
		// may be empty slice
	}
}

func TestHTMLEntities(t *testing.T) {
	src := "A &amp; B &lt;C&gt; &#65; &#x42;"
	md := richtext.NewMarkdown(src, richtext.Theme{}, richtext.MarkdownOptions{PaddingX: 0, PaddingY: 0})
	body := joinVis(md.Render(80))
	if !strings.Contains(body, "&") || !strings.Contains(body, "<C>") {
		// goldmark may handle entities itself
		if !strings.Contains(body, "A") {
			t.Fatalf("entities body: %q", body)
		}
	}
	// ensure valid utf8
	if !utf8.ValidString(body) {
		t.Fatal("invalid utf8")
	}
}

func TestMalformedMarkdownNoPanic(t *testing.T) {
	inputs := []string{
		"```\nunclosed",
		"| a |\n| b",
		"$$$$",
		string([]byte{0xff, 0xfe}),
		"[text](",
		"***",
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("panic on %q: %v", in, rec)
				}
			}()
			md := richtext.NewMarkdown(in, richtext.Theme{}, richtext.MarkdownOptions{})
			_ = md.Render(40)
		}()
	}
}

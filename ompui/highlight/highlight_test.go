package highlight_test

import (
	"strings"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/highlight"
)

func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			// skip CSI ... m
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) {
					c := s[j]
					j++
					if c >= '@' && c <= '~' {
						break
					}
				}
				i = j
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func TestColorModes(t *testing.T) {
	code := "package main\nfunc main() {}\n"

	// ColorNone
	none := highlight.Highlight(code, "go", highlight.Options{ColorMode: highlight.ColorNone})
	if len(none) == 0 {
		t.Fatal("empty none")
	}
	for _, line := range none {
		if strings.Contains(line, "\x1b[") {
			t.Fatalf("ColorNone has SGR: %q", line)
		}
		if strings.HasSuffix(line, "\n") {
			t.Fatalf("trailing newline: %q", line)
		}
	}
	joined := strings.Join(none, "\n")
	if !strings.Contains(joined, "package") || !strings.Contains(joined, "main") {
		t.Fatalf("source lost under ColorNone: %q", joined)
	}

	c256 := highlight.Highlight(code, "go", highlight.Options{ColorMode: highlight.ColorANSI256})
	has256 := false
	for _, line := range c256 {
		if strings.Contains(line, "\x1b[38;5;") {
			has256 = true
		}
		if strings.Contains(line, "\x1b[38;2;") {
			t.Fatalf("256 mode emitted truecolor: %q", line)
		}
		if strings.HasSuffix(line, "\n") {
			t.Fatalf("trailing newline: %q", line)
		}
	}
	if !has256 {
		t.Fatalf("no 38;5 in 256 mode: %#v", c256)
	}

	truec := highlight.Highlight(code, "go", highlight.Options{ColorMode: highlight.ColorTrueColor})
	hasTrue := false
	for _, line := range truec {
		if strings.Contains(line, "\x1b[38;2;") {
			hasTrue = true
		}
		if strings.HasSuffix(line, "\n") {
			t.Fatalf("trailing newline: %q", line)
		}
	}
	if !hasTrue {
		t.Fatalf("no 38;2 in truecolor: %#v", truec)
	}
}

func TestKnownAndUnknownLanguage(t *testing.T) {
	code := "def f(x):\n    return x+1\n"
	known := highlight.Highlight(code, "python", highlight.Options{ColorMode: highlight.ColorTrueColor})
	if len(known) < 2 {
		t.Fatalf("python lines=%d", len(known))
	}
	// known lang should usually emit SGR for keywords
	styled := false
	for _, line := range known {
		if strings.Contains(line, "\x1b[") {
			styled = true
		}
	}
	if !styled {
		t.Fatalf("known lang produced no SGR: %#v", known)
	}

	unknown := highlight.Highlight(code, "definitely-not-a-lexer-xyz", highlight.Options{ColorMode: highlight.ColorTrueColor})
	if len(unknown) == 0 {
		t.Fatal("unknown lang empty")
	}
	// plain fallback still returns source lines without panic
	plain := stripSGR(strings.Join(unknown, "\n"))
	if !strings.Contains(plain, "def f") {
		t.Fatalf("unknown lost source: %q", plain)
	}

	// empty lang → plaintext path
	empty := highlight.Highlight("hello", "", highlight.Options{ColorMode: highlight.ColorNone})
	if len(empty) != 1 || empty[0] != "hello" {
		t.Fatalf("empty lang: %#v", empty)
	}
}

func TestAliasLanguages(t *testing.T) {
	code := "const x = 1;"
	for _, lang := range []string{"js", "ts", "py", "golang", "yml", "rs", "sh"} {
		lines := highlight.Highlight(code, lang, highlight.Options{ColorMode: highlight.ColorANSI256})
		if len(lines) == 0 {
			t.Fatalf("alias %q empty", lang)
		}
	}
}

func TestTruncationBounds(t *testing.T) {
	// MaxLines
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("line\n")
	}
	h := highlight.New(highlight.Options{
		ColorMode: highlight.ColorNone,
		MaxLines:  5,
	})
	lines := h.HighlightCode(b.String(), "text")
	if len(lines) > 5 {
		t.Fatalf("MaxLines not enforced: %d", len(lines))
	}

	// MaxSourceBytes
	src := strings.Repeat("abcdefghi\n", 20)
	h2 := highlight.New(highlight.Options{
		ColorMode:      highlight.ColorNone,
		MaxSourceBytes: 30,
		MaxLines:       100,
	})
	out := h2.HighlightCode(src, "text")
	total := 0
	for _, line := range out {
		total += len(line) + 1
	}
	// truncated input is at most 30 bytes before split
	if total > 40 {
		t.Fatalf("source bound too loose: total≈%d out=%#v", total, out)
	}

	// MaxLineBytes
	long := strings.Repeat("x", 200)
	h3 := highlight.New(highlight.Options{
		ColorMode:    highlight.ColorNone,
		MaxLineBytes: 16,
	})
	one := h3.HighlightCode(long, "text")
	if len(one) != 1 {
		t.Fatalf("expected 1 line got %d", len(one))
	}
	if len(one[0]) > 16 {
		t.Fatalf("MaxLineBytes not enforced: %d", len(one[0]))
	}
}

func TestPanicSafeNilAndEmpty(t *testing.T) {
	var h *highlight.Highlighter
	out := h.HighlightCode("a\nb", "go")
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("nil highlighter: %#v", out)
	}

	out = highlight.Highlight("", "go", highlight.Options{ColorMode: highlight.ColorTrueColor})
	if len(out) != 1 || out[0] != "" {
		t.Fatalf("empty code: %#v", out)
	}
}

func TestCacheDeterministicAndCopySafe(t *testing.T) {
	h := highlight.New(highlight.Options{ColorMode: highlight.ColorTrueColor, Theme: "monokai"})
	code := "fn main() { let x = 1; }\n"
	a := h.HighlightCode(code, "rust")
	b := h.HighlightCode(code, "rust")
	if len(a) != len(b) {
		t.Fatalf("cache len mismatch %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("cache mismatch at %d: %q vs %q", i, a[i], b[i])
		}
	}
	// mutating returned slice header contents of strings is impossible;
	// mutating the slice slots must not poison cache
	if len(a) > 0 {
		a[0] = "mutated"
	}
	c := h.HighlightCode(code, "rust")
	if c[0] == "mutated" {
		t.Fatal("cache returned mutable shared slot")
	}
}

func TestFuncMatchesHighlightCode(t *testing.T) {
	h := highlight.New(highlight.Options{ColorMode: highlight.ColorANSI256})
	fn := h.Func()
	code := "print('hi')\n"
	a := h.HighlightCode(code, "python")
	b := fn(code, "python")
	if len(a) != len(b) {
		t.Fatalf("Func len %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("Func mismatch: %q vs %q", a[i], b[i])
		}
	}
}

func TestNoTrailingNewlinesOnLines(t *testing.T) {
	lines := highlight.Highlight("a\nb\n\nc\n", "go", highlight.Options{ColorMode: highlight.ColorTrueColor})
	for i, line := range lines {
		if strings.HasSuffix(line, "\n") || strings.HasSuffix(line, "\r") {
			t.Fatalf("line %d has trailing break: %q", i, line)
		}
	}
}

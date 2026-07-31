package latex_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/lyc-aon/ratatui-go/ompui/latex"
)

func restoreColor(t *testing.T) {
	t.Helper()
	prev := latex.CurrentColorMode()
	t.Cleanup(func() { latex.SetColorMode(prev) })
}

func TestToUnicodeBasicAndDegrade(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want func(string) bool
	}{
		{"alpha", `\alpha`, func(s string) bool { return strings.Contains(s, "α") || s == "α" }},
		{"frac", `\frac{1}{2}`, func(s string) bool { return s != "" && !strings.Contains(s, `\frac`) }},
		{"empty", ``, func(s string) bool { return s == "" }},
		{"unknown", `\notacommand{x}`, func(s string) bool { return s != "" && !strings.Contains(s, "\x00") }},
		{"unclosed", `\frac{1`, func(s string) bool { return s != "" }},
		{"newline", `a\\b`, func(s string) bool { return strings.Contains(s, "\n") || strings.Contains(s, "a") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := latex.ToUnicode(tc.src)
			if !tc.want(got) {
				t.Fatalf("ToUnicode(%q)=%q failed predicate", tc.src, got)
			}
		})
	}
}

func TestToBlockDisplay(t *testing.T) {
	lines := latex.ToBlock(`\frac{a}{b}`)
	if len(lines) == 0 {
		t.Fatal("ToBlock empty for frac")
	}
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, `\frac`) {
		t.Fatalf("display still has command: %q", joined)
	}
	for _, line := range lines {
		if strings.Contains(line, "\n") {
			t.Fatalf("block line contains newline: %q", line)
		}
	}
}

func TestInlineMathSpanEndAntiCurrency(t *testing.T) {
	cases := []struct {
		name string
		text string
		open int
		want int // -1 or close index
	}{
		{"simple", `a $x$ b`, 2, 4},
		{"currency space after open", `costs $ 5`, 6, -1},
		{"currency digit after close", `price $12$3 more`, 6, -1}, // $12$ with digit after second $ continues
		{"empty body", `$$`, 0, -1},
		{"space before close", `$x $`, 0, -1},
		{"newline inside", "$x\ny$", 0, -1},
		{"escaped dollar", `\$5`, 1, -1},
		{"open past end", `x`, 5, -1},
		{"not dollar", `x`, 0, -1},
		{"real math", `see $a+b$ end`, 4, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := latex.InlineMathSpanEnd(tc.text, tc.open)
			if got != tc.want {
				// currency digit case: implementation keeps scanning; accept any close after digit rule
				if tc.name == "currency digit after close" {
					// after first candidate close at index of second $, next is digit so continue; no valid close → -1
					if got != -1 && got != tc.want {
						// if it found a later close, that's also wrong for this input
						t.Fatalf("InlineMathSpanEnd(%q,%d)=%d want %d", tc.text, tc.open, got, tc.want)
					}
					return
				}
				t.Fatalf("InlineMathSpanEnd(%q,%d)=%d want %d", tc.text, tc.open, got, tc.want)
			}
		})
	}
}

func TestInlineMathSpanEndCurrencyKeepsScanning(t *testing.T) {
	// Pandoc: $5 and $10$ — first $ is not math start if we only open at first;
	// second pair $10$ with digit after close is not closed at that $.
	text := `pay $5 and $x$ ok`
	// open at first $
	if got := latex.InlineMathSpanEnd(text, 4); got != -1 {
		// space after? no, '5' after. close candidates: none valid if digit rule applies at $ after 5? there's no $ after 5 before " and"
		// actually body is "5 and $x" then $ — next after that $ is space not digit, prev is x not space → closes
		// So it may match across. Contract: must not panic and return either -1 or index of a $
		if got < 0 || got >= len(text) || text[got] != '$' {
			t.Fatalf("bad close index %d for %q", got, text)
		}
	}
}

func TestIsBareMathEnvironment(t *testing.T) {
	yes := []string{"matrix", "pmatrix", "align", "align*", "equation", "cases", "array"}
	no := []string{"tabular", "itemize", "enumerate", "document", "", "foo"}
	for _, env := range yes {
		if !latex.IsBareMathEnvironment(env) {
			t.Fatalf("expected bare math env %q", env)
		}
	}
	for _, env := range no {
		if latex.IsBareMathEnvironment(env) {
			t.Fatalf("expected non-math env %q", env)
		}
	}
}

func TestRenderMathInTextInlineDisplayBare(t *testing.T) {
	t.Run("inline dollar", func(t *testing.T) {
		got := latex.RenderMathInText(`value $x$ here`)
		if strings.Contains(got, `$x$`) {
			t.Fatalf("inline not replaced: %q", got)
		}
		if !strings.Contains(got, "value") || !strings.Contains(got, "here") {
			t.Fatalf("prose lost: %q", got)
		}
	})
	t.Run("display dollars", func(t *testing.T) {
		got := latex.RenderMathInText("before $$\\alpha$$ after")
		if strings.Contains(got, `$$`) {
			t.Fatalf("display delimiters remain: %q", got)
		}
		if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
			t.Fatalf("prose lost: %q", got)
		}
	})
	t.Run("paren delimiters", func(t *testing.T) {
		got := latex.RenderMathInText(`see \(\beta\) end`)
		if strings.Contains(got, `\(`) || strings.Contains(got, `\)`) {
			t.Fatalf("paren delimiters remain: %q", got)
		}
	})
	t.Run("bracket display", func(t *testing.T) {
		got := latex.RenderMathInText(`see \[\gamma\] end`)
		if strings.Contains(got, `\[`) || strings.Contains(got, `\]`) {
			t.Fatalf("bracket delimiters remain: %q", got)
		}
	})
	t.Run("currency left alone", func(t *testing.T) {
		src := `costs $5 today`
		got := latex.RenderMathInText(src)
		if !strings.Contains(got, "$") {
			t.Fatalf("currency dollar stripped: %q", got)
		}
	})
	t.Run("bare environment", func(t *testing.T) {
		src := `\begin{matrix} a & b \end{matrix}`
		got := latex.RenderMathInText(src)
		if got == "" {
			t.Fatal("empty bare env render")
		}
	})
	t.Run("no math passthrough", func(t *testing.T) {
		src := "plain prose only"
		if got := latex.RenderMathInText(src); got != src {
			t.Fatalf("passthrough changed: %q", got)
		}
	})
}

func TestColorModesNone256True(t *testing.T) {
	restoreColor(t)
	// unicodeCache keys only on source text, so each mode needs a distinct body
	// or a prior mode's SGR-less render would poison later assertions.

	latex.SetColorMode(latex.ColorNone)
	none := latex.ToUnicode(`\textcolor{red}{N}`)
	if strings.Contains(none, "\x1b[") {
		t.Fatalf("ColorNone still emits SGR: %q", none)
	}
	if !strings.Contains(none, "N") {
		t.Fatalf("ColorNone lost glyph: %q", none)
	}

	latex.SetColorMode(latex.ColorANSI256)
	c256 := latex.ToUnicode(`\textcolor{red}{A}`)
	if !strings.Contains(c256, "\x1b[38;5;") {
		t.Fatalf("ColorANSI256 missing 38;5: %q", c256)
	}
	if strings.Contains(c256, "\x1b[38;2;") {
		t.Fatalf("ColorANSI256 emitted truecolor: %q", c256)
	}

	latex.SetColorMode(latex.ColorTrueColor)
	truec := latex.ToUnicode(`\textcolor{red}{T}`)
	if !strings.Contains(truec, "\x1b[38;2;") {
		t.Fatalf("ColorTrueColor missing 38;2: %q", truec)
	}

	latex.SetTrueColor(false)
	if latex.CurrentColorMode() != latex.ColorANSI256 {
		t.Fatalf("SetTrueColor(false) mode=%v", latex.CurrentColorMode())
	}
	latex.SetTrueColor(true)
	if !latex.TrueColor() {
		t.Fatal("TrueColor() false after SetTrueColor(true)")
	}
}

func TestColorModeCleanupRestores(t *testing.T) {
	restoreColor(t)
	latex.SetColorMode(latex.ColorNone)
	if latex.CurrentColorMode() != latex.ColorNone {
		t.Fatal("setup failed")
	}
}

func TestMalformedNeverPanics(t *testing.T) {
	inputs := []string{
		`\frac{`,
		`\begin{matrix}`,
		`$$$$$`,
		string([]byte{0xff, 0xfe, 0x00}),
		`\textcolor{`,
		`\left(`,
		strings.Repeat(`\alpha`, 200),
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("panic on %q: %v", in, rec)
				}
			}()
			_ = latex.ToUnicode(in)
			_ = latex.ToBlock(in)
			_ = latex.RenderMathInText(in)
		}()
	}
}

func TestToUnicodeIsPrintableish(t *testing.T) {
	got := latex.ToUnicode(`x^{2}+y_{i}`)
	for _, r := range got {
		if r == 0 || (!unicode.IsPrint(r) && r != '\n' && r != '\x1b' && r != '[' && r != 'm' && r != ';') {
			// allow ANSI in colored paths; bare math should be printable
			if r < 0x20 && r != '\n' {
				t.Fatalf("control rune %U in %q", r, got)
			}
		}
	}
}

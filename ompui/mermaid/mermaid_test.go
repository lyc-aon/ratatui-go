package mermaid_test

import (
	"strings"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/mermaid"
)

func TestGraphLRRenders(t *testing.T) {
	src := "graph LR\n  A --> B\n  B --> C\n"
	out, ok, err := mermaid.Render(src, mermaid.Options{})
	if err != nil || !ok {
		t.Fatalf("graph render ok=%v err=%v out=%q", ok, err, out)
	}
	if out == src {
		t.Fatal("graph returned raw source")
	}
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("missing nodes: %q", out)
	}
	// multi-line ascii art
	if !strings.Contains(out, "\n") {
		t.Fatalf("expected multi-line diagram: %q", out)
	}
}

func TestSequenceRenders(t *testing.T) {
	src := "sequenceDiagram\n  participant A\n  participant B\n  A->>B: hi\n"
	out, ok, err := mermaid.Render(src, mermaid.Options{})
	if err != nil || !ok {
		t.Fatalf("sequence ok=%v err=%v out=%q", ok, err, out)
	}
	if out == src {
		t.Fatal("sequence returned raw source")
	}
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("missing participants: %q", out)
	}
}

func TestUnsupportedReturnsFailure(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"pie title Pets\n  \"Dogs\" : 386\n",
		"gantt\n  title A\n",
		"not a diagram at all",
		"graph LR\n  A -.->", // malformed edge
	}
	for _, src := range cases {
		out, ok, err := mermaid.Render(src, mermaid.Options{})
		if ok {
			t.Fatalf("expected fail for %q got out=%q", src, out)
		}
		if out != src && src != "" && strings.TrimSpace(src) != "" {
			// contract: original source returned on failure
			if out != src {
				t.Fatalf("failure should return source; got %q want %q", out, src)
			}
		}
		if err == nil && strings.TrimSpace(src) != "" {
			// empty has err; others should too
			t.Fatalf("expected err for %q", src)
		}
	}
}

func TestWidthOrientationFallback(t *testing.T) {
	// Wide LR graph; tight MaxWidth should still succeed and not exceed a huge width blindly.
	src := "graph LR\n  Start --> Middle --> End --> Finish\n"
	wide, ok, err := mermaid.Render(src, mermaid.Options{MaxWidth: 0})
	if err != nil || !ok {
		t.Fatalf("baseline: %v %v", ok, err)
	}
	wideW := maxLine(wide)

	narrow, ok, err := mermaid.Render(src, mermaid.Options{MaxWidth: 20})
	if err != nil || !ok {
		t.Fatalf("narrow: %v %v", ok, err)
	}
	narrowW := maxLine(narrow)
	if narrowW > wideW {
		t.Fatalf("width fallback widened diagram: narrow=%d wide=%d\n%s", narrowW, wideW, narrow)
	}
	// ResolveMermaidASCII path
	r := mermaid.New(mermaid.Options{})
	got, ok := r.ResolveMermaidASCII(src, 20)
	if !ok || got == "" {
		t.Fatalf("ResolveMermaidASCII failed ok=%v", ok)
	}
	if maxLine(got) > wideW {
		t.Fatalf("resolve widened: %d > %d", maxLine(got), wideW)
	}
}

func TestFailureCacheStable(t *testing.T) {
	r := mermaid.New(mermaid.Options{})
	src := "pie title X\n  \"a\" : 1\n"
	out1, ok1, err1 := r.Render(src)
	out2, ok2, err2 := r.Render(src)
	if ok1 || ok2 {
		t.Fatal("unsupported should fail")
	}
	if out1 != src || out2 != src {
		t.Fatalf("cached failure source mismatch %q %q", out1, out2)
	}
	if (err1 == nil) != (err2 == nil) {
		t.Fatalf("err presence changed: %v vs %v", err1, err2)
	}
	// success cache
	g := "graph TD\n  X --> Y\n"
	a, ok, err := r.Render(g)
	if !ok || err != nil {
		t.Fatalf("graph: %v %v", ok, err)
	}
	b, ok, err := r.Render(g)
	if !ok || err != nil || a != b {
		t.Fatalf("success cache mismatch ok=%v err=%v", ok, err)
	}
}

func TestASCIIVsUnicode(t *testing.T) {
	src := "graph LR\n  A --> B\n"
	uni, ok, err := mermaid.Render(src, mermaid.Options{UseASCII: false})
	if !ok || err != nil {
		t.Fatal(err)
	}
	asc, ok, err := mermaid.Render(src, mermaid.Options{UseASCII: true})
	if !ok || err != nil {
		t.Fatal(err)
	}
	if uni == "" || asc == "" {
		t.Fatal("empty")
	}
	// ASCII mode should avoid common box-drawing codepoints when possible
	if strings.Contains(asc, "─") && !strings.Contains(uni, "─") {
		t.Fatalf("ascii has box draw but unicode does not?\nasc=%q\nuni=%q", asc, uni)
	}
}

func TestSourceByteCap(t *testing.T) {
	r := mermaid.New(mermaid.Options{MaxSourceBytes: 32})
	src := "graph LR\n" + strings.Repeat("A --> B\n", 40)
	out, ok, err := r.Render(src)
	if ok || err == nil {
		t.Fatalf("expected oversize fail ok=%v err=%v", ok, err)
	}
	if out != src {
		t.Fatalf("oversize should return source")
	}
	got, ok := r.ResolveMermaidASCII(src, 80)
	if ok || got != "" {
		t.Fatalf("resolve oversize should false empty, got %q", got)
	}
}

func TestPackageResolveHook(t *testing.T) {
	src := "graph TD\n  A --> B\n"
	out, ok := mermaid.ResolveMermaidASCII(src, 100)
	if !ok || out == "" {
		t.Fatalf("package resolve failed")
	}
	bad, ok := mermaid.ResolveMermaidASCII("nope", 100)
	if ok || bad != "" {
		t.Fatalf("bad resolve: ok=%v out=%q", ok, bad)
	}
}

func maxLine(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		// strip simple SGR if any
		n := len([]rune(line))
		if n > max {
			max = n
		}
	}
	return max
}

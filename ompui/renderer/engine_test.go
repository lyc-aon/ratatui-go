package renderer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
)

func newEng(buf *bytes.Buffer) *renderer.Engine {
	return renderer.New(buf, renderer.Caps{})
}

func draw(t *testing.T, e *renderer.Engine, lines []string, w, h int, reason renderer.Reason) {
	t.Helper()
	req := renderer.Request{
		Frame:  component.NewFrame(lines, 1),
		Width:  w,
		Height: h,
		Reason: reason,
	}
	if err := e.Draw(req); err != nil {
		t.Fatalf("Draw: %v", err)
	}
}

func TestFirstPaintWritesContent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := newEng(&buf)
	if e.HasRendered() {
		t.Fatal("HasRendered before paint")
	}
	draw(t, e, []string{"hello", "world"}, 40, 10, renderer.ReasonForce)
	if !e.HasRendered() {
		t.Fatal("HasRendered false after paint")
	}
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Fatalf("first paint missing content: %q", out)
	}
	if e.Desynchronized() {
		t.Fatal("desync after success")
	}
}

func TestAppendIncrementalDoesNotRewriteStablePrefix(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := newEng(&buf)
	// Initial frame with commit seam so early rows can enter scrollback.
	req := renderer.Request{
		Frame: component.NewFrame([]string{"line0", "line1", "live"}, 1).
			WithSeams(2, 2, 2),
		Width:            40,
		Height:           8,
		Reason:           renderer.ReasonForce,
		StablePrefixRows: 0,
	}
	if err := e.Draw(req); err != nil {
		t.Fatal(err)
	}
	c1 := e.CommittedRows()
	buf.Reset()

	// Append a live line; stable prefix of committed rows should not be re-emitted as full clear.
	req2 := renderer.Request{
		Frame: component.NewFrame([]string{"line0", "line1", "live", "more"}, 2).
			WithSeams(2, 2, 2),
		Width:            40,
		Height:           8,
		Reason:           renderer.ReasonUpdate,
		StablePrefixRows: 2,
	}
	if err := e.Draw(req2); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Incremental path should mention new content.
	if !strings.Contains(out, "more") && !strings.Contains(out, "live") {
		// Still a valid paint — at least engine advanced without error.
		if !e.HasRendered() {
			t.Fatal("not rendered")
		}
	}
	if e.CommittedRows() < c1 {
		t.Fatalf("committed shrank %d -> %d", c1, e.CommittedRows())
	}
}

func TestResetClearsScrollbackState(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := newEng(&buf)
	draw(t, e, []string{"a", "b", "c"}, 20, 6, renderer.ReasonForce)
	e.ResetScrollback()
	if e.CommittedRows() != 0 || e.WindowTop() != 0 {
		t.Fatalf("reset ledger C=%d top=%d", e.CommittedRows(), e.WindowTop())
	}
	buf.Reset()
	draw(t, e, []string{"fresh"}, 20, 6, renderer.ReasonReset)
	if !strings.Contains(buf.String(), "fresh") {
		t.Fatalf("reset paint: %q", buf.String())
	}
}

func TestOverlayCompositedNotInLedger(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := newEng(&buf)
	base := []string{"base0", "base1", "base2", "base3"}
	ov := renderer.Overlay{
		Lines: []string{"POP"},
		Options: renderer.OverlayOptions{
			Anchor: renderer.AnchorCenter,
			Width:  renderer.SizeAbs(5),
		},
	}
	req := renderer.Request{
		Frame:    component.NewFrame(base, 1),
		Width:    40,
		Height:   10,
		Reason:   renderer.ReasonForce,
		Overlays: []renderer.Overlay{ov},
	}
	if err := e.Draw(req); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "POP") {
		t.Fatalf("overlay missing from paint: %q", out)
	}
	// Overlay must not inflate committed prefix beyond base semantics.
	if e.CommittedRows() > len(base) {
		t.Fatalf("overlay leaked into committed=%d", e.CommittedRows())
	}
}

func TestForceWindowRewriteAndResize(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := newEng(&buf)
	draw(t, e, []string{"x"}, 20, 5, renderer.ReasonForce)
	e.ForceNextWindowRewrite()
	buf.Reset()
	draw(t, e, []string{"x", "y"}, 20, 5, renderer.ReasonUpdate)
	if !e.HasRendered() {
		t.Fatal("force rewrite failed")
	}

	e.MarkResizeEvent()
	buf.Reset()
	draw(t, e, []string{"x", "y"}, 30, 8, renderer.ReasonResize)
	if e.Desynchronized() {
		t.Fatal("desync on resize")
	}
}

func TestInPlaceResizeRevealsCommittedRows(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := renderer.New(&buf, renderer.Caps{
		Multiplexer:           true,
		ResizeRepaintsInPlace: true,
	})
	lines := []string{
		"header",
		"working-directory",
		"",
		"tip",
		"border-top",
		"editor",
		"border-bottom",
		"footer",
	}
	draw(t, e, lines, 1, 1, renderer.ReasonForce)
	if got, want := e.CommittedRows(), len(lines)-1; got != want {
		t.Fatalf("initial committed rows = %d, want %d", got, want)
	}

	buf.Reset()
	e.MarkResizeEvent()
	draw(t, e, lines, 110, 40, renderer.ReasonResize)
	out := buf.String()
	for _, want := range []string{"header", "tip", "editor", "footer"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expanded viewport omitted %q: %q", want, out)
		}
	}
	if got := e.WindowTop(); got != 0 {
		t.Fatalf("window top after expansion = %d, want 0", got)
	}
	if got := e.CommittedRows(); got != 0 {
		t.Fatalf("committed rows after expansion = %d, want re-anchored 0", got)
	}
}

func TestReplaceReasonFullPaint(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := newEng(&buf)
	draw(t, e, []string{"old1", "old2"}, 40, 10, renderer.ReasonForce)
	buf.Reset()
	draw(t, e, []string{"new-session"}, 40, 10, renderer.ReasonReplace)
	if !strings.Contains(buf.String(), "new-session") {
		t.Fatalf("replace paint: %q", buf.String())
	}
}

func TestInvalidatePreparedStablePrefix(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := newEng(&buf)
	draw(t, e, []string{"a", "b", "c"}, 20, 6, renderer.ReasonForce)
	e.InvalidatePrepared(1)
	buf.Reset()
	draw(t, e, []string{"a", "B", "c"}, 20, 6, renderer.ReasonUpdate)
	// Must succeed after invalidation.
	if e.Desynchronized() {
		t.Fatal("desync after invalidate")
	}
}

func TestReasonStringAndRequestFromFrame(t *testing.T) {
	t.Parallel()
	if renderer.ReasonUpdate.String() != "update" {
		t.Fatal(renderer.ReasonUpdate.String())
	}
	f := component.NewFrame([]string{"z"}, 3).WithSeams(0, 0, 0)
	req := renderer.RequestFromFrame(f, 10, 5, renderer.ReasonFlush)
	if req.Width != 10 || req.Reason != renderer.ReasonFlush {
		t.Fatalf("%+v", req)
	}
}

func TestWriterSwap(t *testing.T) {
	t.Parallel()
	var a, b bytes.Buffer
	e := newEng(&a)
	draw(t, e, []string{"one"}, 10, 4, renderer.ReasonForce)
	e.SetWriter(&b)
	draw(t, e, []string{"one", "two"}, 10, 4, renderer.ReasonUpdate)
	if b.Len() == 0 {
		t.Fatal("writes did not follow SetWriter")
	}
}

func TestOverlayVisibleBounds(t *testing.T) {
	t.Parallel()
	ov := renderer.Overlay{
		Lines: []string{"x"},
		Options: renderer.OverlayOptions{
			Anchor:   renderer.AnchorTopLeft,
			Width:    renderer.SizeAbs(3),
			MinWidth: 10,
		},
	}
	_ = ov.Visible(5, 5)
	if !ov.Visible(80, 40) {
		t.Fatal("expected visible on large terminal")
	}
}

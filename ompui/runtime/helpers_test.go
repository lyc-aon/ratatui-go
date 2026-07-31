package runtime

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/michaelkelly/ratatui-go/ompui/termcaps"
)

func TestChunkForConPTYBoundsAndUTF8(t *testing.T) {
	t.Parallel()
	in := []byte("hello")
	ch := chunkForConPTY(in, 64)
	if len(ch) != 1 || string(ch[0]) != "hello" {
		t.Fatalf("%q", ch)
	}

	var b strings.Builder
	for range 10 {
		b.WriteString("abcdefghij\n")
	}
	data := []byte(b.String())
	chunks := chunkForConPTY(data, 30)
	if len(chunks) < 2 {
		t.Fatalf("expected split, got %d", len(chunks))
	}
	total := 0
	for _, c := range chunks {
		if len(c) > 30 {
			t.Fatalf("chunk oversized %d", len(c))
		}
		total += len(c)
		if !utf8.Valid(c) {
			t.Fatalf("invalid utf8 chunk %q", c)
		}
	}
	if total != len(data) {
		t.Fatalf("lost bytes %d != %d", total, len(data))
	}

	msg := []byte(strings.Repeat("界", 20))
	chunks = chunkForConPTY(msg, 10)
	rejoined := make([]byte, 0, len(msg))
	for _, c := range chunks {
		if len(c) > 10 {
			t.Fatalf("oversize %d", len(c))
		}
		if !utf8.Valid(c) {
			t.Fatalf("split mid-rune: %x", c)
		}
		rejoined = append(rejoined, c...)
	}
	if string(rejoined) != string(msg) {
		t.Fatal("rejoin mismatch")
	}
}

func TestAppearanceString(t *testing.T) {
	t.Parallel()
	if AppearanceDark.String() == "" || AppearanceLight.String() == "" {
		t.Fatal("empty")
	}
	if Appearance(99).String() == "" {
		t.Fatal("unknown empty")
	}
}

func TestFormatNotificationPaths(t *testing.T) {
	t.Parallel()
	var id int
	_ = formatNotification(termcaps.NotifyProtocol(""), false, Notification{
		Title: "t", Body: "b",
	}, &id)
	_ = formatNotification(termcaps.NotifyProtocolOSC9, false, Notification{
		Title: "hi", Body: "there",
	}, &id)
	out := formatNotification(termcaps.NotifyProtocolOSC99, true, Notification{
		Title: "title", Body: "body", Urgency: UrgencyNormal,
	}, &id)
	if out != "" && !strings.Contains(out, "99") && !strings.Contains(out, "title") {
		t.Logf("osc99=%q", out)
	}
}

func TestDA1OwnerFIFO(t *testing.T) {
	t.Parallel()
	var p probeRouter
	p.pushOwner(da1Owner{kind: da1Keyboard})
	p.pushOwner(da1Owner{kind: da1OSC11})
	if !p.hasOwner(da1Keyboard) || !p.hasOwner(da1OSC11) {
		t.Fatal("hasOwner")
	}
	o, ok := p.shiftOwner()
	if !ok || o.kind != da1Keyboard {
		t.Fatalf("fifo first=%+v", o)
	}
	o, ok = p.shiftOwner()
	if !ok || o.kind != da1OSC11 {
		t.Fatalf("fifo second=%+v", o)
	}
	if _, ok := p.shiftOwner(); ok {
		t.Fatal("empty shift")
	}
}

func TestChunkUTF8Helper(t *testing.T) {
	t.Parallel()
	parts := chunkUTF8(strings.Repeat("ä", 100), 16)
	if len(parts) < 2 {
		t.Fatalf("parts=%d", len(parts))
	}
	var join strings.Builder
	for _, p := range parts {
		if len(p) > 16 {
			t.Fatalf("part len %d", len(p))
		}
		if !utf8.ValidString(p) {
			t.Fatal("invalid part")
		}
		join.WriteString(p)
	}
	if join.String() != strings.Repeat("ä", 100) {
		t.Fatal("rejoin")
	}
}

func TestSequencesNonEmpty(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		seqBracketedPasteEnable, seqKittyPushLevel1, seqMouseSGREnable,
		seqHideCursor, seqEnterAltScreen, seqCPR, seqDA1,
	} {
		if s == "" || s[0] != '\x1b' {
			t.Fatalf("bad seq %q", s)
		}
	}
}
func TestOptionsWithDefaults(t *testing.T) {
	t.Parallel()
	o := (Options{}).withDefaults()
	if o.EventBuffer <= 0 {
		t.Fatalf("%+v", o)
	}
	if o.CPRTimeout <= 0 || o.KittyFallbackTimeout <= 0 {
		t.Fatalf("timeouts %+v", o)
	}
}

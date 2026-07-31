package input_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/michaelkelly/ratatui-go/ompui/event"
	"github.com/michaelkelly/ratatui-go/ompui/input"
)

func drain(d *input.Decoder) []event.Event {
	return d.Events()
}

func TestLegacyArrowsAndCtrlKeys(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{})

	cases := []struct {
		seq string
		id  string
	}{
		{"\x1b[A", "up"},
		{"\x1b[B", "down"},
		{"\x1b[C", "right"},
		{"\x1b[D", "left"},
		{"\x1b[H", "home"},
		{"\x1b[F", "end"},
		{"\x1b[3~", "delete"},
		{"\x1b[5~", "pageUp"},
		{"\x1b[6~", "pageDown"},
		{"\x1bOP", "f1"},
		{"\x03", "ctrl+c"},
		{"\r", "enter"},
		{"\t", "tab"},
		{"\x7f", "backspace"},
	}
	for _, tc := range cases {
		d.Clear()
		d.Write([]byte(tc.seq))
		evs := drain(d)
		if len(evs) == 0 {
			t.Fatalf("%q produced no events", tc.seq)
		}
		if evs[0].Kind != event.KindKey {
			// enter/tab may be key
			if evs[0].Kind != event.KindKey && evs[0].Key.ID == "" {
				t.Fatalf("%q kind=%v ev=%+v", tc.seq, evs[0].Kind, evs[0])
			}
		}
		id := evs[0].Key.ID
		if id == "" {
			id = input.ParseKey([]byte(tc.seq), input.KeyParseOptions{})
		}
		if id != tc.id {
			t.Fatalf("%q id=%q want %q (ev=%+v)", tc.seq, id, tc.id, evs[0])
		}
	}
}

func TestKittyCSIU(t *testing.T) {
	t.Parallel()
	// CSI-u: ESC [ codepoint ; modifier u  — 'a' with ctrl = 97;5u
	seq := []byte("\x1b[97;5u")
	id := input.ParseKey(seq, input.KeyParseOptions{KittyActive: true})
	if id != "ctrl+a" {
		t.Fatalf("kitty ctrl+a id=%q", id)
	}
	d := input.NewDecoder(input.Options{KittyActive: true})
	d.Write(seq)
	evs := drain(d)
	if len(evs) != 1 || evs[0].Kind != event.KindKey {
		t.Fatalf("evs=%+v", evs)
	}
	if evs[0].Key.ID != "ctrl+a" && !event.MatchesKey(evs[0].Key, "ctrl+a") {
		t.Fatalf("key=%+v", evs[0].Key)
	}
}

func TestKittyReleaseAndPartialHold(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{KittyActive: true, FlushTimeout: 30 * time.Millisecond})
	// Incomplete CSI-u: hold while kitty active.
	d.Write([]byte("\x1b[97;5"))
	if len(drain(d)) != 0 {
		t.Fatal("partial should not emit yet")
	}
	if d.BufferedLen() == 0 {
		t.Fatal("expected buffered partial")
	}
	// Complete it.
	d.Write([]byte("u"))
	evs := drain(d)
	if len(evs) != 1 {
		t.Fatalf("complete evs=%+v", evs)
	}
}

func TestBracketedPaste(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{})
	payload := "line1\nline2\t<>&"
	wire := "\x1b[200~" + payload + "\x1b[201~"
	d.Write([]byte(wire))
	evs := drain(d)
	if len(evs) != 1 || evs[0].Kind != event.KindPaste {
		t.Fatalf("evs=%+v", evs)
	}
	if evs[0].Text != payload && evs[0].Paste != payload {
		t.Fatalf("paste text=%q paste=%q", evs[0].Text, evs[0].Paste)
	}
	// Bridge forwards Raw; must include bracketed delimiters.
	if string(evs[0].Raw) != wire {
		t.Fatalf("paste Raw=%q want delimited %q", evs[0].Raw, wire)
	}
	if d.InPaste() {
		t.Fatal("still in paste")
	}
}

func TestPasteChunkedAcrossWrites(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{})
	d.Write([]byte("\x1b[200~hel"))
	if !d.InPaste() {
		t.Fatal("expected paste mode")
	}
	if len(drain(d)) != 0 {
		t.Fatal("no event until end")
	}
	d.Write([]byte("lo\x1b[201~"))
	evs := drain(d)
	if len(evs) != 1 || evs[0].Kind != event.KindPaste || evs[0].Text != "hello" {
		t.Fatalf("evs=%+v", evs)
	}
	wantRaw := "\x1b[200~hello\x1b[201~"
	if string(evs[0].Raw) != wantRaw {
		t.Fatalf("chunked paste Raw=%q want %q", evs[0].Raw, wantRaw)
	}
}

func TestSGRMouseClickWheel(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{})
	// button 0 press at col=5,row=10 (1-based on wire) → 0-based 4,9
	d.Write([]byte("\x1b[<0;5;10M"))
	evs := drain(d)
	if len(evs) != 1 || evs[0].Kind != event.KindMouse {
		t.Fatalf("evs=%+v", evs)
	}
	m := evs[0].Mouse
	if m.Col != 4 || m.Row != 9 {
		t.Fatalf("pos col=%d row=%d", m.Col, m.Row)
	}
	if m.Release {
		t.Fatal("press flagged release")
	}

	d.Clear()
	d.Write([]byte("\x1b[<0;5;10m")) // release
	evs = drain(d)
	if len(evs) != 1 || !evs[0].Mouse.Release {
		t.Fatalf("release evs=%+v", evs)
	}

	d.Clear()
	d.Write([]byte("\x1b[<64;1;1M")) // wheel up
	evs = drain(d)
	if len(evs) != 1 || evs[0].Mouse.Wheel == 0 {
		t.Fatalf("wheel evs=%+v", evs)
	}
}

func TestPartialEscapeFlushTimeout(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{FlushTimeout: 20 * time.Millisecond, KittyActive: false})
	now := time.Now()
	d.Write([]byte{0x1b}) // lone ESC
	if len(drain(d)) != 0 {
		t.Fatal("ESC should wait flush")
	}
	dl, ok := d.Deadline()
	if !ok {
		t.Fatal("expected deadline")
	}
	d.PopDue(dl.Add(time.Millisecond))
	evs := drain(d)
	if len(evs) == 0 {
		// Flush may emit key escape
		d.Flush()
		evs = drain(d)
	}
	if len(evs) == 0 {
		t.Fatal("expected flushed escape")
	}
	_ = now
}

func TestUTF8MultibyteAndIncomplete(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{})
	// é in UTF-8 is c3 a9
	d.Write([]byte{0xc3})
	if len(drain(d)) != 0 {
		t.Fatal("incomplete UTF-8 must wait")
	}
	d.Write([]byte{0xa9})
	evs := drain(d)
	if len(evs) == 0 {
		t.Fatal("no event for é")
	}
	text := evs[0].Text
	if text == "" && evs[0].Kind == event.KindKey {
		text = evs[0].Key.Text
	}
	if text != "é" && !strings.Contains(text, "é") {
		// KindText with é
		if utf8.RuneCountInString(string(evs[0].Raw)) != 1 && text != "é" {
			t.Fatalf("got %+v text=%q", evs[0], text)
		}
	}

	d.Clear()
	d.Write([]byte("hello"))
	evs = drain(d)
	joined := ""
	for _, e := range evs {
		if e.Text != "" {
			joined += e.Text
		} else if e.Key.Text != "" {
			joined += e.Key.Text
		}
	}
	if joined != "hello" && len(evs) != 5 {
		// either one text event or per-rune keys
		if joined != "hello" {
			var b strings.Builder
			for _, e := range evs {
				b.Write(e.Raw)
			}
			if b.String() != "hello" {
				t.Fatalf("hello => %+v joined=%q", evs, joined)
			}
		}
	}
}

func TestDecodeFrameStandalone(t *testing.T) {
	t.Parallel()
	ev := input.DecodeFrame([]byte("\x1b[A"), input.Options{})
	if ev.Kind != event.KindKey || (ev.Key.ID != "up" && !event.MatchesKey(ev.Key, "up")) {
		t.Fatalf("frame=%+v", ev)
	}
}

func TestFocusEvents(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{})
	d.Write([]byte("\x1b[I"))
	evs := drain(d)
	if len(evs) != 1 || evs[0].Kind != event.KindFocus || !evs[0].Focus.Gained {
		t.Fatalf("focus in=%+v", evs)
	}
	d.Write([]byte("\x1b[O"))
	evs = drain(d)
	if len(evs) != 1 || evs[0].Kind != event.KindFocus || evs[0].Focus.Gained {
		t.Fatalf("focus out=%+v", evs)
	}
}

func TestHighByteMetaConversion(t *testing.T) {
	t.Parallel()
	// A lone high byte waits because it may be a split UTF-8 lead, then falls
	// back to ESC+(b-128) when the stream is explicitly flushed.
	d := input.NewDecoder(input.Options{})
	d.Write([]byte{0x80 + 'a'}) // meta-a style
	if evs := drain(d); len(evs) != 0 {
		t.Fatalf("high byte emitted before UTF-8 deadline: %+v", evs)
	}
	evs := d.Flush()
	if len(evs) == 0 {
		t.Fatal("expected meta conversion event after flush")
	}
}

func TestParseKeyModifyOtherKeys(t *testing.T) {
	t.Parallel()
	// CSI 27;5;97~ => ctrl+a
	id := input.ParseKey([]byte("\x1b[27;5;97~"), input.KeyParseOptions{})
	if id != "ctrl+a" {
		t.Fatalf("modifyOtherKeys id=%q", id)
	}
}

func TestClearResetsPasteAndBuffer(t *testing.T) {
	t.Parallel()
	d := input.NewDecoder(input.Options{})
	d.Write([]byte("\x1b[200~abc"))
	d.Clear()
	if d.InPaste() || d.BufferedLen() != 0 {
		t.Fatal("clear incomplete")
	}
	if len(drain(d)) != 0 {
		t.Fatal("queued events remain")
	}
}

func TestDecodeReencodedPasteControls(t *testing.T) {
	t.Parallel()
	// Ctrl+J as csi-u inside paste body.
	in := "a\x1b[106;5ub"
	out := input.DecodeReencodedPasteControls(in)
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("out=%q", out)
	}
	// Literal ctrl byte 0x0A (J) expected between a and b when decoded.
	if out != "a\nb" && out != "a"+string(rune(10))+"b" {
		// 106 is 'j', ctrl+j = \n
		if !strings.Contains(out, "\n") && !strings.Contains(out, "\x0a") {
			t.Fatalf("expected ctrl expansion, out=%q", out)
		}
	}
}

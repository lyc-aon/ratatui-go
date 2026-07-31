package media_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/michaelkelly/ratatui-go/ompui/media"
)

// Tiny fixtures (all well under 10KiB).

// 1x1 red PNG
const png1x1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg=="

// Minimal JPEG with SOF0 announcing 3x2 (sniff only)
var jpegSniff = mustB64([]byte{
	0xFF, 0xD8, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x02, 0x00, 0x03, 0x01, 0x11, 0x00, 0xFF, 0xD9,
})

// GIF 4x5 header-only
var gif4x5 = mustB64([]byte("GIF89a\x04\x00\x05\x00\x00\x00\x00;"))

// WebP VP8X 8x6
var webpVP8X = mustB64(func() []byte {
	payload := []byte{
		'V', 'P', '8', 'X',
		10, 0, 0, 0,
		0, 0, 0, 0,
		7, 0, 0, // width-1 = 7 → 8
		5, 0, 0, // height-1 = 5 → 6
	}
	out := make([]byte, 0, 12+len(payload))
	out = append(out, 'R', 'I', 'F', 'F')
	size := uint32(4 + len(payload))
	out = append(out, byte(size), byte(size>>8), byte(size>>16), byte(size>>24))
	out = append(out, 'W', 'E', 'B', 'P')
	out = append(out, payload...)
	return out
}())

// Static lossy WebP 2x2 (real encoder output, 54 bytes)
const webpLossy2x2 = "UklGRi4AAABXRUJQVlA4ICIAAABwAQCdASoCAAIAAUAmJZQCdAFAAAD+/DeBV/fU6D4r4AAA"

// Real tiny GIF 2x2 for sixel decode
const gifReal2x2 = "R0lGODdhAgACAIEAAAoUHgAAAAAAAAAAACwAAAAAAgACAAAIBgABCAQQEAA7"

func mustB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func TestSniffPNGJPEGGIFWebP(t *testing.T) {
	pngRaw, err := base64.StdEncoding.DecodeString(png1x1)
	if err != nil {
		t.Fatal(err)
	}
	d, err := media.SniffDimensions(pngRaw, media.MIMEPNG)
	if err != nil || d.WidthPx != 1 || d.HeightPx != 1 {
		t.Fatalf("png: %#v err=%v", d, err)
	}

	jpegRaw, _ := base64.StdEncoding.DecodeString(jpegSniff)
	d, err = media.SniffDimensions(jpegRaw, media.MIMEJPEG)
	if err != nil || d.WidthPx != 3 || d.HeightPx != 2 {
		t.Fatalf("jpeg: %#v err=%v", d, err)
	}

	gifRaw, _ := base64.StdEncoding.DecodeString(gif4x5)
	d, err = media.SniffDimensions(gifRaw, media.MIMEGIF)
	if err != nil || d.WidthPx != 4 || d.HeightPx != 5 {
		t.Fatalf("gif: %#v err=%v", d, err)
	}

	webpRaw, _ := base64.StdEncoding.DecodeString(webpVP8X)
	d, err = media.SniffDimensions(webpRaw, media.MIMEWebP)
	if err != nil || d.WidthPx != 8 || d.HeightPx != 6 {
		t.Fatalf("webp vp8x: %#v err=%v", d, err)
	}

	d, err = media.SniffDimensionsBase64(png1x1, media.MIMEPNG)
	if err != nil || d.WidthPx != 1 {
		t.Fatalf("b64 png: %#v %v", d, err)
	}
	d, ok := media.GetImageDimensions(png1x1, media.MIMEPNG)
	if !ok || d.WidthPx != 1 {
		t.Fatalf("GetImageDimensions: %#v ok=%v", d, ok)
	}
	_, ok = media.GetImageDimensions("@@@@", media.MIMEPNG)
	if ok {
		t.Fatal("malformed should miss")
	}
}

func TestSniffMalformedAndUnsupported(t *testing.T) {
	_, err := media.SniffDimensions([]byte("not-an-image"), media.MIMEPNG)
	if !errors.Is(err, media.ErrMalformedImage) {
		t.Fatalf("png garbage: %v", err)
	}
	_, err = media.SniffDimensions([]byte{0x00, 0x01}, "image/bmp")
	if !errors.Is(err, media.ErrUnsupportedFormat) {
		t.Fatalf("bmp: %v", err)
	}
	pngRaw, _ := base64.StdEncoding.DecodeString(png1x1)
	d, err := media.SniffDimensions(pngRaw, "")
	if err != nil || d.WidthPx != 1 {
		t.Fatalf("magic sniff: %#v %v", d, err)
	}
	_, err = media.SniffDimensions(pngRaw[:10], media.MIMEPNG)
	if !errors.Is(err, media.ErrMalformedImage) {
		t.Fatalf("trunc png: %v", err)
	}
}

func TestStaticLossyWebPSixelDecode(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(webpLossy2x2)
	if err != nil {
		t.Fatal(err)
	}
	d, err := media.SniffDimensions(raw, media.MIMEWebP)
	if err != nil || d.WidthPx != 2 || d.HeightPx != 2 {
		t.Fatalf("webp dims: %#v err=%v", d, err)
	}
	seq, err := media.EncodeSixel(raw, 2, 2)
	if err != nil {
		t.Fatalf("sixel webp: %v", err)
	}
	if !strings.HasPrefix(seq, "\x1bP") {
		t.Fatalf("sixel missing DCS prefix: %q", seq[:min(20, len(seq))])
	}
	if !strings.HasSuffix(seq, "\x1b\\") {
		t.Fatalf("sixel missing ST: tail %q", seq[max(0, len(seq)-8):])
	}
	seq2, err := media.EncodeSixelBase64(webpLossy2x2, 2, 2)
	if err != nil || seq2 == "" {
		t.Fatalf("sixel b64: %v", err)
	}
}

func TestSixelPNGAndBounds(t *testing.T) {
	raw, _ := base64.StdEncoding.DecodeString(png1x1)
	seq, err := media.EncodeSixel(raw, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seq, "#") {
		t.Fatalf("sixel missing palette: %q", seq[:min(80, len(seq))])
	}
	_, err = media.EncodeSixel(raw, 0, 4)
	if err == nil {
		t.Fatal("zero dim should fail")
	}
	_, err = media.EncodeSixel(raw, media.MaxSixelEdge+1, 1)
	if !errors.Is(err, media.ErrSixelTooLarge) {
		t.Fatalf("edge bound: %v", err)
	}
	_, err = media.EncodeSixel([]byte("nope"), 2, 2)
	if !errors.Is(err, media.ErrSixelDecode) {
		t.Fatalf("decode err: %v", err)
	}
	graw, _ := base64.StdEncoding.DecodeString(gifReal2x2)
	if _, err := media.EncodeSixel(graw, 2, 2); err != nil {
		t.Fatalf("gif sixel: %v", err)
	}
}

func TestKittyByteInvariants(t *testing.T) {
	data := png1x1
	direct := media.EncodeKittyDirect(data, media.KittyDirectOptions{Columns: 2, Rows: 1, ImageID: 42})
	if !strings.HasPrefix(direct, "\x1b_G") || !strings.HasSuffix(direct, "\x1b\\") {
		t.Fatalf("direct framing: %q", direct[:min(40, len(direct))])
	}
	if !strings.Contains(direct, "a=T") || !strings.Contains(direct, "f=100") {
		t.Fatalf("direct params: %q", direct)
	}
	if !strings.Contains(direct, "i=42") || !strings.Contains(direct, "c=2") || !strings.Contains(direct, "r=1") {
		t.Fatalf("direct geometry: %q", direct)
	}
	if !strings.Contains(direct, data) {
		t.Fatal("direct lost base64 payload")
	}

	tx := media.EncodeKittyTransmit(data, 7)
	if !strings.Contains(tx, "a=t") || !strings.Contains(tx, "i=7") {
		t.Fatalf("transmit: %q", tx)
	}

	place := media.EncodeKittyPlacement(media.KittyPlacementOptions{
		ImageID: 7, PlacementID: 3, Columns: 4, Rows: 2,
	})
	if strings.Contains(place, data) {
		t.Fatal("placement must not carry base64")
	}
	if !strings.Contains(place, "a=p") || !strings.HasSuffix(place, "\x1b\\") {
		t.Fatalf("placement framing: %q", place)
	}

	del := media.EncodeKittyDeleteImage(99)
	if !strings.Contains(del, "a=d") || !strings.Contains(del, "d=I") || !strings.Contains(del, "i=99") {
		t.Fatalf("delete: %q", del)
	}
	dels := media.EncodeKittyDeleteImages([]uint32{1, 2})
	if strings.Count(dels, "\x1b_G") != 2 {
		t.Fatalf("delete multi count: %q", dels)
	}
	if media.EncodeKittyDeleteImages(nil) != "" {
		t.Fatal("empty deletes")
	}

	big := strings.Repeat("A", media.KittyChunkSize+10)
	chunked := media.EncodeKittyTransmit(big, 1)
	if strings.Count(chunked, "\x1b_G") < 2 {
		t.Fatalf("expected multi-chunk, got %d frames", strings.Count(chunked, "\x1b_G"))
	}
	if !strings.Contains(chunked, "m=1") || !strings.Contains(chunked, "m=0") {
		t.Fatalf("chunk m flags missing: %q", chunked[:min(120, len(chunked))])
	}

	if !media.KittyPlaceholdersFit(10, 10) {
		t.Fatal("10x10 should fit")
	}
	if media.KittyPlaceholdersFit(media.KittyPlaceholderMaxCells+1, 1) {
		t.Fatal("oversize cols should not fit")
	}
	grid := media.EncodeKittyPlaceholderGrid(media.KittyVirtualPlacementOptions{
		ImageID: 0x112233, Columns: 2, Rows: 2,
	})
	if len(grid) != 2 {
		t.Fatalf("grid rows %d", len(grid))
	}
	for _, row := range grid {
		if !strings.Contains(row, media.KittyPlaceholder) {
			t.Fatalf("missing placeholder: %q", row)
		}
		if !strings.Contains(row, "\x1b[38;2;") {
			t.Fatalf("missing fg id color: %q", row)
		}
	}
	lines := media.RenderKittyPlaceholderLines(media.KittyVirtualPlacementOptions{
		ImageID: 5, Columns: 1, Rows: 1,
	})
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "\x1b_G") {
		t.Fatalf("virtual lines: %#v", lines)
	}
}

func TestITerm2ByteInvariants(t *testing.T) {
	seq := media.EncodeITerm2(png1x1, media.ITerm2Options{Width: 3, Height: 2, Name: "x.png"})
	if !strings.HasPrefix(seq, "\x1b]1337;File=") {
		t.Fatalf("prefix: %q", seq[:min(40, len(seq))])
	}
	if !strings.HasSuffix(seq, "\x07") {
		t.Fatalf("missing BEL: %q", seq[max(0, len(seq)-4):])
	}
	if !strings.Contains(seq, "inline=1") || !strings.Contains(seq, "width=3") || !strings.Contains(seq, "height=2") {
		t.Fatalf("params: %q", seq)
	}
	if !strings.Contains(seq, ":"+png1x1) {
		t.Fatal("payload not appended raw")
	}
	if !strings.Contains(seq, ";name=") {
		t.Fatal("name missing")
	}
	off := false
	seq2 := media.EncodeITerm2("AA==", media.ITerm2Options{Inline: &off})
	if !strings.Contains(seq2, "inline=0") {
		t.Fatalf("inline=0: %q", seq2)
	}
}

func TestImageBudgetDemotionPurgeTransmitOnce(t *testing.T) {
	b := media.NewImageBudget(2, func() {}, media.SequentialIDSource(1))
	if !b.Enabled() || b.Cap() != 2 {
		t.Fatalf("cap: enabled=%v cap=%d", b.Enabled(), b.Cap())
	}

	idA := b.AcquireID("a")
	idB := b.AcquireID("b")
	idC := b.AcquireID("c")
	if idA == 0 || idA == idB || idB == idC {
		t.Fatalf("ids %d %d %d", idA, idB, idC)
	}
	if b.AcquireID("a") != idA {
		t.Fatal("key unstable")
	}

	b.BeginPass(false)
	if b.Observe(idA) || b.Observe(idB) {
		t.Fatal("under cap should not suppress")
	}
	if b.EndPass() {
		t.Fatal("no purge under cap")
	}

	if !b.ShouldTransmit(idA) {
		t.Fatal("should transmit fresh")
	}
	b.EnqueueTransmit(idA, "TX-A")
	b.EnqueueTransmit(idA, "TX-A-dup")
	if !b.HasPendingTransmits() {
		t.Fatal("pending")
	}
	got := b.TakeTransmits()
	if len(got) != 1 || got[0] != "TX-A" {
		t.Fatalf("transmit once: %#v", got)
	}
	if b.ShouldTransmit(idA) {
		t.Fatal("already transmitted")
	}
	b.EnqueueTransmit(idA, "again")
	if b.HasPendingTransmits() {
		t.Fatal("no re-queue after transmit")
	}

	// Grow past cap so reconcile plans demotion of the oldest image.
	b.BeginPass(false)
	_ = b.Observe(idA)
	_ = b.Observe(idB)
	_ = b.Observe(idC)
	_ = b.EndPass()

	// Next full pass applies the planned demotion.
	b.BeginPass(false)
	s0 := b.Observe(idA)
	s1 := b.Observe(idB)
	s2 := b.Observe(idC)
	if s0 && s1 && s2 {
		t.Fatal("all suppressed with cap=2")
	}
	if !s0 && !s1 && !s2 {
		t.Fatalf("expected at least one demotion suppress A=%v B=%v C=%v", s0, s1, s2)
	}
	purging := b.EndPass()
	if s0 && purging {
		ids := b.TakePurgeIDs()
		if len(ids) == 0 {
			t.Fatal("purge flag without ids")
		}
		joined := media.EncodeKittyDeleteImages(ids)
		if !strings.Contains(joined, "a=d") {
			t.Fatalf("purge seq: %q", joined)
		}
		for _, id := range ids {
			if !b.ShouldTransmit(id) {
				t.Fatalf("purged id %d should retransmit", id)
			}
		}
	}

	b.EnqueueTransmit(idB, "TX-B")
	_ = b.TakeTransmits()
	b.ForgetTransmitted()
	if !b.ShouldTransmit(idB) {
		t.Fatal("forget should allow retransmit")
	}

	b2 := media.NewImageBudget(0, nil, media.SequentialIDSource(10))
	if b2.Enabled() {
		t.Fatal("cap 0 disables")
	}
	b2.BeginPass(false)
	if b2.Observe(1) {
		t.Fatal("unlimited never suppresses")
	}
	b2.EndPass()
}

func TestDetectKittyPlaceholders(t *testing.T) {
	if !media.DetectKittyUnicodePlaceholdersSupport("kitty", nil) {
		t.Fatal("kitty default on")
	}
	if !media.DetectKittyUnicodePlaceholdersSupport("ghostty", nil) {
		t.Fatal("ghostty default on")
	}
	if media.DetectKittyUnicodePlaceholdersSupport("xterm", nil) {
		t.Fatal("xterm default off")
	}
	if media.DetectKittyUnicodePlaceholdersSupport("kitty", map[string]string{"PI_NO_KITTY_PLACEHOLDERS": "1"}) {
		t.Fatal("force off")
	}
	if !media.DetectKittyUnicodePlaceholdersSupport("xterm", map[string]string{"PI_KITTY_PLACEHOLDERS": "1"}) {
		t.Fatal("force on")
	}
}

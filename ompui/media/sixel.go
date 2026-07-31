package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"strings"

	"golang.org/x/image/webp"
)

// Sixel limits — refuse allocation bombs and oversize terminal dumps.
const (
	// MaxSixelPixels is width*height ceiling after resize (≈ 4M px).
	MaxSixelPixels = 4_000_000
	// MaxSixelEdge is the max width or height in pixels.
	MaxSixelEdge = 4096
	// SixelPaletteSize is the quantized palette size (DEC sixel common max).
	SixelPaletteSize = 256
	// sixelTransparentAlpha drops pixels below this alpha (0-255).
	sixelTransparentAlpha = 128
)

// ErrSixelTooLarge is returned when the target raster exceeds bounds.
var ErrSixelTooLarge = errors.New("media: sixel target exceeds size limit")

// ErrSixelDecode is returned when the source image cannot be decoded.
var ErrSixelDecode = errors.New("media: sixel source decode failed")

// EncodeSixel decodes image bytes (PNG/JPEG/GIF/WebP), resizes to target
// pixel dimensions, quantizes to <= SixelPaletteSize colors, and emits a DCS
// sixel sequence (ESC P … ST). Pure Go, no cgo, no external process.
//
// Static lossy and lossless WebP images are decoded with golang.org/x/image.
// Animated WebP is not frame-selected; unsupported payloads return
// ErrSixelDecode while dimension sniffing remains available.
func EncodeSixel(data []byte, targetWidthPx, targetHeightPx int) (string, error) {
	if targetWidthPx <= 0 || targetHeightPx <= 0 {
		return "", errors.New("media: sixel target dimensions must be > 0")
	}
	if targetWidthPx > MaxSixelEdge || targetHeightPx > MaxSixelEdge {
		return "", ErrSixelTooLarge
	}
	if int64(targetWidthPx)*int64(targetHeightPx) > MaxSixelPixels {
		return "", ErrSixelTooLarge
	}
	if len(data) > MaxSniffBytes {
		return "", ErrImageTooLarge
	}

	src, err := decodeRaster(data)
	if err != nil {
		return "", err
	}
	resized := resizeNearest(src, targetWidthPx, targetHeightPx)
	pal, index := quantizeMedianCut(resized, SixelPaletteSize)
	return encodeSixelIndexed(index, targetWidthPx, targetHeightPx, pal), nil
}

// EncodeSixelBase64 decodes base64 then EncodeSixel.
func EncodeSixelBase64(base64Data string, targetWidthPx, targetHeightPx int) (string, error) {
	raw, err := DecodeBase64Bounded(base64Data, MaxSniffBytes)
	if err != nil {
		return "", err
	}
	return EncodeSixel(raw, targetWidthPx, targetHeightPx)
}

func decodeRaster(data []byte) (image.Image, error) {
	var (
		img image.Image
		err error
	)
	if isWebP(data) {
		img, err = webp.Decode(bytes.NewReader(data))
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSixelDecode, err)
	}
	return img, nil
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// ---------------------------------------------------------------------------
// Resize, quantize, sixel bitstream
// ---------------------------------------------------------------------------

func resizeNearest(src image.Image, tw, th int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, tw, th))
	if sw == tw && sh == th {
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		return dst
	}
	for y := range th {
		sy := b.Min.Y + y*sh/th
		for x := range tw {
			sx := b.Min.X + x*sw/tw
			r, g, bl, a := src.At(sx, sy).RGBA()
			off := dst.PixOffset(x, y)
			dst.Pix[off+0] = uint8(r >> 8)
			dst.Pix[off+1] = uint8(g >> 8)
			dst.Pix[off+2] = uint8(bl >> 8)
			dst.Pix[off+3] = uint8(a >> 8)
		}
	}
	return dst
}

type qColor struct{ r, g, b, a uint8 }

type qPix struct{ r, g, b uint8 }

func quantizeMedianCut(img *image.NRGBA, maxColors int) ([]qColor, []uint8) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	n := w * h
	pts := make([]qPix, 0, n)
	for y := range h {
		for x := range w {
			o := img.PixOffset(x, y)
			if img.Pix[o+3] < sixelTransparentAlpha {
				continue
			}
			pts = append(pts, qPix{img.Pix[o], img.Pix[o+1], img.Pix[o+2]})
		}
	}
	if maxColors < 1 {
		maxColors = 1
	}
	if maxColors > SixelPaletteSize {
		maxColors = SixelPaletteSize
	}
	if len(pts) == 0 {
		return []qColor{{0, 0, 0, 0}}, make([]uint8, n)
	}

	type box struct {
		start, end             int
		r0, r1, g0, g1, b0, b1 uint8
	}
	work := pts
	recompute := func(b *box) {
		b.r0, b.g0, b.b0 = 255, 255, 255
		b.r1, b.g1, b.b1 = 0, 0, 0
		for i := b.start; i < b.end; i++ {
			p := work[i]
			if p.r < b.r0 {
				b.r0 = p.r
			}
			if p.r > b.r1 {
				b.r1 = p.r
			}
			if p.g < b.g0 {
				b.g0 = p.g
			}
			if p.g > b.g1 {
				b.g1 = p.g
			}
			if p.b < b.b0 {
				b.b0 = p.b
			}
			if p.b > b.b1 {
				b.b1 = p.b
			}
		}
	}
	boxes := []box{{start: 0, end: len(work)}}
	recompute(&boxes[0])

	for len(boxes) < maxColors {
		bi, best := -1, 0
		for i, b := range boxes {
			if b.end-b.start < 2 {
				continue
			}
			dr := int(b.r1 - b.r0)
			dg := int(b.g1 - b.g0)
			db := int(b.b1 - b.b0)
			rng := dr
			if dg > rng {
				rng = dg
			}
			if db > rng {
				rng = db
			}
			if rng > best {
				best = rng
				bi = i
			}
		}
		if bi < 0 {
			break
		}
		b := boxes[bi]
		dr := int(b.r1 - b.r0)
		dg := int(b.g1 - b.g0)
		db := int(b.b1 - b.b0)
		ch := 0
		if dg >= dr && dg >= db {
			ch = 1
		} else if db >= dr && db >= dg {
			ch = 2
		}
		medianCutSort(work[b.start:b.end], ch)
		mid := (b.start + b.end) / 2
		if mid == b.start {
			mid++
		}
		left := box{start: b.start, end: mid}
		right := box{start: mid, end: b.end}
		recompute(&left)
		recompute(&right)
		boxes[bi] = left
		boxes = append(boxes, right)
	}

	pal := make([]qColor, len(boxes))
	for i, b := range boxes {
		var rs, gs, bs, cnt int
		for j := b.start; j < b.end; j++ {
			p := work[j]
			rs += int(p.r)
			gs += int(p.g)
			bs += int(p.b)
			cnt++
		}
		if cnt == 0 {
			cnt = 1
		}
		pal[i] = qColor{uint8(rs / cnt), uint8(gs / cnt), uint8(bs / cnt), 255}
	}

	idx := make([]uint8, n)
	for y := range h {
		for x := range w {
			o := img.PixOffset(x, y)
			pidx := y*w + x
			if img.Pix[o+3] < sixelTransparentAlpha {
				continue
			}
			r, g, bl := img.Pix[o], img.Pix[o+1], img.Pix[o+2]
			bestI, bestD := 0, 1<<30
			for i, c := range pal {
				dr := int(r) - int(c.r)
				dg := int(g) - int(c.g)
				db := int(bl) - int(c.b)
				d := dr*dr + dg*dg + db*db
				if d < bestD {
					bestD = d
					bestI = i
				}
			}
			idx[pidx] = uint8(bestI)
		}
	}
	return pal, idx
}

func medianCutSort(pts []qPix, channel int) {
	n := len(pts)
	for gap := n / 2; gap > 0; gap /= 2 {
		for i := gap; i < n; i++ {
			tmp := pts[i]
			j := i
			for j >= gap {
				a := pts[j-gap]
				less := false
				switch channel {
				case 0:
					less = a.r > tmp.r
				case 1:
					less = a.g > tmp.g
				default:
					less = a.b > tmp.b
				}
				if !less {
					break
				}
				pts[j] = a
				j -= gap
			}
			pts[j] = tmp
		}
	}
}

func encodeSixelIndexed(index []uint8, w, h int, pal []qColor) string {
	var b strings.Builder
	// DCS raster attributes: aspect 1:1, width, height
	b.WriteString("\x1bP0;0;0q\"1;1;")
	b.WriteString(strconv.Itoa(w))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(h))

	for i, c := range pal {
		b.WriteByte('#')
		b.WriteString(strconv.Itoa(i))
		b.WriteString(";2;")
		b.WriteString(strconv.Itoa(int(c.r) * 100 / 255))
		b.WriteByte(';')
		b.WriteString(strconv.Itoa(int(c.g) * 100 / 255))
		b.WriteByte(';')
		b.WriteString(strconv.Itoa(int(c.b) * 100 / 255))
	}

	for y0 := 0; y0 < h; y0 += 6 {
		bandH := 6
		if y0+bandH > h {
			bandH = h - y0
		}
		used := make([]bool, len(pal))
		for y := range bandH {
			row := (y0 + y) * w
			for x := range w {
				used[index[row+x]] = true
			}
		}
		first := true
		for ci, ok := range used {
			if !ok {
				continue
			}
			if !first {
				b.WriteByte('$')
			}
			first = false
			b.WriteByte('#')
			b.WriteString(strconv.Itoa(ci))

			runChar := byte(0)
			runLen := 0
			flush := func() {
				if runLen == 0 {
					return
				}
				ch := rune('?' + runChar)
				if runLen >= 3 {
					b.WriteByte('!')
					b.WriteString(strconv.Itoa(runLen))
					b.WriteRune(ch)
				} else {
					for range runLen {
						b.WriteRune(ch)
					}
				}
				runLen = 0
			}
			for x := range w {
				var six byte
				for bit := range bandH {
					if index[(y0+bit)*w+x] == uint8(ci) {
						six |= 1 << bit
					}
				}
				if runLen == 0 {
					runChar = six
					runLen = 1
				} else if six == runChar {
					runLen++
				} else {
					flush()
					runChar = six
					runLen = 1
				}
			}
			flush()
		}
		b.WriteByte('-')
	}
	b.WriteString("\x1b\\")
	return b.String()
}

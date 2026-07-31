package media

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/termcaps"
)

// MaxSniffBytes is the hard ceiling for base64→binary decode during dimension
// sniffing. Larger payloads still work for Kitty/iTerm (they stream base64 as
// received) but refuse allocation-bomb sniff/decode paths.
const MaxSniffBytes = 64 << 20 // 64 MiB decoded

// ErrUnsupportedFormat is returned when the mime/type is outside the supported
// set (PNG/JPEG/GIF/WebP).
var ErrUnsupportedFormat = errors.New("media: unsupported image format")

// ErrMalformedImage is returned when the payload is truncated or corrupt.
var ErrMalformedImage = errors.New("media: malformed image data")

// ErrImageTooLarge is returned when decoded size would exceed MaxSniffBytes.
var ErrImageTooLarge = errors.New("media: image data exceeds size limit")

// Supported MIME types for sniff + Sixel decode.
const (
	MIMEPNG  = "image/png"
	MIMEJPEG = "image/jpeg"
	MIMEGIF  = "image/gif"
	MIMEWebP = "image/webp"
)

// Dimensions is an alias of termcaps.ImageDimensions for callers that only
// import media.
type Dimensions = termcaps.ImageDimensions

// DecodeBase64Bounded decodes s as standard or raw base64, refusing payloads
// whose decoded size would exceed limit. limit <= 0 uses MaxSniffBytes.
// Does not allocate the full output when the encoded length already implies
// an oversize payload.
func DecodeBase64Bounded(s string, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = MaxSniffBytes
	}
	// Strip whitespace that some transporters insert.
	if strings.IndexByte(s, '\n') >= 0 || strings.IndexByte(s, '\r') >= 0 || strings.IndexByte(s, ' ') >= 0 {
		s = stripWS(s)
	}
	encLen := len(s)
	// base64 expands 3 bytes → 4 chars; decoded ≈ enc*3/4.
	if encLen > 0 {
		est := encLen/4*3 + 3
		if est > limit+3 && encLen > (limit/3+1)*4 {
			return nil, ErrImageTooLarge
		}
	}
	// Prefer StdEncoding; fall back to RawStdEncoding.
	out := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
	n, err := base64.StdEncoding.Decode(out, []byte(s))
	if err != nil {
		out = make([]byte, base64.RawStdEncoding.DecodedLen(len(s)))
		n, err = base64.RawStdEncoding.Decode(out, []byte(s))
		if err != nil {
			return nil, ErrMalformedImage
		}
	}
	if n > limit {
		return nil, ErrImageTooLarge
	}
	return out[:n], nil
}

func stripWS(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// SniffDimensions reads width/height from encoded image bytes without a full
// decode. Supports PNG, JPEG, GIF, WebP. Returns ErrUnsupportedFormat /
// ErrMalformedImage on failure.
func SniffDimensions(data []byte, mimeType string) (Dimensions, error) {
	mimeType = normalizeMIME(mimeType)
	switch mimeType {
	case MIMEPNG:
		return pngDimensions(data)
	case MIMEJPEG, "image/jpg":
		return jpegDimensions(data)
	case MIMEGIF:
		return gifDimensions(data)
	case MIMEWebP:
		return webpDimensions(data)
	default:
		// Try magic-byte sniff when mime is empty/unknown.
		if mimeType == "" {
			if d, err := pngDimensions(data); err == nil {
				return d, nil
			}
			if d, err := jpegDimensions(data); err == nil {
				return d, nil
			}
			if d, err := gifDimensions(data); err == nil {
				return d, nil
			}
			if d, err := webpDimensions(data); err == nil {
				return d, nil
			}
		}
		return Dimensions{}, ErrUnsupportedFormat
	}
}

// SniffDimensionsBase64 decodes base64 then sniffs. Refuses oversize payloads.
func SniffDimensionsBase64(base64Data, mimeType string) (Dimensions, error) {
	raw, err := DecodeBase64Bounded(base64Data, MaxSniffBytes)
	if err != nil {
		return Dimensions{}, err
	}
	return SniffDimensions(raw, mimeType)
}

// GetImageDimensions matches OMP getImageDimensions: nil-style miss returns
// ok=false without error detail.
func GetImageDimensions(base64Data, mimeType string) (Dimensions, bool) {
	d, err := SniffDimensionsBase64(base64Data, mimeType)
	if err != nil {
		return Dimensions{}, false
	}
	return d, true
}

func normalizeMIME(m string) string {
	m = strings.TrimSpace(strings.ToLower(m))
	if i := strings.IndexByte(m, ';'); i >= 0 {
		m = strings.TrimSpace(m[:i])
	}
	return m
}

func pngDimensions(data []byte) (Dimensions, error) {
	if len(data) < 24 {
		return Dimensions{}, ErrMalformedImage
	}
	if data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4e || data[3] != 0x47 {
		return Dimensions{}, ErrMalformedImage
	}
	w := binary.BigEndian.Uint32(data[16:20])
	h := binary.BigEndian.Uint32(data[20:24])
	if w == 0 || h == 0 {
		return Dimensions{}, ErrMalformedImage
	}
	return Dimensions{WidthPx: int(w), HeightPx: int(h)}, nil
}

func jpegDimensions(data []byte) (Dimensions, error) {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return Dimensions{}, ErrMalformedImage
	}
	offset := 2
	for offset < len(data)-9 {
		if data[offset] != 0xff {
			offset++
			continue
		}
		marker := data[offset+1]
		// SOF0..SOF2 carry dimensions.
		if marker >= 0xc0 && marker <= 0xc2 {
			h := binary.BigEndian.Uint16(data[offset+5 : offset+7])
			w := binary.BigEndian.Uint16(data[offset+7 : offset+9])
			if w == 0 || h == 0 {
				return Dimensions{}, ErrMalformedImage
			}
			return Dimensions{WidthPx: int(w), HeightPx: int(h)}, nil
		}
		if offset+3 >= len(data) {
			return Dimensions{}, ErrMalformedImage
		}
		length := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		if length < 2 {
			return Dimensions{}, ErrMalformedImage
		}
		offset += 2 + length
	}
	return Dimensions{}, ErrMalformedImage
}

func gifDimensions(data []byte) (Dimensions, error) {
	if len(data) < 10 {
		return Dimensions{}, ErrMalformedImage
	}
	sig := string(data[0:6])
	if sig != "GIF87a" && sig != "GIF89a" {
		return Dimensions{}, ErrMalformedImage
	}
	w := binary.LittleEndian.Uint16(data[6:8])
	h := binary.LittleEndian.Uint16(data[8:10])
	if w == 0 || h == 0 {
		return Dimensions{}, ErrMalformedImage
	}
	return Dimensions{WidthPx: int(w), HeightPx: int(h)}, nil
}

func webpDimensions(data []byte) (Dimensions, error) {
	if len(data) < 30 {
		return Dimensions{}, ErrMalformedImage
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return Dimensions{}, ErrMalformedImage
	}
	chunk := string(data[12:16])
	switch chunk {
	case "VP8 ":
		if len(data) < 30 {
			return Dimensions{}, ErrMalformedImage
		}
		w := int(binary.LittleEndian.Uint16(data[26:28]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(data[28:30]) & 0x3fff)
		if w == 0 || h == 0 {
			return Dimensions{}, ErrMalformedImage
		}
		return Dimensions{WidthPx: w, HeightPx: h}, nil
	case "VP8L":
		if len(data) < 25 {
			return Dimensions{}, ErrMalformedImage
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		w := int(bits&0x3fff) + 1
		h := int((bits>>14)&0x3fff) + 1
		return Dimensions{WidthPx: w, HeightPx: h}, nil
	case "VP8X":
		if len(data) < 30 {
			return Dimensions{}, ErrMalformedImage
		}
		w := int(data[24]) | int(data[25])<<8 | int(data[26])<<16
		h := int(data[27]) | int(data[28])<<8 | int(data[29])<<16
		w++
		h++
		if w == 0 || h == 0 {
			return Dimensions{}, ErrMalformedImage
		}
		return Dimensions{WidthPx: w, HeightPx: h}, nil
	default:
		return Dimensions{}, ErrMalformedImage
	}
}

// CalculateImageFit delegates to termcaps with default cell fill-in.
func CalculateImageFit(image Dimensions, maxWidthCells, maxHeightCells int, cell termcaps.CellDimensions) termcaps.ImageFit {
	return termcaps.CalculateImageFit(image, termcaps.ImageFitOptions{
		MaxWidthCells:  maxWidthCells,
		MaxHeightCells: maxHeightCells,
	}, cell)
}

// CalculateImageRows delegates to termcaps.
func CalculateImageRows(image Dimensions, targetWidthCells int, cell termcaps.CellDimensions) int {
	return termcaps.CalculateImageRows(image, targetWidthCells, cell)
}

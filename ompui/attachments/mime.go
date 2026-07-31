package attachments

// Image MIME sniff from magic bytes — mirrors packages/utils/src/mime.ts
// (PNG / JPEG / GIF / WebP only).

const (
	MimePNG  = "image/png"
	MimeJPEG = "image/jpeg"
	MimeGIF  = "image/gif"
	MimeWebP = "image/webp"
)

// SupportedImageMIME lists MIME types this package will attach as ImageContent.
var SupportedImageMIME = []string{MimePNG, MimeJPEG, MimeGIF, MimeWebP}

var (
	pngMagic      = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	jpegMagic     = []byte{0xff, 0xd8, 0xff}
	webpRIFFMagic = []byte{0x52, 0x49, 0x46, 0x46} // "RIFF"
	webpMagic     = []byte{0x57, 0x45, 0x42, 0x50} // "WEBP"
	gif87a        = []byte("GIF87a")
	gif89a        = []byte("GIF89a")
)

func magicEquals(b []byte, off int, magic []byte) bool {
	if off < 0 || len(b) < off+len(magic) {
		return false
	}
	for i := range magic {
		if b[off+i] != magic[i] {
			return false
		}
	}
	return true
}

// SniffImageMIME returns a supported image MIME type from content magic, or "".
func SniffImageMIME(header []byte) string {
	if magicEquals(header, 0, pngMagic) {
		return MimePNG
	}
	if magicEquals(header, 0, jpegMagic) {
		return MimeJPEG
	}
	if magicEquals(header, 0, gif87a) || magicEquals(header, 0, gif89a) {
		return MimeGIF
	}
	if magicEquals(header, 0, webpRIFFMagic) && magicEquals(header, 8, webpMagic) {
		return MimeWebP
	}
	return ""
}

// looksBinary reports whether b is unlikely to be useful UTF-8 text.
// NUL bytes or a high control-byte ratio → binary (non-image handled by caller).
func looksBinary(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	n := len(b)
	if n > 8192 {
		n = 8192
	}
	var control int
	for i := 0; i < n; i++ {
		c := b[i]
		if c == 0 {
			return true
		}
		// Allow common whitespace; count other C0 controls + DEL.
		if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
			control++
		} else if c == 0x7f {
			control++
		}
	}
	// >30% suspicious controls in the sample → binary.
	return control*10 > n*3
}

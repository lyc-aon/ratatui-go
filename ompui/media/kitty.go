package media

import (
	"strconv"
	"strings"
)

// Kitty APC chunk size for base64 payload splits (matches OMP / kitty docs).
const KittyChunkSize = 4096

// KittyPlaceholder is U+10EEEE (Plane 16 PUA), the Kitty Unicode placeholder
// base character. Same value as termcaps.KittyPlaceholder.
const KittyPlaceholder = "\U0010eeee"

// KittyPlaceholderMaxCells is the largest row/column index addressable with a
// single diacritic (len of the 297-entry table).
const KittyPlaceholderMaxCells = 297

// rowColumnDiacritics is the exact kitty gen/rowcolumn-diacritics.txt table
// (Unicode 6.0.0 NSM set), 297 entries. Index i → codepoint used as the
// combining mark naming a placeholder cell's row or column.
var rowColumnDiacritics = [...]rune{
	0x305, 0x30d, 0x30e, 0x310, 0x312, 0x33d, 0x33e, 0x33f, 0x346, 0x34a, 0x34b, 0x34c, 0x350, 0x351, 0x352, 0x357,
	0x35b, 0x363, 0x364, 0x365, 0x366, 0x367, 0x368, 0x369, 0x36a, 0x36b, 0x36c, 0x36d, 0x36e, 0x36f, 0x483, 0x484,
	0x485, 0x486, 0x487, 0x592, 0x593, 0x594, 0x595, 0x597, 0x598, 0x599, 0x59c, 0x59d, 0x59e, 0x59f, 0x5a0, 0x5a1,
	0x5a8, 0x5a9, 0x5ab, 0x5ac, 0x5af, 0x5c4, 0x610, 0x611, 0x612, 0x613, 0x614, 0x615, 0x616, 0x617, 0x657, 0x658,
	0x659, 0x65a, 0x65b, 0x65d, 0x65e, 0x6d6, 0x6d7, 0x6d8, 0x6d9, 0x6da, 0x6db, 0x6dc, 0x6df, 0x6e0, 0x6e1, 0x6e2,
	0x6e4, 0x6e7, 0x6e8, 0x6eb, 0x6ec, 0x730, 0x732, 0x733, 0x735, 0x736, 0x73a, 0x73d, 0x73f, 0x740, 0x741, 0x743,
	0x745, 0x747, 0x749, 0x74a, 0x7eb, 0x7ec, 0x7ed, 0x7ee, 0x7ef, 0x7f0, 0x7f1, 0x7f3, 0x816, 0x817, 0x818, 0x819,
	0x81b, 0x81c, 0x81d, 0x81e, 0x81f, 0x820, 0x821, 0x822, 0x823, 0x825, 0x826, 0x827, 0x829, 0x82a, 0x82b, 0x82c,
	0x82d, 0x951, 0x953, 0x954, 0xf82, 0xf83, 0xf86, 0xf87, 0x135d, 0x135e, 0x135f, 0x17dd, 0x193a, 0x1a17, 0x1a75,
	0x1a76, 0x1a77, 0x1a78, 0x1a79, 0x1a7a, 0x1a7b, 0x1a7c, 0x1b6b, 0x1b6d, 0x1b6e, 0x1b6f, 0x1b70, 0x1b71, 0x1b72,
	0x1b73, 0x1cd0, 0x1cd1, 0x1cd2, 0x1cda, 0x1cdb, 0x1ce0, 0x1dc0, 0x1dc1, 0x1dc3, 0x1dc4, 0x1dc5, 0x1dc6, 0x1dc7,
	0x1dc8, 0x1dc9, 0x1dcb, 0x1dcc, 0x1dd1, 0x1dd2, 0x1dd3, 0x1dd4, 0x1dd5, 0x1dd6, 0x1dd7, 0x1dd8, 0x1dd9, 0x1dda,
	0x1ddb, 0x1ddc, 0x1ddd, 0x1dde, 0x1ddf, 0x1de0, 0x1de1, 0x1de2, 0x1de3, 0x1de4, 0x1de5, 0x1de6, 0x1dfe, 0x20d0,
	0x20d1, 0x20d4, 0x20d5, 0x20d6, 0x20d7, 0x20db, 0x20dc, 0x20e1, 0x20e7, 0x20e9, 0x20f0, 0x2cef, 0x2cf0, 0x2cf1,
	0x2de0, 0x2de1, 0x2de2, 0x2de3, 0x2de4, 0x2de5, 0x2de6, 0x2de7, 0x2de8, 0x2de9, 0x2dea, 0x2deb, 0x2dec, 0x2ded,
	0x2dee, 0x2def, 0x2df0, 0x2df1, 0x2df2, 0x2df3, 0x2df4, 0x2df5, 0x2df6, 0x2df7, 0x2df8, 0x2df9, 0x2dfa, 0x2dfb,
	0x2dfc, 0x2dfd, 0x2dfe, 0x2dff, 0xa66f, 0xa67c, 0xa67d, 0xa6f0, 0xa6f1, 0xa8e0, 0xa8e1, 0xa8e2, 0xa8e3, 0xa8e4,
	0xa8e5, 0xa8e6, 0xa8e7, 0xa8e8, 0xa8e9, 0xa8ea, 0xa8eb, 0xa8ec, 0xa8ed, 0xa8ee, 0xa8ef, 0xa8f0, 0xa8f1, 0xaab0,
	0xaab2, 0xaab3, 0xaab7, 0xaab8, 0xaabe, 0xaabf, 0xaac1, 0xfe20, 0xfe21, 0xfe22, 0xfe23, 0xfe24, 0xfe25, 0xfe26,
	0x10a0f, 0x10a38, 0x1d185, 0x1d186, 0x1d187, 0x1d188, 0x1d189, 0x1d1aa, 0x1d1ab, 0x1d1ac, 0x1d1ad, 0x1d242, 0x1d243,
	0x1d244,
}

func init() {
	if len(rowColumnDiacritics) != KittyPlaceholderMaxCells {
		panic("media: diacritic table length mismatch")
	}
}

// diacritic returns the combining mark for index, or empty if out of range.
func diacritic(index int) string {
	if index < 0 || index >= len(rowColumnDiacritics) {
		return ""
	}
	return string(rowColumnDiacritics[index])
}

// KittyPlaceholdersFit reports whether a columns×rows placeholder grid fits
// within the diacritic table.
func KittyPlaceholdersFit(columns, rows int) bool {
	return columns >= 1 && rows >= 1 &&
		columns <= KittyPlaceholderMaxCells && rows <= KittyPlaceholderMaxCells
}

// DetectKittyUnicodePlaceholdersSupport matches OMP detectKittyUnicodePlaceholdersSupport.
// env values: PI_NO_KITTY_PLACEHOLDERS / PI_KITTY_PLACEHOLDERS force off/on.
// Default: true only for terminalId "kitty" or "ghostty".
func DetectKittyUnicodePlaceholdersSupport(terminalID string, env map[string]string) bool {
	get := func(k string) string {
		if env == nil {
			return ""
		}
		return strings.TrimSpace(strings.ToLower(env[k]))
	}
	off := get("PI_NO_KITTY_PLACEHOLDERS")
	if off == "1" || off == "true" || off == "on" || off == "yes" || off == "y" {
		return false
	}
	force := get("PI_KITTY_PLACEHOLDERS")
	if force == "1" || force == "true" || force == "on" || force == "yes" || force == "y" {
		return true
	}
	if force == "0" || force == "false" || force == "off" || force == "no" || force == "n" {
		return false
	}
	return terminalID == "kitty" || terminalID == "ghostty"
}

// chunkKittyAPC builds one or more APC frames for leadParams + base64Data.
// Chunks at KittyChunkSize base64 characters. Does not re-encode or copy the
// whole base64 into an intermediate buffer beyond the final string join.
func chunkKittyAPC(leadParams, base64Data string) string {
	if len(base64Data) <= KittyChunkSize {
		return "\x1b_G" + leadParams + ";" + base64Data + "\x1b\\"
	}
	var b strings.Builder
	// Rough capacity: chunks * (overhead + chunk)
	nChunks := (len(base64Data) + KittyChunkSize - 1) / KittyChunkSize
	b.Grow(len(base64Data) + nChunks*24 + len(leadParams))
	offset := 0
	first := true
	for offset < len(base64Data) {
		end := offset + KittyChunkSize
		if end > len(base64Data) {
			end = len(base64Data)
		}
		chunk := base64Data[offset:end]
		last := end >= len(base64Data)
		b.WriteString("\x1b_G")
		switch {
		case first:
			b.WriteString(leadParams)
			b.WriteString(",m=1;")
			b.WriteString(chunk)
			b.WriteString("\x1b\\")
			first = false
		case last:
			b.WriteString("m=0;")
			b.WriteString(chunk)
			b.WriteString("\x1b\\")
		default:
			b.WriteString("m=1;")
			b.WriteString(chunk)
			b.WriteString("\x1b\\")
		}
		offset = end
	}
	return b.String()
}

// KittyDirectOptions configures transmit-and-display (a=T).
type KittyDirectOptions struct {
	Columns int
	Rows    int
	ImageID uint32 // 0 omits i=
}

// EncodeKittyDirect emits a=T,f=100,q=2,C=1 with optional c/r/i and chunked base64.
func EncodeKittyDirect(base64Data string, opt KittyDirectOptions) string {
	var params strings.Builder
	params.WriteString("a=T,f=100,q=2,C=1")
	if opt.Columns > 0 {
		params.WriteString(",c=")
		params.WriteString(strconv.Itoa(opt.Columns))
	}
	if opt.Rows > 0 {
		params.WriteString(",r=")
		params.WriteString(strconv.Itoa(opt.Rows))
	}
	if opt.ImageID != 0 {
		params.WriteString(",i=")
		params.WriteString(strconv.FormatUint(uint64(clampImageID(opt.ImageID)), 10))
	}
	return chunkKittyAPC(params.String(), base64Data)
}

// EncodeKittyTransmit emits a=t (data only) keyed by imageID.
func EncodeKittyTransmit(base64Data string, imageID uint32) string {
	id := clampImageID(imageID)
	return chunkKittyAPC("a=t,f=100,q=2,i="+strconv.FormatUint(uint64(id), 10), base64Data)
}

// KittyPlacementOptions configures a=p direct placement.
type KittyPlacementOptions struct {
	ImageID     uint32
	PlacementID uint32 // 0 omits p=
	Columns     int
	Rows        int
}

// EncodeKittyPlacement emits a=p,q=2,C=1 with i/p/c/r. No base64 payload.
func EncodeKittyPlacement(opt KittyPlacementOptions) string {
	var b strings.Builder
	b.WriteString("\x1b_Ga=p,q=2,C=1,i=")
	b.WriteString(strconv.FormatUint(uint64(clampImageID(opt.ImageID)), 10))
	if opt.PlacementID != 0 {
		b.WriteString(",p=")
		b.WriteString(strconv.FormatUint(uint64(clampImageID(opt.PlacementID)), 10))
	}
	if opt.Columns > 0 {
		b.WriteString(",c=")
		b.WriteString(strconv.Itoa(opt.Columns))
	}
	if opt.Rows > 0 {
		b.WriteString(",r=")
		b.WriteString(strconv.Itoa(opt.Rows))
	}
	b.WriteString("\x1b\\")
	return b.String()
}

// EncodeKittyDeleteImage emits d=I purge for one image id (frees data + placements).
func EncodeKittyDeleteImage(imageID uint32) string {
	return "\x1b_Ga=d,d=I,i=" + strconv.FormatUint(uint64(clampImageID(imageID)), 10) + ",q=2\x1b\\"
}

// EncodeKittyDeleteImages concatenates d=I purges for each id.
func EncodeKittyDeleteImages(ids []uint32) string {
	if len(ids) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(ids) * 28)
	for _, id := range ids {
		b.WriteString(EncodeKittyDeleteImage(id))
	}
	return b.String()
}

// KittyVirtualPlacementOptions configures U=1 virtual placement APC.
type KittyVirtualPlacementOptions struct {
	ImageID     uint32
	PlacementID uint32 // 0 omits p=
	Columns     int
	Rows        int
}

// EncodeKittyVirtualPlacement emits a=p,U=1 virtual placement (no cell grid).
func EncodeKittyVirtualPlacement(opt KittyVirtualPlacementOptions) string {
	var b strings.Builder
	b.WriteString("\x1b_Ga=p,U=1,q=2,i=")
	b.WriteString(strconv.FormatUint(uint64(clampImageID(opt.ImageID)), 10))
	if opt.PlacementID != 0 {
		b.WriteString(",p=")
		b.WriteString(strconv.FormatUint(uint64(clampImageID(opt.PlacementID)), 10))
	}
	b.WriteString(",c=")
	b.WriteString(strconv.Itoa(opt.Columns))
	b.WriteString(",r=")
	b.WriteString(strconv.Itoa(opt.Rows))
	b.WriteString("\x1b\\")
	return b.String()
}

// EncodeKittyPlaceholderGrid builds one string per row of U+10EEEE cells with
// row+column diacritics. Image id is the 24-bit FG color; placement id (if any)
// is the underline color. Returns exactly rows strings. Does not include the
// virtual-placement APC (see RenderKittyPlaceholderLines).
func EncodeKittyPlaceholderGrid(opt KittyVirtualPlacementOptions) []string {
	id := clampImageID(opt.ImageID)
	fg := "\x1b[38;2;" +
		strconv.Itoa(int((id>>16)&0xff)) + ";" +
		strconv.Itoa(int((id>>8)&0xff)) + ";" +
		strconv.Itoa(int(id&0xff)) + "m"
	underline := ""
	if opt.PlacementID != 0 {
		p := clampImageID(opt.PlacementID)
		underline = "\x1b[58:2::" +
			strconv.Itoa(int((p>>16)&0xff)) + ":" +
			strconv.Itoa(int((p>>8)&0xff)) + ":" +
			strconv.Itoa(int(p&0xff)) + "m"
	}
	const reset = "\x1b[39;59m"
	lead := fg + underline
	out := make([]string, opt.Rows)
	// Precompute column diacritics once.
	colMarks := make([]string, opt.Columns)
	for c := range opt.Columns {
		colMarks[c] = diacritic(c)
	}
	for r := range opt.Rows {
		rowMark := diacritic(r)
		var b strings.Builder
		// placeholder (4 bytes utf8) + 2 diacritics (~6) per cell + lead/reset
		b.Grow(len(lead) + opt.Columns*12 + len(reset))
		b.WriteString(lead)
		for c := range opt.Columns {
			b.WriteString(KittyPlaceholder)
			b.WriteString(rowMark)
			b.WriteString(colMarks[c])
		}
		b.WriteString(reset)
		out[r] = b.String()
	}
	return out
}

// RenderKittyPlaceholderLines prefixes line 0 with the virtual-placement APC
// and returns exactly rows lines (no cursor moves).
func RenderKittyPlaceholderLines(opt KittyVirtualPlacementOptions) []string {
	grid := EncodeKittyPlaceholderGrid(opt)
	if len(grid) > 0 {
		grid[0] = EncodeKittyVirtualPlacement(opt) + grid[0]
	}
	return grid
}

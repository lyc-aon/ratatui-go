package runtime

import "unicode/utf8"

// chunkForConPTY splits data into UTF-8-safe chunks of at most maxBytes,
// preferring newline boundaries. Port of terminal.ts chunkForConPTY.
func chunkForConPTY(data []byte, maxBytes int) [][]byte {
	if maxBytes <= 0 {
		maxBytes = maxConPTYWriteChunkBytes
	}
	if len(data) <= maxBytes {
		// Still need a copy-free single-element slice for callers that range.
		return [][]byte{data}
	}
	var chunks [][]byte
	pos := 0
	for pos < len(data) {
		bytes := 0
		lastNL := -1
		i := pos
		for i < len(data) {
			r, size := utf8.DecodeRune(data[i:])
			if size <= 0 {
				size = 1
			}
			// utf8.DecodeRune returns RuneError with size 1 for invalid bytes;
			// invalid single bytes still count as 1 UTF-8 replacement path on
			// write — count the raw byte length we will emit (size).
			cuBytes := size
			if r == utf8.RuneError && size == 1 {
				cuBytes = 1
			}
			if bytes+cuBytes > maxBytes && i > pos {
				cut := i
				if lastNL > pos {
					cut = lastNL
				}
				chunks = append(chunks, data[pos:cut])
				pos = cut
				break
			}
			bytes += cuBytes
			i += size
			if size == 1 && data[i-1] == '\n' {
				lastNL = i
			}
		}
		if i >= len(data) {
			chunks = append(chunks, data[pos:])
			pos = len(data)
		}
	}
	return chunks
}

// utf8ByteLen returns the encoded UTF-8 length of data (already bytes).
func utf8ByteLen(data []byte) int { return len(data) }

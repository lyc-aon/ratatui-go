package attachments

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// loadOutcome is the result of loading one path.
type loadOutcome struct {
	// displayPath is the path shown in <file name="..."> (user-facing).
	displayPath string
	// absPath is the absolute path used for I/O.
	absPath string
	// textBlock is a full <file>...</file>\n fragment when text was inlined.
	textBlock string
	// image is set for image attachments.
	image *ImageContent
	// notice is set on failure/skip (still may have empty success).
	notice *Notice
	// bytes is decoded size counted toward MaxTotalBytes on success.
	bytes int64
	// ok means content was attached (text or image).
	ok bool
}

// loadFile reads one path under opts/ctx policy. Never panics; never opens URLs.
func loadFile(ctx context.Context, userPath string, opts Options, usedFiles int, usedBytes int64) loadOutcome {
	out := loadOutcome{displayPath: userPath}

	if err := ctx.Err(); err != nil {
		out.notice = &Notice{Path: userPath, Code: CodeCancelled, Message: "cancelled"}
		return out
	}

	raw := strings.TrimSpace(userPath)
	if raw == "" {
		out.notice = &Notice{Path: userPath, Code: CodeInvalidPath, Message: "empty path"}
		return out
	}
	if isURLPath(raw) {
		out.notice = &Notice{Path: raw, Code: CodeURL, Message: "refusing to open URL: " + raw}
		return out
	}

	abs := resolveAgainstRoot(raw, opts.Root, opts.Home)
	if abs == "" {
		out.notice = &Notice{Path: raw, Code: CodeInvalidPath, Message: "invalid path"}
		return out
	}
	out.absPath = abs
	out.displayPath = abs

	// Traversal / outside-root before stat when the cleaned path already escapes.
	if !opts.FollowOutsideRoot && opts.Root != "" && !confined(abs, opts.Root) {
		// Still allow if path is under root after clean; otherwise reject early.
		// Note: symlink targets re-checked in evalPolicy.
		out.notice = &Notice{Path: abs, Code: CodeOutsideRoot, Message: "path escapes configured root: " + abs}
		return out
	}

	if usedFiles >= opts.MaxFiles {
		out.notice = &Notice{
			Path:    abs,
			Code:    CodeCountLimit,
			Message: fmt.Sprintf("file count limit reached (%d)", opts.MaxFiles),
		}
		return out
	}

	readPath, n := evalPolicy(abs, opts)
	if n != nil {
		out.notice = n
		return out
	}

	if err := ctx.Err(); err != nil {
		out.notice = &Notice{Path: abs, Code: CodeCancelled, Message: "cancelled"}
		return out
	}

	fi, err := os.Stat(readPath)
	if err != nil {
		if os.IsNotExist(err) {
			out.notice = &Notice{Path: abs, Code: CodeMissing, Message: "file not found: " + abs}
			return out
		}
		out.notice = &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot stat: " + err.Error()}
		return out
	}
	size := fi.Size()
	if size == 0 {
		// OMP skips empty images/files silently for mentions; we notice empty.
		out.notice = &Notice{Path: abs, Code: CodeEmpty, Message: "empty file: " + abs}
		return out
	}

	// Cap read by the larger of text/image limits so we can sniff then decide.
	capBytes := opts.MaxFileBytes
	if opts.MaxImageBytes > capBytes {
		capBytes = opts.MaxImageBytes
	}
	// Hard stop: never read more than remaining total + 1 byte to detect overflow.
	remain := opts.MaxTotalBytes - usedBytes
	if remain <= 0 {
		out.notice = &Notice{
			Path:    abs,
			Code:    CodeTotalLimit,
			Message: fmt.Sprintf("total byte limit reached (%d)", opts.MaxTotalBytes),
		}
		return out
	}

	// If file is larger than any allowed category, skip without full read.
	if size > opts.MaxImageBytes && size > opts.MaxFileBytes {
		out.notice = &Notice{
			Path: abs,
			Code: CodeTooLarge,
			Message: fmt.Sprintf(
				"file too large (%s): %s",
				formatBytes(size),
				abs,
			),
		}
		return out
	}

	f, err := os.Open(readPath)
	if err != nil {
		if os.IsNotExist(err) {
			out.notice = &Notice{Path: abs, Code: CodeMissing, Message: "file not found: " + abs}
			return out
		}
		out.notice = &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot open: " + err.Error()}
		return out
	}
	defer f.Close()

	// Bound the read.
	limit := size
	if int64(capBytes) < limit {
		limit = int64(capBytes)
	}
	if remain < limit {
		// may still fit as image or text under remain if smaller — check after
		// we know type; for now read up to min(size, max category, remain+1)
		limit = remain
		if size > remain {
			// need to know type for accurate notice; read header first
			limit = min64(size, int64(capBytes))
		}
	}

	if err := ctx.Err(); err != nil {
		out.notice = &Notice{Path: abs, Code: CodeCancelled, Message: "cancelled"}
		return out
	}

	data, err := readAllCancelable(ctx, f, limit)
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			out.notice = &Notice{Path: abs, Code: CodeCancelled, Message: "cancelled"}
			return out
		}
		out.notice = &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot read: " + err.Error()}
		return out
	}

	mime := SniffImageMIME(data)
	if mime != "" {
		if size > opts.MaxImageBytes || int64(len(data)) > opts.MaxImageBytes {
			out.notice = &Notice{
				Path: abs,
				Code: CodeTooLarge,
				Message: fmt.Sprintf(
					"image too large (%s > %s): %s",
					formatBytes(size),
					formatBytes(opts.MaxImageBytes),
					abs,
				),
			}
			return out
		}
		if usedBytes+int64(len(data)) > opts.MaxTotalBytes {
			out.notice = &Notice{
				Path:    abs,
				Code:    CodeTotalLimit,
				Message: fmt.Sprintf("total byte limit reached (%d)", opts.MaxTotalBytes),
			}
			return out
		}
		// If we truncated the read under a lower cap, re-read full image when needed.
		if int64(len(data)) < size && size <= opts.MaxImageBytes {
			if _, err := f.Seek(0, io.SeekStart); err == nil {
				full, err := readAllCancelable(ctx, f, size)
				if err != nil {
					if err == context.Canceled || err == context.DeadlineExceeded {
						out.notice = &Notice{Path: abs, Code: CodeCancelled, Message: "cancelled"}
						return out
					}
					out.notice = &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot read: " + err.Error()}
					return out
				}
				data = full
			}
		}
		img := ImageContent{
			Type:     "image",
			MimeType: mime,
			Data:     base64.StdEncoding.EncodeToString(data),
		}
		out.image = &img
		// OMP empty image body in <file> for non-resized images.
		out.textBlock = fmt.Sprintf("<file name=\"%s\"></file>\n", escapeAttr(abs))
		out.bytes = int64(len(data))
		out.ok = true
		return out
	}

	// Non-image path.
	if size > opts.MaxFileBytes || int64(len(data)) > opts.MaxFileBytes {
		out.notice = &Notice{
			Path: abs,
			Code: CodeTooLarge,
			Message: fmt.Sprintf(
				"file too large (%s > %s): %s",
				formatBytes(size),
				formatBytes(opts.MaxFileBytes),
				abs,
			),
		}
		return out
	}
	if usedBytes+int64(len(data)) > opts.MaxTotalBytes {
		out.notice = &Notice{
			Path:    abs,
			Code:    CodeTotalLimit,
			Message: fmt.Sprintf("total byte limit reached (%d)", opts.MaxTotalBytes),
		}
		return out
	}
	if int64(len(data)) < size && size <= opts.MaxFileBytes {
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			full, err := readAllCancelable(ctx, f, size)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					out.notice = &Notice{Path: abs, Code: CodeCancelled, Message: "cancelled"}
					return out
				}
				out.notice = &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot read: " + err.Error()}
				return out
			}
			data = full
		}
	}

	if looksBinary(data) {
		out.notice = &Notice{
			Path:    abs,
			Code:    CodeBinary,
			Message: "binary non-image file skipped: " + abs,
		}
		return out
	}
	if !utf8.Valid(data) {
		out.notice = &Notice{
			Path:    abs,
			Code:    CodeBinary,
			Message: "non-UTF-8 file skipped: " + abs,
		}
		return out
	}

	content := string(data)
	// OMP file-processor text wrap:
	//   text += `<file name="${absolutePath}">\n${content}\n</file>\n`
	out.textBlock = fmt.Sprintf("<file name=\"%s\">\n%s\n</file>\n", escapeAttr(abs), content)
	out.bytes = int64(len(data))
	out.ok = true
	return out
}

func readAllCancelable(ctx context.Context, r io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		limit = 0
	}
	// Chunked read so cancellation is observed.
	const chunk = 64 * 1024
	var buf []byte
	var got int64
	tmp := make([]byte, chunk)
	for got < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		need := limit - got
		if need > chunk {
			need = chunk
		}
		n, err := r.Read(tmp[:need])
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			got += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return buf, err
		}
		if n == 0 {
			break
		}
	}
	return buf, nil
}

func escapeAttr(s string) string {
	// Minimal attribute escape for path embedding in name="...".
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return s
}

func formatBytes(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

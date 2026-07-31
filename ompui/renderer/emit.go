package renderer

import (
	"io"
	"strconv"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

// byteBuf is a reusable frame write buffer. One batch per paint; one Write.
type byteBuf struct {
	b []byte
}

func (b *byteBuf) Reset() { b.b = b.b[:0] }

func (b *byteBuf) Len() int { return len(b.b) }

func (b *byteBuf) WriteString(s string) {
	b.b = append(b.b, s...)
}

func (b *byteBuf) AppendByte(c byte) {
	b.b = append(b.b, c)
}

func (b *byteBuf) Bytes() []byte { return b.b }

func (b *byteBuf) writeTo(w io.Writer) (int, error) {
	if len(b.b) == 0 {
		return 0, nil
	}
	n, err := w.Write(b.b)
	if err != nil {
		return n, err
	}
	if n < len(b.b) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

func ioWriteString(w io.Writer, s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return io.WriteString(w, s)
}

// terminalLine closes SGR (and OSC8 when present) on a content row.
func (e *Engine) terminalLine(line string) string {
	if e.caps.IsImageLine(line) {
		return line
	}
	coalesced := ansitext.CoalesceAdjacentSGR(line)
	if strings.Contains(line, "\x1b]8;") {
		return coalesced + ansitext.LineTerminator
	}
	return coalesced + ansitext.SegmentReset
}

// lineRewriteSequence paints one viewport row starting at column 1.
func (e *Engine) lineRewriteSequence(line string, width int) string {
	if e.caps.IsImageLine(line) {
		return eraseLine + line
	}
	tl := e.terminalLine(line)
	if asciiW, ok := ansiASCIILineWidth(line, width); ok {
		if asciiW >= width {
			return tl
		}
		return tl + eraseToEndOfLine
	}
	return ansitext.SegmentReset + eraseToEndOfLine + tl
}

func relMoveUp(n int) string {
	if n <= 0 {
		return ""
	}
	return "\x1b[" + strconv.Itoa(n) + "A"
}

func relMoveDown(n int) string {
	if n <= 0 {
		return ""
	}
	return "\x1b[" + strconv.Itoa(n) + "B"
}

// emitFullPaint clears the viewport (optionally native scrollback via ED3) and
// replays committed prefix + window. ED3 is emitted HERE AND ONLY HERE.
func (e *Engine) emitFullPaint(
	frame []string,
	window []string,
	width, height int,
	cursor *CursorPos,
	purge, imageTransmit string,
	clearScrollback bool,
	chunkTo, windowTop int,
) error {
	buf := &e.writeBuf
	buf.Reset()
	buf.WriteString(e.caps.paintBegin())
	if seq := e.leaveResizeAltSequence(); seq != "" {
		buf.WriteString(seq)
	}
	buf.WriteString(purge)

	if clearScrollback {
		// Single ED3 callsite in the package.
		buf.WriteString(eraseDisplay)
		buf.WriteString(cursorHome)
		buf.WriteString(eraseScrollback)
	} else {
		if e.caps.SupportsScreenToScrollback {
			buf.WriteString(eraseScreenCopy)
		}
		buf.WriteString(eraseDisplay)
		buf.WriteString(cursorHome)
	}
	if imageTransmit != "" {
		buf.WriteString(imageTransmit)
	}

	wrote := false
	for i := range chunkTo {
		if i >= len(frame) {
			break
		}
		if wrote {
			buf.WriteString("\r\n")
		}
		buf.WriteString(e.terminalLine(frame[i]))
		wrote = true
	}
	for screenRow := range height {
		if wrote {
			buf.WriteString("\r\n")
		}
		row := ""
		if screenRow < len(window) {
			row = window[screenRow]
		}
		buf.WriteString(e.terminalLine(row))
		wrote = true
	}

	contentRows := contentRowCount(len(frame), windowTop, height)
	parkUp := height - contentRows
	if parkUp > 0 {
		buf.WriteString(relMoveUp(parkUp))
	}
	contentBottomRow := windowTop + contentRows - 1
	seq, toRow, toCol, visible, full := e.cursorControl(cursor, len(frame), contentBottomRow)
	buf.WriteString(seq)
	buf.WriteString(e.caps.paintEnd())

	n, err := buf.writeTo(e.out)
	e.trace(TraceEvent{
		Kind:            "emit",
		Mode:            "fullPaint",
		ClearScrollback: clearScrollback,
		ChunkFrom:       0,
		ChunkTo:         chunkTo,
		WindowTop:       windowTop,
		FrameLen:        len(frame),
		Width:           width,
		Height:          height,
		BytesWritten:    n,
		Error:           errString(err),
	})
	if err != nil {
		return err
	}
	e.recordHardwareCursorUpdate(toRow, toCol, visible, full)
	e.finishPaint(frame, window, width, height)
	return nil
}

// emitUpdate performs scroll-append, in-window diff, or seam rewrite.
// Never emits ED2/ED3 or absolute cursor home.
func (e *Engine) emitUpdate(
	frame []string,
	window []string,
	width, height int,
	cursor *CursorPos,
	purge string,
	chunkTo, windowTop, prevWindowTop, prevHWCursorRow int,
	forceWindowRewrite bool,
) error {
	// Post-Step CommittedRows: shrink already moved it to chunkTo (empty chunk);
	// ordinary frames still hold the post-audit index to append from.
	chunkFrom := e.ledger.CommittedRows
	chunkLen := chunkTo - chunkFrom
	if chunkLen < 0 {
		chunkLen = 0
	}
	scroll := windowTop - prevWindowTop
	prevWindow := e.previousWindow
	contentRows := contentRowCount(len(frame), windowTop, height)
	contentBottomRow := windowTop + contentRows - 1

	clampedCursor := prevHWCursorRow
	if maxRow := prevWindowTop + height - 1; clampedCursor > maxRow {
		clampedCursor = maxRow
	}
	currentScreenRow := clampedCursor - prevWindowTop
	if currentScreenRow < 0 {
		currentScreenRow = 0
	}
	if currentScreenRow > height-1 {
		currentScreenRow = height - 1
	}

	// --- scroll-append ---
	if !forceWindowRewrite &&
		chunkLen > 0 &&
		chunkLen == scroll &&
		scroll < height &&
		chunkFrom == prevWindowTop {
		prefixIntact := len(prevWindow) == height
		for i := 0; prefixIntact && i < chunkLen; i++ {
			want := ""
			if chunkFrom+i < len(frame) {
				want = frame[chunkFrom+i]
			}
			if i >= len(prevWindow) || prevWindow[i] != want {
				prefixIntact = false
			}
		}
		if prefixIntact {
			buf := &e.writeBuf
			buf.Reset()
			buf.WriteString(e.caps.paintBegin())
			buf.WriteString(purge)
			moveToBottom := height - 1 - currentScreenRow
			if moveToBottom > 0 {
				buf.WriteString(relMoveDown(moveToBottom))
			}
			for r := height - scroll; r < height; r++ {
				row := ""
				if r < len(window) {
					row = window[r]
				}
				buf.WriteString("\r\n")
				buf.WriteString(e.lineRewriteSequence(row, width))
			}
			firstChanged, lastChanged := -1, -1
			limit := height - scroll
			for r := range limit {
				cur := ""
				if r < len(window) {
					cur = window[r]
				}
				prev := ""
				if r+scroll < len(prevWindow) {
					prev = prevWindow[r+scroll]
				}
				if cur == prev {
					continue
				}
				if firstChanged == -1 {
					firstChanged = r
				}
				lastChanged = r
			}
			cursorFromRow := windowTop + height - 1
			if firstChanged != -1 {
				up := height - 1 - firstChanged
				if up > 0 {
					buf.WriteString(relMoveUp(up))
				}
				buf.AppendByte('\r')
				for r := firstChanged; r <= lastChanged; r++ {
					if r > firstChanged {
						buf.WriteString("\r\n")
					}
					row := ""
					if r < len(window) {
						row = window[r]
					}
					buf.WriteString(e.lineRewriteSequence(row, width))
				}
				cursorFromRow = windowTop + lastChanged
			}
			seq, toRow, toCol, visible, full := e.cursorControl(cursor, len(frame), cursorFromRow)
			buf.WriteString(seq)
			buf.WriteString(e.caps.paintEnd())
			n, err := buf.writeTo(e.out)
			e.trace(TraceEvent{
				Kind: "emit", Mode: "scrollAppend",
				ChunkFrom: chunkFrom, ChunkTo: chunkTo, WindowTop: windowTop,
				FrameLen: len(frame), Width: width, Height: height,
				BytesWritten: n, Error: errString(err),
			})
			if err != nil {
				return err
			}
			e.recordHardwareCursorUpdate(toRow, toCol, visible, full)
			e.finishPaint(frame, window, width, height)
			return nil
		}
	}

	// --- in-window diff ---
	if chunkLen == 0 && scroll == 0 {
		firstChanged, lastChanged := -1, -1
		if forceWindowRewrite {
			firstChanged = 0
			lastChanged = height - 1
		} else {
			comparable := len(prevWindow) == height
			for r := range height {
				cur := ""
				if r < len(window) {
					cur = window[r]
				}
				prev := ""
				if comparable && r < len(prevWindow) {
					prev = prevWindow[r]
				}
				if comparable && cur == prev {
					continue
				}
				if firstChanged == -1 {
					firstChanged = r
				}
				lastChanged = r
			}
		}
		if firstChanged == -1 {
			if purge != "" {
				if _, err := ioWriteString(e.out, purge); err != nil {
					return err
				}
			}
			if err := e.writeCursorOnly(cursor, len(frame)); err != nil {
				return err
			}
			e.previousWidth = width
			e.previousHeight = height
			e.trace(TraceEvent{
				Kind: "emit", Mode: "cursorOnly",
				ChunkFrom: chunkFrom, ChunkTo: chunkTo, WindowTop: windowTop,
				FrameLen: len(frame), Width: width, Height: height,
			})
			return nil
		}
		buf := &e.writeBuf
		buf.Reset()
		buf.WriteString(e.caps.paintBegin())
		buf.WriteString(purge)
		rowDelta := firstChanged - currentScreenRow
		if rowDelta > 0 {
			buf.WriteString(relMoveDown(rowDelta))
		} else if rowDelta < 0 {
			buf.WriteString(relMoveUp(-rowDelta))
		}
		buf.AppendByte('\r')
		for r := firstChanged; r <= lastChanged; r++ {
			if r > firstChanged {
				buf.WriteString("\r\n")
			}
			row := ""
			if r < len(window) {
				row = window[r]
			}
			buf.WriteString(e.lineRewriteSequence(row, width))
		}
		cursorFromRow := windowTop + lastChanged
		contentBottomScreenRow := contentBottomRow - windowTop
		if lastChanged > contentBottomScreenRow {
			buf.WriteString(relMoveUp(lastChanged - contentBottomScreenRow))
			cursorFromRow = contentBottomRow
		}
		seq, toRow, toCol, visible, full := e.cursorControl(cursor, len(frame), cursorFromRow)
		buf.WriteString(seq)
		buf.WriteString(e.caps.paintEnd())
		n, err := buf.writeTo(e.out)
		e.trace(TraceEvent{
			Kind: "emit", Mode: "windowDiff",
			ChunkFrom: chunkFrom, ChunkTo: chunkTo, WindowTop: windowTop,
			FrameLen: len(frame), Width: width, Height: height,
			ForceRewrite: forceWindowRewrite, BytesWritten: n, Error: errString(err),
		})
		if err != nil {
			return err
		}
		e.recordHardwareCursorUpdate(toRow, toCol, visible, full)
		e.finishPaint(frame, window, width, height)
		return nil
	}

	// --- seam rewrite ---
	buf := &e.writeBuf
	buf.Reset()
	buf.WriteString(e.caps.paintBegin())
	buf.WriteString(purge)
	if currentScreenRow > 0 {
		buf.WriteString(relMoveUp(currentScreenRow))
	}
	buf.AppendByte('\r')
	wroteLine := false
	for i := chunkFrom; i < chunkTo; i++ {
		if wroteLine {
			buf.WriteString("\r\n")
		}
		row := ""
		if i < len(frame) {
			row = frame[i]
		}
		buf.WriteString(e.lineRewriteSequence(row, width))
		wroteLine = true
	}
	for screenRow := range height {
		if wroteLine {
			buf.WriteString("\r\n")
		}
		row := ""
		if screenRow < len(window) {
			row = window[screenRow]
		}
		buf.WriteString(e.lineRewriteSequence(row, width))
		wroteLine = true
	}
	parkUp := height - 1 - (contentBottomRow - windowTop)
	if parkUp > 0 {
		buf.WriteString(relMoveUp(parkUp))
	}
	seq, toRow, toCol, visible, full := e.cursorControl(cursor, len(frame), contentBottomRow)
	buf.WriteString(seq)
	buf.WriteString(e.caps.paintEnd())
	n, err := buf.writeTo(e.out)
	e.trace(TraceEvent{
		Kind: "emit", Mode: "seamRewrite",
		ChunkFrom: chunkFrom, ChunkTo: chunkTo, WindowTop: windowTop,
		FrameLen: len(frame), Width: width, Height: height,
		ForceRewrite: forceWindowRewrite, BytesWritten: n, Error: errString(err),
	})
	if err != nil {
		return err
	}
	e.recordHardwareCursorUpdate(toRow, toCol, visible, full)
	e.finishPaint(frame, window, width, height)
	return nil
}

func contentRowCount(frameLen, windowTop, height int) int {
	n := frameLen - windowTop
	if n < 1 {
		n = 1
	}
	if n > height {
		n = height
	}
	if height < 1 {
		return 1
	}
	return n
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// finishPaint records post-successful-emit window caches (not ledger — caller
// runs ledger.Finish after this returns nil).
func (e *Engine) finishPaint(frame, window []string, width, height int) {
	e.previousFrameLen = len(frame)
	if cap(e.previousWindow) < len(window) {
		e.previousWindow = make([]string, len(window))
	} else {
		e.previousWindow = e.previousWindow[:len(window)]
	}
	copy(e.previousWindow, window)
	e.forceViewportRepaint = false
	e.previousWidth = width
	e.previousHeight = height
}

func (e *Engine) leaveResizeAltSequence() string {
	if !e.resizeAltActive {
		return ""
	}
	e.resizeAltActive = false
	e.forgetHardwareCursorState()
	return altScreenExit
}

func (e *Engine) enterResizeAltSequence() string {
	if e.resizeAltActive || e.altActive {
		return ""
	}
	e.resizeAltActive = true
	e.forgetHardwareCursorState()
	e.recordHardwareCursorHidden()
	return altScreenEnter
}

// emitResizeViewport paints a throwaway viewport on the alt screen during drag.
// Does not advance ledger/window/diff state.
func (e *Engine) emitResizeViewport(window []string, height, contentRows, width int) error {
	buf := &e.writeBuf
	buf.Reset()
	buf.WriteString(e.caps.paintBegin())
	buf.WriteString(e.enterResizeAltSequence())
	buf.WriteString(cursorHome)
	for r := range height {
		if r > 0 {
			buf.WriteString("\r\n")
		}
		row := ""
		if r < len(window) {
			row = window[r]
		}
		buf.WriteString(e.lineRewriteSequence(row, width))
	}
	cr := contentRows
	if cr < 1 {
		cr = 1
	}
	parkUp := height - cr
	if parkUp > 0 {
		buf.WriteString(relMoveUp(parkUp))
	}
	buf.WriteString(e.caps.paintEnd())
	_, err := buf.writeTo(e.out)
	return err
}

// emitAltFrame full-rewrites the alternate screen for a fullscreen overlay.
func (e *Engine) emitAltFrame(lines []string, width, height int, force bool) error {
	fitted := make([]string, height)
	for r := range height {
		if r < len(lines) {
			fitted[r] = lines[r]
		}
	}
	if !force && len(e.altPreviousLines) == height {
		same := true
		for r := range height {
			if fitted[r] != e.altPreviousLines[r] {
				same = false
				break
			}
		}
		if same {
			return nil
		}
	}
	buf := &e.writeBuf
	buf.Reset()
	buf.WriteString(e.caps.paintBegin())
	buf.WriteString(cursorHome)
	for r := range height {
		if r > 0 {
			buf.WriteString("\r\n")
		}
		buf.WriteString(e.lineRewriteSequence(fitted[r], width))
	}
	buf.WriteString(e.caps.paintEnd())
	_, err := buf.writeTo(e.out)
	if err != nil {
		return err
	}
	e.altPreviousLines = fitted
	return nil
}

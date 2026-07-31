package runtime

import (
	"bytes"
	"regexp"
	"strconv"
)

// da1OwnerKind tags a DA1 sentinel FIFO entry.
type da1OwnerKind uint8

const (
	da1Keyboard da1OwnerKind = iota
	da1OSC11
	da1PrivateMode
	da1OSC99Probe
	da1CPR
)

// da1Owner is one outstanding DA1 (or CPR) sentinel owner.
type da1Owner struct {
	kind da1OwnerKind
	mode int    // private mode number when kind == da1PrivateMode
	id   string // osc99 probe id when kind == da1OSC99Probe
	// cpr delivers the CPR result to a waiting QueryCursorPosition.
	cpr chan cprResult
}

type cprResult struct {
	pos CursorPosition
	err error
}

var (
	reKittyResponse   = regexp.MustCompile(`^\x1b\[\?(\d+)u$`)
	reAppearanceDSR   = regexp.MustCompile(`^\x1b\[\?997;([12])n$`)
	reOSC11Response   = regexp.MustCompile(`^\x1b\]11;rgba?:([0-9a-fA-F]{1,4})/([0-9a-fA-F]{1,4})/([0-9a-fA-F]{1,4})(?:\x07|\x1b\\)$`)
	reDA1Response     = regexp.MustCompile(`^\x1b\[\?[\d;]*c$`)
	rePrivateCSIPartial = regexp.MustCompile(`^\x1b\[\?[\d;]*[\x20-\x2f]*$`)
	reDECRPM          = regexp.MustCompile(`^\x1b\[\?(\d+);(\d+)\$y$`)
	reInBandResize    = regexp.MustCompile(`^\x1b\[48;(\d+);(\d+);(\d+);(\d+)t$`)
	reInBandPartial   = regexp.MustCompile(`^\x1b\[4[\d;]*$`)
	reCPR             = regexp.MustCompile(`^\x1b\[(\d+);(\d+)R$`)
	reOSC99Response   = regexp.MustCompile(`^\x1b\]99;([^;]*);([\s\S]*?)(?:\x07|\x1b\\)$`)
)

// probeRouter holds reassembly buffers and FIFO state for capability replies.
// Touched only from the input coordinator goroutine (plus Start setup before
// the loop begins).
type probeRouter struct {
	da1Owners []da1Owner

	privateCSIBuf []byte
	inBandBuf     []byte
	osc11Buf      []byte
	osc99Buf      []byte

	osc11Pending     bool
	osc11QueryQueued bool
	osc99PendingID   string

	// cprPending is true while a CPR query is outstanding (owner on FIFO).
	cprPending bool
}

func (p *probeRouter) pushOwner(o da1Owner) {
	p.da1Owners = append(p.da1Owners, o)
}

func (p *probeRouter) shiftOwner() (da1Owner, bool) {
	if len(p.da1Owners) == 0 {
		return da1Owner{}, false
	}
	o := p.da1Owners[0]
	p.da1Owners = p.da1Owners[1:]
	return o, true
}

func (p *probeRouter) hasOwner(kind da1OwnerKind) bool {
	for _, o := range p.da1Owners {
		if o.kind == kind {
			return true
		}
	}
	return false
}

func (p *probeRouter) clear() {
	// Fail any outstanding CPR waiters so QueryCursorPosition cannot hang.
	for _, o := range p.da1Owners {
		if o.kind == da1CPR && o.cpr != nil {
			select {
			case o.cpr <- cprResult{err: errTerminalStopped}:
			default:
			}
		}
	}
	p.da1Owners = nil
	p.privateCSIBuf = nil
	p.inBandBuf = nil
	p.osc11Buf = nil
	p.osc99Buf = nil
	p.osc11Pending = false
	p.osc11QueryQueued = false
	p.osc99PendingID = ""
	p.cprPending = false
}

// handleSequence routes one complete framed sequence. Returns true when the
// sequence was consumed as a probe/control reply (do not forward to user).
// The callback delivers CPR / appearance / private-mode / kitty / resize side effects.
type probeCallbacks struct {
	onKittyFlags      func(flags int)
	onOSC11           func(r, g, b string)
	onAppearanceDSR   func()
	onPrivateMode     func(mode int, status string)
	onPrivateModeMiss func(mode int) // DA1 beat DECRPM
	onOSC99           func(meta, payload string) bool
	onOSC99Miss       func(id string)
	onInBandResize    func(rows, cols, yPx, xPx int)
	onKeyboardMiss    func() // DA1 beat kitty
	onCPR             func(pos CursorPosition, ok bool, waiter chan cprResult)
}

// route applies probe handling. seq is not retained.
func (p *probeRouter) route(seq []byte, inBandActive bool, cb probeCallbacks) (consumed bool) {
	// --- private CSI reassembly (DA1 / kitty / Mode 2031) ---
	if len(p.privateCSIBuf) > 0 || (rePrivateCSIPartial.Match(seq) && len(p.da1Owners) > 0) {
		if len(p.privateCSIBuf) > 0 && len(seq) > 0 && seq[0] == 0x1b {
			// New escape mid-reassembly — abandon partial and re-process seq.
			p.privateCSIBuf = nil
		} else {
			p.privateCSIBuf = append(p.privateCSIBuf, seq...)
			if len(p.privateCSIBuf) > maxProbeReassemblyBytes {
				p.privateCSIBuf = nil
				return true
			}
			last := p.privateCSIBuf[len(p.privateCSIBuf)-1]
			if last >= 0x40 && last <= 0x7e {
				seq = append([]byte(nil), p.privateCSIBuf...)
				p.privateCSIBuf = nil
				// fall through
			} else if !rePrivateCSIPartial.Match(p.privateCSIBuf) {
				p.privateCSIBuf = nil
				return true
			} else {
				return true
			}
		}
	}

	// --- in-band resize reassembly ---
	isInBandPartial := inBandActive && reInBandPartial.Match(seq)
	if len(p.inBandBuf) > 0 && len(seq) > 0 && seq[0] == 0x1b {
		if isInBandPartial {
			p.inBandBuf = append([]byte(nil), seq...)
			return true
		}
		p.inBandBuf = nil
		// fall through with seq
	} else if len(p.inBandBuf) > 0 || isInBandPartial {
		p.inBandBuf = append(p.inBandBuf, seq...)
		if len(p.inBandBuf) > maxProbeReassemblyBytes {
			p.inBandBuf = nil
			return true
		}
		last := p.inBandBuf[len(p.inBandBuf)-1]
		if last >= 0x40 && last <= 0x7e {
			seq = append([]byte(nil), p.inBandBuf...)
			p.inBandBuf = nil
		} else if !reInBandPartial.Match(p.inBandBuf) {
			p.inBandBuf = nil
			return true
		} else {
			return true
		}
	}

	// In-band resize report.
	if m := reInBandResize.FindSubmatch(seq); m != nil {
		rows, _ := strconv.Atoi(string(m[1]))
		cols, _ := strconv.Atoi(string(m[2]))
		yPx, _ := strconv.Atoi(string(m[3]))
		xPx, _ := strconv.Atoi(string(m[4]))
		if cb.onInBandResize != nil {
			cb.onInBandResize(rows, cols, yPx, xPx)
		}
		return true
	}

	// CPR reply (CSI row;col R) — not private-mode form.
	if m := reCPR.FindSubmatch(seq); m != nil {
		row, _ := strconv.Atoi(string(m[1]))
		col, _ := strconv.Atoi(string(m[2]))
		if row < 1 {
			row = 1
		}
		if col < 1 {
			col = 1
		}
		pos := CursorPosition{Col: col - 1, Row: row - 1}
		// Prefer explicit CPR owner on FIFO; else first CPR-less waiter is wrong —
		// only deliver when a CPR owner is at head or anywhere.
		if o, ok := p.takeCPROwner(); ok {
			if cb.onCPR != nil {
				cb.onCPR(pos, true, o.cpr)
			}
			return true
		}
		// Stray CPR — swallow.
		return true
	}

	// DECRPM.
	if m := reDECRPM.FindSubmatch(seq); m != nil {
		mode, _ := strconv.Atoi(string(m[1]))
		status := string(m[2])
		if cb.onPrivateMode != nil {
			cb.onPrivateMode(mode, status)
		}
		return true
	}

	// DA1 sentinel.
	if reDA1Response.Match(seq) && len(p.da1Owners) > 0 {
		o, ok := p.shiftOwner()
		if !ok {
			return true
		}
		switch o.kind {
		case da1OSC11:
			if p.osc11Pending {
				p.osc11Pending = false
				p.osc11Buf = nil
			}
		case da1PrivateMode:
			if cb.onPrivateModeMiss != nil {
				cb.onPrivateModeMiss(o.mode)
			}
		case da1Keyboard:
			if cb.onKeyboardMiss != nil {
				cb.onKeyboardMiss()
			}
		case da1OSC99Probe:
			if cb.onOSC99Miss != nil {
				cb.onOSC99Miss(o.id)
			}
		case da1CPR:
			// DA1 should not own CPR; if mis-queued, fail waiter.
			if o.cpr != nil {
				select {
				case o.cpr <- cprResult{err: errCPRTimeout}:
				default:
				}
			}
			p.cprPending = false
		}
		return true
	}

	// Kitty keyboard reply.
	if m := reKittyResponse.FindSubmatch(seq); m != nil {
		flags, _ := strconv.Atoi(string(m[1]))
		if cb.onKittyFlags != nil {
			cb.onKittyFlags(flags)
		}
		return true
	}

	// OSC 11 reassembly / match.
	if p.osc11Pending && (len(p.osc11Buf) > 0 || bytes.HasPrefix(seq, []byte("\x1b]11;"))) {
		if len(p.osc11Buf) > 0 && len(seq) > 0 && seq[0] == 0x1b && !bytes.Equal(seq, []byte("\x1b\\")) {
			p.osc11Buf = nil
			// fall through to normal handling of seq
		} else {
			p.osc11Buf = append(p.osc11Buf, seq...)
			if m := reOSC11Response.FindSubmatch(p.osc11Buf); m != nil {
				p.osc11Pending = false
				p.osc11Buf = nil
				if cb.onOSC11 != nil {
					cb.onOSC11(string(m[1]), string(m[2]), string(m[3]))
				}
				return true
			}
			if len(p.osc11Buf) > maxProbeReassemblyBytes*4 {
				p.osc11Buf = nil
				return true
			}
			return true
		}
	}

	// OSC 99 capability reply.
	if p.osc99PendingID != "" && (len(p.osc99Buf) > 0 || bytes.HasPrefix(seq, []byte("\x1b]99;"))) {
		if len(p.osc99Buf) > 0 && len(seq) > 0 && seq[0] == 0x1b && !bytes.Equal(seq, []byte("\x1b\\")) {
			p.osc99Buf = nil
		} else {
			p.osc99Buf = append(p.osc99Buf, seq...)
			if m := reOSC99Response.FindSubmatch(p.osc99Buf); m != nil {
				p.osc99Buf = nil
				if cb.onOSC99 != nil && cb.onOSC99(string(m[1]), string(m[2])) {
					return true
				}
				// Unmatched OSC 99 — still consumed as probe noise.
				return true
			}
			if len(p.osc99Buf) > maxProbeReassemblyBytes*8 {
				p.osc99Buf = nil
				return true
			}
			return true
		}
	}

	// Mode 2031 appearance DSR.
	if reAppearanceDSR.Match(seq) {
		if cb.onAppearanceDSR != nil {
			cb.onAppearanceDSR()
		}
		return true
	}

	return false
}

func (p *probeRouter) takeCPROwner() (da1Owner, bool) {
	for i, o := range p.da1Owners {
		if o.kind == da1CPR {
			p.da1Owners = append(p.da1Owners[:i], p.da1Owners[i+1:]...)
			p.cprPending = false
			return o, true
		}
	}
	return da1Owner{}, false
}

func isPrivateModeSet(status string) bool {
	return status == "1" || status == "3"
}

func isPrivateModeSupported(status string) bool {
	return status != "0" && status != "4"
}

func isXtermScrollToBottomMode(mode int) bool {
	return mode == modeScrollBottomOnOutput || mode == modeScrollBottomOnKey
}

// luminanceAppearance maps OSC 11 hex components to dark/light via BT.601.
func luminanceAppearance(rHex, gHex, bHex string) Appearance {
	norm := func(hex string) float64 {
		v, err := strconv.ParseInt(hex, 16, 64)
		if err != nil {
			return 0
		}
		max := float64(1)
		for i := 0; i < len(hex); i++ {
			max *= 16
		}
		max--
		if max <= 0 {
			return 0
		}
		return float64(v) / max
	}
	l := 0.299*norm(rHex) + 0.587*norm(gHex) + 0.114*norm(bHex)
	if l < 0.5 {
		return AppearanceDark
	}
	return AppearanceLight
}

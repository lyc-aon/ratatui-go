package app

import (
	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/interact"
	"github.com/michaelkelly/ratatui-go/ompui/renderer"
)

func (a *App) requestRender(reason renderer.Reason) {
	a.needRender = true
	if reasonRankApp(reason) >= reasonRankApp(a.renderReason) {
		a.renderReason = reason
	}
	switch reason {
	case renderer.ReasonForce, renderer.ReasonReplace, renderer.ReasonReset, renderer.ReasonResize, renderer.ReasonFlush:
		a.forceRender = true
	}
}

func reasonRankApp(r renderer.Reason) int {
	switch r {
	case renderer.ReasonUpdate:
		return 1
	case renderer.ReasonFlush:
		return 2
	case renderer.ReasonForce:
		return 3
	case renderer.ReasonResize:
		return 4
	case renderer.ReasonReplace, renderer.ReasonReset:
		return 5
	default:
		return 0
	}
}

func (a *App) paint() {
	if a.sched == nil || a.root == nil {
		a.needRender = false
		a.forceRender = false
		return
	}
	reason := a.renderReason
	force := a.forceRender
	a.needRender = false
	a.forceRender = false
	a.renderReason = renderer.ReasonUpdate

	w, h := a.width, a.height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	if a.budget != nil {
		a.budget.BeginPass(false)
	}
	if a.engine != nil {
		component.NotifyCommittedRows(a.transcript, a.engine.CommittedRows())
	}

	frame := a.root.Render(w)
	stable, _ := component.StablePrefixRows(a.root)

	var overlays []renderer.Overlay
	if a.overlays != nil && a.overlays.HasVisible(w, h) {
		for _, of := range a.overlays.Frames(w, h) {
			overlays = append(overlays, renderer.Overlay{
				Lines: of.Lines,
				Options: renderer.OverlayOptions{
					Anchor:     mapAnchor(of.Options.Anchor),
					Row:        renderer.SizeAbs(of.Layout.Row),
					Col:        renderer.SizeAbs(of.Layout.Col),
					Width:      renderer.SizeAbs(of.Layout.Width),
					MaxHeight:  renderer.SizeAbs(of.Layout.Height),
					Fullscreen: of.Fullscreen,
				},
			})
		}
	}

	req := renderer.Request{
		Frame:            frame,
		Width:            w,
		Height:           h,
		StablePrefixRows: stable,
		Overlays:         overlays,
		Reason:           reason,
		Notify:           a.root,
	}

	if a.budget != nil {
		if a.budget.EndPass() {
			req.Reason = renderer.ReasonForce
			force = true
		}
		req.ImageTransmit = a.budget.TakeTransmitString()
		req.ImagePurge = a.budget.TakePurgeString()
	}

	if force {
		a.sched.RequestImmediate(req)
	} else {
		a.sched.Request(req)
	}
}

func mapAnchor(a interact.OverlayAnchor) renderer.OverlayAnchor {
	switch a {
	case interact.AnchorTopLeft:
		return renderer.AnchorTopLeft
	case interact.AnchorTopCenter:
		return renderer.AnchorTopCenter
	case interact.AnchorTopRight:
		return renderer.AnchorTopRight
	case interact.AnchorLeftCenter:
		return renderer.AnchorLeftCenter
	case interact.AnchorRightCenter:
		return renderer.AnchorRightCenter
	case interact.AnchorBottomLeft:
		return renderer.AnchorBottomLeft
	case interact.AnchorBottomCenter:
		return renderer.AnchorBottomCenter
	case interact.AnchorBottomRight:
		return renderer.AnchorBottomRight
	default:
		return renderer.AnchorCenter
	}
}

// staticText is a one-shot text component for overlay titles.
type staticText struct {
	line string
	gen  component.Gen
}

func newStaticText(s string) *staticText { return &staticText{line: s} }

func (t *staticText) Render(width int) component.Frame {
	line := ansitext.TruncateToWidth(t.line, width, "")
	return component.NewFrame([]string{line}, t.gen.Next())
}

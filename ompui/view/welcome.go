package view

import (
	"strconv"
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/model"
)

// WelcomeInfo is everything the first-run block shows. The view reads no
// environment of its own; the host supplies the facts.
type WelcomeInfo struct {
	// AppName is the product name, e.g. "omp".
	AppName string
	// Version is the running build's version string.
	Version string
	// Path is the working directory.
	Path string
	// Home, when set, is collapsed to "~" in Path.
	Home string
	// Tip is an optional one-line hint. The host chooses it; the view never
	// rolls dice, so the same session always opens the same way.
	Tip string
}

// WelcomeRows renders the session-opening block.
//
// It is four short rows and no drawing. A first screen has one job — tell you
// which model is about to spend your money, in which directory — and a logo
// does not do that job. The tip is the only optional row, and it is the host's
// decision, not a random draw.
func (r Renderer) WelcomeRows(snap model.Snapshot, info WelcomeInfo, width int) []string {
	layout := r.opts.layout(width)
	sym := r.theme.Symbols
	inset := padding(layout.Inset)
	out := make([]string, 0, 5)

	name := info.AppName
	if name == "" {
		name = "omp"
	}
	title := apply(r.theme.Accent, apply(r.theme.Bold, name))
	if info.Version != "" {
		title += apply(r.theme.Dim, "  "+info.Version)
	}
	out = append(out, inset+fit(title, layout.Body, sym.Ellipsis))

	modelInfo := ParseModel(snap.Session.Model)
	if label := modelInfo.Label(sym.Sep); label != "" {
		row := apply(r.theme.Muted, sym.IconModel+" "+label)
		if level := parseThinkingLevel(snap.Session.ThinkingLevel); level != "" {
			row += apply(r.theme.Dim, sym.Sep+level)
		}
		out = append(out, inset+fit(row, layout.Body, sym.Ellipsis))
	}

	if path := shortenPath(info.Path, info.Home); path != "" {
		out = append(out, inset+apply(r.theme.StatusPath,
			elidePath(path, layout.Body, sym.Ellipsis)))
	}

	if session := welcomeSessionLabel(snap); session != "" {
		out = append(out, inset+apply(r.theme.Dim, fit(session, layout.Body, sym.Ellipsis)))
	}

	if tip := strings.TrimSpace(flattenLine(info.Tip)); tip != "" {
		out = append(out, "", inset+apply(r.theme.Dim, "Tip  ")+
			apply(r.theme.Muted, fit(tip, layout.Body-5, sym.Ellipsis)))
	}
	return out
}

// welcomeSessionLabel names the session being resumed, falling back to the
// message count so a resumed session never looks empty.
func welcomeSessionLabel(snap model.Snapshot) string {
	session := snap.Session
	parts := make([]string, 0, 2)
	if session.SessionName != "" {
		parts = append(parts, session.SessionName)
	}
	if session.MessageCount > 0 {
		parts = append(parts, strconv.Itoa(session.MessageCount)+" "+
			pluralize("message", session.MessageCount))
	}
	return strings.Join(parts, "  ")
}

// Welcome renders the session-opening block.
type Welcome struct {
	widget
	snap model.Snapshot
	info WelcomeInfo
}

// NewWelcome constructs a welcome block.
func NewWelcome(theme Theme, opts Options, info WelcomeInfo) *Welcome {
	return &Welcome{widget: newWidget(theme, opts), info: info}
}

// SetSnapshot installs session state.
func (w *Welcome) SetSnapshot(snap model.Snapshot) { w.snap = snap }

// SetInfo installs the host-supplied facts.
func (w *Welcome) SetInfo(info WelcomeInfo) { w.info = info }

// Info returns the host-supplied facts.
func (w *Welcome) Info() WelcomeInfo { return w.info }

// Render implements component.Component.
func (w *Welcome) Render(width int) component.Frame {
	return w.frame(width, w.r.WelcomeRows(w.snap, w.info, width))
}

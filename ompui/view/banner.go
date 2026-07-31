package view

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// noticeStyle maps a core notice level onto a glyph and a color.
func (r Renderer) noticeStyle(level string) (string, StyleFunc) {
	sym := r.theme.Symbols
	switch strings.ToLower(level) {
	case "error", "fatal":
		return sym.Error, r.theme.Error
	case "warn", "warning":
		return sym.Warning, r.theme.Warning
	case "success":
		return sym.Success, r.theme.Success
	default:
		return sym.Info, r.theme.Accent
	}
}

// NoticeRows renders a core notice inline: one marked line plus continuation
// rows, with the source attributed dimly at the end of the first row so the
// message itself keeps the reading position.
func (r Renderer) NoticeRows(notice model.Notice, width int) []string {
	if strings.TrimSpace(notice.Message) == "" {
		return nil
	}
	layout := r.opts.layout(width)
	glyph, style := r.noticeStyle(notice.Level)
	inset := padding(layout.Inset)
	marker := glyph + " "
	markerWidth := ansitext.VisibleWidth(marker)
	body := layout.Body - markerWidth
	if body < 1 {
		body = 1
	}

	lines := previewLines(notice.Message, maxBannerLines, body, r.theme.Symbols.Ellipsis)
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	head := inset + apply(style, marker+lines[0])
	if notice.Source != "" && !layout.Narrow {
		head += apply(r.theme.Dim, r.theme.Symbols.Sep+notice.Source)
	}
	out = append(out, fit(head, layout.Width, r.theme.Symbols.Ellipsis))
	for _, line := range lines[1:] {
		out = append(out, inset+padding(markerWidth)+apply(style, line))
	}
	return out
}

// ErrorBannerRows renders the pinned error banner shown above the editor after
// a turn ends on a provider error.
//
// This is the one place in the package that spends full-width rules: the banner
// has to survive being glanced past, it sits outside the transcript, and it
// carries its own dismissal contract. Bounded to a few lines so a proxy's HTML
// error page cannot take over the screen.
func (r Renderer) ErrorBannerRows(message string, width int) []string {
	layout := r.opts.layout(width)
	lines := previewLines(message, maxBannerLines, layout.Body-2, r.theme.Symbols.Ellipsis)
	if len(lines) == 0 {
		lines = []string{"Unknown error"}
	}
	inset := padding(layout.Inset)
	divider := apply(r.theme.Error, rule(r.theme.Symbols.Rule, layout.Width))

	out := make([]string, 0, len(lines)+3)
	out = append(out, divider)
	out = append(out, inset+apply(r.theme.Error, apply(r.theme.Bold, r.theme.Symbols.Error+" "+lines[0])))
	for _, line := range lines[1:] {
		out = append(out, inset+"  "+apply(r.theme.Error, line))
	}
	out = append(out, inset+apply(r.theme.Dim, "Dismissed when you send your next message."))
	return append(out, divider)
}

// NoticeBanner renders the most recent core notice.
type NoticeBanner struct {
	widget
	notice model.Notice
	has    bool
}

// NewNoticeBanner constructs a notice banner.
func NewNoticeBanner(theme Theme, opts Options) *NoticeBanner {
	return &NoticeBanner{widget: newWidget(theme, opts)}
}

// SetNotice installs a notice. Pass nil to clear.
func (b *NoticeBanner) SetNotice(notice *model.Notice) {
	if notice == nil {
		b.notice, b.has = model.Notice{}, false
		return
	}
	b.notice, b.has = *notice, true
}

// SetSnapshot takes the notice straight from a snapshot.
func (b *NoticeBanner) SetSnapshot(snap model.Snapshot) {
	b.SetNotice(snap.Status.LastNotice)
}

// Render implements component.Component.
func (b *NoticeBanner) Render(width int) component.Frame {
	if !b.has {
		return component.EmptyFrame(b.cache.gen.Current())
	}
	return b.frame(width, b.r.NoticeRows(b.notice, width))
}

// ErrorBanner renders the pinned turn-ending error above the editor.
type ErrorBanner struct {
	widget
	message string
}

// NewErrorBanner constructs an error banner.
func NewErrorBanner(theme Theme, opts Options) *ErrorBanner {
	return &ErrorBanner{widget: newWidget(theme, opts)}
}

// SetMessage installs the error text. Empty clears the banner.
func (b *ErrorBanner) SetMessage(message string) { b.message = message }

// SetSnapshot takes the last error straight from a snapshot.
func (b *ErrorBanner) SetSnapshot(snap model.Snapshot) { b.message = snap.Status.LastError }

// Message returns the currently pinned error.
func (b *ErrorBanner) Message() string { return b.message }

// Render implements component.Component.
func (b *ErrorBanner) Render(width int) component.Frame {
	if strings.TrimSpace(b.message) == "" {
		return component.EmptyFrame(b.cache.gen.Current())
	}
	return b.frame(width, b.r.ErrorBannerRows(b.message, width))
}

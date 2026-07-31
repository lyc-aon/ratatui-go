package view

import "github.com/lyc-aon/ratatui-go/ompui/component"

// Compile-time interface satisfaction checks for the views this package
// exports. The optional interfaces matter as much as Component: a Transcript
// that silently stopped reporting its stable prefix or its committed rows would
// keep rendering correctly while quietly costing the engine a full re-scan and
// its scrollback commits every frame.
var (
	_ component.Component            = (*Transcript)(nil)
	_ component.Invalidator          = (*Transcript)(nil)
	_ component.TightLayoutAware     = (*Transcript)(nil)
	_ component.StablePrefix         = (*Transcript)(nil)
	_ component.CommittedRowsAware   = (*Transcript)(nil)
	_ component.ViewportTailProvider = (*Transcript)(nil)

	_ component.Component = (*StatusLine)(nil)
	_ component.Component = (*Footer)(nil)
	_ component.Component = (*Welcome)(nil)
	_ component.Component = (*WorkingIndicator)(nil)
	_ component.Component = (*NoticeBanner)(nil)
	_ component.Component = (*ErrorBanner)(nil)
	_ component.Component = (*TodoSummary)(nil)
	_ component.Component = (*SubagentSummary)(nil)

	_ component.Invalidator      = (*StatusLine)(nil)
	_ component.TightLayoutAware = (*StatusLine)(nil)
	_ component.Invalidator      = (*Footer)(nil)
	_ component.TightLayoutAware = (*Footer)(nil)
	_ component.Invalidator      = (*Welcome)(nil)
	_ component.TightLayoutAware = (*Welcome)(nil)
	_ component.Invalidator      = (*WorkingIndicator)(nil)
	_ component.TightLayoutAware = (*WorkingIndicator)(nil)
	_ component.Invalidator      = (*NoticeBanner)(nil)
	_ component.TightLayoutAware = (*NoticeBanner)(nil)
	_ component.Invalidator      = (*ErrorBanner)(nil)
	_ component.TightLayoutAware = (*ErrorBanner)(nil)
	_ component.Invalidator      = (*TodoSummary)(nil)
	_ component.TightLayoutAware = (*TodoSummary)(nil)
	_ component.Invalidator      = (*SubagentSummary)(nil)
	_ component.TightLayoutAware = (*SubagentSummary)(nil)
)

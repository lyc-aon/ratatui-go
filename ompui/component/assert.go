package component

// Compile-time interface satisfaction checks for retained types.

var (
	_ Component            = (*Container)(nil)
	_ Invalidator          = (*Container)(nil)
	_ Disposable           = (*Container)(nil)
	_ TightLayoutAware     = (*Container)(nil)
	_ StablePrefix         = (*Container)(nil)
	_ CommittedRowsAware   = (*Container)(nil)
	_ ViewportTailProvider = (*Container)(nil)
	_ InputHandler         = (*Container)(nil)

	_ Component            = (*Remote)(nil)
	_ InputHandler         = (*Remote)(nil)
	_ KeyReleaseInterest   = (*Remote)(nil)
	_ Focusable            = (*Remote)(nil)
	_ TerminalCursorAware  = (*Remote)(nil)
	_ Invalidator          = (*Remote)(nil)
	_ Disposable           = (*Remote)(nil)
	_ CommittedRowsAware   = (*Remote)(nil)
	_ ViewportTailProvider = (*Remote)(nil)

	_ Focusable           = (*FocusState)(nil)
	_ TerminalCursorAware = (*FocusState)(nil)
)

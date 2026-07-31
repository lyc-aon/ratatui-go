package termcaps

import "strings"

// SynchronizedOutputUserOverride resolves an explicit user override for DEC 2026
// synchronized output. Returns (false, true) for opt-out, (true, true) for
// force-on, or (_, false) when the user expressed no preference.
// Opt-out beats force-on when both are set.
func SynchronizedOutputUserOverride(env Env) (enabled bool, set bool) {
	if env.Has("PI_NO_SYNC_OUTPUT") || env.Get("PI_TUI_SYNC_OUTPUT") == "0" {
		return false, true
	}
	if env.Get("PI_FORCE_SYNC_OUTPUT") == "1" || env.Get("PI_TUI_SYNC_OUTPUT") == "1" {
		return true, true
	}
	return false, false
}

// AdvertisesSynchronizedOutput reports whether TERM_FEATURES contains the Sy token.
func AdvertisesSynchronizedOutput(termFeatures string) bool {
	return strings.Contains(termFeatures, "Sy")
}

// ShouldEnableSynchronizedOutputByDefault decides the static DEC 2026 default.
//
// Precedence (highest first):
//  1. Explicit user override (opt-out beats force-on)
//  2. TERM_FEATURES contains Sy
//  3. WT_SESSION present (Windows Terminal / WSL)
//  4. Risky multiplexers (TMUX/STY/ZELLIJ/tmux*|screen* TERM) → off
//  5. Known direct terminals (kitty/ghostty/wezterm/iterm2/alacritty/vscode) → on
//  6. Everything else → off (runtime DECRQM may upgrade later)
func ShouldEnableSynchronizedOutputByDefault(env Env, terminalID TerminalID) bool {
	if enabled, set := SynchronizedOutputUserOverride(env); set {
		return enabled
	}
	if AdvertisesSynchronizedOutput(env.Get("TERM_FEATURES")) {
		return true
	}
	if env.Has("WT_SESSION") {
		return true
	}
	if isRiskyMultiplexer(env) {
		return false
	}
	switch terminalID {
	case TerminalKitty, TerminalGhostty, TerminalWezTerm, TerminalITerm2, TerminalAlacritty, TerminalVSCode:
		return true
	default:
		return false
	}
}

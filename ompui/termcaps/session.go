package termcaps

import (
	"strings"
)

// SessionIDFromTTYPath derives a stable session id from a TTY device path.
// "/dev/pts/3" → "pts-3". Returns "" when path is empty or does not start with "/dev/".
func SessionIDFromTTYPath(ttyPath string) string {
	if !strings.HasPrefix(ttyPath, "/dev/") {
		return ""
	}
	rest := ttyPath[len("/dev/"):]
	if rest == "" {
		return ""
	}
	return strings.ReplaceAll(rest, "/", "-")
}

// SessionIDFromEnv builds a terminal-session identity from multiplexer / host
// env markers when no TTY path is available. Prefer inner multiplexers over
// host emulators. Empty marker values are ignored (fall through).
//
// Order: ZELLIJ_PANE_ID → TMUX_PANE → CMUX_SURFACE_ID → KITTY_WINDOW_ID →
// WEZTERM_PANE → TERM_SESSION_ID → WT_SESSION.
func SessionIDFromEnv(env Env) string {
	if zellijPane := env.Get("ZELLIJ_PANE_ID"); zellijPane != "" {
		// Session names are user-chosen and used as breadcrumb filenames —
		// normalize path separators like the TTY branch does.
		if sess := env.Get("ZELLIJ_SESSION_NAME"); sess != "" {
			sess = strings.Map(func(r rune) rune {
				if r == '/' || r == '\\' {
					return '-'
				}
				return r
			}, sess)
			return "zellij-" + sess + "-" + zellijPane
		}
		return "zellij-" + zellijPane
	}
	if tmuxPane := env.Get("TMUX_PANE"); tmuxPane != "" {
		return "tmux-" + tmuxPane
	}
	if cmux := env.Get("CMUX_SURFACE_ID"); cmux != "" {
		return "cmux-" + cmux
	}
	// Kitty before WezTerm/others, matching DetectTerminalID order.
	if kitty := env.Get("KITTY_WINDOW_ID"); kitty != "" {
		return "kitty-" + kitty
	}
	if wez := env.Get("WEZTERM_PANE"); wez != "" {
		return "wezterm-" + wez
	}
	if apple := env.Get("TERM_SESSION_ID"); apple != "" {
		return "apple-" + apple
	}
	if wt := env.Get("WT_SESSION"); wt != "" {
		return "wt-" + wt
	}
	return ""
}

// ResolveSessionID picks a stable terminal-session identity.
//
// When stdinIsTTY is true and ttyPath yields a /dev/… id, that wins.
// Otherwise falls back to SessionIDFromEnv. Empty string means unidentified
// (e.g. fully piped with no mux/host markers).
func ResolveSessionID(stdinIsTTY bool, ttyPath string, env Env) string {
	if stdinIsTTY {
		if id := SessionIDFromTTYPath(ttyPath); id != "" {
			return id
		}
	}
	return SessionIDFromEnv(env)
}

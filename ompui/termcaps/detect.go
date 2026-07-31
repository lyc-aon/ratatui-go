package termcaps

import "strings"

// DetectTerminalID resolves the terminal family from env using the OMP order:
// explicit window/pane markers, then TERM_PROGRAM, then TERM ghostty, then COLORTERM.
func DetectTerminalID(env Env) TerminalID {
	if env.Has("KITTY_WINDOW_ID") {
		return TerminalKitty
	}
	if env.Has("GHOSTTY_RESOURCES_DIR") {
		return TerminalGhostty
	}
	if env.Has("WEZTERM_PANE") {
		return TerminalWezTerm
	}
	if env.Has("ITERM_SESSION_ID") {
		return TerminalITerm2
	}
	if env.Has("VSCODE_PID") {
		return TerminalVSCode
	}
	if env.Has("ALACRITTY_WINDOW_ID") {
		return TerminalAlacritty
	}

	if tp := env.Get("TERM_PROGRAM"); tp != "" {
		switch strings.ToLower(tp) {
		case "kitty":
			return TerminalKitty
		case "ghostty":
			return TerminalGhostty
		case "wezterm":
			return TerminalWezTerm
		case "iterm.app":
			return TerminalITerm2
		case "vscode":
			return TerminalVSCode
		case "alacritty":
			return TerminalAlacritty
		}
	}

	if strings.Contains(strings.ToLower(env.Get("TERM")), "ghostty") {
		return TerminalGhostty
	}

	if ct := env.Get("COLORTERM"); ct != "" {
		switch strings.ToLower(ct) {
		case "truecolor", "24bit":
			return TerminalTrueColor
		}
	}
	return TerminalBase
}

// IsMultiplexerSession reports whether the process is inside a terminal
// multiplexer where ED3 / destructive scrollback clears are hostile.
//
// Signals: TMUX, STY, ZELLIJ, CMUX_WORKSPACE_ID, CMUX_SURFACE_ID, or a
// TERM prefix of tmux*/screen*. CMUX_SOCKET_PATH alone is not a session signal.
func IsMultiplexerSession(env Env) bool {
	if env.Has("TMUX") || env.Has("STY") || env.Has("ZELLIJ") {
		return true
	}
	if env.Has("CMUX_WORKSPACE_ID") || env.Has("CMUX_SURFACE_ID") {
		return true
	}
	term := strings.ToLower(env.Get("TERM"))
	return strings.HasPrefix(term, "tmux") || strings.HasPrefix(term, "screen")
}

// ReportsSizeOnAltScreenToggle is true when entering/leaving the alternate
// screen causes the host to re-report size (Warp). PI_TUI_RESIZE_IN_PLACE=1|0
// forces the answer on/off.
func ReportsSizeOnAltScreenToggle(env Env) bool {
	override := env.Get("PI_TUI_RESIZE_IN_PLACE")
	if override == "0" || strings.EqualFold(override, "false") {
		return false
	}
	if override == "1" || strings.EqualFold(override, "true") {
		return true
	}
	return strings.EqualFold(env.Get("TERM_PROGRAM"), "warpterminal")
}

// ResizeRepaintsInPlace is true when resize should avoid alt-screen borrow and
// ED3 rewrap (multiplexer panes and alt-toggle size-loop hosts).
func ResizeRepaintsInPlace(env Env) bool {
	return IsMultiplexerSession(env) || ReportsSizeOnAltScreenToggle(env)
}

// isRiskyMultiplexer matches the shared tmux/screen/zellij TERM+env gate used
// by sync-output and DECCARA policies (does not include CMUX).
func isRiskyMultiplexer(env Env) bool {
	if env.Has("TMUX") || env.Has("STY") || env.Has("ZELLIJ") {
		return true
	}
	term := strings.ToLower(env.Get("TERM"))
	return strings.HasPrefix(term, "tmux") || strings.HasPrefix(term, "screen")
}

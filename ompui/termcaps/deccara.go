package termcaps

import "strings"

// DetectRectangularSGRSupport reports whether Kitty-style DECCARA rectangular
// SGR (including background) is safe to emit.
//
// Only kitty implements the SGR-background extension. Ghostty does not.
// Disabled under tmux/screen/zellij multiplexers and via PI_NO_DECCARA
// (any truthy value other than "0"/"false").
func DetectRectangularSGRSupport(terminalID TerminalID, env Env) bool {
	if terminalID != TerminalKitty {
		return false
	}
	kill := env.Get("PI_NO_DECCARA")
	if kill != "" && kill != "0" && !strings.EqualFold(kill, "false") {
		return false
	}
	if isRiskyMultiplexer(env) {
		return false
	}
	return true
}

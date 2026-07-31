package termcaps

import "strings"

// HyperlinksUserOverride resolves an explicit OSC 8 hyperlink override.
// Only the literal value "1" toggles either flag. Opt-out beats force-on.
// Returns (enabled, set).
func HyperlinksUserOverride(env Env) (enabled bool, set bool) {
	if env.Get("PI_NO_HYPERLINKS") == "1" {
		return false, true
	}
	if env.Get("PI_FORCE_HYPERLINKS") == "1" {
		return true, true
	}
	return false, false
}

// ShouldEnableHyperlinksByDefault decides the OSC 8 default.
//
// Precedence (highest first):
//  1. Explicit user override (opt-out beats force-on)
//  2. Static terminal table hyperlinks=false → off
//  3. STY (GNU screen) → off (vetoes nested tmux)
//  4. TMUX set: on only when TERM_PROGRAM=tmux reports version >= 3.4
//  5. TERM screen* or tmux* without TMUX → off
//  6. Otherwise honor the static terminal capability (true)
func ShouldEnableHyperlinksByDefault(env Env, terminalID TerminalID) bool {
	if enabled, set := HyperlinksUserOverride(env); set {
		return enabled
	}
	if !LookupTerminalInfo(terminalID).Hyperlinks {
		return false
	}
	// STY is GNU screen's explicit session marker. It vetoes tmux enabling when
	// multiplexers are nested because screen cannot forward OSC 8 anywhere in the path.
	if env.Has("STY") {
		return false
	}
	// TMUX is authoritative over TERM (which may be screen-256color under tmux).
	if env.Has("TMUX") {
		v, ok := ParseTmuxVersionFromEnv(env)
		if !ok {
			return false
		}
		return v.AtLeast(3, 4)
	}
	term := strings.ToLower(env.Get("TERM"))
	if strings.HasPrefix(term, "screen") {
		return false
	}
	if strings.HasPrefix(term, "tmux") {
		return false
	}
	return true
}

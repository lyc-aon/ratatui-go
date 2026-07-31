package termcaps

import (
	"strconv"
	"strings"
)

// ParseMajorMinorVersion parses the leading major.minor from versionRaw.
// Trailing suffixes (e.g. "3.5a", "1.22.10341.0") are accepted; only the first
// two numeric components are returned. Returns ok=false when unparsable.
func ParseMajorMinorVersion(versionRaw string) (Version, bool) {
	s := strings.TrimSpace(versionRaw)
	if s == "" {
		return Version{}, false
	}
	// Match ^(\d+)\.(\d+)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) || s[i] != '.' {
		return Version{}, false
	}
	major, err := strconv.Atoi(s[:i])
	if err != nil {
		return Version{}, false
	}
	j := i + 1
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i+1 {
		return Version{}, false
	}
	minor, err := strconv.Atoi(s[i+1 : j])
	if err != nil {
		return Version{}, false
	}
	return Version{Major: major, Minor: minor}, true
}

// ParseTmuxVersionFromEnv reads tmux's self-reported version from
// TERM_PROGRAM=tmux + TERM_PROGRAM_VERSION. Older tmux (or stripped env) yields ok=false.
func ParseTmuxVersionFromEnv(env Env) (Version, bool) {
	if !strings.EqualFold(env.Get("TERM_PROGRAM"), "tmux") {
		return Version{}, false
	}
	return ParseMajorMinorVersion(env.Get("TERM_PROGRAM_VERSION"))
}

// AtLeast reports whether v is >= (major, minor).
func (v Version) AtLeast(major, minor int) bool {
	if v.Major > major {
		return true
	}
	return v.Major == major && v.Minor >= minor
}

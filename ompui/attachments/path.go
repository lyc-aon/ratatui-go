package attachments

import (
	"os"
	"path/filepath"
	"strings"
)

// isURLPath rejects anything that would require network or non-file open.
func isURLPath(p string) bool {
	s := strings.TrimSpace(p)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "ftp://") ||
		strings.HasPrefix(lower, "file://") {
		return true
	}
	// scheme:// form (but not Windows drive)
	if i := strings.Index(s, "://"); i > 0 {
		scheme := s[:i]
		if strings.IndexFunc(scheme, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '+' || r == '.' || r == '-')
		}) < 0 {
			return true
		}
	}
	return false
}

// expandUser expands a leading ~ using home (empty → os user home when possible).
func expandUser(p, home string) string {
	if p == "~" {
		if home == "" {
			if h, err := os.UserHomeDir(); err == nil {
				return h
			}
			return p
		}
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		h := home
		if h == "" {
			var err error
			h, err = os.UserHomeDir()
			if err != nil {
				return p
			}
		}
		return filepath.Join(h, p[2:])
	}
	return p
}

// normalizeUnicodeSpaces maps common Unicode spaces to ASCII space (OMP path-utils).
func normalizeUnicodeSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	changed := false
	for _, r := range s {
		switch r {
		case '\u00A0', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004',
			'\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200A',
			'\u202F', '\u205F', '\u3000':
			b.WriteByte(' ')
			changed = true
		default:
			b.WriteRune(r)
		}
	}
	if !changed {
		return s
	}
	return b.String()
}


// resolveAgainstRoot expands ~, normalizes spaces, and joins relative paths to root.
func resolveAgainstRoot(raw, root, home string) string {
	p := normalizeUnicodeSpaces(strings.TrimSpace(raw))
	p = expandUser(p, home)
	if p == "" {
		return ""
	}
	// Bare "/" is a workspace-root alias in OMP tools; keep same for relative safety.
	if p == "/" || p == string(filepath.Separator) {
		if root != "" {
			return filepath.Clean(root)
		}
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	base := root
	if base == "" {
		if wd, err := os.Getwd(); err == nil {
			base = wd
		}
	}
	if base == "" {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}

// confined reports whether abs is inside root (after Clean). Empty root → true.
func confined(abs, root string) bool {
	if root == "" {
		return true
	}
	abs = filepath.Clean(abs)
	root = filepath.Clean(root)
	if abs == root {
		return true
	}
	sep := string(filepath.Separator)
	prefix := root
	if !strings.HasSuffix(prefix, sep) {
		prefix += sep
	}
	return strings.HasPrefix(abs, prefix)
}

// evalPolicy applies symlink + root rules. Returns the path to read (may be the
// symlink path when AllowSymlinks is false we never get here for links), or a notice.
func evalPolicy(abs string, opts Options) (readPath string, notice *Notice) {
	fi, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &Notice{Path: abs, Code: CodeMissing, Message: "file not found: " + abs}
		}
		return "", &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot stat: " + err.Error()}
	}

	isSymlink := fi.Mode()&os.ModeSymlink != 0
	if isSymlink && !opts.AllowSymlinks {
		return "", &Notice{Path: abs, Code: CodeSymlink, Message: "symlink rejected: " + abs}
	}

	// Resolve final target for root checks and regular-file test.
	real := abs
	if isSymlink {
		target, err := filepath.EvalSymlinks(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return "", &Notice{Path: abs, Code: CodeMissing, Message: "symlink target missing: " + abs}
			}
			return "", &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot resolve symlink: " + err.Error()}
		}
		real = target
		fi, err = os.Stat(real)
		if err != nil {
			if os.IsNotExist(err) {
				return "", &Notice{Path: abs, Code: CodeMissing, Message: "file not found: " + abs}
			}
			return "", &Notice{Path: abs, Code: CodeUnreadable, Message: "cannot stat target: " + err.Error()}
		}
	}

	if !opts.FollowOutsideRoot && !confined(real, opts.Root) {
		return "", &Notice{Path: abs, Code: CodeOutsideRoot, Message: "path escapes configured root: " + abs}
	}

	if !fi.Mode().IsRegular() {
		return "", &Notice{Path: abs, Code: CodeNotFile, Message: "not a regular file: " + abs}
	}

	// Read via the original path when it is a permitted symlink so open goes
	// through the link; otherwise real == abs.
	return abs, nil
}

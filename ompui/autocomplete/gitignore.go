package autocomplete

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// simpleGitignore is a practical, partial .gitignore matcher.
// Supports: blank/comment lines, trailing slash (dirs only), leading slash
// (root-relative), * single-segment wildcards, and ** as "any path prefix".
// Negation (!) is honored for previously-matched patterns in the same file.
type simpleGitignore struct {
	// patterns are relative to the directory that owns the .gitignore.
	patterns []giPattern
}

type giPattern struct {
	raw       string
	negated   bool
	dirOnly   bool
	rooted    bool
	segments  []string // split on / after normalization; "**" kept as token
	matchAll  bool     // bare "**"
}

func parseGitignoreFile(path string) (*simpleGitignore, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g := &simpleGitignore{}
	sc := bufio.NewScanner(f)
	// Cap line length; ignore-file abuse should not blow memory.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p, ok := parseGIPattern(line)
		if ok {
			g.patterns = append(g.patterns, p)
		}
	}
	return g, sc.Err()
}

func parseGIPattern(line string) (giPattern, bool) {
	p := giPattern{raw: line}
	if strings.HasPrefix(line, "!") {
		p.negated = true
		line = line[1:]
		if line == "" {
			return p, false
		}
	}
	// Unescape leading \# or \! — rare; keep simple.
	if strings.HasPrefix(line, `\`) && len(line) > 1 {
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		p.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		p.rooted = true
		line = strings.TrimPrefix(line, "/")
	}
	line = strings.TrimPrefix(line, "./")
	if line == "" || line == "**" {
		p.matchAll = line == "**"
		if p.matchAll {
			return p, true
		}
		return p, false
	}
	// Normalize to forward slashes.
	line = filepath.ToSlash(line)
	p.segments = strings.Split(line, "/")
	return p, true
}

// ignored reports whether rel (forward-slash, relative to the gitignore dir)
// should be ignored. isDir distinguishes directory-only patterns.
func (g *simpleGitignore) ignored(rel string, isDir bool) bool {
	if g == nil || len(g.patterns) == 0 {
		return false
	}
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	rel = strings.TrimPrefix(rel, "/")
	matched := false
	for _, p := range g.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		ok := p.matches(rel)
		if !ok {
			continue
		}
		if p.negated {
			matched = false
		} else {
			matched = true
		}
	}
	return matched
}

func (p giPattern) matches(rel string) bool {
	if p.matchAll {
		return true
	}
	if len(p.segments) == 0 {
		return false
	}
	// Rooted patterns match from the start of rel.
	// Unrooted single-segment patterns match any path segment (basename or deeper).
	// Unrooted multi-segment patterns match as a suffix or anywhere via **.
	if p.rooted {
		return matchSegments(p.segments, strings.Split(rel, "/"), true)
	}
	// No slash in pattern → match against any single path segment.
	if len(p.segments) == 1 && p.segments[0] != "**" {
		parts := strings.Split(rel, "/")
		for _, part := range parts {
			if matchSegment(p.segments[0], part) {
				return true
			}
		}
		return false
	}
	// Multi-segment unrooted: try every suffix start.
	parts := strings.Split(rel, "/")
	for i := 0; i < len(parts); i++ {
		if matchSegments(p.segments, parts[i:], true) {
			return true
		}
	}
	return false
}

func matchSegments(pat, parts []string, full bool) bool {
	// Simple DP-free matcher with ** support.
	pi, ti := 0, 0
	for pi < len(pat) && ti < len(parts) {
		if pat[pi] == "**" {
			// Trailing ** matches the rest.
			if pi == len(pat)-1 {
				return true
			}
			// Try consuming zero or more path segments.
			for k := ti; k <= len(parts); k++ {
				if matchSegments(pat[pi+1:], parts[k:], full) {
					return true
				}
			}
			return false
		}
		if !matchSegment(pat[pi], parts[ti]) {
			return false
		}
		pi++
		ti++
	}
	// Consume trailing **
	for pi < len(pat) && pat[pi] == "**" {
		pi++
	}
	if pi != len(pat) {
		return false
	}
	if full {
		return ti == len(parts)
	}
	return true
}

func matchSegment(pat, name string) bool {
	if pat == "*" {
		return true
	}
	// Single-segment glob: * only.
	if !strings.Contains(pat, "*") {
		return pat == name
	}
	return matchStar(pat, name)
}

func matchStar(pat, name string) bool {
	// Recursive star matcher for a single segment.
	for {
		if pat == "" {
			return name == ""
		}
		if pat[0] == '*' {
			// Greedy: try skip 0..n of name.
			for i := 0; i <= len(name); i++ {
				if matchStar(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if name == "" || pat[0] != name[0] {
			return false
		}
		pat = pat[1:]
		name = name[1:]
	}
}

// gitignoreStack tracks nested .gitignore files during a walk.
type gitignoreStack struct {
	mu    sync.Mutex
	layers []giLayer
}

type giLayer struct {
	// dirRel is the walk-relative directory that owns this .gitignore ("" = root).
	dirRel string
	g      *simpleGitignore
}

func (s *gitignoreStack) push(dirRel string, g *simpleGitignore) {
	if g == nil || len(g.patterns) == 0 {
		return
	}
	s.mu.Lock()
	s.layers = append(s.layers, giLayer{dirRel: dirRel, g: g})
	s.mu.Unlock()
}

func (s *gitignoreStack) pop(dirRel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.layers); n > 0 && s.layers[n-1].dirRel == dirRel {
		s.layers = s.layers[:n-1]
	}
}

// ignored walks layers bottom-up; later (deeper) patterns win via sequential apply.
func (s *gitignoreStack) ignored(rel string, isDir bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := false
	for _, layer := range s.layers {
		// Path relative to the layer's directory.
		local := rel
		if layer.dirRel != "" {
			prefix := layer.dirRel
			if !strings.HasPrefix(rel, prefix) {
				continue
			}
			rest := strings.TrimPrefix(rel, prefix)
			rest = strings.TrimPrefix(rest, "/")
			local = rest
		}
		// Apply each pattern in file order within this layer.
		for _, p := range layer.g.patterns {
			if p.dirOnly && !isDir {
				continue
			}
			if !p.matches(local) {
				continue
			}
			if p.negated {
				matched = false
			} else {
				matched = true
			}
		}
	}
	return matched
}

// loadGitignoreAt tries to read dirAbs/.gitignore.
func loadGitignoreAt(dirAbs string) *simpleGitignore {
	p := filepath.Join(dirAbs, ".gitignore")
	g, err := parseGitignoreFile(p)
	if err != nil {
		return nil
	}
	return g
}

// pathJoinRel joins a parent rel with a name using forward slashes.
func pathJoinRel(parent, name string) string {
	if parent == "" {
		return name
	}
	return path.Join(parent, name)
}

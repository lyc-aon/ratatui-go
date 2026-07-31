package autocomplete

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Default trash directory basenames skipped during recursive discovery.
// Immediate prefix listing still shows them (except .git, which OMP always hides).
var defaultSkipDirs = map[string]struct{}{
	".git":         {},
	"vendor":       {},
	"node_modules": {},
	"dist":         {},
	"build":        {},
	"target":       {},
	".hg":          {},
	".svn":         {},
	".jj":          {},
	"__pycache__":  {},
	".tox":         {},
	".venv":        {},
	"venv":         {},
	".next":        {},
	".nuxt":        {},
	".turbo":       {},
	".cache":       {},
	"coverage":     {},
}

// FSSource is the production bounded filesystem FileSource.
//
// No shell/find subprocess, no cwd mutation, no process-global cache.
// Discover always walks fresh; ListDir is a single readdir with a width cap.
type FSSource struct {
	MaxResults     int
	MaxScanEntries int
	MaxDepth       int
	// SkipDirs overrides defaultSkipDirs when non-nil.
	SkipDirs map[string]struct{}
	// RespectGitignore enables simple .gitignore filtering (default true).
	RespectGitignore bool
	// IncludeHidden includes dotfiles in discovery (default true, OMP hidden:true).
	IncludeHidden bool
}

// NewFSSource builds a production source with default bounds.
func NewFSSource() *FSSource {
	return &FSSource{
		MaxResults:       DefaultMaxResults,
		MaxScanEntries:   DefaultMaxScanEntries,
		MaxDepth:         DefaultMaxDepth,
		RespectGitignore: true,
		IncludeHidden:    true,
	}
}

func (s *FSSource) bounds() (maxResults, maxScan, maxDepth int) {
	maxResults = s.MaxResults
	if maxResults <= 0 {
		maxResults = DefaultMaxResults
	}
	maxScan = s.MaxScanEntries
	if maxScan <= 0 {
		maxScan = DefaultMaxScanEntries
	}
	maxDepth = s.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return
}

func (s *FSSource) skipSet() map[string]struct{} {
	if s.SkipDirs != nil {
		return s.SkipDirs
	}
	return defaultSkipDirs
}

// ListDir implements FileSource.
func (s *FSSource) ListDir(ctx context.Context, absDir string) ([]FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absDir = filepath.Clean(absDir)
	f, err := os.Open(absDir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Cap directory width so huge dirs cannot stall the UI.
	const maxDirents = 2000
	infos, err := f.ReadDir(maxDirents)
	if err != nil && len(infos) == 0 {
		return nil, err
	}
	out := make([]FileEntry, 0, len(infos))
	for _, de := range infos {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := de.Name()
		// OMP always skips .git in listing.
		if name == ".git" {
			continue
		}
		isDir, ok := resolveDirent(absDir, de)
		if !ok {
			continue
		}
		out = append(out, FileEntry{
			Name:  name,
			IsDir: isDir,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Discover implements FileSource — cancellable recursive walk with bounds.
func (s *FSSource) Discover(ctx context.Context, root, query string) ([]FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maxResults, maxScan, maxDepth := s.bounds()
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, err
	}

	lowerQ := strings.ToLower(query)
	skip := s.skipSet()

	type hit struct {
		entry FileEntry
		score int
		ord   int
	}
	var hits []hit
	scanned := 0
	ord := 0

	// Symlink cycle guard: resolved real paths of directories we entered.
	visited := map[string]struct{}{}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		visited[real] = struct{}{}
	} else {
		visited[root] = struct{}{}
	}

	gi := &gitignoreStack{}
	if s.RespectGitignore {
		if g := loadGitignoreAt(root); g != nil {
			gi.push("", g)
		}
	}

	var walk func(abs, rel string, depth int) error
	walk = func(abs, rel string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if scanned >= maxScan {
			return errScanBudget
		}
		if depth > maxDepth {
			return nil
		}
		f, err := os.Open(abs)
		if err != nil {
			return nil // unreadable — skip
		}
		defer f.Close()

		const batch = 256
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			if scanned >= maxScan || len(hits) >= maxResults*4 && maxResults > 0 {
				// Keep collecting a bit past maxResults so ranking can pick best;
				// hard stop at 4× to bound memory.
				if scanned >= maxScan {
					return errScanBudget
				}
			}
			batchEnts, readErr := f.ReadDir(batch)
			for _, de := range batchEnts {
				if err := ctx.Err(); err != nil {
					return err
				}
				scanned++
				if scanned > maxScan {
					return errScanBudget
				}
				name := de.Name()
				if name == "." || name == ".." {
					continue
				}
				// Always skip .git content in recursive walks.
				if name == ".git" {
					continue
				}
				if !s.IncludeHidden && strings.HasPrefix(name, ".") {
					continue
				}
				childRel := pathJoinRel(rel, name)
				// Reject .git path segments anywhere.
				if hasGitSegment(childRel) {
					continue
				}
				isDir, ok := resolveDirent(abs, de)
				if !ok {
					continue
				}
				if s.RespectGitignore && gi.ignored(childRel, isDir) {
					continue
				}
				// Trash dirs: do not emit or descend.
				if isDir {
					if _, trash := skip[name]; trash {
						continue
					}
				}

				// Score / collect files and non-trash dirs.
				display := childRel
				if isDir {
					display = childRel + "/"
				}
				if lowerQ == "" || fuzzyMatch(lowerQ, strings.ToLower(filepath.ToSlash(childRel))) {
					sc := 1
					if lowerQ != "" {
						sc = fuzzyScore(lowerQ, strings.ToLower(filepath.ToSlash(childRel)))
						// Also try basename for tighter matches.
						baseSc := fuzzyScore(lowerQ, strings.ToLower(name))
						if baseSc > sc {
							sc = baseSc
						}
					}
					if sc > 0 || lowerQ == "" {
						if lowerQ == "" {
							sc = 1
						}
						hits = append(hits, hit{
							entry: FileEntry{
								RelPath: filepath.ToSlash(display),
								Name:    name,
								IsDir:   isDir,
							},
							score: sc,
							ord:   ord,
						})
						ord++
					}
				}

				if !isDir {
					continue
				}
				if depth+1 > maxDepth {
					continue
				}
				childAbs := filepath.Join(abs, name)
				// Symlink cycle check.
				real := childAbs
				if de.Type()&fs.ModeSymlink != 0 {
					if r, err := filepath.EvalSymlinks(childAbs); err == nil {
						real = r
					}
				} else if r, err := filepath.EvalSymlinks(childAbs); err == nil {
					real = r
				}
				if _, seen := visited[real]; seen {
					continue
				}
				visited[real] = struct{}{}

				// Nested gitignore.
				var pushed string
				if s.RespectGitignore {
					if g := loadGitignoreAt(childAbs); g != nil {
						pushed = childRel
						gi.push(childRel, g)
					}
				}
				_ = walk(childAbs, childRel, depth+1)
				if pushed != "" {
					gi.pop(pushed)
				}
				// Keep real in visited permanently to block symlink cycles.
			}
			if readErr != nil {
				break
			}
			if len(batchEnts) < batch {
				break
			}
		}
		return nil
	}

	_ = walk(root, "", 0)

	// Rank: higher score first; stable by encounter order.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].ord < hits[j].ord
	})
	if len(hits) > maxResults {
		hits = hits[:maxResults]
	}
	out := make([]FileEntry, len(hits))
	for i, h := range hits {
		out[i] = h.entry
	}
	return out, nil
}

// errScanBudget is a private sentinel; Discover returns partial results instead.
var errScanBudget = errBudget{}

type errBudget struct{}

func (errBudget) Error() string { return "autocomplete: scan budget exhausted" }

func hasGitSegment(rel string) bool {
	rel = filepath.ToSlash(rel)
	if rel == ".git" {
		return true
	}
	return strings.HasPrefix(rel, ".git/") || strings.Contains(rel, "/.git/") || strings.HasSuffix(rel, "/.git")
}

func resolveDirent(parent string, de os.DirEntry) (isDir bool, ok bool) {
	if de.IsDir() {
		return true, true
	}
	// Symlink-to-dir: OMP stats through the link.
	if de.Type()&fs.ModeSymlink != 0 {
		info, err := os.Stat(filepath.Join(parent, de.Name()))
		if err != nil {
			return false, false
		}
		return info.IsDir(), true
	}
	// Regular file (or other non-dir).
	if de.Type().IsRegular() || de.Type() == 0 {
		return false, true
	}
	// Fallback stat for unusual types.
	info, err := de.Info()
	if err != nil {
		return false, false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		st, err := os.Stat(filepath.Join(parent, de.Name()))
		if err != nil {
			return false, false
		}
		return st.IsDir(), true
	}
	return info.IsDir(), true
}

// StaticFileSource is an injectable in-memory source for tests.
type StaticFileSource struct {
	// Dirs maps absolute directory path → immediate children.
	Dirs map[string][]FileEntry
	// Tree is a flat list of RelPath entries under a virtual root used by Discover.
	// RelPath uses forward slashes; directories end with "/".
	Tree []FileEntry
	// DiscoverRoot, when set, is the only root Discover answers for.
	DiscoverRoot string
}

// ListDir implements FileSource.
func (s *StaticFileSource) ListDir(ctx context.Context, absDir string) ([]FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.Dirs == nil {
		return nil, os.ErrNotExist
	}
	// Try exact and cleaned keys.
	ents, ok := s.Dirs[absDir]
	if !ok {
		ents, ok = s.Dirs[filepath.Clean(absDir)]
	}
	if !ok {
		return nil, os.ErrNotExist
	}
	out := append([]FileEntry(nil), ents...)
	return out, nil
}

// Discover implements FileSource.
func (s *StaticFileSource) Discover(ctx context.Context, root, query string) ([]FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.DiscoverRoot != "" && filepath.Clean(root) != filepath.Clean(s.DiscoverRoot) {
		return nil, os.ErrNotExist
	}
	lowerQ := strings.ToLower(query)
	type hit struct {
		e     FileEntry
		score int
		ord   int
	}
	var hits []hit
	for i, e := range s.Tree {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p := strings.TrimSuffix(e.RelPath, "/")
		if hasGitSegment(p) {
			continue
		}
		if lowerQ != "" && !fuzzyMatch(lowerQ, strings.ToLower(p)) {
			continue
		}
		sc := 1
		if lowerQ != "" {
			sc = fuzzyScore(lowerQ, strings.ToLower(p))
			if b := fuzzyScore(lowerQ, strings.ToLower(e.Name)); b > sc {
				sc = b
			}
		}
		hits = append(hits, hit{e: e, score: sc, ord: i})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].ord < hits[j].ord
	})
	const capN = DefaultMaxResults
	if len(hits) > capN {
		hits = hits[:capN]
	}
	out := make([]FileEntry, len(hits))
	for i, h := range hits {
		out[i] = h.e
	}
	return out, nil
}

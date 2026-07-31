package autocomplete

import "strings"

// OMP subsequence fuzzy match: query chars appear in order in target.
func fuzzyMatch(query, target string) bool {
	if query == "" {
		return true
	}
	if len(query) > len(target) {
		return false
	}
	qi := 0
	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if query[qi] == target[ti] {
			qi++
		}
	}
	return qi == len(query)
}

// fuzzyScore is OMP's higher-is-better score:
// exact > starts-with > contains > tight subsequence.
func fuzzyScore(query, target string) int {
	if query == "" {
		return 1
	}
	if target == query {
		return 100
	}
	if strings.HasPrefix(target, query) {
		return 80
	}
	if strings.Contains(target, query) {
		return 60
	}
	qi := 0
	gaps := 0
	lastMatchIdx := -1
	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if query[qi] == target[ti] {
			if lastMatchIdx >= 0 && ti-lastMatchIdx > 1 {
				gaps++
			}
			lastMatchIdx = ti
			qi++
		}
	}
	if qi != len(query) {
		return 0
	}
	sc := 40 - gaps*5
	if sc < 1 {
		return 1
	}
	return sc
}

// scoreCommandTextMatch preserves registry order for equal prefix matches
// (flat 900) so Enter applies the earlier-registered command (OMP /set → settings).
func scoreCommandTextMatch(lowerPrefix, lowerTarget string) int {
	if lowerPrefix == "" {
		return 1
	}
	if lowerPrefix == lowerTarget {
		return 1000
	}
	if strings.HasPrefix(lowerTarget, lowerPrefix) {
		return 900
	}
	if fuzzyMatch(lowerPrefix, lowerTarget) {
		return fuzzyScore(lowerPrefix, lowerTarget)
	}
	return 0
}

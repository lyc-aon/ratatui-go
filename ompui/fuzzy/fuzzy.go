// Package fuzzy implements OMP's word-local fuzzy ranking.
// Lower scores are better.
package fuzzy

import (
	"regexp"
	"sort"
	"strings"
)

const (
	alphanumericSwapPenalty = 5.0
	compactPhraseBonus      = 1200.0
	phraseBonus             = 1000.0
)

var (
	acronymBoundaryPattern = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	camelBoundaryPattern   = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	nonAlphanumericPattern = regexp.MustCompile(`[^a-z0-9]+`)
	spacesPattern          = regexp.MustCompile(`\s+`)
)

// Match is the result of matching a query against text.
type Match struct {
	Matches bool
	Score   float64
}

// Result couples a ranked item with its score.
type Result[T any] struct {
	Item  T
	Score float64
}

type characterMatch struct {
	matches bool
	score   float64
	span    int
}

type searchWord struct {
	text    string
	index   int
	ordinal int
}

type searchIndex struct {
	normalized        string
	compact           string
	compactWordStarts map[int]struct{}
	words             []searchWord
}

func normalize(value string) string {
	value = acronymBoundaryPattern.ReplaceAllString(value, "$1 $2")
	value = camelBoundaryPattern.ReplaceAllString(value, "$1 $2")
	value = strings.ToLower(value)
	value = nonAlphanumericPattern.ReplaceAllString(value, " ")
	value = strings.TrimSpace(value)
	return spacesPattern.ReplaceAllString(value, " ")
}

func buildIndex(value string) searchIndex {
	normalized := normalize(value)
	if normalized == "" {
		return searchIndex{compactWordStarts: map[int]struct{}{}}
	}

	parts := strings.Split(normalized, " ")
	words := make([]searchWord, 0, len(parts))
	starts := make(map[int]struct{}, len(parts))
	textIndex := 0
	compactIndex := 0
	for ordinal, word := range parts {
		words = append(words, searchWord{text: word, index: textIndex, ordinal: ordinal})
		starts[compactIndex] = struct{}{}
		textIndex += len(word) + 1
		compactIndex += len(word)
	}
	return searchIndex{
		normalized:        normalized,
		compact:           strings.ReplaceAll(normalized, " ", ""),
		compactWordStarts: starts,
		words:             words,
	}
}

func scoreCharacters(query, text string) characterMatch {
	if query == "" {
		return characterMatch{matches: true}
	}
	if len(query) > len(text) {
		return characterMatch{}
	}

	queryIndex := 0
	score := 0.0
	first := -1
	last := -1
	consecutive := 0
	for i := 0; i < len(text) && queryIndex < len(query); i++ {
		if text[i] != query[queryIndex] {
			continue
		}
		if first < 0 {
			first = i
		}
		if last == i-1 {
			consecutive++
			score -= float64(consecutive * 5)
		} else {
			consecutive = 0
			if last >= 0 {
				score += float64((i - last - 1) * 2)
			}
		}
		score += float64(i) * 0.1
		last = i
		queryIndex++
	}
	if queryIndex < len(query) {
		return characterMatch{}
	}
	return characterMatch{matches: true, score: score, span: last - first + 1}
}

func alphanumericSwapQueries(query string) []string {
	variants := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	for i := 0; i+1 < len(query); i++ {
		current, next := query[i], query[i+1]
		alphaDigit := current >= 'a' && current <= 'z' && next >= '0' && next <= '9'
		digitAlpha := current >= '0' && current <= '9' && next >= 'a' && next <= 'z'
		if !alphaDigit && !digitAlpha {
			continue
		}
		var b strings.Builder
		b.Grow(len(query))
		b.WriteString(query[:i])
		b.WriteByte(next)
		b.WriteByte(current)
		b.WriteString(query[i+2:])
		variant := b.String()
		if _, ok := seen[variant]; ok {
			continue
		}
		seen[variant] = struct{}{}
		variants = append(variants, variant)
	}
	return variants
}

func withPosition(score float64, index int) float64 {
	return score + float64(index)*0.01
}

func isWordBoundaryPhrase(normalized string, index, length int) bool {
	before := index == 0 || normalized[index-1] == ' '
	afterIndex := index + length
	after := afterIndex == len(normalized) || normalized[afterIndex] == ' '
	return before && after
}

func scoreTokenAgainstWord(token string, word searchWord) (Match, bool) {
	if word.text == token {
		return Match{Matches: true, Score: withPosition(-200, word.index)}, true
	}
	if strings.HasPrefix(word.text, token) {
		score := -170.0 + float64(len(word.text)-len(token))*0.5
		return Match{Matches: true, Score: withPosition(score, word.index)}, true
	}
	if strings.HasPrefix(token, word.text) && len(token)-len(word.text) <= 2 {
		score := -150.0 + float64(len(token)-len(word.text))
		return Match{Matches: true, Score: withPosition(score, word.index)}, true
	}
	if index := strings.Index(word.text, token); index >= 0 {
		return Match{Matches: true, Score: withPosition(-20+float64(index), word.index)}, true
	}

	characters := scoreCharacters(token, word.text)
	if !characters.matches {
		return Match{}, false
	}
	maxSpan := len(token) + 2
	if scaled := (len(token)*18 + 9) / 10; scaled > maxSpan {
		maxSpan = scaled
	}
	if characters.span > maxSpan {
		return Match{}, false
	}
	return Match{Matches: true, Score: withPosition(-40+characters.score, word.index)}, true
}

func scoreAcronym(token string, index searchIndex) (Match, bool) {
	if len(token) < 2 || len(token) > 4 || len(index.words) == 0 {
		return Match{}, false
	}
	queryIndex := 0
	firstOrdinal := -1
	lastOrdinal := -1
	firstTextIndex := 0
	for _, word := range index.words {
		if word.text[0] != token[queryIndex] {
			continue
		}
		if firstOrdinal < 0 {
			firstOrdinal = word.ordinal
			firstTextIndex = word.index
		}
		lastOrdinal = word.ordinal
		queryIndex++
		if queryIndex == len(token) {
			break
		}
	}
	if queryIndex < len(token) || firstOrdinal < 0 || lastOrdinal < 0 {
		return Match{}, false
	}
	span := lastOrdinal - firstOrdinal + 1
	if span > len(token)+2 {
		return Match{}, false
	}
	score := -30.0 + float64(span*4-len(token)*2)
	return Match{Matches: true, Score: withPosition(score, firstTextIndex)}, true
}

func scoreTokenDirect(token string, index searchIndex) Match {
	if token == "" {
		return Match{Matches: true}
	}
	best := Match{}
	if compactIndex := strings.Index(index.compact, token); compactIndex >= 0 {
		if _, ok := index.compactWordStarts[compactIndex]; ok {
			best = Match{Matches: true, Score: withPosition(-140, compactIndex)}
		}
	}
	for _, word := range index.words {
		match, ok := scoreTokenAgainstWord(token, word)
		if ok && (!best.Matches || match.Score < best.Score) {
			best = match
		}
	}
	if acronym, ok := scoreAcronym(token, index); ok && (!best.Matches || acronym.Score < best.Score) {
		best = acronym
	}
	return best
}

func scoreToken(token string, index searchIndex) Match {
	best := scoreTokenDirect(token, index)
	if best.Matches {
		return best
	}
	for _, variant := range alphanumericSwapQueries(token) {
		match := scoreTokenDirect(variant, index)
		if !match.Matches {
			continue
		}
		match.Score += alphanumericSwapPenalty
		if !best.Matches || match.Score < best.Score {
			best = match
		}
	}
	return best
}

// MatchText matches a query against text using OMP's word-local scoring rules.
func MatchText(query, text string) Match {
	normalizedQuery := normalize(query)
	if normalizedQuery == "" {
		return Match{Matches: true}
	}
	index := buildIndex(text)
	if len(index.words) == 0 {
		return Match{}
	}

	total := 0.0
	if phraseIndex := strings.Index(index.normalized, normalizedQuery); phraseIndex >= 0 && isWordBoundaryPhrase(index.normalized, phraseIndex, len(normalizedQuery)) {
		total -= phraseBonus
		total += float64(phraseIndex) * 0.01
	}
	compactQuery := strings.ReplaceAll(normalizedQuery, " ", "")
	if compactIndex := strings.Index(index.compact, compactQuery); compactIndex >= 0 {
		if _, ok := index.compactWordStarts[compactIndex]; ok {
			total -= compactPhraseBonus
			total += float64(compactIndex) * 0.01
		}
	}
	for _, token := range strings.Split(normalizedQuery, " ") {
		match := scoreToken(token, index)
		if !match.Matches {
			return Match{}
		}
		total += match.Score
	}
	return Match{Matches: true, Score: total}
}

// Rank filters and stably sorts items from best to worst fuzzy match.
func Rank[T any](items []T, query string, text func(T) string) []Result[T] {
	if strings.TrimSpace(query) == "" {
		results := make([]Result[T], len(items))
		for i, item := range items {
			results[i] = Result[T]{Item: item}
		}
		return results
	}
	results := make([]Result[T], 0, len(items))
	for _, item := range items {
		match := MatchText(query, text(item))
		if match.Matches {
			results = append(results, Result[T]{Item: item, Score: match.Score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score < results[j].Score })
	return results
}

// Filter returns only matching items, ordered from best to worst.
func Filter[T any](items []T, query string, text func(T) string) []T {
	ranked := Rank(items, query, text)
	results := make([]T, len(ranked))
	for i, result := range ranked {
		results[i] = result.Item
	}
	return results
}

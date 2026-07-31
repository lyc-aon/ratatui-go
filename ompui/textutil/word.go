// Package textutil contains interaction-oriented Unicode text helpers.
package textutil

import (
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// WordKind is the coarse class used by option/alt word navigation.
type WordKind uint8

const (
	WordOther WordKind = iota
	WordWhitespace
	WordDelimiter
	WordCJK
	WordText
)

// Kind classifies a grapheme by its first code point.
func Kind(grapheme string) WordKind {
	r, _ := utf8.DecodeRuneInString(grapheme)
	if r == utf8.RuneError && grapheme == "" {
		return WordOther
	}
	if unicode.IsSpace(r) {
		return WordWhitespace
	}
	if unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return WordDelimiter
	}
	if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
		return WordCJK
	}
	if r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) {
		return WordText
	}
	return WordOther
}

// IsJoiner reports whether a delimiter may join two word runs.
func IsJoiner(grapheme string) bool {
	r, _ := utf8.DecodeRuneInString(grapheme)
	switch r {
	case '\'', '’', '-', '‐', '‑':
		return true
	default:
		return false
	}
}

// IsASCIIWhitespace reports whether the first byte is one of the six ASCII whitespace bytes.
func IsASCIIWhitespace(value string) bool {
	if value == "" {
		return false
	}
	switch value[0] {
	case '\t', '\n', '\v', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

// IsASCIIPunctuation reports whether the first byte is punctuation recognized by OMP text editing.
func IsASCIIPunctuation(value string) bool {
	if value == "" || value[0] >= utf8.RuneSelf {
		return false
	}
	const punctuation = "(){}[]<>.,;:'\"!?+-=*/\\|&%^$#@~`"
	for i := 0; i < len(punctuation); i++ {
		if value[0] == punctuation[i] {
			return true
		}
	}
	return false
}

type cluster struct {
	text       string
	start, end int
}

func clusters(value string) []cluster {
	g := uniseg.NewGraphemes(value)
	result := make([]cluster, 0, utf8.RuneCountInString(value))
	for g.Next() {
		start, end := g.Positions()
		result = append(result, cluster{text: value[start:end], start: start, end: end})
	}
	return result
}

func clampCursor(value string, cursor int) int {
	if cursor <= 0 {
		return 0
	}
	if cursor >= len(value) {
		return len(value)
	}
	for cursor > 0 && !utf8.RuneStart(value[cursor]) {
		cursor--
	}
	return cursor
}

// MoveWordLeft returns the previous Unicode-aware word boundary. Cursor and
// result are UTF-8 byte offsets.
func MoveWordLeft(value string, cursor int) int {
	cursor = clampCursor(value, cursor)
	if cursor == 0 {
		return 0
	}
	parts := clusters(value[:cursor])
	if len(parts) == 0 {
		return 0
	}
	index := len(parts) - 1
	for index >= 0 && Kind(parts[index].text) == WordWhitespace {
		cursor = parts[index].start
		index--
	}
	if index < 0 || cursor == 0 {
		return cursor
	}

	kind := Kind(parts[index].text)
	if kind == WordDelimiter || kind == WordCJK {
		for index >= 0 && Kind(parts[index].text) == kind {
			cursor = parts[index].start
			index--
		}
		return cursor
	}
	if kind == WordText {
		hasRightWord := false
		for index >= 0 {
			currentKind := Kind(parts[index].text)
			if currentKind == WordText {
				hasRightWord = true
				cursor = parts[index].start
				index--
				continue
			}
			if hasRightWord && currentKind == WordDelimiter && IsJoiner(parts[index].text) && index > 0 && Kind(parts[index-1].text) == WordText {
				cursor = parts[index].start
				index--
				continue
			}
			break
		}
		return cursor
	}
	return parts[index].start
}

// MoveWordRight returns the next Unicode-aware word boundary. Cursor and
// result are UTF-8 byte offsets.
func MoveWordRight(value string, cursor int) int {
	cursor = clampCursor(value, cursor)
	if cursor == len(value) {
		return cursor
	}
	parts := clusters(value[cursor:])
	if len(parts) == 0 {
		return cursor
	}
	index := 0
	for index < len(parts) && Kind(parts[index].text) == WordWhitespace {
		cursor += parts[index].end - parts[index].start
		index++
	}
	if index == len(parts) {
		return cursor
	}

	kind := Kind(parts[index].text)
	if kind == WordDelimiter || kind == WordCJK {
		for index < len(parts) && Kind(parts[index].text) == kind {
			cursor += parts[index].end - parts[index].start
			index++
		}
		return cursor
	}
	if kind == WordText {
		hasLeftWord := false
		for index < len(parts) {
			currentKind := Kind(parts[index].text)
			if currentKind == WordText {
				hasLeftWord = true
				cursor += parts[index].end - parts[index].start
				index++
				continue
			}
			if hasLeftWord && currentKind == WordDelimiter && IsJoiner(parts[index].text) && index+1 < len(parts) && Kind(parts[index+1].text) == WordText {
				cursor += parts[index].end - parts[index].start
				index++
				continue
			}
			break
		}
		return cursor
	}
	return cursor + parts[index].end - parts[index].start
}

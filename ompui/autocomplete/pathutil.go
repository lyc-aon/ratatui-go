package autocomplete

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// pathDelimiters match OMP PATH_DELIMITERS.
var pathDelimiters = map[byte]struct{}{
	' ':  {},
	'\t': {},
	'"':  {},
	'\'': {},
	'=':  {},
}

func findLastDelimiter(text string) int {
	for i := len(text) - 1; i >= 0; i-- {
		if _, ok := pathDelimiters[text[i]]; ok {
			return i
		}
	}
	return -1
}

// findUnclosedQuoteStart returns the byte index of an unclosed " or -1.
func findUnclosedQuoteStart(text string) int {
	inQuotes := false
	quoteStart := -1
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == '"' {
			inQuotes = !inQuotes
			if inQuotes {
				quoteStart = i
			}
		}
		i += size
	}
	if inQuotes {
		return quoteStart
	}
	return -1
}

func isTokenStart(text string, index int) bool {
	if index <= 0 {
		return index == 0
	}
	_, ok := pathDelimiters[text[index-1]]
	return ok
}

// FindLeadingSlashCommandStart returns the byte index of a leading '/' after
// optional whitespace, or -1 when the line is not a slash command.
// Aligns with OMP findLeadingSlashCommandStart / trimStart semantics.
func FindLeadingSlashCommandStart(text string) int {
	i := 0
	for i < len(text) {
		if text[i] != ' ' && text[i] != '\t' {
			break
		}
		i++
	}
	if i >= len(text) || text[i] != '/' {
		return -1
	}
	return i
}

func extractQuotedPrefix(text string) string {
	quoteStart := findUnclosedQuoteStart(text)
	if quoteStart < 0 {
		return ""
	}
	if quoteStart > 0 && text[quoteStart-1] == '@' {
		if !isTokenStart(text, quoteStart-1) {
			return ""
		}
		return text[quoteStart-1:]
	}
	if !isTokenStart(text, quoteStart) {
		return ""
	}
	return text[quoteStart:]
}

type pathPrefixKind struct {
	rawPrefix      string
	isAtPrefix     bool
	isQuotedPrefix bool
}

func parsePathPrefix(prefix string) pathPrefixKind {
	switch {
	case strings.HasPrefix(prefix, `@"`):
		return pathPrefixKind{rawPrefix: prefix[2:], isAtPrefix: true, isQuotedPrefix: true}
	case strings.HasPrefix(prefix, `"`):
		return pathPrefixKind{rawPrefix: prefix[1:], isAtPrefix: false, isQuotedPrefix: true}
	case strings.HasPrefix(prefix, "@"):
		return pathPrefixKind{rawPrefix: prefix[1:], isAtPrefix: true, isQuotedPrefix: false}
	default:
		return pathPrefixKind{rawPrefix: prefix, isAtPrefix: false, isQuotedPrefix: false}
	}
}

func buildCompletionValue(p string, isDirectory, isAtPrefix, isQuotedPrefix bool) string {
	needsQuotes := isQuotedPrefix || strings.Contains(p, " ")
	prefix := ""
	if isAtPrefix {
		prefix = "@"
	}
	if !needsQuotes {
		return prefix + p
	}
	openQuote := prefix + `"`
	closeQuote := `"`
	if isDirectory {
		// Keep the quote open so the user can keep typing inside the dir.
		closeQuote = ""
	}
	return openQuote + p + closeQuote
}

func expandHomePath(filePath, home string) string {
	if filePath == "~" {
		return home
	}
	if strings.HasPrefix(filePath, "~/") {
		expanded := filepath.Join(home, filePath[2:])
		if strings.HasSuffix(filePath, "/") && !strings.HasSuffix(expanded, string(filepath.Separator)) && !strings.HasSuffix(expanded, "/") {
			return expanded + string(filepath.Separator)
		}
		return expanded
	}
	return filePath
}

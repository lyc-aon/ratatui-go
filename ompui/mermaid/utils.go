// Ported from mermaid-ascii 1.4.0 (github.com/AlexanderGrooff/mermaid-ascii),
// commit 823db562a4439e342541643bbd5cb7d75c930e8e, MIT License
// Copyright (c) 2023 Alexander Grooff.
// Bounded pure graph/flowchart + sequenceDiagram port for ompui; no cmd/web,
// cobra, gin, logrus, or gookit/color dependencies.

package mermaid

import (
	"regexp"
	"strings"
)

// removeComments removes Mermaid comment lines from input.
// This function is shared between graph and sequence diagram parsers.
// It handles both full-line comments (%% at start) and inline comments (%% after content).
func RemoveComments(lines []string) []string {
	cleaned := make([]string, 0, len(lines))

	for _, line := range lines {
		// Skip lines that start with %% (full-line comments)
		if strings.HasPrefix(strings.TrimSpace(line), "%%") {
			continue
		}

		// Remove inline comments (anything after %%)
		if idx := strings.Index(line, "%%"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		// Only keep non-empty lines after comment removal
		if len(strings.TrimSpace(line)) > 0 {
			cleaned = append(cleaned, line)
		}
	}

	return cleaned
}

// SplitLines splits input on both actual newlines and escaped newlines (for curl compatibility).
func SplitLines(input string) []string {
	newlinePattern := regexp.MustCompile(`\n|\\n`)
	return newlinePattern.Split(input, -1)
}

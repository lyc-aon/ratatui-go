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

var htmlBreakPattern = regexp.MustCompile(`(?i)<br\s*/?>`)

const graphLabelLineGap = 1

type graphLabel struct {
	lines []string
	width int
}

func newGraphLabel(raw string) graphLabel {
	normalized := htmlBreakPattern.ReplaceAllString(raw, "\n")
	normalized = strings.ReplaceAll(normalized, `\n`, "\n")

	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	width := 0
	for _, line := range lines {
		width = Max(width, stringWidth(line))
	}

	return graphLabel{
		lines: lines,
		width: width,
	}
}

func (l graphLabel) height() int {
	return len(l.lines)
}

func (l graphLabel) contentHeight() int {
	if len(l.lines) == 0 {
		return 0
	}
	return len(l.lines) + (len(l.lines)-1)*graphLabelLineGap
}

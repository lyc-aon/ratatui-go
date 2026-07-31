// Ported from mermaid-ascii 1.4.0 (github.com/AlexanderGrooff/mermaid-ascii),
// commit 823db562a4439e342541643bbd5cb7d75c930e8e, MIT License
// Copyright (c) 2023 Alexander Grooff.
// Bounded pure graph/flowchart + sequenceDiagram port for ompui; no cmd/web,
// cobra, gin, logrus, or gookit/color dependencies.

package mermaid

// BoxChars defines the characters used for drawing the diagram.
type BoxChars struct {
	TopLeft      rune
	TopRight     rune
	BottomLeft   rune
	BottomRight  rune
	Horizontal   rune
	Vertical     rune
	TeeDown      rune
	TeeRight     rune
	TeeLeft      rune
	Cross        rune
	ArrowRight   rune
	ArrowLeft    rune
	SolidLine    rune
	DottedLine   rune
	SelfTopRight rune
	SelfBottom   rune
}

var ASCII = BoxChars{
	TopLeft:      '+',
	TopRight:     '+',
	BottomLeft:   '+',
	BottomRight:  '+',
	Horizontal:   '-',
	Vertical:     '|',
	TeeDown:      '+',
	TeeRight:     '+',
	TeeLeft:      '+',
	Cross:        '+',
	ArrowRight:   '>',
	ArrowLeft:    '<',
	SolidLine:    '-',
	DottedLine:   '.',
	SelfTopRight: '+',
	SelfBottom:   '+',
}

var Unicode = BoxChars{
	TopLeft:      '┌',
	TopRight:     '┐',
	BottomLeft:   '└',
	BottomRight:  '┘',
	Horizontal:   '─',
	Vertical:     '│',
	TeeDown:      '┬',
	TeeRight:     '├',
	TeeLeft:      '┤',
	Cross:        '┼',
	ArrowRight:   '►',
	ArrowLeft:    '◄',
	SolidLine:    '─',
	DottedLine:   '┈',
	SelfTopRight: '┐',
	SelfBottom:   '┘',
}

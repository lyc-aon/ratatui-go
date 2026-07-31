// Ported from mermaid-ascii 1.4.0 (github.com/AlexanderGrooff/mermaid-ascii),
// commit 823db562a4439e342541643bbd5cb7d75c930e8e, MIT License
// Copyright (c) 2023 Alexander Grooff.
// Bounded pure graph/flowchart + sequenceDiagram port for ompui; no cmd/web,
// cobra, gin, logrus, or gookit/color dependencies.

package mermaid

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func CeilDiv(x, y int) int {
	if x%y == 0 {
		return x / y
	}
	return x/y + 1
}

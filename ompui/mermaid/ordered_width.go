// Ported from mermaid-ascii 1.4.0 (github.com/AlexanderGrooff/mermaid-ascii),
// commit 823db562a4439e342541643bbd5cb7d75c930e8e, MIT License
// Copyright (c) 2023 Alexander Grooff.
// Bounded pure graph/flowchart + sequenceDiagram port for ompui; no cmd/web,
// cobra, gin, logrus, or gookit/color dependencies.
package mermaid

import (
	"github.com/lyc-aon/ratatui-go/text"
)


// orderedMap is a minimal insertion-ordered map replacing
// github.com/elliotchance/orderedmap/v2 for the graph adjacency list.
type orderedMap[K comparable, V any] struct {
	keys []K
	m    map[K]V
}

func newOrderedMap[K comparable, V any]() *orderedMap[K, V] {
	return &orderedMap[K, V]{m: make(map[K]V)}
}

func (o *orderedMap[K, V]) Get(k K) (V, bool) {
	v, ok := o.m[k]
	return v, ok
}

func (o *orderedMap[K, V]) Set(k K, v V) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *orderedMap[K, V]) Len() int { return len(o.keys) }

func (o *orderedMap[K, V]) Keys() []K {
	out := make([]K, len(o.keys))
	copy(out, o.keys)
	return out
}

// Front/Next iteration via index helper used by ported loops.
type orderedMapElem[K comparable, V any] struct {
	Key   K
	Value V
	next  int
	om    *orderedMap[K, V]
}

func (o *orderedMap[K, V]) Front() *orderedMapElem[K, V] {
	if len(o.keys) == 0 {
		return nil
	}
	k := o.keys[0]
	return &orderedMapElem[K, V]{Key: k, Value: o.m[k], next: 1, om: o}
}

func (e *orderedMapElem[K, V]) Next() *orderedMapElem[K, V] {
	if e == nil || e.om == nil || e.next >= len(e.om.keys) {
		return nil
	}
	k := e.om.keys[e.next]
	return &orderedMapElem[K, V]{Key: k, Value: e.om.m[k], next: e.next + 1, om: e.om}
}


func stringWidth(s string) int {
	if s == "" {
		return 0
	}
	return text.GraphemeWidth(s)
}

func runeWidth(r rune) int {
	if r == 0 {
		return 0
	}
	w := text.GraphemeWidth(string(r))
	if w < 1 && r != 0 {
		return 1
	}
	return w
}

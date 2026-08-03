package view

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

// Bounds on rendered JSON. Mirrors OMP json-tree so a collapsed card stays a
// glance and an expanded one stays bounded — raw JSON is never flooded into the
// transcript at any expansion level.
const (
	jsonMaxDepthCollapsed  = 2
	jsonMaxDepthExpanded   = 6
	jsonMaxLinesCollapsed  = 6
	jsonMaxLinesExpanded   = 200
	jsonScalarLenCollapsed = 60
	jsonScalarLenExpanded  = 2000

	// argsInlineTailReserve is the minimum footprint held back for each
	// not-yet-rendered key so one long value cannot starve the keys after it.
	argsInlineTailReserve = 4
)

// intentField and partialJSONField are argument keys OMP hides from display:
// the intent is already rendered in the card header, and __partialJson is
// transport scaffolding for a half-streamed call.
const (
	intentField      = "i"
	partialJSONField = "__partialJson"
)

type jsonKind uint8

const (
	jsonNull jsonKind = iota
	jsonBool
	jsonNumber
	jsonString
	jsonArray
	jsonObject
)

type jsonField struct {
	key   string
	value jsonValue
}

// jsonValue is an order-preserving JSON tree. Go maps randomize iteration, and
// a tool's argument order is meaningful to the reader (path before content,
// command before timeout), so object members are kept as an ordered slice.
type jsonValue struct {
	kind    jsonKind
	literal string // bool/number source text
	str     string // string payload
	arr     []jsonValue
	obj     []jsonField
}

func (v jsonValue) isContainer() bool { return v.kind == jsonArray || v.kind == jsonObject }

// parseJSON decodes raw into an order-preserving tree. Numbers keep their
// source text so 1e9 and 1000000000 render as written. A half-streamed or
// trailing-garbage payload fails cleanly instead of rendering a partial tree.
func parseJSON(raw []byte) (jsonValue, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return jsonValue{}, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	value, err := decodeJSONValue(dec)
	if err != nil {
		return jsonValue{}, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return jsonValue{}, false
	}
	return value, true
}

func decodeJSONValue(dec *json.Decoder) (jsonValue, error) {
	token, err := dec.Token()
	if err != nil {
		return jsonValue{}, err
	}
	return decodeJSONFrom(dec, token)
}

func decodeJSONFrom(dec *json.Decoder, token json.Token) (jsonValue, error) {
	switch t := token.(type) {
	case nil:
		return jsonValue{kind: jsonNull}, nil
	case bool:
		return jsonValue{kind: jsonBool, literal: strconv.FormatBool(t)}, nil
	case json.Number:
		return jsonValue{kind: jsonNumber, literal: t.String()}, nil
	case string:
		return jsonValue{kind: jsonString, str: t}, nil
	case json.Delim:
		switch t {
		case '[':
			value := jsonValue{kind: jsonArray}
			for dec.More() {
				item, err := decodeJSONValue(dec)
				if err != nil {
					return jsonValue{}, err
				}
				value.arr = append(value.arr, item)
			}
			if _, err := dec.Token(); err != nil { // ']'
				return jsonValue{}, err
			}
			return value, nil
		case '{':
			value := jsonValue{kind: jsonObject}
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return jsonValue{}, err
				}
				key, _ := keyToken.(string)
				item, err := decodeJSONValue(dec)
				if err != nil {
					return jsonValue{}, err
				}
				value.obj = append(value.obj, jsonField{key: key, value: item})
			}
			if _, err := dec.Token(); err != nil { // '}'
				return jsonValue{}, err
			}
			return value, nil
		}
	}
	return jsonValue{}, io.ErrUnexpectedEOF
}

func hiddenArgKey(key string) bool {
	return key == intentField || key == partialJSONField
}

// visibleFields returns the object's displayable members in source order.
func visibleFields(v jsonValue) []jsonField {
	if v.kind != jsonObject {
		return nil
	}
	out := make([]jsonField, 0, len(v.obj))
	for _, field := range v.obj {
		if !hiddenArgKey(field.key) {
			out = append(out, field)
		}
	}
	return out
}

// formatScalar renders one value for inline display, mirroring OMP formatScalar.
// A clipped string keeps the ellipsis inside its quotes so a truncated path is
// never mistaken for the real one.
func formatScalar(v jsonValue, maxLen int, ellipsis string) string {
	switch v.kind {
	case jsonNull:
		return "null"
	case jsonBool, jsonNumber:
		return v.literal
	case jsonString:
		escaped := strings.ReplaceAll(v.str, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\t", "\\t")
		return `"` + ansitext.TruncateToWidth(escaped, maxLen, "") + `"`
	case jsonArray:
		return "[" + strconv.Itoa(len(v.arr)) + " " + pluralize("item", len(v.arr)) + "]"
	case jsonObject:
		return "{" + strconv.Itoa(len(v.obj)) + " " + pluralize("key", len(v.obj)) + "}"
	}
	return ""
}

// formatArgsInline renders `key=value, key=value` bounded to maxWidth cells,
// reserving a minimum footprint for every remaining key so a long leading value
// cannot swallow the whole budget. Port of OMP formatArgsInline.
func formatArgsInline(args jsonValue, maxWidth int, ellipsis string) string {
	fields := visibleFields(args)
	if len(fields) == 0 || maxWidth <= 0 {
		return ""
	}
	ellipsisWidth := ansitext.VisibleWidth(ellipsis)

	var b strings.Builder
	width := 0
	for i, field := range fields {
		sep := ""
		sepWidth := 0
		if width > 0 {
			sep, sepWidth = ", ", 2
		}
		current := width + sepWidth
		room := maxWidth - current - ellipsisWidth
		if room <= 0 {
			b.WriteString(ellipsis)
			return b.String()
		}
		tailReserve := 0
		for _, pending := range fields[i+1:] {
			tailReserve += 2 + ansitext.VisibleWidth(pending.key) + 1 + argsInlineTailReserve
		}
		pieceBudget := room
		if remaining := maxWidth - current - tailReserve; remaining < pieceBudget {
			pieceBudget = remaining
		}
		valueMax := pieceBudget - ansitext.VisibleWidth(field.key) - 3
		if valueMax < 1 {
			valueMax = 1
		}
		piece := field.key + "=" + formatScalar(field.value, valueMax, ellipsis)
		pieceWidth := ansitext.VisibleWidth(piece)
		if pieceWidth > pieceBudget {
			b.WriteString(sep)
			b.WriteString(ansitext.TruncateToWidth(piece, room, ellipsis))
			return b.String()
		}
		b.WriteString(sep)
		b.WriteString(piece)
		width = current + pieceWidth
	}
	return b.String()
}

// jsonTreeOptions bounds one tree render.
type jsonTreeOptions struct {
	maxDepth  int
	maxLines  int
	maxScalar int
	width     int
}

func (o Options) jsonTreeOptions(width int) jsonTreeOptions {
	if o.toolsExpanded() {
		return jsonTreeOptions{
			maxDepth:  jsonMaxDepthExpanded,
			maxLines:  jsonMaxLinesExpanded,
			maxScalar: jsonScalarLenExpanded,
			width:     width,
		}
	}
	return jsonTreeOptions{
		maxDepth:  jsonMaxDepthCollapsed,
		maxLines:  jsonMaxLinesCollapsed,
		maxScalar: jsonScalarLenCollapsed,
		width:     width,
	}
}

type jsonTreeWriter struct {
	theme     Theme
	opts      jsonTreeOptions
	lines     []string
	truncated bool
}

// renderJSONTree renders value as an indented tree. Depth, line count, and
// scalar length are all bounded; the returned flag reports whether anything was
// dropped so the caller can show an honest continuation marker.
func renderJSONTree(value jsonValue, theme Theme, opts jsonTreeOptions) ([]string, bool) {
	w := &jsonTreeWriter{theme: theme, opts: opts}
	switch value.kind {
	case jsonObject:
		fields := visibleFields(value)
		for i, field := range fields {
			w.node(field.value, field.key, nil, i == len(fields)-1, 1)
			if w.full() {
				break
			}
		}
	case jsonArray:
		for i, item := range value.arr {
			w.node(item, "["+strconv.Itoa(i)+"]", nil, i == len(value.arr)-1, 1)
			if w.full() {
				break
			}
		}
	default:
		w.node(value, "", nil, true, 0)
	}
	return w.lines, w.truncated
}

func (w *jsonTreeWriter) full() bool {
	if len(w.lines) >= w.opts.maxLines {
		w.truncated = true
		return true
	}
	return false
}

func (w *jsonTreeWriter) push(line string) {
	if len(w.lines) >= w.opts.maxLines {
		w.truncated = true
		return
	}
	if w.opts.width > 0 {
		line = ansitext.TruncateToWidth(line, w.opts.width, w.theme.Symbols.Ellipsis)
	}
	w.lines = append(w.lines, line)
}

func (w *jsonTreeWriter) prefix(ancestors []bool) string {
	if len(ancestors) == 0 {
		return ""
	}
	var b strings.Builder
	for _, hasNext := range ancestors {
		if hasNext {
			b.WriteString(apply(w.theme.Dim, w.theme.Symbols.TreeVertical))
			b.WriteString("  ")
		} else {
			b.WriteString("   ")
		}
	}
	return b.String()
}

func (w *jsonTreeWriter) node(value jsonValue, key string, ancestors []bool, isLast bool, depth int) {
	if w.full() {
		return
	}
	sym := w.theme.Symbols
	connector := sym.TreeBranch
	if isLast {
		connector = sym.TreeLast
	}
	head := w.prefix(ancestors) + apply(w.theme.Dim, connector) + " "
	// Copy rather than append in place: sibling recursion at the same depth
	// would otherwise observe a mutated backing array.
	childAncestors := make([]bool, len(ancestors)+1)
	copy(childAncestors, ancestors)
	childAncestors[len(ancestors)] = !isLast

	label := key
	if label == "" {
		switch value.kind {
		case jsonArray:
			label = "array"
		case jsonObject:
			label = "object"
		default:
			label = "value"
		}
	}
	styledLabel := apply(w.theme.Muted, label)

	switch {
	case !value.isContainer():
		if value.kind == jsonString && strings.Contains(value.str, "\n") {
			w.multilineString(value, head, styledLabel, childAncestors)
			return
		}
		w.push(head + apply(w.theme.Dim, sym.IconFile) + " " + styledLabel + ": " +
			apply(w.theme.Dim, formatScalar(value, w.opts.maxScalar, w.theme.Symbols.Ellipsis)))
	case value.kind == jsonArray:
		w.push(head + apply(w.theme.Dim, sym.IconPackage) + " " + styledLabel)
		inner := w.prefix(childAncestors) + apply(w.theme.Dim, sym.TreeLast) + " "
		if len(value.arr) == 0 {
			w.push(inner + apply(w.theme.Dim, "[]"))
			return
		}
		if depth >= w.opts.maxDepth {
			w.truncated = true
			w.push(inner + apply(w.theme.Dim, sym.Ellipsis))
			return
		}
		for i, item := range value.arr {
			w.node(item, "["+strconv.Itoa(i)+"]", childAncestors, i == len(value.arr)-1, depth+1)
			if w.full() {
				return
			}
		}
	default:
		w.push(head + apply(w.theme.Dim, sym.IconFolder) + " " + styledLabel)
		inner := w.prefix(childAncestors) + apply(w.theme.Dim, sym.TreeLast) + " "
		if len(value.obj) == 0 {
			w.push(inner + apply(w.theme.Dim, "{}"))
			return
		}
		if depth >= w.opts.maxDepth {
			w.truncated = true
			w.push(inner + apply(w.theme.Dim, sym.Ellipsis))
			return
		}
		for i, field := range value.obj {
			w.node(field.value, field.key, childAncestors, i == len(value.obj)-1, depth+1)
			if w.full() {
				return
			}
		}
	}
}

// multilineString renders a string value across rows, keeping the opening quote
// on the label row and closing it honestly — either a quote or a dropped-line
// count — so the reader always knows whether they saw all of it.
func (w *jsonTreeWriter) multilineString(value jsonValue, head, label string, childAncestors []bool) {
	sym := w.theme.Symbols
	parts := strings.Split(value.str, "\n")
	shown := clampInt(w.opts.maxLines-len(w.lines)-1, 1, len(parts))
	continuation := w.prefix(childAncestors) + "   "

	w.push(head + apply(w.theme.Dim, sym.IconFile) + " " + label + ": " +
		apply(w.theme.Dim, `"`+ansitext.TruncateToWidth(parts[0], w.opts.maxScalar, w.theme.Symbols.Ellipsis)))
	for i := 1; i < shown; i++ {
		if len(w.lines) >= w.opts.maxLines {
			w.truncated = true
			return
		}
		w.push(continuation + apply(w.theme.Dim, " "+ansitext.TruncateToWidth(parts[i], w.opts.maxScalar, w.theme.Symbols.Ellipsis)))
	}
	if len(parts) > shown {
		w.truncated = true
		dropped := len(parts) - shown
		w.push(continuation + apply(w.theme.Dim, " "+sym.Ellipsis+"("+
			strconv.Itoa(dropped)+" more "+pluralize("line", dropped)+`)"`))
		return
	}
	if len(w.lines) > 0 {
		w.lines[len(w.lines)-1] += apply(w.theme.Dim, `"`)
	}
}

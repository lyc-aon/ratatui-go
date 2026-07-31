package widgets

import (
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

// ListDirection is the order in which list items are painted.
//
// If there are too few items to fill the screen, the list sticks to the
// starting edge (top for TopToBottom, bottom for BottomToTop).
type ListDirection int

const (
	// ListTopToBottom paints the first item at the top (default).
	ListTopToBottom ListDirection = iota
	// ListBottomToTop paints the first item at the bottom.
	ListBottomToTop
)

// String returns a stable name for the list direction.
func (d ListDirection) String() string {
	switch d {
	case ListBottomToTop:
		return "BottomToTop"
	default:
		return "TopToBottom"
	}
}

// ListItem is one row group in a List.
//
// Height is the number of lines in Content. Style is patched under the
// content's own styles when the item is painted.
type ListItem struct {
	Content text.Text
	Style   style.Style
}

// NewListItem creates a list item from text content.
func NewListItem(content text.Text) ListItem {
	return ListItem{Content: content}
}

// WithStyle sets the item base style (fluent).
func (it ListItem) WithStyle(st style.Style) ListItem {
	it.Style = st
	return it
}

// Height returns the number of lines in the item content.
func (it ListItem) Height() int {
	return it.Content.Height()
}

// Width returns the max terminal cell width of the item's lines.
func (it ListItem) Width() int {
	return it.Content.Width()
}

// List displays a vertical collection of ListItems with optional selection.
//
// Value builder: setters return a modified copy. Render uses a fresh default
// ListState; RenderStateful repairs offset/selection and paints the window.
type List struct {
	block                 *Block
	items                 []ListItem
	style                 style.Style
	direction             ListDirection
	highlightStyle        style.Style
	highlightSymbol       *text.Line
	repeatHighlightSymbol bool
	highlightSpacing      HighlightSpacing
	scrollPadding         int
}

// NewList creates a list from items. The items slice is copied.
func NewList(items ...ListItem) List {
	return List{
		items:            copyListItems(items),
		highlightSpacing: HighlightWhenSelected,
	}
}

// Items replaces the list items. The slice is copied.
func (l List) Items(items ...ListItem) List {
	l.items = copyListItems(items)
	return l
}

// Block wraps the list in a border/title block (copied by value into a pointer).
func (l List) Block(b Block) List {
	cp := b
	l.block = &cp
	return l
}

// Style sets the base style applied to the whole list area before the block.
func (l List) Style(st style.Style) List {
	l.style = st
	return l
}

// HighlightSymbol sets the symbol drawn in front of the selected item.
// An empty line clears any previously set symbol.
func (l List) HighlightSymbol(sym text.Line) List {
	cp := copyLine(sym)
	l.highlightSymbol = &cp
	return l
}

// HighlightStyle sets the style patched onto the selected item area.
func (l List) HighlightStyle(st style.Style) List {
	l.highlightStyle = st
	return l
}

// RepeatHighlightSymbol controls whether multi-line selected items draw the
// highlight symbol on every content line (true) or only the first (false).
func (l List) RepeatHighlightSymbol(repeat bool) List {
	l.repeatHighlightSymbol = repeat
	return l
}

// WithHighlightSpacing sets when the selection column is reserved.
func (l List) WithHighlightSpacing(hs HighlightSpacing) List {
	l.highlightSpacing = hs
	return l
}

// Direction sets top-to-bottom or bottom-to-top painting order.
func (l List) Direction(d ListDirection) List {
	l.direction = d
	return l
}

// ScrollPadding sets how many items of context to keep around the selection.
func (l List) ScrollPadding(padding int) List {
	if padding < 0 {
		padding = 0
	}
	l.scrollPadding = padding
	return l
}

// Len returns the number of items.
func (l List) Len() int {
	return len(l.items)
}

// IsEmpty reports whether the list has no items.
func (l List) IsEmpty() bool {
	return len(l.items) == 0
}

func copyListItems(items []ListItem) []ListItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]ListItem, len(items))
	copy(out, items)
	return out
}

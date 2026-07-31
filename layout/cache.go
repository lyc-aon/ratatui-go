package layout

import "sync"

// DefaultCacheSize is the default capacity of the process-wide SplitWithSpacers LRU
// (matches ratatui Layout::DEFAULT_CACHE_SIZE = 500).
const DefaultCacheSize = 500

// maxCachedConstraints is the largest constraint list that participates in the
// comparable fixed-key cache. Larger layouts bypass the cache rather than risk
// a hashed/collision-prone key.
const maxCachedConstraints = 32

// layoutCacheKey fully identifies a SplitWithSpacers input for common layouts.
// All fields are comparable; no hashing of constraint slices.
type layoutCacheKey struct {
	areaX, areaY, areaW, areaH int
	direction                  Direction
	marginH, marginV           int
	flex                       Flex
	spacing                    int
	n                          int
	constraints                [maxCachedConstraints]Constraint
}

type cacheValue struct {
	segments []Rect
	spacers  []Rect
}

type lruNode struct {
	key        layoutCacheKey
	val        cacheValue
	prev, next *lruNode
}

// process-wide layout cache (concurrency-safe).
var globalLayoutCache = newLayoutLRU(DefaultCacheSize)

type layoutLRU struct {
	mu    sync.Mutex
	cap   int
	n     int
	head  *lruNode // sentinel: head.next = MRU
	tail  *lruNode // sentinel: tail.prev = LRU
	items map[layoutCacheKey]*lruNode
}

func newLayoutLRU(capacity int) *layoutLRU {
	if capacity < 1 {
		capacity = DefaultCacheSize
	}
	c := &layoutLRU{
		cap:   capacity,
		items: make(map[layoutCacheKey]*lruNode, capacity),
		head:  &lruNode{},
		tail:  &lruNode{},
	}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

func (c *layoutLRU) get(key layoutCacheKey) (cacheValue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	node, ok := c.items[key]
	if !ok {
		return cacheValue{}, false
	}
	c.moveToFront(node)
	// Return defensive copies so caller mutation cannot poison the entry.
	return cacheValue{
		segments: copyRects(node.val.segments),
		spacers:  copyRects(node.val.spacers),
	}, true
}

func (c *layoutLRU) put(key layoutCacheKey, segments, spacers []Rect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if node, ok := c.items[key]; ok {
		node.val = cacheValue{
			segments: copyRects(segments),
			spacers:  copyRects(spacers),
		}
		c.moveToFront(node)
		return
	}
	node := &lruNode{
		key: key,
		val: cacheValue{
			segments: copyRects(segments),
			spacers:  copyRects(spacers),
		},
	}
	c.items[key] = node
	c.insertFront(node)
	c.n++
	for c.n > c.cap {
		c.evictLRU()
	}
}

func (c *layoutLRU) moveToFront(node *lruNode) {
	c.detach(node)
	c.insertFront(node)
}

func (c *layoutLRU) insertFront(node *lruNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

func (c *layoutLRU) detach(node *lruNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
	node.prev = nil
	node.next = nil
}

func (c *layoutLRU) evictLRU() {
	node := c.tail.prev
	if node == c.head {
		return
	}
	c.detach(node)
	delete(c.items, node.key)
	c.n--
}

// cacheKey builds a comparable key when the layout fits the fixed-capacity table.
// ok is false when the layout should bypass the cache.
func (l Layout) cacheKey(area Rect) (layoutCacheKey, bool) {
	n := len(l.constraints)
	if n > maxCachedConstraints {
		return layoutCacheKey{}, false
	}
	var key layoutCacheKey
	key.areaX, key.areaY, key.areaW, key.areaH = area.X, area.Y, area.Width, area.Height
	key.direction = l.direction
	key.marginH, key.marginV = l.margin.Horizontal, l.margin.Vertical
	key.flex = l.flex
	key.spacing = l.spacing
	key.n = n
	for i := range n {
		key.constraints[i] = l.constraints[i]
	}
	return key, true
}

func copyRects(in []Rect) []Rect {
	if in == nil {
		return nil
	}
	out := make([]Rect, len(in))
	copy(out, in)
	return out
}

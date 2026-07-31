package layout

import (
	"sync"
	"testing"
)

// resetLayoutCacheForTest clears the process cache (tests only).
func resetLayoutCacheForTest() {
	globalLayoutCache.mu.Lock()
	defer globalLayoutCache.mu.Unlock()
	globalLayoutCache.items = make(map[layoutCacheKey]*lruNode, globalLayoutCache.cap)
	globalLayoutCache.head.next = globalLayoutCache.tail
	globalLayoutCache.tail.prev = globalLayoutCache.head
	globalLayoutCache.n = 0
}

// layoutCacheLenForTest returns current entry count (tests only).
func layoutCacheLenForTest() int {
	globalLayoutCache.mu.Lock()
	defer globalLayoutCache.mu.Unlock()
	return globalLayoutCache.n
}

func TestLayoutCacheHitEquivalence(t *testing.T) {
	resetLayoutCacheForTest()
	area := NewRect(0, 0, 80, 24)
	l := Horizontal(Length(10), Fill(1), Percentage(25)).Flex(FlexStart).Spacing(1)

	a, as := l.SplitWithSpacers(area)
	if layoutCacheLenForTest() != 1 {
		t.Fatalf("cache len after miss = %d, want 1", layoutCacheLenForTest())
	}
	b, bs := l.SplitWithSpacers(area)
	if layoutCacheLenForTest() != 1 {
		t.Fatalf("cache len after hit = %d, want 1", layoutCacheLenForTest())
	}
	if len(a) != len(b) || len(as) != len(bs) {
		t.Fatalf("hit length mismatch segs %d/%d sps %d/%d", len(a), len(b), len(as), len(bs))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("seg %d miss=%+v hit=%+v", i, a[i], b[i])
		}
	}
	for i := range as {
		if as[i] != bs[i] {
			t.Fatalf("sp %d miss=%+v hit=%+v", i, as[i], bs[i])
		}
	}
}

func TestLayoutCacheCallerMutationIsolation(t *testing.T) {
	resetLayoutCacheForTest()
	area := NewRect(2, 3, 40, 10)
	l := Vertical(Min(2), Fill(1), Max(5)).Flex(FlexCenter)

	segs, sps := l.SplitWithSpacers(area)
	if len(segs) == 0 || len(sps) == 0 {
		t.Fatal("expected non-empty result")
	}
	// Poison returned slices.
	origSeg := segs[0]
	origSp := sps[0]
	segs[0] = NewRect(999, 999, 999, 999)
	sps[0] = NewRect(888, 888, 888, 888)

	segs2, sps2 := l.SplitWithSpacers(area)
	if segs2[0] != origSeg {
		t.Fatalf("cached segment poisoned: got %+v want %+v", segs2[0], origSeg)
	}
	if sps2[0] != origSp {
		t.Fatalf("cached spacer poisoned: got %+v want %+v", sps2[0], origSp)
	}
	// And the second return is also isolated from further mutation.
	segs2[0] = NewRect(1, 1, 1, 1)
	segs3, _ := l.SplitWithSpacers(area)
	if segs3[0] != origSeg {
		t.Fatalf("second caller mutation leaked: got %+v want %+v", segs3[0], origSeg)
	}
}

func TestLayoutCacheKeyDistinctions(t *testing.T) {
	resetLayoutCacheForTest()
	area := NewRect(0, 0, 50, 20)
	base := []Constraint{Length(5), Fill(1)}

	// Each distinct layout input must miss (grow cache), not collide.
	type call struct {
		l    Layout
		area Rect
	}
	calls := []call{
		{Horizontal(base...).Flex(FlexStart), area},
		{Horizontal(base...).Flex(FlexEnd), area},
		{Horizontal(base...).Flex(FlexStart).Spacing(2), area},
		{Horizontal(base...).Flex(FlexStart).Margin(1), area},
		{Vertical(base...).Flex(FlexStart), area},
		{Horizontal(base...).Flex(FlexStart), NewRect(1, 0, 50, 20)},
		{Horizontal(Length(5), Fill(2)).Flex(FlexStart), area},
		{Horizontal(Length(6), Fill(1)).Flex(FlexStart), area},
		{Horizontal(base...).Flex(FlexStart).HorizontalMargin(3), area},
		{Horizontal(base...).Flex(FlexStart).VerticalMargin(3), area},
		{Horizontal(Length(5)).Flex(FlexStart), area},
		{Horizontal(Length(5), Min(0)).Flex(FlexStart), area},
	}
	for i, c := range calls {
		c.l.SplitWithSpacers(c.area)
		if got := layoutCacheLenForTest(); got != i+1 {
			t.Fatalf("after call %d cache len=%d want %d (key collision?)", i, got, i+1)
		}
	}

	// Repeating the first call is a hit; length unchanged.
	calls[0].l.SplitWithSpacers(calls[0].area)
	if got := layoutCacheLenForTest(); got != len(calls) {
		t.Fatalf("hit grew cache: len=%d want %d", got, len(calls))
	}
}

func TestLayoutCacheBypassAboveCapacity(t *testing.T) {
	resetLayoutCacheForTest()
	cons := make([]Constraint, maxCachedConstraints+1)
	for i := range cons {
		cons[i] = Fill(1)
	}
	area := NewRect(0, 0, 100, 1)
	Horizontal(cons...).SplitWithSpacers(area)
	if got := layoutCacheLenForTest(); got != 0 {
		t.Fatalf("expected bypass for %d constraints, cache len=%d", len(cons), got)
	}
	// Boundary: exactly maxCachedConstraints is cached.
	cons = cons[:maxCachedConstraints]
	Horizontal(cons...).SplitWithSpacers(area)
	if got := layoutCacheLenForTest(); got != 1 {
		t.Fatalf("expected cache for %d constraints, len=%d", maxCachedConstraints, got)
	}
}

func TestLayoutCacheConcurrentCalls(t *testing.T) {
	resetLayoutCacheForTest()
	area := NewRect(0, 0, 100, 20)
	layouts := []Layout{
		Horizontal(Length(10), Fill(1), Percentage(30)).Flex(FlexStart),
		Horizontal(Fill(1), Fill(2)).Flex(FlexSpaceBetween).Spacing(1),
		Vertical(Min(3), Max(10), Fill(1)).Flex(FlexCenter),
		Horizontal(Ratio(1, 2), Ratio(1, 2)).Flex(FlexSpaceAround),
	}
	// Uncached reference results.
	wantSegs := make([][]Rect, len(layouts))
	wantSps := make([][]Rect, len(layouts))
	for i, l := range layouts {
		wantSegs[i], wantSps[i] = l.splitUncached(area)
	}

	const workers = 32
	const rounds = 40
	var wg sync.WaitGroup
	errCh := make(chan string, workers)
	for w := range workers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				li := (id + r) % len(layouts)
				segs, sps := layouts[li].SplitWithSpacers(area)
				if len(segs) != len(wantSegs[li]) || len(sps) != len(wantSps[li]) {
					errCh <- "length mismatch"
					return
				}
				for i := range segs {
					if segs[i] != wantSegs[li][i] {
						errCh <- "segment mismatch"
						return
					}
				}
				for i := range sps {
					if sps[i] != wantSps[li][i] {
						errCh <- "spacer mismatch"
						return
					}
				}
				// Mutate returned slices; must not affect others.
				if len(segs) > 0 {
					segs[0].X = -id - r
				}
				if len(sps) > 0 {
					sps[0].Y = -id - r
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Fatal(msg)
	}
	if got := layoutCacheLenForTest(); got != len(layouts) {
		t.Fatalf("cache len=%d want %d", got, len(layouts))
	}
}

func TestLayoutCacheLRUEviction(t *testing.T) {
	// Use a private small cache to verify eviction without thrashing the global one.
	c := newLayoutLRU(2)
	area := NewRect(0, 0, 10, 1)
	k1, _ := Horizontal(Length(1)).cacheKey(area)
	k2, _ := Horizontal(Length(2)).cacheKey(area)
	k3, _ := Horizontal(Length(3)).cacheKey(area)
	c.put(k1, []Rect{NewRect(0, 0, 1, 1)}, []Rect{NewRect(0, 0, 0, 1), NewRect(1, 0, 0, 1)})
	c.put(k2, []Rect{NewRect(0, 0, 2, 1)}, []Rect{NewRect(0, 0, 0, 1), NewRect(2, 0, 0, 1)})
	if c.n != 2 {
		t.Fatalf("len=%d want 2", c.n)
	}
	// Touch k1 so k2 is LRU.
	if _, ok := c.get(k1); !ok {
		t.Fatal("expected k1 hit")
	}
	c.put(k3, []Rect{NewRect(0, 0, 3, 1)}, []Rect{NewRect(0, 0, 0, 1), NewRect(3, 0, 0, 1)})
	if c.n != 2 {
		t.Fatalf("after evict len=%d want 2", c.n)
	}
	if _, ok := c.get(k2); ok {
		t.Fatal("k2 should have been evicted")
	}
	if _, ok := c.get(k1); !ok {
		t.Fatal("k1 should remain")
	}
	if _, ok := c.get(k3); !ok {
		t.Fatal("k3 should be present")
	}
}

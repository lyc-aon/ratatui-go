package media

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"sync/atomic"
)

// IDSource yields a fresh 24-bit Kitty image/placement id in [1, 0xffffff].
// Must never return 0. Injected for tests; production uses crypto/rand seed +
// atomic increment (never math/rand, never collision-retry loops).
type IDSource func() uint32

// defaultIDState holds the process-wide id counter seeded from crypto/rand.
var defaultIDState struct {
	once sync.Once
	next atomic.Uint32
}

func seedDefaultIDs() {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Extremely rare; fall back to a non-zero fixed seed rather than panic.
		defaultIDState.next.Store(0xA5A5A5)
		return
	}
	v := binary.BigEndian.Uint32(buf[:]) & 0xffffff
	if v == 0 {
		v = 1
	}
	defaultIDState.next.Store(v)
}

// DefaultIDSource returns the process-default crypto-seeded id generator.
func DefaultIDSource() IDSource {
	defaultIDState.once.Do(seedDefaultIDs)
	return func() uint32 {
		for {
			cur := defaultIDState.next.Load()
			next := (cur + 1) & 0xffffff
			if next == 0 {
				next = 1
			}
			if defaultIDState.next.CompareAndSwap(cur, next) {
				if cur == 0 {
					return 1
				}
				return cur
			}
		}
	}
}

// SequentialIDSource returns a deterministic source starting at seed (clamped
// into 24-bit non-zero space). Useful for tests.
func SequentialIDSource(seed uint32) IDSource {
	seed &= 0xffffff
	if seed == 0 {
		seed = 1
	}
	var next atomic.Uint32
	next.Store(seed)
	return func() uint32 {
		for {
			cur := next.Load()
			n := (cur + 1) & 0xffffff
			if n == 0 {
				n = 1
			}
			if next.CompareAndSwap(cur, n) {
				return cur
			}
		}
	}
}

// clampImageID forces id into the Kitty 24-bit non-zero id space.
func clampImageID(id uint32) uint32 {
	id &= 0xffffff
	if id == 0 {
		return 1
	}
	return id
}

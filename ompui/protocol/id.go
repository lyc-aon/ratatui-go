package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// IDGenerator produces correlation ids for request/response pairs.
// Safe for concurrent use.
type IDGenerator struct {
	counter atomic.Uint64
	prefix  string
}

// NewIDGenerator returns a generator with an optional prefix (e.g. "fe", "core").
// When prefix is empty, ids are bare hex tokens.
func NewIDGenerator(prefix string) *IDGenerator {
	return &IDGenerator{prefix: prefix}
}

// Next returns a new unique id.
// Format: [prefix-]timestamp_hex-counter_hex-rand4
func (g *IDGenerator) Next() string {
	if g == nil {
		return randomID("")
	}
	n := g.counter.Add(1)
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(time.Now().UnixNano()))
	var r [4]byte
	_, _ = rand.Read(r[:])
	id := fmt.Sprintf("%s-%x-%x", hex.EncodeToString(ts[:]), n, r)
	if g.prefix == "" {
		return id
	}
	return g.prefix + "-" + id
}

// NewID returns a one-shot random id without a generator.
func NewID() string {
	return randomID("")
}

func randomID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to time+counter-ish.
		binary.BigEndian.PutUint64(b[:8], uint64(time.Now().UnixNano()))
		binary.BigEndian.PutUint64(b[8:], uint64(time.Now().UnixNano()^0xdeadbeef))
	}
	id := hex.EncodeToString(b[:])
	if prefix == "" {
		return id
	}
	return prefix + "-" + id
}

// defaultIDs is used by package helpers that need an id without a caller-owned generator.
var defaultIDs = NewIDGenerator("omp")

// NextID returns an id from the package-default generator.
func NextID() string {
	return defaultIDs.Next()
}

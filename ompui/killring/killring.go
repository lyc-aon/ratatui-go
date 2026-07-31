// Package killring provides the bounded Emacs-style kill/yank history used by OMP editors.
package killring

const maxEntries = 60

// Ring stores killed text in oldest-to-newest order.
type Ring struct {
	entries []string
}

// Push records text. When accumulate is true, text is merged into the newest
// entry. prepend selects backward-delete order; otherwise text is appended.
func (r *Ring) Push(text string, prepend, accumulate bool) {
	if text == "" {
		return
	}
	if accumulate && len(r.entries) > 0 {
		last := len(r.entries) - 1
		if prepend {
			r.entries[last] = text + r.entries[last]
		} else {
			r.entries[last] += text
		}
		return
	}

	if len(r.entries) == maxEntries {
		copy(r.entries, r.entries[1:])
		r.entries[maxEntries-1] = text
		return
	}
	r.entries = append(r.entries, text)
}

// Peek returns the newest entry without changing the ring.
func (r *Ring) Peek() (string, bool) {
	if len(r.entries) == 0 {
		return "", false
	}
	return r.entries[len(r.entries)-1], true
}

// Rotate moves the newest entry to the oldest position for yank-pop cycling.
func (r *Ring) Rotate() {
	if len(r.entries) < 2 {
		return
	}
	last := r.entries[len(r.entries)-1]
	copy(r.entries[1:], r.entries[:len(r.entries)-1])
	r.entries[0] = last
}

// Len returns the number of stored entries.
func (r *Ring) Len() int { return len(r.entries) }

// Reset drops all entries while retaining bounded backing storage.
func (r *Ring) Reset() { r.entries = r.entries[:0] }

//go:build !windows

package pty

import (
	"strings"
	"sync"
)

// RingBuffer stores the last N lines of PTY output in a fixed-capacity ring.
// It implements io.Writer and is safe for concurrent use.
type RingBuffer struct {
	mu       sync.Mutex
	ring     []string
	head     int
	count    int
	maxLines int
	partial  string
}

// NewRingBuffer creates a RingBuffer that retains at most maxLines complete
// lines. If maxLines is less than 1 it is clamped to 1.
func NewRingBuffer(maxLines int) *RingBuffer {
	if maxLines < 1 {
		maxLines = 1
	}
	return &RingBuffer{
		ring:     make([]string, maxLines),
		maxLines: maxLines,
	}
}

// Write implements io.Writer. It splits p on newline boundaries, appending any
// buffered partial line from a prior call to the front of the incoming data.
// Each complete line (terminated by \n) is stored in the ring, evicting the
// oldest entry when the ring is full. Any trailing data without a terminating
// newline is held in an internal partial buffer until the next Write call or
// until Lines is called.
//
// Write always returns (len(p), nil).
func (b *RingBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data := b.partial + string(p)
	b.partial = ""

	segments := strings.Split(data, "\n")
	// All segments except the last are complete lines (they were followed by \n).
	// The last segment is a new partial (may be empty string).
	for i := 0; i < len(segments)-1; i++ {
		b.storeLine(segments[i])
	}
	b.partial = segments[len(segments)-1]

	return len(p), nil
}

// storeLine writes a single complete line into the ring, advancing head and
// evicting the oldest entry when the ring is full. Must be called with mu held.
func (b *RingBuffer) storeLine(line string) {
	b.ring[b.head] = line
	b.head = (b.head + 1) % b.maxLines
	if b.count < b.maxLines {
		b.count++
	}
}

// Lines returns a copy of all stored complete lines in oldest-to-newest order.
// If no lines have been stored it returns a non-nil empty slice.
func (b *RingBuffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]string, b.count)
	if b.count == 0 {
		return out
	}

	// The oldest entry sits at (head - count + maxLines) % maxLines.
	start := (b.head - b.count + b.maxLines) % b.maxLines
	for i := 0; i < b.count; i++ {
		out[i] = b.ring[(start+i)%b.maxLines]
	}
	return out
}

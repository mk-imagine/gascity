//go:build !windows

package pty

import (
	"strings"
	"sync"
	"testing"
)

// TestNewRingBuffer_HappyPath verifies that NewRingBuffer returns a non-nil
// RingBuffer for a positive maxLines value and that Lines() returns a
// non-nil empty slice on a fresh buffer.
func TestNewRingBuffer_HappyPath(t *testing.T) {
	b := NewRingBuffer(10)
	if b == nil {
		t.Fatal("NewRingBuffer(10) returned nil")
	}
	lines := b.Lines()
	if lines == nil {
		t.Fatal("Lines() on fresh buffer returned nil, want non-nil empty slice")
	}
	if len(lines) != 0 {
		t.Fatalf("Lines() on fresh buffer = %v, want empty slice", lines)
	}
}

// TestNewRingBuffer_ClampZero verifies that NewRingBuffer(0) clamps to 1 and
// stores at most 1 line.
func TestNewRingBuffer_ClampZero(t *testing.T) {
	b := NewRingBuffer(0)
	if b == nil {
		t.Fatal("NewRingBuffer(0) returned nil")
	}
	// Write two lines — only the last should survive with maxLines=1.
	b.Write([]byte("first\nsecond\n"))
	lines := b.Lines()
	if len(lines) != 1 {
		t.Fatalf("NewRingBuffer(0) clamped to 1: Lines() len = %d, want 1; got %v", len(lines), lines)
	}
	if lines[0] != "second" {
		t.Fatalf("NewRingBuffer(0) clamped to 1: Lines()[0] = %q, want %q", lines[0], "second")
	}
}

// TestRingBuffer_WriteReturnsLenNil verifies that Write always returns
// (len(p), nil) regardless of content.
func TestRingBuffer_WriteReturnsLenNil(t *testing.T) {
	b := NewRingBuffer(10)
	cases := [][]byte{
		[]byte("hello\n"),
		[]byte(""),
		[]byte("no newline"),
		[]byte("\n\n\n"),
		[]byte("partial"),
	}
	for _, p := range cases {
		n, err := b.Write(p)
		if err != nil {
			t.Fatalf("Write(%q) returned error: %v", p, err)
		}
		if n != len(p) {
			t.Fatalf("Write(%q) returned n=%d, want %d", p, n, len(p))
		}
	}
}

// TestRingBuffer_BasicLines verifies that two complete lines are stored and
// returned oldest-to-newest.
func TestRingBuffer_BasicLines(t *testing.T) {
	b := NewRingBuffer(10)
	n, err := b.Write([]byte("hello\nworld\n"))
	if err != nil {
		t.Fatalf("Write returned unexpected error: %v", err)
	}
	if n != len("hello\nworld\n") {
		t.Fatalf("Write returned n=%d, want %d", n, len("hello\nworld\n"))
	}
	lines := b.Lines()
	want := []string{"hello", "world"}
	if len(lines) != len(want) {
		t.Fatalf("Lines() = %v, want %v", lines, want)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("Lines()[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

// TestRingBuffer_EvictionOldest verifies that when maxLines is exceeded, the
// oldest lines are evicted and only the most recent maxLines lines remain.
func TestRingBuffer_EvictionOldest(t *testing.T) {
	b := NewRingBuffer(2)
	b.Write([]byte("line1\nline2\nline3\n"))
	lines := b.Lines()
	want := []string{"line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("Lines() = %v, want %v", lines, want)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("Lines()[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

// TestRingBuffer_PartialLineBuffered verifies that a partial (unterminated)
// line is not returned by Lines() until completed with a newline.
func TestRingBuffer_PartialLineBuffered(t *testing.T) {
	b := NewRingBuffer(10)
	b.Write([]byte("partial"))
	if lines := b.Lines(); len(lines) != 0 {
		t.Fatalf("Lines() after writing partial = %v, want empty", lines)
	}
	b.Write([]byte(" end\n"))
	lines := b.Lines()
	if len(lines) != 1 {
		t.Fatalf("Lines() after completing partial = %v, want [\"partial end\"]", lines)
	}
	if lines[0] != "partial end" {
		t.Fatalf("Lines()[0] = %q, want %q", lines[0], "partial end")
	}
}

// TestRingBuffer_SequentialWrites verifies that multiple sequential writes
// accumulate lines correctly.
func TestRingBuffer_SequentialWrites(t *testing.T) {
	b := NewRingBuffer(10)
	b.Write([]byte("a\nb\n"))
	b.Write([]byte("c\n"))
	lines := b.Lines()
	want := []string{"a", "b", "c"}
	if len(lines) != len(want) {
		t.Fatalf("Lines() = %v, want %v", lines, want)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("Lines()[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

// TestRingBuffer_WriteEmpty verifies that writing an empty byte slice is a
// no-op: Lines() remains unchanged and Write returns (0, nil).
func TestRingBuffer_WriteEmpty(t *testing.T) {
	b := NewRingBuffer(10)
	b.Write([]byte("existing\n"))
	before := b.Lines()

	n, err := b.Write([]byte(""))
	if err != nil {
		t.Fatalf("Write(\"\") returned error: %v", err)
	}
	if n != 0 {
		t.Fatalf("Write(\"\") returned n=%d, want 0", n)
	}

	after := b.Lines()
	if len(after) != len(before) {
		t.Fatalf("Lines() after Write(\"\") = %v, want %v", after, before)
	}
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("Lines()[%d] changed after Write(\"\") = %q, want %q", i, after[i], before[i])
		}
	}
}

// TestRingBuffer_WriteOnlyNewlines verifies that writing "\n\n\n" stores
// three empty lines.
func TestRingBuffer_WriteOnlyNewlines(t *testing.T) {
	b := NewRingBuffer(10)
	b.Write([]byte("\n\n\n"))
	lines := b.Lines()
	if len(lines) != 3 {
		t.Fatalf("Lines() after \"\\n\\n\\n\" = %v, want 3 empty strings", lines)
	}
	for i, l := range lines {
		if l != "" {
			t.Fatalf("Lines()[%d] = %q, want empty string", i, l)
		}
	}
}

// TestRingBuffer_NoNewlineNotReturned verifies that a write with no newline
// does not appear in Lines(), and becomes visible after a subsequent write
// that completes the line.
func TestRingBuffer_NoNewlineNotReturned(t *testing.T) {
	b := NewRingBuffer(10)
	b.Write([]byte("no newline"))
	if lines := b.Lines(); len(lines) != 0 {
		t.Fatalf("Lines() = %v, want empty after unterminated write", lines)
	}
	b.Write([]byte("\n"))
	lines := b.Lines()
	if len(lines) != 1 {
		t.Fatalf("Lines() = %v, want [\"no newline\"]", lines)
	}
	if lines[0] != "no newline" {
		t.Fatalf("Lines()[0] = %q, want %q", lines[0], "no newline")
	}
}

// TestRingBuffer_MaxLinesPlusOne verifies that writing maxLines+1 lines evicts
// the first line and retains the last maxLines lines.
func TestRingBuffer_MaxLinesPlusOne(t *testing.T) {
	const max = 5
	b := NewRingBuffer(max)
	var sb strings.Builder
	for i := 0; i < max+1; i++ {
		sb.WriteString(strings.Repeat("x", i+1))
		sb.WriteByte('\n')
	}
	b.Write([]byte(sb.String()))
	lines := b.Lines()
	if len(lines) != max {
		t.Fatalf("Lines() len = %d after maxLines+1 writes, want %d; got %v", len(lines), max, lines)
	}
	// The first line (single "x") must be gone.
	for _, l := range lines {
		if l == "x" {
			t.Fatalf("Lines() still contains the evicted first line: %v", lines)
		}
	}
	// The last line must be present.
	last := strings.Repeat("x", max+1)
	if lines[max-1] != last {
		t.Fatalf("Lines()[%d] = %q, want %q", max-1, lines[max-1], last)
	}
}

// TestRingBuffer_LinesReturnsCopy verifies that Lines() returns a copy: mutating
// the returned slice does not affect subsequent calls.
func TestRingBuffer_LinesReturnsCopy(t *testing.T) {
	b := NewRingBuffer(10)
	b.Write([]byte("alpha\nbeta\n"))
	first := b.Lines()
	first[0] = "MUTATED"
	second := b.Lines()
	if second[0] != "alpha" {
		t.Fatalf("Lines() not a copy: second call returned %q after mutating first result", second[0])
	}
}

// TestRingBuffer_LargeSplit verifies that a large payload with many newlines
// is split correctly and only the most recent maxLines lines are retained.
func TestRingBuffer_LargeSplit(t *testing.T) {
	const maxLines = 100
	b := NewRingBuffer(maxLines)

	// Build ~5000 bytes with newlines. 200 lines of 24 chars each = 4800 bytes.
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("0123456789abcdefghijklmn\n") // 25 bytes per line
	}
	payload := []byte(sb.String())
	n, err := b.Write(payload)
	if err != nil {
		t.Fatalf("Write large payload returned error: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write large payload returned n=%d, want %d", n, len(payload))
	}
	lines := b.Lines()
	if len(lines) != maxLines {
		t.Fatalf("Lines() len = %d after 200-line write with maxLines=%d, want %d", len(lines), maxLines, maxLines)
	}
}

// TestRingBuffer_FreshBufferNonNilSlice verifies that Lines() on a fresh buffer
// returns a non-nil empty slice (not nil).
func TestRingBuffer_FreshBufferNonNilSlice(t *testing.T) {
	b := NewRingBuffer(5)
	lines := b.Lines()
	if lines == nil {
		t.Fatal("Lines() on fresh buffer returned nil, want non-nil empty slice")
	}
	if len(lines) != 0 {
		t.Fatalf("Lines() on fresh buffer len = %d, want 0", len(lines))
	}
}

// TestRingBuffer_ConcurrentWrites verifies that concurrent writes from
// multiple goroutines do not produce data races. Run with -race.
func TestRingBuffer_ConcurrentWrites(t *testing.T) {
	const goroutines = 10
	const linesPerGoroutine = 100
	b := NewRingBuffer(goroutines * linesPerGoroutine)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < linesPerGoroutine; i++ {
				b.Write([]byte(strings.Repeat("x", g+1) + "\n"))
			}
		}()
	}
	wg.Wait()

	// Just reading must also be safe under -race.
	lines := b.Lines()
	if lines == nil {
		t.Fatal("Lines() after concurrent writes returned nil")
	}
}

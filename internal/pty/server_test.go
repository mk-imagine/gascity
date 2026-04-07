//go:build !windows

package pty

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestNewServer_PositiveCases verifies that NewServer returns a non-nil *Server
// and nil error for valid inputs, and that defaults are applied correctly.
func TestNewServer_PositiveCases(t *testing.T) {
	tests := []struct {
		name            string
		cmd             string
		args            []string
		opts            ServerOptions
		wantBufferLines int
	}{
		{
			name:            "default options apply: zero BufferLines becomes 1000",
			cmd:             "/bin/sh",
			args:            nil,
			opts:            ServerOptions{},
			wantBufferLines: 1000,
		},
		{
			name:            "explicit BufferLines is preserved",
			cmd:             "/bin/sh",
			args:            []string{"-l"},
			opts:            ServerOptions{BufferLines: 500},
			wantBufferLines: 500,
		},
		{
			name:            "custom logger is accepted",
			cmd:             "echo",
			args:            []string{"hello"},
			opts:            ServerOptions{Logger: log.New(log.Writer(), "test: ", 0)},
			wantBufferLines: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewServer(tt.cmd, tt.args, tt.opts)
			if err != nil {
				t.Fatalf("NewServer(%q, %v, %+v) returned unexpected error: %v", tt.cmd, tt.args, tt.opts, err)
			}
			if s == nil {
				t.Fatalf("NewServer(%q, %v, %+v) returned nil *Server, want non-nil", tt.cmd, tt.args, tt.opts)
			}
			if s.bufferLines != tt.wantBufferLines {
				t.Fatalf("NewServer bufferLines = %d, want %d", s.bufferLines, tt.wantBufferLines)
			}
		})
	}
}

// TestNewServer_NegativeCases verifies that NewServer returns nil and a
// descriptive error for invalid inputs.
func TestNewServer_NegativeCases(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		args        []string
		opts        ServerOptions
		errContains string
	}{
		{
			name:        "empty command returns error containing 'command'",
			cmd:         "",
			args:        nil,
			opts:        ServerOptions{},
			errContains: "command",
		},
		{
			name:        "negative BufferLines returns error containing 'buffer'",
			cmd:         "/bin/sh",
			args:        nil,
			opts:        ServerOptions{BufferLines: -1},
			errContains: "buffer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewServer(tt.cmd, tt.args, tt.opts)
			if err == nil {
				t.Fatalf("NewServer(%q, %v, %+v) returned nil error, want error containing %q", tt.cmd, tt.args, tt.opts, tt.errContains)
			}
			if s != nil {
				t.Fatalf("NewServer(%q, %v, %+v) returned non-nil *Server on error, want nil", tt.cmd, tt.args, tt.opts)
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("NewServer(%q, %v, %+v) error = %q, want to contain %q", tt.cmd, tt.args, tt.opts, err.Error(), tt.errContains)
			}
		})
	}
}

// TestHandleConn_EchoBinaryFrame verifies that a WebSocket client can send
// a binary frame to handleConn and receive the PTY's output back.
func TestHandleConn_EchoBinaryFrame(t *testing.T) {
	// Use a command that echoes then exits, avoiding ptyFile.Read blocking.
	s, err := NewServer("/bin/sh", []string{"-c", "read line && echo $line"}, ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { s.stop() })

	srv := httptest.NewServer(http.HandlerFunc(s.handleConn))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send "hello\n" — sh reads one line, echoes it, then exits.
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("hello\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read back until we see "hello" or connection closes (process exited).
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var collected string
	for i := 0; i < 50; i++ {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		collected += string(data)
		if strings.Contains(collected, "hello") {
			return // success
		}
	}
	t.Fatalf("did not receive echo of 'hello' in PTY output, got: %q", collected)
}

// TestHandleConn_ResizeFrame verifies that sending a resize text frame does
// not crash the server and the connection remains open.
func TestHandleConn_ResizeFrame(t *testing.T) {
	// Use sh that reads two lines — allows us to verify the connection
	// survives a resize between the two interactions.
	s, err := NewServer("/bin/sh", []string{"-c", "read a && echo $a && read b && echo $b"}, ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { s.stop() })

	srv := httptest.NewServer(http.HandlerFunc(s.handleConn))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send first line.
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("before\n")); err != nil {
		t.Fatalf("write before resize: %v", err)
	}

	// Wait for echo.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var collected string
	for i := 0; i < 50; i++ {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		collected += string(data)
		if strings.Contains(collected, "before") {
			break
		}
	}

	// Send resize text frame.
	resizeData, _ := EncodeResize(50, 120)
	if err := conn.WriteMessage(websocket.TextMessage, resizeData); err != nil {
		t.Fatalf("write resize: %v", err)
	}

	// Send second line after resize — verifies connection survived.
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("after\n")); err != nil {
		t.Fatalf("write after resize: %v", err)
	}

	collected = ""
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for i := 0; i < 50; i++ {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		collected += string(data)
		if strings.Contains(collected, "after") {
			return // success
		}
	}
	t.Fatalf("connection died after resize, collected: %q", collected)
}

// TestServerStart_BadCommand verifies that start() with a nonexistent command
// returns an error whose message contains "start".
func TestServerStart_BadCommand(t *testing.T) {
	s, err := NewServer("/nonexistent/binary", nil, ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	startErr := s.start()
	if startErr == nil {
		t.Fatal("start() with nonexistent command returned nil error, want error containing \"start\"")
	}
	if !strings.Contains(startErr.Error(), "start") {
		t.Fatalf("start() error = %q, want message to contain \"start\"", startErr.Error())
	}
}

// TestServerStart_AlreadyStarted verifies that calling start() twice without
// stopping returns an error whose message contains "already".
func TestServerStart_AlreadyStarted(t *testing.T) {
	s, err := NewServer("/bin/sh", []string{"-c", "sleep 5"}, ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.start(); err != nil {
		t.Fatalf("first start() returned unexpected error: %v", err)
	}
	t.Cleanup(func() {
		if s.process != nil {
			_ = s.process.Process.Kill()
			_, _ = s.process.Process.Wait()
		}
	})
	secondErr := s.start()
	if secondErr == nil {
		t.Fatal("second start() returned nil error, want error containing \"already\"")
	}
	if !strings.Contains(secondErr.Error(), "already") {
		t.Fatalf("second start() error = %q, want message to contain \"already\"", secondErr.Error())
	}
}

// TestServerStart_NoOutput verifies that start() with a command that produces
// no output (/bin/true) succeeds and s.buf.Lines() returns an empty slice after exit.
func TestServerStart_NoOutput(t *testing.T) {
	s, err := NewServer("/bin/true", nil, ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.start(); err != nil {
		t.Fatalf("start() returned unexpected error: %v", err)
	}
	if err := s.process.Wait(); err != nil {
		_ = err
	}
	lines := s.buf.Lines()
	if len(lines) != 0 {
		t.Fatalf("start(): buf.Lines() = %v, want empty slice for /bin/true", lines)
	}
}

// TestServerStart_PtyFileNonNil verifies that after a successful start(),
// s.ptyFile is non-nil.
func TestServerStart_PtyFileNonNil(t *testing.T) {
	s, err := NewServer("/bin/sh", []string{"-c", "sleep 5"}, ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.start(); err != nil {
		t.Fatalf("start() returned unexpected error: %v", err)
	}
	t.Cleanup(func() {
		if s.process != nil {
			_ = s.process.Process.Kill()
			_, _ = s.process.Process.Wait()
		}
		if s.ptyFile != nil {
			_ = s.ptyFile.Close()
		}
	})
	if s.ptyFile == nil {
		t.Fatal("start(): s.ptyFile is nil after successful start, want non-nil")
	}
}

// TestNewServer_EdgeCases verifies boundary conditions: empty-but-non-nil args
// slice is valid, and a BufferLines of 1 (minimum positive value) is accepted.
func TestNewServer_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		cmd             string
		args            []string
		opts            ServerOptions
		wantBufferLines int
	}{
		{
			name:            "empty args slice is valid; zero BufferLines defaults to 1000",
			cmd:             "/bin/sh",
			args:            []string{},
			opts:            ServerOptions{BufferLines: 0},
			wantBufferLines: 1000,
		},
		{
			name:            "BufferLines of 1 is the minimum valid value",
			cmd:             "/bin/sh",
			args:            nil,
			opts:            ServerOptions{BufferLines: 1},
			wantBufferLines: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewServer(tt.cmd, tt.args, tt.opts)
			if err != nil {
				t.Fatalf("NewServer(%q, %v, %+v) returned unexpected error: %v", tt.cmd, tt.args, tt.opts, err)
			}
			if s == nil {
				t.Fatalf("NewServer(%q, %v, %+v) returned nil *Server, want non-nil", tt.cmd, tt.args, tt.opts)
			}
			if s.bufferLines != tt.wantBufferLines {
				t.Fatalf("NewServer bufferLines = %d, want %d", s.bufferLines, tt.wantBufferLines)
			}
		})
	}
}

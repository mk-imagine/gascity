//go:build !windows

package pty

import (
	"log"
	"strings"
	"testing"
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

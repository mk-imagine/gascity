//go:build !windows

package pty

import (
	"fmt"
	"io"
	"log"
)

// ServerOptions configures a PTY server. Zero values are valid: BufferLines
// defaults to 1000 and Logger defaults to a discard logger.
type ServerOptions struct {
	BufferLines int         // number of lines to buffer; defaults to 1000 if zero
	Logger      *log.Logger // logger for server events; defaults to discard if nil
}

// Server manages a PTY subprocess and proxies its I/O over WebSocket
// connections. Create one with NewServer; start it with ListenAndServe.
type Server struct {
	cmd         string
	args        []string
	bufferLines int
	logger      *log.Logger
}

// NewServer creates a Server that will run the given command with args when
// started. opts may be a zero value. Returns an error if cmd is empty or
// opts.BufferLines is negative.
func NewServer(cmd string, args []string, opts ServerOptions) (*Server, error) {
	if cmd == "" {
		return nil, fmt.Errorf("creating server: command must not be empty")
	}
	if opts.BufferLines < 0 {
		return nil, fmt.Errorf("creating server: buffer lines must not be negative")
	}
	if opts.BufferLines == 0 {
		opts.BufferLines = 1000
	}
	if opts.Logger == nil {
		opts.Logger = log.New(io.Discard, "", 0)
	}
	return &Server{
		cmd:         cmd,
		args:        args,
		bufferLines: opts.BufferLines,
		logger:      opts.Logger,
	}, nil
}

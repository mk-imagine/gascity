//go:build !windows

package pty

import (
	"fmt"
	"io"
	"log"
	"os"
)

// ClientOptions configures a Client before it is created. All fields are
// optional; zero values apply documented defaults.
type ClientOptions struct {
	Raw    bool        // whether to set terminal raw mode on the local tty
	Stdin  io.Reader   // input source; defaults to os.Stdin if nil
	Stdout io.Writer   // output sink; defaults to os.Stdout if nil
	Logger *log.Logger // diagnostic logger; defaults to discard if nil
}

// Client connects to a PTY server and proxies data between the server and a
// local terminal. Create one with NewClient; Connect/proxy methods are not
// yet implemented.
type Client struct {
	addr   string
	raw    bool
	stdin  io.Reader
	stdout io.Writer
	logger *log.Logger
}

// NewClient constructs a Client targeting the given network address using the
// supplied options. addr must be non-empty. Nil option fields are replaced
// with sensible defaults: Stdin → os.Stdin, Stdout → os.Stdout, Logger →
// discard logger.
func NewClient(addr string, opts ClientOptions) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("creating client: addr must not be empty")
	}

	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &Client{
		addr:   addr,
		raw:    opts.Raw,
		stdin:  stdin,
		stdout: stdout,
		logger: logger,
	}, nil
}

//go:build !windows

package pty

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
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
// local terminal. Create one with NewClient; call Connect to establish the
// WebSocket connection.
type Client struct {
	addr   string
	raw    bool
	stdin  io.Reader
	stdout io.Writer
	logger *log.Logger
	conn   *websocket.Conn
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

// Connect dials the PTY server over WebSocket, optionally enters raw terminal
// mode on the local tty, then proxies I/O between the connection and the
// configured stdin/stdout until the context is cancelled or the server closes
// the connection. The terminal is restored to its original state before
// Connect returns.
func (c *Client) Connect(ctx context.Context) error {
	url := c.addr
	if !strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://") {
		url = "ws://" + url
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", url, err)
	}
	defer conn.Close()

	if c.raw {
		if f, ok := c.stdin.(*os.File); ok {
			fd := int(f.Fd())
			if term.IsTerminal(fd) {
				oldState, err := term.MakeRaw(fd)
				if err != nil {
					c.logger.Printf("pty: failed to set raw mode: %v", err)
				} else {
					defer term.Restore(fd, oldState) //nolint:errcheck
				}
			}
		}
	}

	return c.proxy(ctx, conn)
}

// proxy is a stub that will be implemented in a subsequent packet to copy data
// bidirectionally between conn and the client's stdin/stdout.
func (c *Client) proxy(ctx context.Context, conn *websocket.Conn) error {
	return nil
}

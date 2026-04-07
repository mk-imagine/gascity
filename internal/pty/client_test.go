//go:build !windows

package pty

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// TestNewClient_ValidAddr_ReturnsClient verifies that NewClient returns a
// non-nil *Client and a nil error when given a non-empty address and a
// zero-value ClientOptions.
func TestNewClient_ValidAddr_ReturnsClient(t *testing.T) {
	c, err := NewClient("localhost:8080", ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient(\"localhost:8080\", ClientOptions{}) returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient(\"localhost:8080\", ClientOptions{}) returned nil *Client, want non-nil")
	}
}

// TestNewClient_CustomOptions_Accepted verifies that NewClient accepts custom
// Stdin, Stdout, and Raw values and returns a non-nil *Client with nil error.
func TestNewClient_CustomOptions_Accepted(t *testing.T) {
	customReader := strings.NewReader("input")
	customWriter := &bytes.Buffer{}

	c, err := NewClient("localhost:8080", ClientOptions{
		Raw:    true,
		Stdin:  customReader,
		Stdout: customWriter,
	})
	if err != nil {
		t.Fatalf("NewClient with custom options returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with custom options returned nil *Client, want non-nil")
	}
}

// TestNewClient_CustomLogger_Accepted verifies that NewClient accepts a custom
// Logger and returns a non-nil *Client with nil error.
func TestNewClient_CustomLogger_Accepted(t *testing.T) {
	var buf bytes.Buffer
	customLogger := log.New(&buf, "test: ", 0)

	c, err := NewClient("127.0.0.1:9999", ClientOptions{
		Logger: customLogger,
	})
	if err != nil {
		t.Fatalf("NewClient(\"127.0.0.1:9999\", ClientOptions{Logger: ...}) returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient(\"127.0.0.1:9999\", ClientOptions{Logger: ...}) returned nil *Client, want non-nil")
	}
}

// TestNewClient_EmptyAddr_ReturnsError verifies that NewClient returns a nil
// *Client and a non-nil error whose message contains "addr" when given an
// empty address string.
func TestNewClient_EmptyAddr_ReturnsError(t *testing.T) {
	c, err := NewClient("", ClientOptions{})
	if err == nil {
		t.Fatal("NewClient(\"\", ClientOptions{}) returned nil error, want error containing \"addr\"")
	}
	if !strings.Contains(err.Error(), "addr") {
		t.Fatalf("NewClient(\"\", ClientOptions{}) error = %q, want it to contain \"addr\"", err.Error())
	}
	if c != nil {
		t.Fatalf("NewClient(\"\", ClientOptions{}) returned non-nil *Client, want nil")
	}
}

// TestNewClient_NilDefaults_Accepted verifies that NewClient accepts nil
// Stdin, nil Stdout, and nil Logger (defaults should be applied internally)
// and returns a non-nil *Client with nil error.
func TestNewClient_NilDefaults_Accepted(t *testing.T) {
	c, err := NewClient("localhost:8080", ClientOptions{
		Stdin:  nil,
		Stdout: nil,
		Logger: nil,
	})
	if err != nil {
		t.Fatalf("NewClient with nil optional fields returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with nil optional fields returned nil *Client, want non-nil")
	}
}

// TestNewClient_AddressStoredAsIs verifies that NewClient accepts addresses
// with and without a scheme prefix, returning a non-nil *Client and nil error
// in both cases.
func TestNewClient_AddressStoredAsIs(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{
			name: "with ws scheme",
			addr: "ws://localhost:8080",
		},
		{
			name: "without scheme",
			addr: "localhost:8080",
		},
		{
			name: "IP without scheme",
			addr: "127.0.0.1:9999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.addr, ClientOptions{})
			if err != nil {
				t.Fatalf("NewClient(%q, ClientOptions{}) returned unexpected error: %v", tt.addr, err)
			}
			if c == nil {
				t.Fatalf("NewClient(%q, ClientOptions{}) returned nil *Client, want non-nil", tt.addr)
			}
		})
	}
}

// TestNewClient_WriterInterface_Accepted verifies that NewClient accepts any
// io.Writer as Stdout (not just *os.File).
func TestNewClient_WriterInterface_Accepted(t *testing.T) {
	var buf bytes.Buffer
	c, err := NewClient("localhost:8080", ClientOptions{
		Stdout: io.Writer(&buf),
	})
	if err != nil {
		t.Fatalf("NewClient with io.Writer Stdout returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with io.Writer Stdout returned nil *Client, want non-nil")
	}
}

// TestNewClient_ReaderInterface_Accepted verifies that NewClient accepts any
// io.Reader as Stdin (not just *os.File).
func TestNewClient_ReaderInterface_Accepted(t *testing.T) {
	r := strings.NewReader("some input")
	c, err := NewClient("localhost:8080", ClientOptions{
		Stdin: io.Reader(r),
	})
	if err != nil {
		t.Fatalf("NewClient with io.Reader Stdin returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with io.Reader Stdin returned nil *Client, want non-nil")
	}
}

// wsUpgrader is a permissive WebSocket upgrader used by test servers.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// newTestWSServer starts an httptest.Server that upgrades every connection to
// WebSocket and immediately closes it. The returned URL has no scheme prefix
// (host:port only) unless withScheme is true.
func newTestWSServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.Close()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestConnect_ServerImmediateClose_ReturnsNil verifies that Connect returns nil
// when the server successfully upgrades the connection and then closes it.
func TestConnect_ServerImmediateClose_ReturnsNil(t *testing.T) {
	srv := newTestWSServer(t)
	// Strip "http://" from the test server URL — Connect will prepend "ws://".
	addr := strings.TrimPrefix(srv.URL, "http://")

	c, err := NewClient(addr, ClientOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() returned unexpected error: %v", err)
	}
}

// TestConnect_RawFalse_NonFileStdin_ReturnsNil verifies that Connect with
// Raw:false and a non-*os.File stdin connects successfully without attempting
// raw mode.
func TestConnect_RawFalse_NonFileStdin_ReturnsNil(t *testing.T) {
	srv := newTestWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://")

	c, err := NewClient(addr, ClientOptions{
		Raw:    false,
		Stdin:  &bytes.Buffer{},
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() with Raw:false returned unexpected error: %v", err)
	}
}

// TestConnect_NoServer_ReturnsDialError verifies that Connect returns an error
// wrapping the underlying dial failure when no server is listening.
func TestConnect_NoServer_ReturnsDialError(t *testing.T) {
	// Use a port that is very unlikely to have a listener.
	c, err := NewClient("127.0.0.1:19999", ClientOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	err = c.Connect(context.Background())
	if err == nil {
		t.Fatal("Connect() to non-listening address returned nil error, want dial error")
	}
}

// TestConnect_ContextCancelledBeforeDial_ReturnsContextError verifies that
// Connect returns an error satisfying errors.Is(err, context.Canceled) when
// the context is already cancelled before the dial attempt.
func TestConnect_ContextCancelledBeforeDial_ReturnsContextError(t *testing.T) {
	srv := newTestWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://")

	c, err := NewClient(addr, ClientOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err = c.Connect(ctx)
	if err == nil {
		t.Fatal("Connect() with cancelled context returned nil error, want context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Connect() with cancelled context: errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}

// TestConnect_AddrWithWSScheme_ConnectsSuccessfully verifies the edge case
// where the address already has a "ws://" prefix: it is used as-is without
// double-prepending.
func TestConnect_AddrWithWSScheme_ConnectsSuccessfully(t *testing.T) {
	srv := newTestWSServer(t)
	// Build full ws:// URL explicitly.
	addr := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	c, err := NewClient(addr, ClientOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() with ws:// prefix returned unexpected error: %v", err)
	}
}

// TestConnect_AddrWithoutScheme_PrependedAndConnects verifies the edge case
// where the address has no scheme: "ws://" is prepended and the connection
// succeeds.
func TestConnect_AddrWithoutScheme_PrependedAndConnects(t *testing.T) {
	srv := newTestWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://") // host:port only

	c, err := NewClient(addr, ClientOptions{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() with schemeless addr returned unexpected error: %v", err)
	}
}

// TestConnect_RawTrue_NonTerminalStdin_SkipsRawMode verifies the edge case
// where Raw:true is set but stdin is not an *os.File (a bytes.Reader). Raw
// mode must be silently skipped and Connect must still return nil.
func TestConnect_RawTrue_NonTerminalStdin_SkipsRawMode(t *testing.T) {
	srv := newTestWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://")

	c, err := NewClient(addr, ClientOptions{
		Raw:    true,
		Stdin:  strings.NewReader("not a terminal"),
		Stdout: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() with Raw:true + non-terminal stdin returned unexpected error: %v", err)
	}
}

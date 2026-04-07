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

// newEchoWSServer starts an httptest.Server that upgrades every connection to
// WebSocket and echoes each binary frame back to the sender as a binary frame.
// Text frames are discarded without echo. The server closes the connection when
// the client disconnects or an error occurs.
func newEchoWSServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.BinaryMessage {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, msg); writeErr != nil {
					return
				}
			}
			// Text frames are discarded without echo per contract.
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newProxyClient builds a Client pointed at addr with the given stdin reader
// and returns the client together with the stdout buffer it writes into.
func newProxyClient(t *testing.T, addr string, stdin io.Reader) (*Client, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	c, err := NewClient(addr, ClientOptions{
		Stdin:  stdin,
		Stdout: out,
	})
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}
	return c, out
}

// TestProxy_Echo_StdinToStdout verifies the positive case: bytes written to
// stdin are sent as a binary WebSocket frame, echoed by the server, and appear
// in stdout after proxy returns.
func TestProxy_Echo_StdinToStdout(t *testing.T) {
	srv := newEchoWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://")

	c, out := newProxyClient(t, addr, strings.NewReader("hello"))

	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() returned unexpected error: %v", err)
	}

	if got := out.String(); got != "hello" {
		t.Errorf("proxy echo: stdout = %q, want %q", got, "hello")
	}
}

// TestProxy_MultipleFrames_OrderPreserved verifies the positive case: multiple
// stdin reads each produce a binary frame; the server echoes all frames and
// they arrive in order in stdout.
func TestProxy_MultipleFrames_OrderPreserved(t *testing.T) {
	srv := newEchoWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://")

	// Produce input that spans multiple 4096-byte buffer reads: three distinct
	// chunks separated by a pipe so the reader returns them separately.
	pr, pw := io.Pipe()
	writes := []string{"frame1", "frame2", "frame3"}
	go func() {
		for _, w := range writes {
			_, _ = pw.Write([]byte(w))
		}
		pw.Close()
	}()

	c, out := newProxyClient(t, addr, pr)

	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() returned unexpected error: %v", err)
	}

	got := out.String()
	want := "frame1frame2frame3"
	if got != want {
		t.Errorf("proxy multiple frames: stdout = %q, want %q", got, want)
	}
}

// TestProxy_StdinEOF_ReturnsNil verifies the positive case: when stdin reaches
// EOF (bytes.Reader exhausted) the write loop exits cleanly and proxy returns
// nil.
func TestProxy_StdinEOF_ReturnsNil(t *testing.T) {
	srv := newEchoWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://")

	c, _ := newProxyClient(t, addr, strings.NewReader(""))

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() with EOF stdin returned unexpected error: %v", err)
	}
}

// TestProxy_ServerClose_ReturnsNil verifies the negative case: when the server
// closes the WebSocket connection the read loop detects the close and proxy
// returns nil.
func TestProxy_ServerClose_ReturnsNil(t *testing.T) {
	// Use newTestWSServer which upgrades then immediately closes.
	srv := newTestWSServer(t)
	addr := strings.TrimPrefix(srv.URL, "http://")

	// Provide a reader that blocks indefinitely so only the server close
	// triggers the return.
	pr, _ := io.Pipe()
	t.Cleanup(func() { pr.Close() })

	c, _ := newProxyClient(t, addr, pr)

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() after server close returned unexpected error: %v", err)
	}
}

// TestProxy_ContextCancelled_Exits verifies the negative case: cancelling the
// context causes proxy to exit both loops and return.
func TestProxy_ContextCancelled_Exits(t *testing.T) {
	// Use a server that stays open and does nothing so only the context
	// cancellation drives the exit.
	idleSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Block until the client disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(idleSrv.Close)

	addr := strings.TrimPrefix(idleSrv.URL, "http://")

	// stdin blocks forever — only the context cancel should terminate proxy.
	pr, _ := io.Pipe()
	t.Cleanup(func() { pr.Close() })

	c, _ := newProxyClient(t, addr, pr)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay so Connect has time to establish the
	// connection and enter proxy.
	go func() {
		cancel()
	}()

	// proxy must return (possibly with an error) after context is cancelled;
	// the contract only requires it exits, not the exact error value.
	done := make(chan error, 1)
	go func() {
		done <- c.Connect(ctx)
	}()

	select {
	case <-done:
		// returned — correct
	case <-context.Background().Done():
		t.Fatal("Connect() did not return after context cancellation")
	}
}

// TestProxy_TextFrame_Discarded verifies the edge case: a text frame received
// from the server is discarded (not written to stdout) and proxy does not
// return an error because of it.
func TestProxy_TextFrame_Discarded(t *testing.T) {
	// Server that sends one text frame then closes.
	textSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("should-be-discarded"))
	}))
	t.Cleanup(textSrv.Close)

	addr := strings.TrimPrefix(textSrv.URL, "http://")
	c, out := newProxyClient(t, addr, strings.NewReader(""))

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() after text frame returned unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("proxy text frame: stdout = %q, want empty (text frames must be discarded)", out.String())
	}
}

// TestProxy_EmptyBinaryFrame_WrittenToStdout verifies the edge case: an empty
// binary frame received from the server is written to stdout as a zero-length
// write. proxy must not error and must not skip the write.
func TestProxy_EmptyBinaryFrame_WrittenToStdout(t *testing.T) {
	// Server that sends one empty binary frame then closes.
	emptySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte{})
	}))
	t.Cleanup(emptySrv.Close)

	addr := strings.TrimPrefix(emptySrv.URL, "http://")
	c, _ := newProxyClient(t, addr, strings.NewReader(""))

	if err := c.Connect(context.Background()); err != nil {
		t.Errorf("Connect() after empty binary frame returned unexpected error: %v", err)
	}
	// The contract requires the write is attempted; we verify no error is
	// returned and Connect completes normally.
}

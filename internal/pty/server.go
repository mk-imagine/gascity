//go:build !windows

package pty

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	cpty "github.com/creack/pty"
	"github.com/gorilla/websocket"
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
	upgrader    websocket.Upgrader

	mu       sync.Mutex
	ptyFile  *os.File
	process  *exec.Cmd
	buf      *RingBuffer
	listener net.Listener
	done     chan struct{} // closed when PTY process exits
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
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}, nil
}

// start creates a PTY and execs the configured command. It does not start
// any I/O goroutines; handleConn owns the PTY read loop. Returns an error if
// the server is already running or if the PTY cannot be created.
func (s *Server) start() error {
	s.mu.Lock()
	if s.ptyFile != nil {
		s.mu.Unlock()
		return fmt.Errorf("starting server: already running")
	}
	s.mu.Unlock()

	cmd := exec.Command(s.cmd, s.args...)
	f, err := cpty.Start(cmd)
	if err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	s.mu.Lock()
	s.ptyFile = f
	s.process = cmd
	s.buf = NewRingBuffer(s.bufferLines)
	s.mu.Unlock()

	return nil
}

// stop closes the PTY and waits for the subprocess to exit.
func (s *Server) stop() error {
	s.mu.Lock()
	f := s.ptyFile
	proc := s.process
	s.ptyFile = nil
	s.mu.Unlock()

	if f != nil {
		f.Close()
	}
	if proc != nil {
		proc.Wait() //nolint:errcheck
	}
	return nil
}

// ListenAndServe starts the PTY process, binds an HTTP server on the given
// address, and blocks until either the context is cancelled or the PTY process
// exits. On either event, the HTTP server is shut down gracefully and the PTY
// is cleaned up. Use ":0" for addr to bind to a random available port; call
// Addr() after ListenAndServe starts to discover the actual address.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.start(); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.stop() //nolint:errcheck
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	s.mu.Lock()
	s.listener = ln
	s.done = make(chan struct{})
	s.mu.Unlock()

	// Watch for process exit.
	go func() {
		if s.process != nil {
			s.process.Wait() //nolint:errcheck
		}
		s.mu.Lock()
		done := s.done
		s.mu.Unlock()
		if done != nil {
			close(done)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleConn)

	httpServer := &http.Server{Handler: mux}

	// Serve in a goroutine.
	serveErr := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	// Wait for shutdown trigger.
	select {
	case <-ctx.Done():
	case <-s.done:
	case err := <-serveErr:
		s.stop() //nolint:errcheck
		return err
	}

	// Graceful shutdown.
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(shutCtx) //nolint:errcheck

	s.stop() //nolint:errcheck

	s.mu.Lock()
	s.listener = nil
	s.mu.Unlock()

	return nil
}

// Addr returns the address the server is listening on, or an empty string if
// the server is not currently listening.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// handleConn upgrades an HTTP request to a WebSocket connection and proxies
// bidirectional I/O between the WebSocket client and the PTY subprocess.
//
// Binary frames from the client are written to PTY stdin. PTY output is sent
// to the client as binary frames and simultaneously teed into the ring buffer.
// Text frames are decoded as resize events (via DecodeResize) and applied to
// the PTY; invalid text frames are logged and discarded. When the PTY reaches
// EOF a WebSocket close frame is sent. Either side disconnecting causes both
// goroutines to exit cleanly.
//
// If the PTY has not been started, handleConn responds with HTTP 503.
func (s *Server) handleConn(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	ptyFile := s.ptyFile
	buf := s.buf
	s.mu.Unlock()

	if ptyFile == nil {
		http.Error(w, "PTY not started", http.StatusServiceUnavailable)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("pty: WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	done := make(chan struct{}, 2)

	// PTY → WebSocket: read PTY output, tee to ring buffer, send binary frames.
	go func() {
		defer func() { done <- struct{}{} }()
		dst := io.MultiWriter(
			&wsWriter{conn: conn},
			buf,
		)
		if _, err := io.Copy(dst, ptyFile); err != nil {
			s.logger.Printf("pty: PTY read error: %v", err)
		}
		// PTY reached EOF — signal client to close.
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	// WebSocket → PTY: read client frames, write binary to PTY, handle resize.
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			switch msgType {
			case websocket.BinaryMessage:
				if _, err := ptyFile.Write(data); err != nil {
					s.logger.Printf("pty: PTY write error: %v", err)
					return
				}
			case websocket.TextMessage:
				msg, err := DecodeResize(data)
				if err != nil {
					s.logger.Printf("pty: invalid resize frame: %v", err)
					continue
				}
				if err := cpty.Setsize(ptyFile, &cpty.Winsize{
					Rows: msg.Rows,
					Cols: msg.Cols,
				}); err != nil {
					s.logger.Printf("pty: Setsize error: %v", err)
				}
			}
		}
	}()

	// Wait for either goroutine to exit, then close both sides to unblock
	// the other. PTY EOF → close conn. Client disconnect → close ptyFile
	// won't work (shared resource), so we close conn to unblock wsWriter.
	<-done
	conn.Close()
}

// wsWriter wraps a *websocket.Conn and implements io.Writer by sending each
// Write call as a binary WebSocket frame.
type wsWriter struct {
	conn *websocket.Conn
}

// Write sends p as a binary WebSocket frame. Returns len(p), nil on success,
// or 0 and the underlying error on failure.
func (w *wsWriter) Write(p []byte) (int, error) {
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

//go:build !windows

package pty

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"

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

	mu      sync.Mutex
	ptyFile *os.File
	process *exec.Cmd
	buf     *RingBuffer
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

	var wg sync.WaitGroup

	// PTY → WebSocket: read PTY output, tee to ring buffer, send binary frames.
	wg.Add(1)
	go func() {
		defer wg.Done()
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				// Normal disconnect or close — exit without logging.
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

	wg.Wait()
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

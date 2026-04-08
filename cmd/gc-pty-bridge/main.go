//go:build !windows

// gc-pty-bridge creates a real PTY for commands inside Docker containers
// and serves the PTY I/O over WebSocket. This bypasses Docker's PTY
// multiplexing bridge, which deadlocks TUI renderers like Claude Code's
// Ink/React interface.
//
// Usage:
//
//	gc-pty-bridge serve [--port PORT] [--buffer-lines N] COMMAND [ARGS...]
//	gc-pty-bridge attach HOST:PORT
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gastownhall/gascity/internal/pty"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "attach":
		os.Exit(runAttach(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "gc-pty-bridge: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  gc-pty-bridge serve [--port PORT] [--buffer-lines N] COMMAND [ARGS...]
  gc-pty-bridge attach HOST:PORT

Subcommands:
  serve    Create a PTY running COMMAND and serve it over WebSocket.
  attach   Connect to a PTY server and proxy terminal I/O.`)
}

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 7682, "WebSocket listen port")
	bufferLines := fs.Int("buffer-lines", 0, "number of output lines to buffer (0 = default 1000)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "gc-pty-bridge serve: command required")
		return 2
	}

	cmd := fs.Arg(0)
	cmdArgs := fs.Args()[1:]

	srv, err := pty.NewServer(cmd, cmdArgs, pty.ServerOptions{
		BufferLines: *bufferLines,
		Logger:      log.New(os.Stderr, "gc-pty-bridge: ", log.LstdFlags),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gc-pty-bridge: %v\n", err)
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	addr := ":" + strconv.Itoa(*port)
	fmt.Fprintf(os.Stderr, "gc-pty-bridge: serving %s on %s\n", cmd, addr)

	if err := srv.ListenAndServe(ctx, addr); err != nil {
		fmt.Fprintf(os.Stderr, "gc-pty-bridge: %v\n", err)
		return 1
	}
	return 0
}

func runAttach(args []string) int {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	raw := fs.Bool("raw", true, "set terminal to raw mode")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "gc-pty-bridge attach: address required (HOST:PORT)")
		return 2
	}

	addr := fs.Arg(0)

	client, err := pty.NewClient(addr, pty.ClientOptions{
		Raw:    *raw,
		Logger: log.New(os.Stderr, "gc-pty-bridge: ", log.LstdFlags),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gc-pty-bridge: %v\n", err)
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "gc-pty-bridge: %v\n", err)
		return 1
	}
	return 0
}

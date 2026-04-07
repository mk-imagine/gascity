# Module Brief: gc-pty

| Field | Value |
|-------|-------|
| **Module Name** | gc-pty |
| **Purpose** | PTY-over-WebSocket subcommand for the `gc` binary that enables interactive terminal sessions inside Docker containers, bypassing Docker's broken PTY multiplexing bridge that deadlocks TUI renderers (confirmed with Claude Code's Ink/React TUI). |
| **Boundary: Owns** | 1. `gc pty serve <command> [args...]` subcommand -- creates a real PTY via `creack/pty` (forkpty), execs the command, serves PTY I/O over WebSocket with terminal resize support and output ring buffer for peek. 2. `gc pty attach <host:port>` subcommand -- connects to a WebSocket PTY server, sets local terminal to raw mode, proxies stdin/stdout bidirectionally, forwards SIGWINCH as WebSocket control messages. 3. `internal/pty/` package -- reusable PTY-over-WebSocket server and client types (not CLI-specific). 4. WebSocket message protocol definition (data frames for I/O, control messages for resize). |
| **Boundary: Consumes** | 1. `cobra` command registration pattern from `cmd/gc/main.go` (`root.AddCommand`). 2. `golang.org/x/term` for raw mode (already an indirect dep via `golang.org/x/term`). 3. `gorilla/websocket` (already in go.mod as indirect dep -- will become direct). 4. `creack/pty` (new direct dependency). 5. Does NOT consume `internal/runtime.Provider` -- gc-pty is a standalone tool. The Docker exec provider script (`gc-session-docker`) will consume gc-pty, but that integration is out of scope for this module. |
| **Public Surface** | **CLI surface (cmd/gc/):** `gc pty serve --port <port> [--buffer-lines <n>] <command> [args...]` -- starts PTY WebSocket server. `gc pty attach [--raw] <host:port>` -- connects terminal to PTY server. **Go API (internal/pty/):** `type Server struct` -- `NewServer(cmd string, args []string, opts ServerOptions) *Server`, `Server.ListenAndServe(ctx context.Context, addr string) error`. `type Client struct` -- `NewClient(addr string, opts ClientOptions) *Client`, `Client.Connect(ctx context.Context) error`. `type ResizeMessage struct { Rows, Cols uint16 }`. WebSocket protocol: binary frames for PTY data, text frames with JSON `{"type":"resize","rows":N,"cols":N}` for control. |
| **External Dependencies** | `github.com/creack/pty` v1.1.24+ -- Go wrapper for forkpty(). `github.com/gorilla/websocket` -- already in go.mod (promote from indirect to direct). `golang.org/x/term` -- already available transitively. |
| **Inherited Constraints** | 1. ZERO hardcoded roles -- gc-pty is generic (runs any command, not Claude-specific). 2. `internal/` packages only -- no `pkg/` exports until API stabilizes. 3. TDD -- tests first. 4. cobra CLI pattern -- match existing `cmd_*.go` naming. 5. No panics in library code -- return errors with context. 6. Unit tests next to code (`pty_test.go`), testscript for CLI surface. 7. Tmux safety -- gc-pty does not interact with tmux at all. |
| **Repo Location** | `internal/pty/` -- server and client library code (`server.go`, `client.go`, `protocol.go`, tests). `cmd/gc/cmd_pty.go` -- cobra subcommand wiring (`gc pty serve`, `gc pty attach`). `cmd/gc/cmd_pty_test.go` -- unit tests for CLI flag parsing and command construction. `cmd/gc/testdata/script/pty_*.txtar` -- testscript integration tests for CLI behavior. |
| **Parallelism Hints** | Three independent work streams: (A) `internal/pty/server.go` + `server_test.go` -- PTY creation, WebSocket serving, output buffer. (B) `internal/pty/client.go` + `client_test.go` -- WebSocket client, raw mode, resize forwarding. (C) `cmd/gc/cmd_pty.go` + cobra wiring -- depends on A and B being at least stub-complete. Stream A and B can be built in parallel. Stream C is sequential after A+B. Protocol types (`protocol.go`) should be defined first as a shared dependency for A and B. |
| **Cross-File Coupling** | `internal/pty/protocol.go` defines shared message types used by both `server.go` and `client.go` -- must be defined before either. `cmd/gc/cmd_pty.go` imports `internal/pty` and must be added to `cmd/gc/main.go` command registration. |
| **Execution Mode Preference** | `Tool-Integrated` -- the design is fully specified from the ttyd validation. No ambiguous design decisions remain. The WebSocket protocol, PTY creation mechanism, and CLI interface are all determined. |
| **Definition of Done** | 1. `gc pty serve /bin/sh` starts a WebSocket server on a configurable port and creates a real PTY running `/bin/sh`. 2. `gc pty attach localhost:<port>` connects to the server, enters raw mode, and provides full interactive terminal access. 3. Terminal resize (SIGWINCH) propagates from client to server and resizes the PTY. 4. Output ring buffer supports peek-style reads (configurable line count). 5. Clean shutdown: client disconnect does not crash server; server exit closes all clients; SIGINT/SIGTERM handled gracefully. 6. `go test ./internal/pty/...` passes with unit tests covering: server startup/shutdown, client connect/disconnect, resize message round-trip, output buffer overflow, concurrent client access. 7. `go test ./cmd/gc/ -run TestPty` passes with testscript coverage for CLI flag parsing and basic serve/attach round-trip. 8. `go vet ./internal/pty/... ./cmd/gc/...` clean. 9. All exported types and functions have doc comments. |

---

## Supplementary Analysis

### Why internal/pty/ and not cmd/gc/ inline

The PTY-over-WebSocket server and client are reusable infrastructure that the Docker exec provider script will eventually call via `gc pty serve` inside containers. Keeping the library in `internal/pty/` follows the existing pattern where runtime providers live in `internal/runtime/<provider>/` and CLI commands in `cmd/gc/cmd_*.go` are thin wrappers. This also enables future providers (K8s, etc.) to reuse the same PTY bridge without depending on CLI code.

### WebSocket protocol design

Keeping the protocol simple and ttyd-compatible where possible:

- **Binary frames**: raw PTY output (server-to-client) and raw terminal input (client-to-server)
- **Text frames**: JSON control messages -- currently only `{"type":"resize","rows":N,"cols":N}`
- No authentication in v1 (container-internal communication only; the WebSocket listener binds to the container's loopback or a container-internal port)
- No TLS in v1 (same reason -- container-local traffic)

### Dependency on gorilla/websocket

Already present as an indirect dependency (pulled in by k8s client-go). Promoting to direct is safe and avoids adding a second WebSocket library. `nhooyr.io/websocket` was considered but gorilla is already in the dependency tree and is the more established library for this use case.

### What is explicitly NOT in scope

1. **Modifications to `gc-session-docker`** -- the exec provider script will be updated separately to use `gc pty serve`/`gc pty attach` instead of tmux-in-Docker. That is a separate module.
2. **Authentication or TLS** -- gc-pty is for container-internal or trusted-network use. Security hardening is future work if the scope expands.
3. **Multiple concurrent PTY sessions on one server** -- v1 is one PTY per server instance (matching ttyd's model). The exec provider runs one `gc pty serve` per container.
4. **ACP (Agent Communication Protocol) integration** -- the ACP provider in `internal/runtime/acp/` is headless JSON-RPC and orthogonal to interactive PTY access.
5. **Any changes to `internal/runtime.Provider` interface** -- gc-pty operates below the provider abstraction; the Docker exec provider script calls it as a subprocess.

### Risk: Platform constraints

`creack/pty` uses forkpty() which is Unix-only. gc-pty will not work on Windows. This is acceptable because:
- Docker containers run Linux
- The `serve` subcommand only runs inside containers
- The `attach` subcommand runs on macOS/Linux hosts (the development targets)
- A `//go:build !windows` constraint on `internal/pty/` prevents compilation errors on Windows

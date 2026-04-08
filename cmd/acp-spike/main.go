// Command acp-spike demonstrates the ACP (Agent Client Protocol) for
// interacting with Claude Code running in a Docker container.
//
// It uses Gas City's ACP provider to manage the full lifecycle:
//
//  1. Build a minimal Docker image (node:20-slim + Claude Code)
//  2. Start Claude Code in the container via "docker run -i" (stdio pipes)
//  3. ACP handshake: initialize → initialized → session/new
//  4. Exchange prompts via session/prompt + session/update
//  5. Clean shutdown
//
// The ACP provider (internal/runtime/acp) handles all JSON-RPC framing,
// handshake sequencing, and busy-state tracking. This spike shows how
// a consumer wires it up.
//
// Prerequisites:
//   - Docker running (Docker Desktop on Mac, or native on Linux)
//   - Claude credentials at ~/.claude/.credentials.json
//     (run "claude login" first if missing)
//
// Usage:
//
//	go run ./cmd/acp-spike
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gastownhall/gascity/internal/runtime"
	sessionacp "github.com/gastownhall/gascity/internal/runtime/acp"
)

const (
	imageName   = "gc-acp-spike"
	sessionName = "spike"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("acp-spike: ")

	// ── Step 1: Verify credentials ──────────────────────────────────
	home := os.Getenv("HOME")
	credFiles := map[string]string{
		"credentials": filepath.Join(home, ".claude", ".credentials.json"),
		"settings":    filepath.Join(home, ".claude", "settings.json"),
		"top config":  filepath.Join(home, ".claude.json"),
	}
	for label, path := range credFiles {
		if _, err := os.Stat(path); err != nil {
			log.Fatalf("missing %s at %s\nRun 'claude login' to create credentials.", label, path)
		}
	}
	log.Println("credentials found")

	// ── Step 2: Build Docker image ──────────���───────────────────────
	if err := ensureImage(); err != nil {
		log.Fatalf("image build: %v", err)
	}

	// ���─ Step 3: Create ACP provider ─────────────────────────────────
	//
	// The provider manages:
	//   - Process spawning with stdio pipes
	//   - JSON-RPC read/write loop
	//   - Handshake sequencing
	//   - Busy-state tracking (waits for idle before next Nudge)
	//   - Circular output buffer for Peek
	//   - Control socket for cross-process discovery
	//
	stateDir, err := os.MkdirTemp("", "acp-spike-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(stateDir)

	provider := sessionacp.NewProviderWithDir(stateDir, sessionacp.Config{
		HandshakeTimeout:  90 * time.Second,
		NudgeBusyTimeout:  180 * time.Second,
		OutputBufferLines: 500,
	})

	// ── Step 4: Start ACP session ───────────────────────────────────
	//
	// Under the hood, the provider:
	//   1. Runs: sh -c "docker run --rm -i <mounts> <image> claude <args>"
	//   2. Pipes stdin/stdout for JSON-RPC
	//   3. Sends "initialize" request → waits for server info
	//   4. Sends "initialized" notification
	//   5. Sends "session/new" → receives session ID
	//
	// Claude Code detects non-TTY stdin and speaks ACP automatically.
	//
	cmd := dockerRunCommand(credFiles)
	log.Printf("command: %s", cmd)
	log.Println("starting ACP session (handshake may take a moment)...")

	ctx := context.Background()
	if err := provider.Start(ctx, sessionName, runtime.Config{
		Command: cmd,
	}); err != nil {
		log.Fatalf("start failed: %v", err)
	}
	defer func() {
		log.Println("stopping session...")
		if err := provider.Stop(sessionName); err != nil {
			log.Printf("stop: %v", err)
		}
		log.Println("done")
	}()

	log.Println("handshake complete — session is live")

	// ── Step 5: Exchange prompts ─────────���──────────────────────────
	//
	// Nudge() sends a "session/prompt" JSON-RPC request. If the agent
	// is still processing a prior prompt, it blocks until idle.
	//
	// session/update notifications arrive asynchronously and are buffered
	// in the provider's circular output buffer, readable via Peek().
	//
	prompts := []string{
		"Say hello in exactly 5 words. No tool use, just text.",
		"What is 2+2? Answer with just the number.",
		"Say goodbye in exactly 5 words. No tool use, just text.",
	}

	for i, prompt := range prompts {
		fmt.Printf("\n━━━ Prompt %d/%d ━━━\n> %s\n\n", i+1, len(prompts), prompt)

		// Clear the output buffer so we only see this response.
		_ = provider.ClearScrollback(sessionName)

		// Send the prompt. Nudge internally:
		//   1. Waits for idle (previous prompt done)
		//   2. Builds session/prompt JSON-RPC request
		//   3. Writes to stdin pipe
		//   4. Returns immediately (response drained in background)
		if err := provider.Nudge(sessionName, runtime.TextContent(prompt)); err != nil {
			log.Printf("nudge %d failed: %v", i+1, err)
			continue
		}

		// Poll Peek() until output stabilizes (agent done writing).
		output := waitForStableOutput(provider, 120*time.Second)
		if output == "" {
			log.Printf("prompt %d: no output received (timeout)", i+1)
		} else {
			fmt.Println(output)
		}
	}

	fmt.Println("\n━━━ Spike complete ━━━")
}

// waitForStableOutput polls Peek until the output buffer stops changing
// for 5 consecutive seconds, indicating the agent has finished writing.
func waitForStableOutput(p *sessionacp.Provider, timeout time.Duration) string {
	deadline := time.After(timeout)
	var lastOutput string
	stableCount := 0
	const stableThreshold = 5 // seconds of no change = done

	for {
		select {
		case <-deadline:
			return lastOutput
		case <-time.After(time.Second):
			output, _ := p.Peek(sessionName, 0)
			if output != "" && output == lastOutput {
				stableCount++
				if stableCount >= stableThreshold {
					return output
				}
			} else {
				lastOutput = output
				stableCount = 0
			}
		}
	}
}

// dockerRunCommand builds the docker run command string.
//
// The ACP provider wraps this in: sh -c "<command>"
// Docker's -i flag (interactive, no tty) relays stdio between
// the provider's pipes and the container's entrypoint (claude).
func dockerRunCommand(credFiles map[string]string) string {
	parts := []string{
		"docker", "run", "--rm", "-i",
		// Credential mounts (read-only).
		"-v", credFiles["credentials"] + ":/root/.claude/.credentials.json:ro",
		"-v", credFiles["settings"] + ":/root/.claude/settings.json:ro",
		"-v", credFiles["top config"] + ":/root/.claude.json:ro",
		// Image + claude args.
		imageName,
		"--dangerously-skip-permissions",
	}
	return strings.Join(parts, " ")
}

// ensureImage builds the Docker image if it doesn't exist locally.
func ensureImage() error {
	// Check if image already exists.
	check := exec.Command("docker", "image", "inspect", imageName)
	check.Stdout = nil
	check.Stderr = nil
	if check.Run() == nil {
		log.Println("image exists, skipping build")
		return nil
	}

	log.Println("building Docker image (this may take a minute)...")

	// Build from inline Dockerfile via stdin.
	dockerfile := `FROM node:20-slim
RUN npm install -g @anthropic-ai/claude-code 2>/dev/null
ENTRYPOINT ["claude"]
`
	cmd := exec.Command("docker", "build", "-t", imageName, "-f", "-", ".")
	cmd.Dir = "/" // context doesn't matter, no COPY instructions
	cmd.Stdin = strings.NewReader(dockerfile)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

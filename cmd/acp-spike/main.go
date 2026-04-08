// Command acp-spike demonstrates multi-turn conversation with Claude Code
// inside a Docker container using the raw CLI + JSON output.
//
// This validates the Go-native approach: no Agent SDK, no Node.js, no ACP.
// Just spawn `claude -p <prompt> --output-format json --resume <id>` per
// turn and parse the JSON result.
//
// Usage:
//
//	go run ./cmd/acp-spike
//
// Prerequisites:
//   - Docker running with gc-acp-spike image built
//   - Claude credentials at ~/.claude/.credentials.json
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// claudeResult is the JSON output from `claude -p --output-format json`.
type claudeResult struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	SessionID    string  `json:"session_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	DurationMs   int     `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	StopReason   string  `json:"stop_reason"`
}

const imageName = "gc-acp-spike"

func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("spike: ")

	// Verify credentials exist.
	home := os.Getenv("HOME")
	creds := filepath.Join(home, ".claude", ".credentials.json")
	settings := filepath.Join(home, ".claude", "settings.json")
	topCfg := filepath.Join(home, ".claude.json")
	for _, f := range []string{creds, settings, topCfg} {
		if _, err := os.Stat(f); err != nil {
			log.Fatalf("missing credential file: %s", f)
		}
	}

	// Create a temp directory for session persistence across container runs.
	// Each `docker run --rm` destroys the container filesystem, but sessions
	// are stored at ~/.claude/projects/. By mounting a host directory there,
	// sessions survive across runs and --resume works.
	sessionDir, err := os.MkdirTemp("", "gc-spike-sessions-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(sessionDir)
	log.Printf("session dir: %s", sessionDir)

	prompts := []string{
		"Remember the number 42. Just say OK.",
		"What number did I ask you to remember? Answer with just the number.",
		"Say goodbye in exactly 5 words.",
	}

	var sessionID string

	for i, prompt := range prompts {
		fmt.Printf("\n━━━ Prompt %d/%d ━━━\n> %s\n\n", i+1, len(prompts), prompt)

		result, err := runClaude(prompt, sessionID, sessionDir, creds, settings, topCfg)
		if err != nil {
			log.Fatalf("prompt %d: %v", i+1, err)
		}

		sessionID = result.SessionID
		fmt.Printf("Response: %s\n", result.Result)
		fmt.Printf("[%s, turns: %d, cost: $%.4f, time: %dms, session: %s]\n",
			result.Subtype, result.NumTurns, result.TotalCostUSD,
			result.DurationMs, sessionID[:8])
	}

	fmt.Printf("\n━━━ Spike complete ━━━\nSession: %s\n", sessionID)
}

// runClaude executes a single turn of conversation with Claude Code
// inside a Docker container. Returns the parsed result.
//
// This is the core pattern a Go provider would use:
//  1. Build docker run command with credential mounts
//  2. Append --resume <id> for multi-turn
//  3. Capture stdout (JSON result)
//  4. Parse and return
func runClaude(prompt, sessionID, sessionDir, creds, settings, topCfg string) (*claudeResult, error) {
	// Build the docker run command.
	args := []string{
		"run", "--rm",
		// Credential mounts (read-only).
		"-v", creds + ":/root/.claude/.credentials.json:ro",
		"-v", settings + ":/root/.claude/settings.json:ro",
		"-v", topCfg + ":/root/.claude.json:ro",
		// Session persistence: mount host temp dir so sessions survive
		// across container runs. Without this, --resume fails because
		// each container has a fresh filesystem.
		"-v", sessionDir + ":/root/.claude/projects",
		// Override entrypoint to call claude directly.
		"--entrypoint", "claude",
		imageName,
		// Claude args.
		"-p", prompt,
		"--output-format", "json",
	}

	// Resume session for multi-turn.
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}

	log.Printf("running: docker %s", strings.Join(args[:6], " ")+"...")

	start := time.Now()
	out, err := exec.Command("docker", args...).Output()
	elapsed := time.Since(start)

	if err != nil {
		// Include stderr in error message if available.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("claude exited %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("exec: %w", err)
	}

	log.Printf("completed in %s (%d bytes)", elapsed.Round(time.Millisecond), len(out))

	// Parse JSON result. The output is a single JSON object.
	var result claudeResult
	if err := json.Unmarshal(out, &result); err != nil {
		// Output might have multiple lines (verbose mode). Try the last line.
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		lastLine := lines[len(lines)-1]
		if err2 := json.Unmarshal([]byte(lastLine), &result); err2 != nil {
			return nil, fmt.Errorf("parse JSON: %w\nraw output: %s", err, string(out[:min(len(out), 500)]))
		}
	}

	if result.IsError {
		return nil, fmt.Errorf("claude error: %s — %s", result.Subtype, result.Result)
	}

	return &result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

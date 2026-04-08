// Agent SDK spike — multi-turn conversation with Claude Code in Docker.
//
// Demonstrates:
//   1. OAuth credential injection (no API key needed)
//   2. Agent SDK query() with streaming messages
//   3. Multi-turn conversation via session resume
//   4. Running headless in a container (no TTY, no tmux)
//
// The Agent SDK spawns a Claude Code process under the hood. That process
// reads ~/.claude/.credentials.json for subscription auth — the same
// mechanism Gas Town uses for all its agents.
//
// Usage:
//   docker build -t gc-acp-spike -f cmd/acp-spike/Dockerfile cmd/acp-spike/
//   docker run --rm \
//     -v ~/.claude/.credentials.json:/home/node/.claude/.credentials.json:ro \
//     -v ~/.claude/settings.json:/home/node/.claude/settings.json:ro \
//     -v ~/.claude.json:/home/node/.claude.json:ro \
//     gc-acp-spike

import { query } from "@anthropic-ai/claude-agent-sdk";

const prompts = [
  "Say hello in exactly 5 words. No tool use, just text.",
  "What is 2+2? Answer with just the number.",
  "Say goodbye in exactly 5 words. No tool use, just text.",
];

let sessionId;

for (let i = 0; i < prompts.length; i++) {
  const prompt = prompts[i];
  console.log(`\n━━━ Prompt ${i + 1}/${prompts.length} ━━━`);
  console.log(`> ${prompt}\n`);

  // Build options for this turn.
  //
  // - permissionMode "dontAsk": denies any tool not in allowedTools,
  //   no interactive prompts. Perfect for headless containers.
  // - allowedTools []: no tools needed for simple text responses.
  //   The agent can only reply with text, not call tools.
  // - resume: continues the same session so Claude has context from
  //   prior turns. On the first turn, sessionId is undefined (new session).
  const options = {
    allowedTools: [],
    permissionMode: "dontAsk",
  };

  if (sessionId) {
    options.resume = sessionId;
  }

  // query() returns an async generator that streams messages as Claude
  // works. Message types:
  //   - system (subtype: init)  — session ID, tools, model info
  //   - assistant               — Claude's response (text + tool calls)
  //   - result                  — final outcome with cost/usage
  for await (const message of query({ prompt, options })) {
    // Debug: log every message type so we can see what the SDK emits.
    console.error(`[debug] message.type=${message.type} subtype=${message.subtype ?? "-"}`);

    // Capture session ID from init or result for resume.
    if (message.type === "system" && message.subtype === "init") {
      sessionId = message.session_id;
      console.error(`[debug] session_id=${sessionId}`);
    }

    // Print Claude's text response.
    if (message.type === "assistant" && message.message?.content) {
      for (const block of message.message.content) {
        if ("text" in block) {
          process.stdout.write(block.text);
        }
      }
    }

    // Print result summary.
    if (message.type === "result") {
      sessionId = message.session_id;
      const cost = message.total_cost_usd?.toFixed(4) ?? "?";
      console.log(`\n[${message.subtype}, cost: $${cost}]`);
    }
  }
}

console.log("\n━━━ Spike complete ━━━");

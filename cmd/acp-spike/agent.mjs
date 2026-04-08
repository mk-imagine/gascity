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
//     -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
//     -v ~/.claude/settings.json:/root/.claude/settings.json:ro \
//     -v ~/.claude.json:/root/.claude.json:ro \
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

  const options = {
    allowedTools: [],
    permissionMode: "dontAsk",
  };

  if (sessionId) {
    options.resume = sessionId;
  }

  try {
    for await (const message of query({ prompt, options })) {
      // Capture session ID for resume.
      if (message.type === "system" && message.subtype === "init") {
        sessionId = message.session_id;
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
  } catch (err) {
    console.error(`[error] query failed: ${err.message}`);
  }
}

console.log("\n━━━ Spike complete ━━━");

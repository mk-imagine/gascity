// Spike: Agent SDK V2 persistent session.
//
// Validates multi-turn conversation over a single persistent Claude Code
// process using the V2 send()/stream() API. This is the transport layer
// for the mayor chat UI.
//
// Usage:
//   docker run --rm --entrypoint node \
//     -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
//     -v ~/.claude/settings.json:/root/.claude/settings.json:ro \
//     -v ~/.claude.json:/root/.claude.json:ro \
//     -v $(pwd)/cmd/acp-spike/test-streaming-input.mjs:/app/test.mjs:ro \
//     gc-acp-spike /app/test.mjs

import { unstable_v2_createSession } from "@anthropic-ai/claude-agent-sdk";

const prompts = [
  "Remember the number 42. Just say OK.",
  "What number did I ask you to remember? Answer with just the number.",
  "Say goodbye in exactly 5 words.",
];

async function main() {
  console.log("[spike] creating persistent session...");

  const session = unstable_v2_createSession({
    permissionMode: "dontAsk",
    allowedTools: [],
  });

  console.log("[spike] session created, sending prompts...\n");

  for (let i = 0; i < prompts.length; i++) {
    const prompt = prompts[i];
    console.log(`━━━ Prompt ${i + 1}/${prompts.length} ━━━`);
    console.log(`> ${prompt}\n`);

    const start = Date.now();
    await session.send(prompt);

    for await (const msg of session.stream()) {
      // Print assistant text as it streams.
      if (msg.type === "assistant" && msg.message?.content) {
        for (const block of msg.message.content) {
          if (block.type === "text") {
            process.stdout.write(block.text);
          }
        }
      }

      // Print result summary.
      if (msg.type === "result") {
        const elapsed = Date.now() - start;
        const cost = msg.total_cost_usd?.toFixed(4) ?? "?";
        console.log(`\n[${msg.subtype}, ${elapsed}ms, cost: $${cost}]\n`);
      }
    }
  }

  console.log("━━━ Spike complete ━━━");
  console.log(`Session ID: ${session.sessionId}`);
  session.close();
}

main().catch((err) => {
  console.error(`[error] ${err.message}`);
  process.exit(1);
});

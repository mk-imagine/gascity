// Spike: Agent SDK streaming input mode.
//
// Validates that query() accepts an AsyncIterable<SDKUserMessage> for
// multi-turn conversation over a single persistent process. This is
// the foundation for the mayor chat UI — one long-lived Claude Code
// process, messages pushed in as the user types.
//
// Usage:
//   docker run --rm --entrypoint node \
//     -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
//     -v ~/.claude/settings.json:/root/.claude/settings.json:ro \
//     -v ~/.claude.json:/root/.claude.json:ro \
//     gc-acp-spike /app/test-streaming-input.mjs

import { query } from "@anthropic-ai/claude-agent-sdk";

// A push-based async iterable. Call push() to send messages,
// done() to signal completion. The query() function pulls from this.
function createMessageStream() {
  const queue = [];
  let resolve = null;
  let finished = false;

  return {
    push(msg) {
      if (resolve) {
        const r = resolve;
        resolve = null;
        r({ value: msg, done: false });
      } else {
        queue.push(msg);
      }
    },
    done() {
      finished = true;
      if (resolve) {
        const r = resolve;
        resolve = null;
        r({ value: undefined, done: true });
      }
    },
    [Symbol.asyncIterator]() {
      return {
        next() {
          if (queue.length > 0) {
            return Promise.resolve({ value: queue.shift(), done: false });
          }
          if (finished) {
            return Promise.resolve({ value: undefined, done: true });
          }
          return new Promise((r) => { resolve = r; });
        }
      };
    }
  };
}

// Helper: wait for ms milliseconds.
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function main() {
  const stream = createMessageStream();

  const options = {
    allowedTools: [],
    permissionMode: "dontAsk",
  };

  console.log("[spike] starting query with streaming input...");

  // Start the query with our async iterable as the prompt source.
  // This spawns ONE Claude Code process that stays alive.
  const q = query({ prompt: stream, options });

  // Read responses in background.
  const reader = (async () => {
    let turnCount = 0;
    for await (const message of q) {
      if (message.type === "system" && message.subtype === "init") {
        console.log(`[init] session=${message.session_id}, model=${message.model}`);
      }

      if (message.type === "assistant" && message.message?.content) {
        for (const block of message.message.content) {
          if ("text" in block) {
            process.stdout.write(block.text);
          }
        }
      }

      if (message.type === "result") {
        turnCount++;
        const cost = message.total_cost_usd?.toFixed(4) ?? "?";
        console.log(`\n[turn ${turnCount}: ${message.subtype}, cost: $${cost}]`);
      }
    }
    return turnCount;
  })();

  // Push messages with pauses to simulate interactive chat.
  const prompts = [
    "Remember the number 42. Just say OK.",
    "What number did I ask you to remember? Answer with just the number.",
    "Say goodbye in exactly 5 words.",
  ];

  for (let i = 0; i < prompts.length; i++) {
    // Wait a bit for previous response to complete.
    if (i > 0) await sleep(5000);

    console.log(`\n━━━ Sending prompt ${i + 1}/${prompts.length} ━━━`);
    console.log(`> ${prompts[i]}\n`);

    stream.push({ type: "user", content: prompts[i] });
  }

  // Wait for last response, then close.
  await sleep(10000);
  stream.done();

  const turns = await reader;
  console.log(`\n━━━ Streaming input spike complete (${turns} turns) ━━━`);
}

main().catch((err) => {
  console.error(`[error] ${err.message}`);
  process.exit(1);
});

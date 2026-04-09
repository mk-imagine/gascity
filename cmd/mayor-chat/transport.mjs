// Transport layer: V1 query() + resume for multi-turn conversation.
//
// Each turn spawns a new Claude Code subprocess via the Agent SDK.
// The session ID is captured from the first turn and passed to
// subsequent turns via --resume for conversation continuity.

import { query } from "@anthropic-ai/claude-agent-sdk";

export class MayorTransport {
  constructor(options = {}) {
    this.sessionId = null;
    // Start with minimal options that are known to work with OAuth.
    // The earlier agent.mjs spike validated this exact combination.
    this.options = {
      permissionMode: options.permissionMode ?? "dontAsk",
      allowedTools: options.allowedTools ?? [],
      ...options,
    };
  }

  // Send a prompt and yield messages as they stream back.
  // The caller iterates with: for await (const msg of transport.send(text))
  async *send(prompt) {
    const opts = { ...this.options };
    if (this.sessionId) {
      opts.resume = this.sessionId;
    }

    const q = query({ prompt, options: opts });

    for await (const message of q) {
      // Capture session ID from init or result.
      if (message.type === "system" && message.subtype === "init") {
        this.sessionId = message.session_id;
      }
      if (message.type === "result" && message.session_id) {
        this.sessionId = message.session_id;
      }

      yield message;
    }
  }
}

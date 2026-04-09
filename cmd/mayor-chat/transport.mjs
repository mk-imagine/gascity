// Transport layer: V2 persistent session for multi-turn conversation.
//
// Uses unstable_v2_createSession() to keep a single Claude Code process
// alive across all turns. send() + stream() per turn — no subprocess
// startup overhead after the first turn.

import { unstable_v2_createSession } from "@anthropic-ai/claude-agent-sdk";

export class MayorTransport {
  constructor(options = {}) {
    this.session = null;
    this.sessionId = null;
    this.options = {
      permissionMode: options.permissionMode ?? "dontAsk",
      allowedTools: options.allowedTools ?? [],
      includePartialMessages: true,
      ...options,
    };
  }

  // Lazily create the session on first send.
  _ensureSession() {
    if (!this.session) {
      this.session = unstable_v2_createSession(this.options);
      // sessionId is not available until after first stream() yields.
    }
  }

  // Send a prompt and yield messages as they stream back.
  async *send(prompt) {
    this._ensureSession();

    await this.session.send(prompt);

    for await (const message of this.session.stream()) {
      // Capture session ID from any message that has it.
      if (message.session_id) {
        this.sessionId = message.session_id;
      }

      yield message;
    }
  }

  close() {
    if (this.session) {
      this.session.close();
      this.session = null;
    }
  }
}

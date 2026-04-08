# Agent SDK spike — Claude Code in a container via the TypeScript SDK.
#
# No tmux, no ACP, no PTY. The Agent SDK spawns Claude Code as a
# subprocess and handles all JSON-RPC framing internally.
#
# Build:
#   docker build -t gc-acp-spike -f cmd/acp-spike/Dockerfile cmd/acp-spike/
#
# Run (OAuth credentials from host):
#   docker run --rm \
#     -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
#     -v ~/.claude/settings.json:/root/.claude/settings.json:ro \
#     -v ~/.claude.json:/root/.claude.json:ro \
#     gc-acp-spike

FROM node:20-slim

WORKDIR /app

# Install Claude Code globally (for direct CLI use in tests)
# and the Agent SDK as a project dependency.
RUN npm install -g @anthropic-ai/claude-code 2>/dev/null
COPY package.json .
RUN npm install --omit=dev 2>/dev/null

COPY agent.mjs .

# Run as root for now. The Agent SDK's Claude Code subprocess needs
# credentials at $HOME/.claude/, and non-root users hit credential
# discovery issues in containers. Use permissionMode "dontAsk" (not
# "bypassPermissions" which is blocked for root).

ENTRYPOINT ["node", "agent.mjs"]

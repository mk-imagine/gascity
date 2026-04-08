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
#     -v ~/.claude/.credentials.json:/home/node/.claude/.credentials.json:ro \
#     -v ~/.claude/settings.json:/home/node/.claude/settings.json:ro \
#     -v ~/.claude.json:/home/node/.claude.json:ro \
#     gc-acp-spike

FROM node:20-slim

WORKDIR /app

# Install the Agent SDK (includes bundled Claude Code CLI).
COPY package.json .
RUN npm install --omit=dev 2>/dev/null

COPY agent.mjs .

# node:20-slim ships with a 'node' user (UID 1000).
# Run as non-root so Claude Code's permission modes work correctly
# (bypassPermissions is blocked for root).
# Ensure .claude dir exists for session persistence.
RUN mkdir -p /home/node/.claude && chown node:node /home/node/.claude

USER node

ENTRYPOINT ["node", "agent.mjs"]

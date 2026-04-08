# Minimal Docker image for ACP (Agent Client Protocol) spike.
#
# No tmux — ACP uses JSON-RPC 2.0 over stdio, not terminal emulation.
# Claude Code detects non-TTY stdin and speaks ACP automatically.
#
# Build:
#   docker build -t gc-acp-spike -f cmd/acp-spike/Dockerfile .
#
# Manual test (one-shot, non-ACP):
#   docker run --rm \
#     -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
#     -v ~/.claude/settings.json:/root/.claude/settings.json:ro \
#     -v ~/.claude.json:/root/.claude.json:ro \
#     gc-acp-spike -p "Say hello"

FROM node:20-slim

# Install Claude Code globally. This is the only dependency.
RUN npm install -g @anthropic-ai/claude-code 2>/dev/null

# Claude Code speaks ACP when stdin is piped (no TTY).
ENTRYPOINT ["claude"]

# Mayor Chat — Ink terminal UI for containerized Claude Code.
#
# Build:
#   docker build -t mayor-chat -f contrib/mayor-chat/Dockerfile contrib/mayor-chat/
#
# Run via gc-session-docker (recommended):
#   Configure in city.toml with GC_DOCKER_HOME_MOUNT=true to provide
#   credentials via the standard Gas City HOME mount pattern.
#
# Run standalone:
#   docker run -d --name mayor-chat --entrypoint sleep mayor-chat infinity
#   docker exec -it mayor-chat node /app/chat.mjs
#
# Credentials: The Agent SDK discovers credentials from $HOME/.claude/
# at runtime. Use GC_DOCKER_HOME_MOUNT or your own volume mounts to
# provide them. Never bake credentials into the image.

FROM node:20-slim

WORKDIR /app

# Install Claude Code globally (used as subprocess by Agent SDK).
RUN npm install -g @anthropic-ai/claude-code 2>/dev/null

# Install project dependencies.
COPY package.json .
RUN npm install --omit=dev 2>/dev/null

# Copy app files.
COPY chat.mjs transport.mjs ./

ENTRYPOINT ["node", "chat.mjs"]

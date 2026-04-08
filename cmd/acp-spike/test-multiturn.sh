#!/usr/bin/env bash
# Test multi-turn stream-json via CLI only (no Agent SDK).
#
# Validates that `claude -p` with --resume can maintain conversation
# context across multiple invocations. This is the foundation for a
# native Go stream-json provider — if this works, Go can do the same
# without Node.js.
#
# Usage (inside a container with credentials mounted):
#   bash test-multiturn.sh
#
# Or from host:
#   docker run --rm \
#     -v ~/.claude/.credentials.json:/root/.claude/.credentials.json:ro \
#     -v ~/.claude/settings.json:/root/.claude/settings.json:ro \
#     -v ~/.claude.json:/root/.claude.json:ro \
#     -v $(pwd)/cmd/acp-spike/test-multiturn.sh:/test.sh:ro \
#     gc-acp-spike bash /test.sh

set -euo pipefail

PROMPTS=(
  "Remember the number 42. Just say OK."
  "What number did I ask you to remember? Answer with just the number."
  "Say goodbye in exactly 5 words."
)

SESSION_ID=""

for i in "${!PROMPTS[@]}"; do
  prompt="${PROMPTS[$i]}"
  n=$((i + 1))
  echo ""
  echo "━━━ Prompt $n/${#PROMPTS[@]} ━━━"
  echo "> $prompt"
  echo ""

  # Build the claude command.
  CMD=(claude -p "$prompt" --output-format json --verbose)
  if [ -n "$SESSION_ID" ]; then
    CMD+=(--resume "$SESSION_ID")
  fi

  # Run and capture output.
  OUTPUT=$("${CMD[@]}" 2>/dev/null)

  # Parse the result line (last JSON object with type=result).
  RESULT=$(echo "$OUTPUT" | grep '"type":"result"' | tail -1)

  if [ -z "$RESULT" ]; then
    echo "[error] No result message received"
    echo "Raw output:"
    echo "$OUTPUT"
    exit 1
  fi

  # Extract fields with basic pattern matching (no jq in slim image).
  # session_id
  SESSION_ID=$(echo "$RESULT" | sed 's/.*"session_id":"\([^"]*\)".*/\1/')

  # result text
  ANSWER=$(echo "$RESULT" | sed 's/.*"result":"\([^"]*\)".*/\1/')

  # cost
  COST=$(echo "$RESULT" | sed 's/.*"total_cost_usd":\([0-9.]*\).*/\1/')

  echo "Response: $ANSWER"
  echo "[session: ${SESSION_ID:0:8}..., cost: \$$COST]"
done

echo ""
echo "━━━ Multi-turn test complete ━━━"
echo "Session ID: $SESSION_ID"

#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
USER_ID="${USER_ID:-operator-user}"
CHAT_ID="${CHAT_ID:-local-chat}"
TOKEN="${LARK_VERIFICATION_TOKEN:-}"

post_message() {
  local id="$1"
  local text="$2"
  curl -sS -X POST "${BASE_URL}/lark/events" \
    -H 'Content-Type: application/json' \
    -d "{
      \"header\": {
        \"token\": \"${TOKEN}\",
        \"event_type\": \"im.message.receive_v1\"
      },
      \"event\": {
        \"sender\": {
          \"sender_id\": {
            \"user_id\": \"${USER_ID}\"
          }
        },
        \"message\": {
          \"message_id\": \"${id}\",
          \"chat_id\": \"${CHAT_ID}\",
          \"chat_type\": \"p2p\",
          \"message_type\": \"text\",
          \"content\": \"{\\\"text\\\":\\\"${text}\\\"}\"
        }
      }
    }"
  printf '\n'
}

post_message "local-msg-1-$(date +%s)" "查一下 0xabc"
#post_message "local-msg-2-$(date +%s)" "把他的昵称改成 abcd"
#post_message "local-msg-3-$(date +%s)" "确认"

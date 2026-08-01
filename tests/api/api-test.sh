#!/usr/bin/env bash
# API smoke test for a self-hosted httpSMS instance.
#
# Sends an SMS via the API, polls its status, and searches received SMS.
#
# Usage:
#   API_KEY=uk_... [FROM=+4915229268907] [TO=+4915...] [BASE_URL=https://httpsms.bigt.ai] ./api-test.sh

set -euo pipefail

API_KEY="${API_KEY:-}"
BASE_URL="${BASE_URL:-https://httpsms.bigt.ai}"
FROM="${FROM:-+4915229268907}"
TO="${TO:-}"
CONTENT="${CONTENT:-Hello from httpSMS API test $(date +%s)}"

if [ -z "$API_KEY" ]; then
  echo "API_KEY is required" >&2
  exit 1
fi
if [ -z "$TO" ]; then
  echo "TO (recipient number) is required" >&2
  exit 1
fi

echo "==> Sending SMS from $FROM to $TO"
SEND_HTTP_CODE=$(curl -s -o /tmp/opencode/api-send.json -w "%{http_code}" -X POST "$BASE_URL/v1/messages/send" \
  -H "x-api-key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"from\":\"$FROM\",\"to\":\"$TO\",\"content\":\"$CONTENT\"}")
echo "    HTTP $SEND_HTTP_CODE"
if [ "$SEND_HTTP_CODE" != "200" ]; then
  echo "FAIL: send request failed:" >&2
  cat /tmp/opencode/api-send.json >&2; echo >&2
  exit 1
fi
SEND_RESPONSE=$(cat /tmp/opencode/api-send.json)
echo "$SEND_RESPONSE"

MESSAGE_ID=$(echo "$SEND_RESPONSE" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
echo "==> Message ID: $MESSAGE_ID"

echo "==> Polling message status"
STATUS=""
for _ in $(seq 1 60); do
  STATUS=$(curl -sf "$BASE_URL/v1/messages/$MESSAGE_ID" -H "x-api-key: $API_KEY" \
    | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])")
  echo "    status: $STATUS"
  case "$STATUS" in
    delivered) break ;;
    failed|expired) break ;;
  esac
  sleep 5
done

case "$STATUS" in
  delivered) echo "OK: message delivered" ;;
  sent) echo "OK: message sent (delivery receipt pending)" ;;
  failed|expired) echo "FAIL: message ended in $STATUS" >&2; exit 1 ;;
  *) echo "FAIL: timed out waiting for status (last: $STATUS)" >&2; exit 1 ;;
esac

echo ""
echo "==> Searching received SMS (mobile-originated, latest 10)"
curl -sf "$BASE_URL/v1/messages/search?type=mobile-originated&limit=10&sort_by=created_at&sort_descending=true" \
  -H "x-api-key: $API_KEY" \
  | python3 -c "
import sys,json
data=json.load(sys.stdin)['data']
if not data:
    print('    (no received messages found)')
for m in data:
    print('    {} | {} | {} | {}'.format(m['created_at'], m['owner'], m['contact'], m['content'][:60]))
"

echo ""
echo "==> All received messages count"
curl -sf "$BASE_URL/v1/messages/search?type=mobile-originated&limit=1&sort_by=created_at&sort_descending=true" \
  -H "x-api-key: $API_KEY" > /dev/null \
  && echo "    search endpoint OK"

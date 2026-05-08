#!/usr/bin/env bash
# Smoke test the full pipeline against a running docker compose stack.
# Usage: ./demo/run.sh [SYMBOL] [INTERVAL]

set -euo pipefail

SYMBOL="${1:-BTCUSDT}"
INTERVAL="${2:-1h}"
ANALYST_URL="${ANALYST_URL:-http://localhost:8081}"

echo "Analyzing ${SYMBOL} @ ${INTERVAL} via ${ANALYST_URL}"

curl -fsS -X POST "${ANALYST_URL}/analyze" \
    -H "Content-Type: application/json" \
    -d "{\"symbol\":\"${SYMBOL}\",\"interval\":\"${INTERVAL}\"}" \
    | jq .

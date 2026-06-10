#!/usr/bin/env bash
# Drive mixed MCP traffic through the gateway so the Control Tower dashboards
# light up. Port-forwards the gateway broker locally unless GATEWAY_URL is set.
set -euo pipefail

cd "$(dirname "$0")"

GATEWAY_NS="${GATEWAY_NS:-mcp-system}"
GATEWAY_DEPLOY="${GATEWAY_DEPLOY:-mcp-gateway}"
GATEWAY_PORT="${GATEWAY_PORT:-8080}"
export GATEWAY_URL="${GATEWAY_URL:-http://localhost:${GATEWAY_PORT}/mcp}"
export WORKERS="${WORKERS:-4}"
export RPS="${RPS:-5}"
export DURATION_SEC="${DURATION_SEC:-0}"
export ERROR_PCT="${ERROR_PCT:-15}"

PF_PID=""
if [[ "${GATEWAY_URL}" == http://localhost:* ]]; then
  echo "Port-forwarding ${GATEWAY_DEPLOY}.${GATEWAY_NS} -> localhost:${GATEWAY_PORT}"
  kubectl port-forward -n "${GATEWAY_NS}" "deployment/${GATEWAY_DEPLOY}" "${GATEWAY_PORT}:${GATEWAY_PORT}" >/dev/null 2>&1 &
  PF_PID=$!
  trap '[[ -n "${PF_PID}" ]] && kill "${PF_PID}" 2>/dev/null || true' EXIT
  sleep 3
fi

echo "Driving traffic at ${GATEWAY_URL} (Ctrl-C to stop)"
go run .

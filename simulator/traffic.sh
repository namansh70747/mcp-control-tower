#!/usr/bin/env bash
# Drive mixed MCP traffic through the gateway so the Control Tower dashboards
# light up. Port-forwards the gateway broker locally unless GATEWAY_URL is set.
set -euo pipefail

cd "$(dirname "$0")"

# Default to the Envoy data path (kind host-port -> Istio gateway -> router -> broker),
# which is what the gateway's `make local-env-setup` exposes. This is important:
# the router only parses tools/call and emits the tool-call spans (gen_ai.tool.name,
# mcp.method.name) on THIS path. Hitting the broker pod directly bypasses the router.
GATEWAY_NS="${GATEWAY_NS:-mcp-system}"
GATEWAY_DEPLOY="${GATEWAY_DEPLOY:-mcp-gateway}"
GATEWAY_PORT="${GATEWAY_PORT:-8080}"
export GATEWAY_URL="${GATEWAY_URL:-http://mcp.127-0-0-1.sslip.io:8001/mcp}"
export WORKERS="${WORKERS:-4}"
export RPS="${RPS:-5}"
export DURATION_SEC="${DURATION_SEC:-0}"
export ERROR_PCT="${ERROR_PCT:-15}"

# Only port-forward when explicitly targeting the broker pod directly
# (GATEWAY_URL set to localhost). This bypasses Envoy/router — diagnostic use only.
PF_PID=""
if [[ "${GATEWAY_URL}" == http://localhost:* ]]; then
  echo "WARNING: targeting the broker pod directly — this bypasses Envoy/router and"
  echo "         will NOT produce tool-call spans. Use the default URL for full data."
  echo "Port-forwarding ${GATEWAY_DEPLOY}.${GATEWAY_NS} -> localhost:${GATEWAY_PORT}"
  kubectl port-forward -n "${GATEWAY_NS}" "deployment/${GATEWAY_DEPLOY}" "${GATEWAY_PORT}:${GATEWAY_PORT}" >/dev/null 2>&1 &
  PF_PID=$!
  trap '[[ -n "${PF_PID}" ]] && kill "${PF_PID}" 2>/dev/null || true' EXIT
  sleep 3
fi

echo "Driving traffic at ${GATEWAY_URL} (Ctrl-C to stop)"
go run .

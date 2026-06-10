#!/usr/bin/env bash
# One-shot narrated demo of MCP Control Tower.
# Assumes a kind cluster already running the MCP Gateway with servers registered.
set -euo pipefail
cd "$(dirname "$0")"

say() { printf '\n\033[1;36m▶ %s\033[0m\n' "$1"; }

say "1/4  Deploying the observability & governance stack and wiring the gateway"
make up

say "2/4  Port-forwarding Grafana (:3000) and Prometheus (:9090) in the background"
kubectl port-forward -n observability svc/grafana 3000:3000 >/dev/null 2>&1 &
GF_PID=$!
kubectl port-forward -n observability svc/prometheus 9090:9090 >/dev/null 2>&1 &
PROM_PID=$!
trap 'kill "$GF_PID" "$PROM_PID" 2>/dev/null || true' EXIT
sleep 3
echo "   Grafana:    http://localhost:3000  (dashboard: MCP Control Tower — Traffic & Governance)"
echo "   Prometheus: http://localhost:9090"

say "3/4  Driving mixed MCP traffic for ${DEMO_SECONDS:-90}s (success + deliberate errors)"
DURATION_SEC="${DEMO_SECONDS:-90}" WORKERS="${WORKERS:-6}" RPS="${RPS:-8}" ERROR_PCT="${ERROR_PCT:-20}" \
  ./simulator/traffic.sh || true

say "4/4  Traffic done. Explore the dashboard; drill a spike into its Tempo trace."
echo "   When finished:  make down"
echo "   (Grafana/Prometheus port-forwards stop when you exit this script.)"
read -r -p "Press Enter to stop port-forwards and exit..." _

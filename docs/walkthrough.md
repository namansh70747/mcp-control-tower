# Walkthrough

A 5-minute run from nothing to live dashboards.

## Prerequisites

- A kind cluster running the MCP Gateway with at least one MCP server registered. The fastest path
  is the gateway repo's own local setup:
  ```bash
  # in your Kuadrant/mcp-gateway clone
  make local-env-setup      # kind + gateway + test servers
  make reload               # build/load/restart
  ```
  Confirm tools are served (the gateway's test servers expose `hello_world`, `greet`, etc.).
- `kubectl`, `go` (1.23+), and a free :3000 / :9090 / :8080 locally.

## 1. Bring up Control Tower

```bash
cd mcp-control-tower
make up
```

This deploys Tempo, Loki, Prometheus, the enriching collector, and Grafana into the
`observability` namespace, loads the dashboards, and points the gateway's OTLP export at the
collector (`make wire-gateway`).

## 2. Open Grafana and Prometheus

```bash
make forward
```

- Grafana: http://localhost:3000 → dashboard **"MCP Control Tower — Traffic & Governance"**
  (folder *MCP Control Tower*).
- Prometheus: http://localhost:9090 → try `mcp_calls_total` (empty until traffic flows).

## 3. Drive traffic

```bash
make traffic        # or: WORKERS=6 RPS=8 ERROR_PCT=20 make traffic
```

The simulator connects with the real MCP client, lists tools, and issues a mix of successful and
deliberately failing `tools/call`s.

## 4. Watch it light up

Within ~15s (the metrics flush + scrape interval) the dashboard shows:

- **Tool calls/min**, **error rate**, **p95 latency**, **active servers** (top row).
- **Tool-call rate by tool** and **latency p50/p95/p99**.
- **Calls by server** and **errors by source**.
- **Per-tool RED summary** table.
- **Governance** logs panel: authorization / annotation-hint / elicitation activity from Loki.

Click any spike → drill into the Tempo trace to see the full router→broker→upstream span tree.

## 5. Tear down

```bash
make down
```

## Troubleshooting

- *No data*: confirm the gateway has OTLP export set — `kubectl describe deploy/mcp-gateway -n
  mcp-system | grep OTEL`. `make wire-gateway` sets it.
- *Different gateway deployment name/namespace*: override, e.g.
  `make up GATEWAY_DEPLOY=my-gw GATEWAY_NS=my-ns`.
- *`mcp_*` metrics missing in Prometheus but traces present in Tempo*: the spanmetrics connector
  needs spans with the expected attributes; verify the gateway is on a recent `main`.

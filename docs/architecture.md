# Architecture

MCP Control Tower adds no code to the gateway. It attaches to the gateway's existing OpenTelemetry
export and turns those signals into MCP-native views.

```
                         (existing gateway behavior)
 MCP client ──► Envoy ──► Router (ext_proc) ──► Broker ──► upstream MCP servers
                                │   spans + structured logs (OTLP)
                                ▼
                    ┌───────────────────────────┐
                    │   OTel Collector           │
                    │   • receives OTLP          │
                    │   • spanmetrics connector ─┼─► RED metrics per tool/server
                    │   • transform (status_class)│
                    └───────┬───────┬───────┬────┘
                       traces│   metrics│   logs│
                            ▼         ▼        ▼
                          Tempo   Prometheus  Loki
                            └────────┼────────┘
                                     ▼
                                  Grafana  ◄── MCP-native dashboards
                                     ▲
                            (Phase 2) Control Tower console
```

## The key idea: metrics from traces

The gateway emits a span per request with attributes the MCP world cares about —
`gen_ai.tool.name`, `mcp.server`, `mcp.method.name`, `http.status_code`, `error_source`. It does
**not** emit per-tool/per-server metrics today (the in-flight PR #1044 adds only infrastructure
counters). Rather than wait for new instruments, Control Tower runs the OpenTelemetry
**`spanmetrics` connector** in the collector: it derives rate/error/duration (RED) metrics directly
from those spans, with the MCP attributes as labels.

Result: `mcp_calls_total{mcp_server, gen_ai_tool_name, mcp_method_name, mcp_status_class, error_source}`
and `mcp_duration_milliseconds_bucket{...}` appear in Prometheus — on the gateway's current `main`,
with zero gateway changes. See [enrichment/otel-collector.yaml](../enrichment/otel-collector.yaml).

## Signals and where each panel comes from

| Question | Source | Mechanism |
|---|---|---|
| Tool-call rate / latency by server & tool | Prometheus | spanmetrics from spans |
| Error & 404 attribution (router/broker/upstream) | Prometheus | `error_source` + `mcp_status_class` labels |
| Authorization denials / annotation-hint activity | Loki | gateway structured logs (`x-mcp-authorized`, `x-mcp-annotation-hints`) |
| Elicitation funnel | Loki / Tempo | `elicitation/create` events vs tool-calls |
| Trace drill-down (one slow call) | Tempo | full router→broker span tree |

## Why a collector connector and not a Grafana-only solution

Putting the spanmetrics connector in the collector keeps tool/server cardinality in **metrics**
(cheap, long-retention) while leaving high-cardinality detail (session IDs, request IDs) in
**traces**, exactly as the gateway's own `docs/design/observability.md` recommends. Session IDs are
never used as metric labels.

## Compatibility

- Collector image: `otel/opentelemetry-collector-contrib:0.96.0` (matches the gateway's stack).
- If/when gateway PR #1044 lands, its pushed OTLP metrics flow through the same collector
  (`metrics/otlp` pipeline) and can power an additional dashboard — no rework here.

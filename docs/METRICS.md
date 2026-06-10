# Metrics catalog

Control Tower does not add instruments to the gateway. The OpenTelemetry Collector's
`spanmetrics` connector derives these metrics from the spans the gateway already emits
(see [enrichment/otel-collector.yaml](../enrichment/otel-collector.yaml)). All metrics are
namespaced `mcp_` and exposed to Prometheus.

## Metrics

| Metric | Type | Description |
|---|---|---|
| `mcp_calls_total` | counter | Count of gateway spans, one increment per span. Filter `mcp_method_name="tools/call"` for tool calls. |
| `mcp_duration_milliseconds_bucket` / `_sum` / `_count` | histogram | Span duration. Drives latency percentiles via `histogram_quantile`. |

## Labels (dimensions)

Each label is taken from a span attribute the gateway sets. A label is empty when the
underlying span does not carry that attribute.

| Label | Source span attribute | Notes |
|---|---|---|
| `mcp_server` | `mcp.server` | Set on broker / get-server-info spans. |
| `gen_ai_tool_name` | `gen_ai.tool.name` | The tool being called. |
| `mcp_method_name` | `mcp.method.name` | `initialize`, `tools/list`, `tools/call`, … |
| `mcp_status_class` | derived in collector | `2xx` / `4xx` / `5xx` / `error`, from `http.status_code` / span status. |
| `error_source` | `error_source` | `router` / `broker` / upstream — where a failure originated. |
| `span_name` | span name | e.g. `mcp-router.tool-call`, `mcp-broker.tools-list`. |

## Derived views (PromQL)

| View | Query |
|---|---|
| Tool calls / min | `sum(rate(mcp_calls_total{mcp_method_name="tools/call"}[1m])) * 60` |
| Error rate | `sum(rate(mcp_calls_total{mcp_status_class=~"4xx\|5xx\|error"}[5m])) / clamp_min(sum(rate(mcp_calls_total[5m])),1)` |
| p95 latency | `histogram_quantile(0.95, sum by (le)(rate(mcp_duration_milliseconds_bucket[5m])))` |
| Rate by tool | `sum by (gen_ai_tool_name)(rate(mcp_calls_total{mcp_method_name="tools/call"}[1m]))` |
| Errors by source | `sum by (error_source, mcp_status_class)(rate(mcp_calls_total{mcp_status_class=~"4xx\|5xx\|error"}[1m]))` |
| Distinct tools seen | `count(group by (gen_ai_tool_name)(mcp_calls_total{gen_ai_tool_name!=""}))` |

## Known limits (need gateway-side data, not derivable here)

These are deliberately **not** charted, because the gateway does not currently put the
required data on spans — deriving them would mean inventing numbers. They are tracked in
[coverage.md](coverage.md).

- **Per-user** metrics — no user/identity attribute on spans (only `mcp.session.id`, a connection).
- **Payload-size** metrics (tools/list size, tool response size) — no size attribute on spans.

# Integration pitch (for the Kuadrant/mcp-gateway maintainers)

## The problem this addresses

Two accepted epics ask for exactly this and are unbuilt:

- **#161** — metrics + default Grafana dashboards so admins can see traffic patterns and hotspots.
- **#25** — Auditing, Logging, and Metrics (umbrella). The `auditing-design` branch designs a
  caller-identity audit trail that was never implemented.

Today an operator can enable OTLP export and get raw spans/logs, but there is no MCP-native view:
no per-tool/per-server metrics, no default dashboards, no governance lens over authorization and
risky tool calls.

## What this prototype shows

A standalone layer that produces MCP-native dashboards **on the current `main`, with no gateway
code changes**, plus a governance view. The one non-obvious technique:

> Derive per-tool/per-server RED metrics from the spans the gateway already emits, using the
> OpenTelemetry `spanmetrics` connector — instead of adding new instruments.

This means the dashboards in #161 don't have to block on new metric code. They work now, and they
keep working (and gain detail) if/when PR #1044's instruments land.

## What you could adopt

Three independent, small pieces — any subset is useful upstream:

1. **The collector spanmetrics config** ([enrichment/otel-collector.yaml](../enrichment/otel-collector.yaml))
   as the recommended way to get tool/server metrics today → directly satisfies a chunk of #161.
2. **The dashboard JSON** ([dashboards/mcp-traffic.json](../dashboards/mcp-traffic.json)) as the
   "default Grafana dashboard" #161 asks for → could live under `config/grafana/dashboards/`.
3. **The governance row** (authorization + annotation-hint + elicitation activity) as a starting
   point for #25's audit story, complementary to the `auditing-design` branch.

## What this is not

- Not a replacement for the gateway's auth/authz — it observes, it does not enforce.
- Not affiliated with or endorsed by Kuadrant. A demonstration built on top of the project.
- Honest framing: a working prototype that maps to #161/#25, offered for feedback — not a claim to
  have solved everything.

## Suggested next step

If useful, the spanmetrics config + a dashboard JSON could be contributed upstream as a small,
focused PR against #161, behind the existing `make otel` stack. Happy to do that work.

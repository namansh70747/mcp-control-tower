# Coverage vs gateway issues #161 and #25

An honest map of what Control Tower addresses, what it partly addresses, and what it
cannot — so the scope is precise, not overstated. Verified against a live
`make local-env-setup` run.

Legend: ✅ done · 🟡 partial · ⬜ needs gateway-side data (an external tool can't produce it).

## #161 — Metrics: Journey 2 (default dashboards + traffic visibility)

| Ask | Status | How / why |
|---|---|---|
| Default Grafana dashboards | ✅ | [dashboards/mcp-traffic.json](../dashboards/mcp-traffic.json) |
| Admins see traffic patterns + hotspots | ✅ | rate / latency / errors by tool & server |
| Metrics implemented + documented | ✅ | derived via `spanmetrics`; catalog in [METRICS.md](METRICS.md) |
| Per-tool metrics | ✅ | `gen_ai_tool_name` dimension |
| Tool count per server / gateway | ✅ | "Tools seen" stat (distinct tools) |
| Per-**user** metrics (#162) | ⬜ | gateway puts no user identity on spans — only `mcp.session.id` |
| Tool output / **payload-size** metrics (recent ask) | ⬜ | no size attribute on spans |
| Istio Telemetry tag-override approach | n/a | took the OTel `spanmetrics` route instead |

## #25 — Auditing, Logging, and Metrics (umbrella)

| Sub-area | Status | How / why |
|---|---|---|
| Phase 1 logs (#429) | ✅ in gateway | Control Tower surfaces them (Loki) |
| Phase 2 tracing & errors (#428) | ✅ in gateway | Control Tower visualizes traces (Tempo) + error-source |
| Phase 3 metrics (#161) | 🟡 | the dashboard/metrics rows above |
| Auditing — governance activity view | 🟡 | a Loki row for `x-mcp-authorized` / annotation-hint / elicitation activity |
| Audit trail with **identity** (#710, "who did what") | ⬜ | needs the gateway to inject a verified identity into access logs/spans |

## What "claim correctly" means here

Defensible to say:

> Control Tower delivers the **dashboards + per-tool/server metrics + traffic-and-hotspot
> visibility** that #161 targets, on the gateway's current `main` with no gateway changes,
> and surfaces the tracing/logging already built under #25 as MCP-native views.

Not claimed: full completion of #161 (no per-user, no payload-size) or #25's identity audit
trail. Those require the gateway to emit the data — a gateway change, not an external layer.
That work is scoped in the project plan as a separate, gateway-side contribution.

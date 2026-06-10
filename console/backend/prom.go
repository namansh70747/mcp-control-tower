package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// clean replaces NaN/Inf (e.g. from histogram_quantile over empty buckets) with
// 0 — otherwise encoding/json fails and the whole response comes back empty.
func clean(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// promClient is a tiny Prometheus HTTP API client (stdlib only).
type promClient struct {
	base string
	http *http.Client
}

func newPromClient(base string) *promClient {
	return &promClient{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 5 * time.Second}}
}

type promResult struct {
	Metric map[string]string
	Value  float64
}

// query runs an instant PromQL query and returns the result vector.
func (p *promClient) query(ctx context.Context, q string) ([]promResult, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", p.base, url.QueryEscape(q))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var raw struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]any            `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw.Status != "success" {
		return nil, fmt.Errorf("prometheus status %q", raw.Status)
	}
	out := make([]promResult, 0, len(raw.Data.Result))
	for _, r := range raw.Data.Result {
		s, _ := r.Value[1].(string)
		f, _ := strconv.ParseFloat(s, 64)
		out = append(out, promResult{Metric: r.Metric, Value: f})
	}
	return out, nil
}

// scalar runs a query expected to return a single value (0 if empty/NaN).
func (p *promClient) scalar(ctx context.Context, q string) float64 {
	res, err := p.query(ctx, q)
	if err != nil || len(res) == 0 {
		return 0
	}
	return clean(res[0].Value)
}

// Model is the MCP-aware view the console serves.
type Model struct {
	GeneratedAt   time.Time `json:"generatedAt"`
	CallsPerMin   float64   `json:"callsPerMin"`
	ErrorRate     float64   `json:"errorRate"`
	P95ms         float64   `json:"p95ms"`
	ActiveServers int       `json:"activeServers"`
	Tools         []ToolRow `json:"tools"`
	Source        string    `json:"source"`
}

type ToolRow struct {
	Server     string  `json:"server"`
	Tool       string  `json:"tool"`
	RatePerMin float64 `json:"ratePerMin"`
	ErrorRate  float64 `json:"errorRate"`
	P95ms      float64 `json:"p95ms"`
	Risk       string  `json:"risk"` // ok | elevated | high
}

// buildModel assembles the MCP model from spanmetrics in Prometheus.
func (p *promClient) buildModel(ctx context.Context, highRisk map[string]bool) Model {
	m := Model{GeneratedAt: time.Now().UTC(), Source: p.base}
	m.CallsPerMin = p.scalar(ctx, `sum(rate(mcp_calls_total{mcp_method_name="tools/call"}[5m])) * 60`)
	m.ErrorRate = p.scalar(ctx, `sum(rate(mcp_calls_total{mcp_status_class=~"4xx|5xx|error"}[5m])) / clamp_min(sum(rate(mcp_calls_total[5m])),1)`)
	m.P95ms = p.scalar(ctx, `histogram_quantile(0.95, sum by (le) (rate(mcp_duration_milliseconds_bucket[5m])))`)
	m.ActiveServers = int(p.scalar(ctx, `count(count by (mcp_server)(mcp_calls_total))`))

	rate, _ := p.query(ctx, `sum by (mcp_server, gen_ai_tool_name) (rate(mcp_calls_total{mcp_method_name="tools/call"}[5m])) * 60`)
	errs, _ := p.query(ctx, `sum by (mcp_server, gen_ai_tool_name) (rate(mcp_calls_total{mcp_method_name="tools/call", mcp_status_class=~"4xx|5xx|error"}[5m])) * 60`)
	p95, _ := p.query(ctx, `histogram_quantile(0.95, sum by (le, mcp_server, gen_ai_tool_name) (rate(mcp_duration_milliseconds_bucket[5m])))`)

	type key struct{ server, tool string }
	rows := map[key]*ToolRow{}
	get := func(lbl map[string]string) *ToolRow {
		k := key{lbl["mcp_server"], lbl["gen_ai_tool_name"]}
		if k.tool == "" {
			return nil
		}
		if rows[k] == nil {
			rows[k] = &ToolRow{Server: k.server, Tool: k.tool}
		}
		return rows[k]
	}
	for _, r := range rate {
		if t := get(r.Metric); t != nil {
			t.RatePerMin = clean(r.Value)
		}
	}
	errByKey := map[key]float64{}
	for _, r := range errs {
		errByKey[key{r.Metric["mcp_server"], r.Metric["gen_ai_tool_name"]}] = clean(r.Value)
	}
	for _, r := range p95 {
		if t := get(r.Metric); t != nil {
			t.P95ms = clean(r.Value)
		}
	}
	for k, t := range rows {
		if t.RatePerMin > 0 {
			t.ErrorRate = clean(errByKey[k] / t.RatePerMin)
		}
		t.Risk = classifyRisk(t, highRisk)
		m.Tools = append(m.Tools, *t)
	}
	sort.Slice(m.Tools, func(i, j int) bool {
		if m.Tools[i].Risk != m.Tools[j].Risk {
			return riskRank(m.Tools[i].Risk) > riskRank(m.Tools[j].Risk)
		}
		return m.Tools[i].RatePerMin > m.Tools[j].RatePerMin
	})
	return m
}

// classifyRisk derives a health/risk signal from observed RED metrics, plus an
// optional operator-supplied high-risk tool allowlist (e.g. known destructive
// tools). Honest and fully metric-driven — it flags what is actually misbehaving.
func classifyRisk(t *ToolRow, highRisk map[string]bool) string {
	if highRisk[t.Tool] || highRisk[t.Server+"/"+t.Tool] {
		return "high"
	}
	switch {
	case t.ErrorRate > 0.2 || t.P95ms > 2000:
		return "high"
	case t.ErrorRate > 0.05 || t.P95ms > 500:
		return "elevated"
	default:
		return "ok"
	}
}

func riskRank(r string) int {
	switch r {
	case "high":
		return 2
	case "elevated":
		return 1
	default:
		return 0
	}
}

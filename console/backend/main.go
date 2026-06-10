// Command console is the MCP Control Tower backend: it queries Prometheus for
// the spanmetrics derived from the gateway's traces, builds an MCP-aware model
// (tools, servers, outcomes, risk), and serves a live web UI over JSON + SSE.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	var (
		addr        = ":" + env("PORT", "8080")
		promURL     = env("PROMETHEUS_URL", "http://localhost:9090")
		highRiskCSV = env("HIGH_RISK_TOOLS", "")
	)
	highRisk := map[string]bool{}
	for _, s := range strings.Split(highRiskCSV, ",") {
		if s = strings.TrimSpace(s); s != "" {
			highRisk[s] = true
		}
	}

	prom := newPromClient(promURL)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/model", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		writeJSON(w, prom.buildModel(ctx, highRisk))
	})

	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		send := func() {
			ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
			defer cancel()
			b, _ := json.Marshal(prom.buildModel(ctx, highRisk))
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(b)
			_, _ = w.Write([]byte("\n\n"))
			fl.Flush()
		}
		send()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				send()
			}
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Printf("MCP Control Tower console on %s (prometheus=%s)", addr, promURL)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

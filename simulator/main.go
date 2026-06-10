// Command simulator drives mixed MCP traffic through the gateway so the
// Control Tower dashboards light up: successful tool calls, deliberate errors,
// and multi-tool fan-out. It uses the same mcp-go streamable-HTTP client the
// gateway's own e2e tests use, so traffic is realistic.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	var (
		gatewayURL = env("GATEWAY_URL", "http://localhost:8080/mcp")
		workers    = envInt("WORKERS", 4)
		rps        = envInt("RPS", 5)          // requests/sec per worker
		duration   = envInt("DURATION_SEC", 0) // 0 = run until interrupted
		errorPct   = envInt("ERROR_PCT", 15)   // % of calls that intentionally error
	)

	log.Printf("MCP Control Tower simulator → %s | workers=%d rps=%d/worker error_pct=%d%%",
		gatewayURL, workers, rps, errorPct)

	ctx := context.Background()
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(duration)*time.Second)
		defer cancel()
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runWorker(ctx, id, gatewayURL, rps, errorPct)
		}(w)
	}
	wg.Wait()
	log.Println("simulator done")
}

func runWorker(ctx context.Context, id int, gatewayURL string, rps, errorPct int) {
	client, tools, err := connect(ctx, gatewayURL, id)
	if err != nil {
		log.Printf("worker %d: connect failed: %v", id, err)
		return
	}
	defer func() { _ = client.Close() }()
	if len(tools) == 0 {
		log.Printf("worker %d: no tools available — register MCP servers first (see README)", id)
		return
	}

	interval := time.Second / time.Duration(max(rps, 1))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
	calls := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("worker %d: %d calls issued", id, calls)
			return
		case <-ticker.C:
			tool := tools[rng.Intn(len(tools))]
			args := map[string]any{}
			// Inject a deliberate error on a fraction of calls (bogus tool name)
			// so the error/governance panels populate.
			name := tool.Name
			if rng.Intn(100) < errorPct {
				name = tool.Name + "__does_not_exist"
			}
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := client.CallTool(callCtx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{Name: name, Arguments: args},
			})
			cancel()
			calls++
			if err != nil {
				log.Printf("worker %d: call %q -> err: %v", id, name, err)
			}
		}
	}
}

func connect(ctx context.Context, gatewayURL string, id int) (*mcpclient.Client, []mcp.Tool, error) {
	options := []transport.StreamableHTTPCOption{
		transport.WithHTTPHeaders(map[string]string{"mcp-control-tower": "simulator"}),
		transport.WithContinuousListening(),
	}
	c, err := mcpclient.NewStreamableHttpClient(gatewayURL, options...)
	if err != nil {
		return nil, nil, err
	}
	if err := c.Start(ctx); err != nil {
		return nil, nil, err
	}
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo:      mcp.Implementation{Name: fmt.Sprintf("ct-sim-%d", id), Version: "0.1.0"},
		},
	}); err != nil {
		return nil, nil, err
	}
	list, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, nil, err
	}
	return c, list.Tools, nil
}

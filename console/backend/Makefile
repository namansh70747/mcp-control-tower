# Build/run the MCP Control Tower console backend.
PROMETHEUS_URL ?= http://localhost:9090
PORT           ?= 8080

.PHONY: run
run: ## Run the console locally (expects Prometheus reachable, e.g. via `make forward`)
	PROMETHEUS_URL=$(PROMETHEUS_URL) PORT=$(PORT) go run .

.PHONY: build
build: ## Build the console binary into ./bin
	CGO_ENABLED=0 go build -o bin/console .

.PHONY: image
image: ## Build the container image
	docker build -t mcp-control-tower-console:dev .

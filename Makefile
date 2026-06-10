# MCP Control Tower — standalone observability & governance layer for mcp-gateway.
#
# Assumes a kind cluster already running the MCP Gateway (e.g. via the gateway's
# `make local-env-setup && make reload`). This stack deploys into the
# `observability` namespace and points the gateway at its collector.

GATEWAY_NS        ?= mcp-system
GATEWAY_DEPLOY    ?= mcp-gateway
OBS_NS            ?= observability
OTEL_COLLECTOR    ?= http://otel-collector.$(OBS_NS).svc.cluster.local:4318
DASHBOARDS_CM     ?= mcp-control-tower-dashboards

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: up
up: dashboards-configmap ## Deploy the full stack and wire the gateway to it
	kubectl apply -f deploy/namespace.yaml
	kubectl apply -f deploy/tempo.yaml -f deploy/loki.yaml -f deploy/prometheus.yaml \
		-f enrichment/otel-collector.yaml -f deploy/grafana.yaml
	@echo "Waiting for the stack to become ready..."
	@kubectl wait --for=condition=Available deployment -n $(OBS_NS) --all --timeout=180s
	@$(MAKE) wire-gateway
	@echo ""
	@echo "Stack is up. Run 'make forward' (Grafana :3000, Prometheus :9090), then 'make traffic'."

.PHONY: dashboards-configmap
dashboards-configmap: ## (Re)create the Grafana dashboards ConfigMap from dashboards/
	kubectl create namespace $(OBS_NS) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create configmap $(DASHBOARDS_CM) -n $(OBS_NS) \
		--from-file=dashboards/ --dry-run=client -o yaml | kubectl apply -f -

.PHONY: dashboards
dashboards: dashboards-configmap ## Reload dashboards into a running Grafana
	kubectl rollout restart deployment/grafana -n $(OBS_NS)

.PHONY: wire-gateway
wire-gateway: ## Point the MCP Gateway at this stack's collector (OTLP export)
	kubectl set env deployment/$(GATEWAY_DEPLOY) -n $(GATEWAY_NS) \
		OTEL_EXPORTER_OTLP_ENDPOINT="$(OTEL_COLLECTOR)" OTEL_EXPORTER_OTLP_INSECURE="true"
	@kubectl rollout status deployment/$(GATEWAY_DEPLOY) -n $(GATEWAY_NS) --timeout=120s

.PHONY: unwire-gateway
unwire-gateway: ## Remove the OTLP export env from the gateway
	-kubectl set env deployment/$(GATEWAY_DEPLOY) -n $(GATEWAY_NS) \
		OTEL_EXPORTER_OTLP_ENDPOINT- OTEL_EXPORTER_OTLP_INSECURE-

.PHONY: down
down: unwire-gateway ## Tear down the stack
	-kubectl delete -f deploy/grafana.yaml -f enrichment/otel-collector.yaml \
		-f deploy/prometheus.yaml -f deploy/loki.yaml -f deploy/tempo.yaml --ignore-not-found
	-kubectl delete configmap $(DASHBOARDS_CM) -n $(OBS_NS) --ignore-not-found
	-kubectl delete -f deploy/namespace.yaml --ignore-not-found

.PHONY: status
status: ## Show stack pod status
	@kubectl get pods -n $(OBS_NS) 2>/dev/null || echo "Namespace '$(OBS_NS)' not found. Run 'make up'."

.PHONY: forward
forward: ## Port-forward Grafana (3000) and Prometheus (9090)
	@echo "Grafana:    http://localhost:3000  (dashboard: MCP Control Tower)"
	@echo "Prometheus: http://localhost:9090"
	@kubectl port-forward -n $(OBS_NS) svc/grafana 3000:3000 & \
	 kubectl port-forward -n $(OBS_NS) svc/prometheus 9090:9090 & \
	 wait

.PHONY: traffic
traffic: ## Drive mixed MCP traffic so the dashboards light up
	./simulator/traffic.sh

.PHONY: console
console: ## Phase 2: build and run the Control Tower console
	$(MAKE) -C console/backend run

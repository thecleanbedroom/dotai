.PHONY: dev build run test coverage vet clean install setup install-mcp

BINARY  := mcp-gemini-gateway
BUILD   := ./cmd/mcp-gemini-gateway
OUT     := ./$(BINARY)

# ── Development ──────────────────────────────────────────────

## Start dev server with hot-reload
dev:
	go run github.com/air-verse/air@latest

## Run the server directly
run: build
	$(OUT)

# ── Build ────────────────────────────────────────────────────

## Build the binary
build:
	go build -o $(OUT) $(BUILD)

## Install binary to GOPATH/bin
install:
	go install $(BUILD)

# ── Testing ──────────────────────────────────────────────────

## Run all tests
test:
	go test ./... -count=1

## Run tests with coverage report
coverage:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo "---"
	@go tool cover -func=coverage.out | tail -1

## Run tests with HTML coverage report
coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser"

# ── Quality ──────────────────────────────────────────────────

## Run go vet
vet:
	go vet ./...

## Run all checks (vet + test)
check: vet test

# ── Setup ────────────────────────────────────────────────────

## First-time setup: copy .env, tidy deps
setup:
	@test -f .env || cp .env.example .env && echo "Created .env from .env.example"
	go mod tidy
	@mkdir -p data
	@echo "Ready. Run 'make dev' to start."

MCP_CONFIG := $(HOME)/.gemini/antigravity/mcp_config.json
GW_BIN     := $(abspath $(OUT))

## Register gateway in Antigravity mcp_config.json
install-mcp: build
	@command -v jq >/dev/null 2>&1 || { echo "Error: jq is required. Install with: sudo apt install jq"; exit 1; }
	@mkdir -p $(dir $(MCP_CONFIG))
	@test -f $(MCP_CONFIG) || echo '{"mcpServers":{}}' > $(MCP_CONFIG)
	@if jq -e '.mcpServers["gemini-gateway"]' $(MCP_CONFIG) >/dev/null 2>&1; then \
		echo "gemini-gateway already registered in $(MCP_CONFIG)"; \
	else \
		jq '.mcpServers["gemini-gateway"] = {"command": "$(GW_BIN)", "args": []}' $(MCP_CONFIG) > $(MCP_CONFIG).tmp && \
		mv $(MCP_CONFIG).tmp $(MCP_CONFIG) && \
		echo "Registered gemini-gateway (stdio) in $(MCP_CONFIG)"; \
	fi

# ── Cleanup ──────────────────────────────────────────────────

## Remove build artifacts
clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf tmp/

# ── Help ─────────────────────────────────────────────────────

## Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:' $(MAKEFILE_LIST) | awk -F: '{print "  " $$1}' | sort

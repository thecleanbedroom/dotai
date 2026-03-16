.PHONY: build run test coverage vet clean install setup install-mcp

BINARY      := mcp-project-memory
BUILD       := ./cmd/mcp-project-memory
OUT         := ./$(BINARY)
MCP_CONFIG  := $(HOME)/.gemini/antigravity/mcp_config.json
BINARY_PATH := $(abspath $(OUT))

# ── Development ──────────────────────────────────────────────

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
	go test ./... -count=1 -coverprofile=coverage.out -covermode=count
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

# ── Build Pipeline ───────────────────────────────────────────

## Run the memory build pipeline (extract from git)
build-memories:
	$(OUT) build

## Run build pipeline with synthesis pass
build-synthesis:
	$(OUT) build --synthesis

## Reset the database (rebuild from JSON)
reset:
	$(OUT) reset

## List free models that meet minimum context requirements
models:
	$(OUT) models

# ── Setup ────────────────────────────────────────────────────

## First-time setup: copy .env, tidy deps
setup:
	@test -f .env || cp .env.example .env && echo "Created .env from .env.example"
	go mod tidy
	@mkdir -p data
	@echo "Ready. Run 'make run' to start."



## Register project-memory in Antigravity mcp_config.json (stdio mode)
install-mcp: build
	@command -v jq >/dev/null 2>&1 || { echo "Error: jq is required. Install with: sudo apt install jq"; exit 1; }
	@mkdir -p $(dir $(MCP_CONFIG))
	@test -f $(MCP_CONFIG) || echo '{"mcpServers":{}}' > $(MCP_CONFIG)
	@if jq -e '.mcpServers["project-memory"]' $(MCP_CONFIG) >/dev/null 2>&1; then \
		echo "project-memory already registered in $(MCP_CONFIG)"; \
	else \
		jq --arg bin "$(BINARY_PATH)" '.mcpServers["project-memory"] = {"command": $$bin, "args": []}' $(MCP_CONFIG) > $(MCP_CONFIG).tmp && \
		mv $(MCP_CONFIG).tmp $(MCP_CONFIG) && \
		echo "Added project-memory (command: $(BINARY_PATH)) to $(MCP_CONFIG)"; \
	fi


# ── Cleanup ──────────────────────────────────────────────────

## Remove build artifacts
clean:
	rm -f $(BINARY) coverage.out coverage.html

# ── Help ─────────────────────────────────────────────────────

## Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:' $(MAKEFILE_LIST) | awk -F: '{print "  " $$1}' | sort

.PHONY: run run-be be run-fe fe build test tidy vet mcp-echo docker-build docker-up docker-restart docker-down dockerize up restart down

NAME=buatpostingan
APP=./cmd/app
BIN=bin/buatpostingan
MCP_ECHO_BIN=bin/mcp-echo
WEB_DIR=web
FE_PORT?=5173

# Version from VERSION file; injectable via ldflags. Override with VERSION=foo.
VERSION?=$(shell cat VERSION 2>/dev/null | tr -d '[:space:]')
LDFLAGS?=-X buatpostingan/internal/version.Version=$(VERSION)

# --- backend ---

run: run-be

run-be be:
	go install github.com/cespare/reflex@latest
	reflex -s -r '\.go$$' go run $(APP)

build:
	mkdir -p bin
	go build -ldflags="$(LDFLAGS)" -o $(BIN) $(APP)

mcp-echo:
	mkdir -p bin
	go build -o $(MCP_ECHO_BIN) ./cmd/mcp-echo

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# --- frontend (static + livereload; default real → :8080 API) ---
# Needs Node.js (npx). No committed node_modules — live-server via npx --yes.

run-fe fe:
	@command -v npx >/dev/null 2>&1 || { \
		echo "make fe requires Node.js (npx). Install Node, or use make be + http://localhost:8080/ (no FE livereload)."; \
		exit 1; \
	}
	@echo "FE → http://localhost:$(FE_PORT)/  (live-reload; real default → :8080 API; needs make be)"
	@echo "Mock: ?mock=1  |  Go-served (no FE livereload): make be + http://localhost:8080/"
	cd $(WEB_DIR) && npx --yes live-server --port=$(FE_PORT) --no-browser

# --- docker ---

docker-build:
	docker compose -f compose.yml build

docker-up: docker-build
	docker compose -f compose.yml up -d

docker-restart:
	docker compose -f compose.yml restart

docker-down:
	docker compose -f compose.yml down --remove-orphans

# Backward-compatible aliases.
dockerize: docker-build
up: docker-up
restart: docker-restart
down: docker-down

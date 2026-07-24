.PHONY: run run-be be run-fe fe build test tidy vet dockerize up down

NAME=buatpostingan
APP=./cmd/app
BIN=bin/buatpostingan
NETWORK_NAME=app_network

WEB_DIR=web
FE_PORT?=5173

# --- backend ---

run: run-be

run-be be:
	go install github.com/cespare/reflex@latest
	reflex -s -r '\.go$$' go run $(APP)

build:
	mkdir -p bin
	go build -o $(BIN) $(APP)

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# --- frontend (static; default real → :8080 API) ---

run-fe fe:
	@echo "FE → http://localhost:$(FE_PORT)/  (real default → :8080 API; needs make be)"
	@echo "Mock: ?mock=1  |  Go-served: make be + http://localhost:8080/"
	cd $(WEB_DIR) && python3 -m http.server $(FE_PORT)

# --- docker ---

dockerize:
	@if ! docker network inspect $(NETWORK_NAME) >/dev/null 2>&1; then \
		docker network create --driver=overlay --attachable $(NETWORK_NAME); \
	fi
	docker build . -f deploy/Dockerfile -t $(NAME):dev

up: dockerize
	docker compose -f deploy/compose.yml up -d

down:
	docker compose -f deploy/compose.yml down --remove-orphans

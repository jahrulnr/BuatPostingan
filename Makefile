.PHONY: run build test tidy vet

NAME=buatpostingan
APP=./cmd/app
BIN=bin/buatpostingan
NETWORK_NAME=app_network

run:
	go install github.com/cespare/reflex@latest
	reflex -s -r '\.go$$' go run $(APP)

build:
	mkdir -p bin
	go build -o $(BIN) $(APP)

dockerize:
	@if ! docker network inspect $(NETWORK_NAME) >/dev/null 2>&1; then \
		docker network create --driver=overlay --attachable $(NETWORK_NAME); \
	fi
	docker build . -f deploy/Dockerfile -t $(NAME):dev

up: dockerize
	docker compose -f deploy/compose.yml up -d

down:
	docker compose -f deploy/compose.yml down --remove-orphans

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# AGENTS.md — BuatPostingan

Instructions for coding agents in this repo.

## Stack

- **Frontend:** vanilla JS ES modules under `web/` (dual-driver mock|real)
- **Backend:** Go 1.22+ Clean Architecture (light — no Wire/logmanager yet)
- **Persistence (planned):** JSONL under `storage/webchat/`
- **Knowledge (planned):** Markdown under `docs/webchat/`

## Foldering

Public / edge:

- `cmd/app` — binary entry + manual DI
- `delivery/` — HTTP/SSE adapters + presenter (bukan resep dapur)
- `web/` — static FE
- `docs/` — human docs
- `storage/` — runtime data

**Resep dapur** (private — under `internal/` only):

- `internal/domain`, `internal/application`, `internal/infrastructure`
- `internal/config`, `internal/pkg`

Kitchen import prefix: `buatpostingan/internal/...`  
Delivery import prefix: `buatpostingan/delivery/...`

## Dependency rule

```
delivery → application → domain ← infrastructure
```

Do **not** import `delivery` or `infrastructure` from `domain`.
Do **not** put business rules in HTTP handlers.
Do **not** put resep dapur (`domain` / `application` / `infrastructure`) at repo root — those live under `internal/`. `delivery/` stays at root.

## FE dual-driver

UI (`web/js/ui/*`) must not call `fetch` / `EventSource` directly.
Add endpoints in `web/js/api/mock` + `web/js/api/real` + bind in `web/js/api/index.js`.

Default `mockMode=true` until Go real backend is ready.

## Webchat product locks

- Reader/instructor only — `write_enabled` stays false
- No mutation tools in the LLM tools array
- Turn loop in worker, not HTTP request
- SSE mirrors durable JSONL seq

## Make targets

| Target | Purpose |
|---|---|
| `make run` | `go run ./cmd/app` |
| `make build` | binary → `bin/buatpostingan` |
| `make test` | `go test ./...` |
| `make tidy` | `go mod tidy` |

# AGENTS.md — BuatPostingan

Instructions for coding agents in this repo.

## Stack

- **Frontend:** vanilla JS ES modules under `web/` (dual-driver mock|real)
- **Backend:** Go 1.22+ Clean Architecture (light — no Wire/logmanager yet)
- **Persistence:** JSONL under `storage/webchat/` (`BP_STORAGE_ROOT`)
- **Knowledge:** Markdown under `docs/webchat/` + lexical index in storage
- **Prompts / tools:** `resources/webchat/prompts/`, `resources/webchat/tools/`

## Foldering

Public / edge:

- `cmd/app` — binary entry + manual DI
- `delivery/` — HTTP/SSE adapters + presenter (bukan resep dapur)
- `web/` — static FE
- `docs/` — human docs
- `storage/` — runtime data
- `resources/webchat/` — prompts + tool JSON schemas

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
Do **not** put business rules in HTTP handlers — handlers call `webchat.Usecase` interface only.
Do **not** put resep dapur (`domain` / `application` / `infrastructure`) at repo root — those live under `internal/`. `delivery/` stays at root.
Concrete usecase: `webchat.NewService(Deps{...})` orchestrates ports (AIPedia controller order).
Port adapters stay under `internal/infrastructure` (jsonl / docs / tools / llm / worker / sse).
Handlers call `webchat.Usecase` interface only.

## FE dual-driver

UI (`web/js/ui/*`) must not call `fetch` / `EventSource` directly.
Add endpoints in `web/js/api/mock` + `web/js/api/real` + bind in `web/js/api/index.js`.

Default is **real** (`mockMode=false`) — hits Go BE. Mock via `?mock=1`. Preferred: `make be` + http://localhost:8080/ ; or `make fe` + http://localhost:5173/ (API base auto → `:8080`; override with `?api=`).

## Portable AI kit

Reuse over rewrite: treat kitchen + webchat delivery as a **copyable kit**, not a shared Go module yet.

- **Copy list / consumer checklist:** [`docs/architecture/portable-ai-kit.md`](docs/architecture/portable-ai-kit.md)
- **AI core:** `internal/{domain,application,infrastructure,config,pkg}` + `delivery/http` webchat handler + `delivery/presenter` + `resources/webchat`
- **Leave as product shell:** `web/index.html`, `web/css/shell.css`, `web/js/app.js`, `cmd/app`, `docs/webchat` content, Makefile
- **FE widget slice:** `web/js/api/*` + `web/js/ui/chat.js|render.js` + `web/css/webchat*.css` — mount with `bootChat({ root })`
- **Config:** prefer portable `WEBCHAT_*` env names; `BP_*` is this product’s alias (both work via `config.Load`)
- **HTTP:** `MountWebchatAPI(mux, uc)` — do not couple consumers to static FE serving
- Do **not** rename the whole module to a shared package unless a second in-tree consumer needs it

## Webchat product locks

- Reader/instructor only — `write_enabled` stays false
- No mutation tools in the LLM tools array
- Turn loop in worker, not HTTP request
- SSE mirrors durable JSONL seq

## Ready-to-use LLM

- Default `BP_LLM_STUB=true` when no provider API key is set → canned `agent_message` without HTTP.
- Real LLM: set provider key (e.g. `BP_LLM_OPENROUTER_API_KEY`) and `BP_LLM_STUB=false` (or omit stub once a key exists).
- `BP_LLM_*_API=responses` (preferred, AIPedia-aligned) or `chat`. Client uses `stream=true` and parses proxy SSE (`text/event-stream`); JSON non-stream remains a fallback.

## Make targets

| Target | Purpose |
|---|---|
| `make fe` / `make run-fe` | Static FE di `web/` → `:5173` (`FE_PORT=` untuk ganti) |
| `make be` / `make run-be` / `make run` | Go app + reflex reload |
| `make build` | binary → `bin/buatpostingan` |
| `make test` | `go test ./...` |
| `make tidy` | `go mod tidy` |

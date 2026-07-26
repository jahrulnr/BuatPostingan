# AGENTS.md — BuatPostingan

Instructions for coding agents in this repo.

## Stack

- **Frontend:** vanilla JS ES modules under `web/` (dual-driver mock|real)
- **Backend:** Go 1.22+ Clean Architecture (light — no Wire/logmanager yet)
- **Persistence:** JSONL under `storage/webchat/` (`BP_STORAGE_ROOT`)
- **Static pages:** draft source under `storage/pages/<page-id>/`; publication is only `.published/<page-id> -> ../<page-id>`
- **Knowledge:** Markdown under `resources/webchat/docs/` + lexical index in storage
- **Prompts / tools / skills:** `resources/webchat/prompts/`, `resources/webchat/tools/`, `resources/webchat/skills/`

## Foldering

Public / edge:

- `cmd/app` — binary entry + manual DI
- `delivery/` — HTTP/SSE adapters + presenter (bukan resep dapur)
- `web/` — static FE
- `docs/` — human docs
- `storage/` — runtime data
- `resources/webchat/` — prompts + tool JSON schemas

**Resep dapur** (private — under `internal/` only):

- `internal/domain`, `internal/usecase`, `internal/infrastructure`
- `internal/config`, `internal/pkg`

Kitchen import prefix: `buatpostingan/internal/...`  
Delivery import prefix: `buatpostingan/delivery/...`

## Dependency rule

```
delivery → usecase → domain ← infrastructure
```

Do **not** import `delivery` or `infrastructure` from `domain`.
Do **not** put business rules in HTTP handlers — handlers call `webchat.Usecase` interface only.
Do **not** put resep dapur (`domain` / `usecase` / `infrastructure`) at repo root — those live under `internal/`. `delivery/` stays at root.
Concrete usecase: `webchat.NewService(Deps{...})` in `internal/usecase/webchat` orchestrates ports (AIPedia controller order).
Port adapters stay under `internal/infrastructure` (jsonl / docs / tools / llm / worker / sse).
Handlers call `webchat.Usecase` interface only.

## FE dual-driver

UI (`web/js/ui/*`) must not call `fetch` / `EventSource` directly.
Add endpoints in `web/js/api/mock` + `web/js/api/real` + bind in `web/js/api/index.js`.

Default is **real** (`mockMode=false`) — hits Go BE. Mock via `?mock=1`. Preferred: `make be` + http://localhost:8080/ ; or `make fe` + http://localhost:5173/ (API base auto → `:8080`; override with `?api=`).

## Docs map

Human + agent docs (grounded in current code): [`docs/README.md`](docs/README.md) — architecture, [turn loop](docs/architecture/turn-loop.md), [LLM providers](docs/architecture/llm-providers.md), [runbook](docs/operations/runbook.md), portable kit.

## Portable AI kit

Reuse over rewrite: treat kitchen + webchat delivery as a **copyable kit**, not a shared Go module yet.

- **Copy list / consumer checklist:** [`docs/architecture/portable-ai-kit.md`](docs/architecture/portable-ai-kit.md)
- **AI core:** `internal/{domain,usecase,infrastructure,config,pkg}` + `delivery/http` webchat handler + `delivery/presenter` + `resources/webchat`
- **Leave as product shell:** `web/index.html`, `web/css/shell.css`, `web/js/app.js`, `cmd/app`, `resources/webchat/docs` content, Makefile
- **FE widget slice:** `web/js/api/*` + `web/js/ui/chat.js|render.js` + `web/css/webchat*.css` — mount with `bootChat({ root })`
- **Config:** `BP_*` env only in this repo; when copying the kit, rename the env prefix to your product (or keep `BP_`)
- **HTTP:** `MountWebchatAPI(mux, uc)` — do not couple consumers to static FE serving
- Do **not** rename the whole module to a shared package unless a second in-tree consumer needs it

## Webchat product locks

- **MCP:** configure `mcp.servers` in `storage/config.json` (stdio MVP); agent uses progressive meta-tools — mutations default-denied. See [`docs/architecture/mcp-support.md`](docs/architecture/mcp-support.md).
- Turn loop in worker, not HTTP request
- SSE mirrors durable JSONL seq; ephemeral `item.delta` (hub) streams agent text without JSONL/seq — see [`docs/architecture/realtime-streaming.md`](docs/architecture/realtime-streaming.md)
- Uploads under `storage/webchat/attachments/{threadId}/` — attachment tools keyed by `attachment_id` for the active thread
- **Skills:** `resources/webchat/skills/<name>/SKILL.md` (`BP_SKILLS_ROOT`); discover via `list_skills`, load via `read_skill` — jailed to skills root (unlike local-dev FS tools). See [`docs/architecture/skills-tools.md`](docs/architecture/skills-tools.md).
- **MCP:** `mcp.servers` in `storage/config.json`; discover via `list_mcp_tools`, invoke via `call_mcp_tool` — mutations default-denied. See [`docs/architecture/mcp-support.md`](docs/architecture/mcp-support.md).
- **Static pages:** tools `page_list`, `page_search`, `page_create`, `page_edit`, `page_read`, `page_publish`, and `page_unpublish` own page authoring and lifecycle. `GET /api/pages/<page-id>/…` serves the jailed no-cache draft for the FE preview iframe. Publish/unpublish must use their dedicated tools, so draft versus published differs only by `.published/<page-id>` symlink. `page_snapshot` remains a documented future requirement until a local renderer is installed. See [`docs/architecture/static-pages.md`](docs/architecture/static-pages.md).
- **Authentication:** chat/settings/page APIs require a SQLite-backed `bp_session` cookie when the app is wired with auth. JSONL remains the source of truth for conversations. Bootstrap the first admin with `BP_AUTH_ADMIN_USERNAME` + `BP_AUTH_ADMIN_PASSWORD`; never commit the password. See [`docs/operations/runbook.md`](docs/operations/runbook.md).

## Ready-to-use LLM

- Real LLM: configure `llm.providers[]` in `storage/config.json` (auto-generated from struct defaults on first boot) and set `BP_LLM_STUB=false`.
- **Source of truth:** `storage/config.json` (gitignored; auto-generated on first boot) holds limits, `llm.*` globals (strategy/providers/stream/vision/effort/retry backoff), `context.*`, `docs.*`, `web_search.*`, and `mcp.*`. When a JSON key is omitted, the hardcoded default in `config.Load()` wins. Env-only: `BP_HTTP_ADDR`, `BP_WEB_ROOT`, `BP_STORAGE_ROOT`, `BP_DOCS_ROOT`, `BP_PROMPTS_ROOT`, `BP_TOOLS_ROOT`, `BP_SKILLS_ROOT`, `BP_WORKSPACE_ROOT`, `BP_CONFIG_PATH`, `BP_LLM_RETRY_STATUSES`, `BP_LLM_STUB`. See [`docs/architecture/settings-config.md`](docs/architecture/settings-config.md).
- **Source of truth:** `storage/config.json` (gitignored; auto-generated on first boot) holds limits, `llm.*` globals (strategy/providers/stream/vision/effort/retry backoff), `context.*`, `docs.*`, `web_search.*`, and `mcp.*`. Auth bootstrap/session settings are process env (`BP_AUTH_DB_PATH`, `BP_AUTH_ADMIN_USERNAME`, `BP_AUTH_ADMIN_PASSWORD`, `BP_AUTH_SESSION_TTL_HOURS`, `BP_CORS_ORIGIN`). When a JSON key is omitted, the hardcoded default in `config.Load()` wins. Env-only: `BP_HTTP_ADDR`, `BP_WEB_ROOT`, `BP_STORAGE_ROOT`, `BP_DOCS_ROOT`, `BP_PROMPTS_ROOT`, `BP_TOOLS_ROOT`, `BP_SKILLS_ROOT`, `BP_WORKSPACE_ROOT`, `BP_CONFIG_PATH`, `BP_LLM_RETRY_STATUSES`, `BP_LLM_STUB`. See [`docs/architecture/settings-config.md`](docs/architecture/settings-config.md).
- `llm.providers[].api=responses` (preferred, AIPedia-aligned) or `chat`. Default `llm.stream=true`: client sends `stream=true` and parses proxy SSE (`text/event-stream`); JSON non-stream remains a path. Set `llm.stream=false` to force JSON. If streaming is rejected as unsupported, client retries once with `stream=false`.
- `llm.vision=auto|on|off` (default `auto`): gate multimodal image parts — see [`docs/architecture/llm-vision.md`](docs/architecture/llm-vision.md).
- `llm.effort=auto|none|minimal|low|medium|high|xhigh|max` (default `auto`): reasoning effort when the model supports it — see [`docs/architecture/llm-effort.md`](docs/architecture/llm-effort.md).
- Model picker (composer): `GET /api/webchat/models` + optional StartTurn `model`/`effort` — see [`docs/architecture/llm-model-picker.md`](docs/architecture/llm-model-picker.md).
- Settings UI + JSON config: `storage/config.json` (gitignored; auto-generated on first boot) is the source of truth — [`docs/architecture/settings-config.md`](docs/architecture/settings-config.md). API under `/api/settings`.

## Make targets

| Target | Purpose |
|---|---|
| `make fe` / `make run-fe` | Static FE + livereload (`npx live-server`) → `:5173` (`FE_PORT=`; needs Node) |
| `make be` / `make run-be` / `make run` | Go app + reflex reload |
| `make build` | binary → `bin/buatpostingan` |
| `make test` | `go test ./...` |
| `make tidy` | `go mod tidy` |

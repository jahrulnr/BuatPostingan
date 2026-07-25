# Architecture — BuatPostingan (Go)

Clean Architecture **ringan** (tanpa Wire / logmanager / salt-pkg). Dependency rule:

```
delivery → usecase → domain ← infrastructure
```

**Resep dapur** ada di **`internal/`**. **Delivery** (ruang depan) di root — adapter HTTP/SSE, bukan business rules.

**Portability:** AI core is **partially** copy-paste ready — see [`portable-ai-kit.md`](portable-ai-kit.md) for exact dirs to copy vs product shell to leave. HTTP: `MountWebchatAPI` does not require static `web/`.

Deep dives:

- [`turn-loop.md`](turn-loop.md) — StartTurn → worker → tools → JSONL → SSE  
- [`realtime-streaming.md`](realtime-streaming.md) — token deltas, SSE notify, perceived-latency vs industry  
- [`llm-providers.md`](llm-providers.md) — stub, OpenAI-compatible `chat`/`responses`, router  
- [`llm-model-picker.md`](llm-model-picker.md) — composer model/effort picker + `/models`  
- [`settings-config.md`](settings-config.md) — JSON settings UI + `/api/settings`  
- [`observability.md`](observability.md) — `trace_id` + slog boundaries  
- [`codex-gap-analysis.md`](codex-gap-analysis.md) — Codex vs BP feature gaps (P0–P2) for reader/instructor  
- [`../operations/runbook.md`](../operations/runbook.md) — local run  

## Layout

```
cmd/app/                         entrypoint + manual DI
delivery/
  http/                          net/http handlers (thin)
  presenter/                     JSON / SSE DTO helpers
internal/                        resep dapur (private)
  config/                        env loader (BP_* only)
  usecase/webchat/               orchestration
  domain/
    entity/ valueobject/ enum/
    repository/                  ThreadStore, ThreadLock, InterruptFlag
    service/                     SpeakFloor, DocsIndex, LLM*, TurnWorker, EventStreamer
  infrastructure/
    repository/jsonl/            durable JSONL + lock + interrupt + floor
    ratelimit/                   turn rate limiter
    service/docs|tools|llm       RAG index, host tools, LLM client/router
    worker/                      in-process agent loop
    sse/                         EventStreamer (poll JSONL → SSE events)
    stub/                        501 adapters (tests / fallback)
  pkg/apperr|idgen|redact|logging
web/                             vanilla FE (dual-driver mock|real)
resources/webchat/               prompts + *.tool.json
storage/webchat/                 runtime JSONL + docs_index + interrupt/rl/llm
docs/webchat/                    knowledge MD corpus
```

## Dependency rule (detail)

| May import | Must not |
|---|---|
| `delivery` → `usecase` → `domain` | `domain` → `delivery` or `infrastructure` |
| `infrastructure` → `domain` (implements ports) | Business rules inside HTTP handlers |
| Kitchen under `internal/` only | Resep dapur packages at repo root |

Handlers call `webchat.Usecase` only. Concrete service: `webchat.NewService(Deps{…})` in `internal/usecase/webchat`. Port adapters live under `internal/infrastructure`.

## Ports ↔ FE real driver

| FE export | HTTP | Usecase |
|---|---|---|
| `listConversations` | `GET /api/webchat/conversations` | `ListConversations` |
| `listModels` | `GET /api/webchat/models` | `ListModels` |
| `createThread` | `POST /api/webchat/threads` | `CreateThread` |
| `getThread` | `GET /api/webchat/threads/{id}` | `GetThread` |
| `renameThread` | `PATCH /api/webchat/threads/{id}` | `RenameThread` |
| `startTurn` | `POST …/turns` | `StartTurn` → `TurnWorker.Enqueue` |
| `retryTurn` | `POST …/retry` | `RetryTurn` |
| `interruptTurn` | `POST …/interrupt` | `InterruptTurn` |
| `subscribeEvents` | `GET …/events` SSE | `EventStreamer.Subscribe` |

## Non-negotiables (ported from AIPedia / contracts)

1. Chat history = **JSONL** under `BP_STORAGE_ROOT` — no chat tables.  
2. Agent loop runs in **TurnWorker**, not inside the HTTP handler.  
3. SSE tails durable seq — not LLM token streaming (keepalive ping in HTTP adapter).  
4. `write_enabled = true` (dev phase) — mutation tools on; set `BP_WRITE_ENABLED=false` to relock.  
5. Soft tool failures return envelopes; do not crash the turn.  

## Runtime wiring (`cmd/app`)

1. Ensure storage dirs; `docs.NewIndex` (+ Reindex) + Gate log  
2. `tools.NewRegistry` (allowlist: search_docs, list_dir, read_file, grep, read_attachment, read_image, web_search, web_fetch, list_skills, read_skill, list_mcp_tools, call_mcp_tool; plus write_file / edit_file / delete_file when `write_enabled=true`) + optional `mcp.NewManager`  
3. `llm.NewClient` + `llm.NewRouter` (failover / circuit)  
4. `worker.New` + `sse.NewStreamer`  
5. `webchat.NewService(Deps{…})` → HTTP server  

**LLM stub:** when `BP_LLM_STUB=true` (default if no API key), worker appends canned `agent_message` + `turn.completed` without calling providers. See [`llm-providers.md`](llm-providers.md).

## Tools

Allowlist (hardcoded in `tools.Allowlist`; schemas from disk):

| Tool | Role |
|---|---|
| `search_docs` | Lexical RAG over indexed Markdown (`BP_DOCS_ROOT`) |
| `list_dir` | List host directories (`data.listing` ls-style incl. `.` / `..`) |
| `read_file` | Read host files (absolute or relative) |
| `grep` | Regex search; prefers host `rg` (`--json`, no shell); Go RE2 fallback |
| `read_attachment` | Read text uploads for the current thread (`attachment_id`) |
| `read_image` | Image upload metadata; pixels auto-injected into multimodal LLM user messages when under vision size limits |
| `web_search` | Public metasearch via [searchwire](https://github.com/jahrulnr/searchwire) (Brave/Startpage/Wikipedia/GitHub); query string only |
| `web_fetch` | SSRF-safe HTTP GET of public http(s) URLs → title + truncated text |
| `list_skills` | Catalog of project skills (`name` + `description` only) under `BP_SKILLS_ROOT` |
| `read_skill` | Full `SKILL.md` body for one skill by kebab-case name (skills-root jail) |
| `list_mcp_tools` | Catalog tools from configured MCP servers (progressive; see [mcp-support.md](mcp-support.md)) |
| `call_mcp_tool` | Invoke one MCP tool; mutations default-denied |

- Schemas: `resources/webchat/tools/{name}.tool.json` (`BP_TOOLS_ROOT`)  
- Skills: `resources/webchat/skills/<name>/SKILL.md` (`BP_SKILLS_ROOT`) — see [skills-tools.md](skills-tools.md)  
- **Local-dev FS:** `list_dir` / `read_file` / `grep` use the real host filesystem (absolute paths including `/` allowed; optional `Options.FSRoot` is a relative-path base for tests only — not a jail). **Insecure for multi-tenant production.**  
- **Skills jail:** `list_skills` / `read_skill` resolve only under `BP_SKILLS_ROOT` (even in local-dev)  
- Docs corpus for `search_docs` = `BP_DOCS_ROOT` (default `docs/webchat`) — indexing only; does not jail other FS tools  
- Attachments = `{BP_STORAGE_ROOT}/attachments/{threadId}/` — tools require worker thread context + `attachment_id`  
- `web_fetch` blocks localhost/private/link-local/metadata IPs; max body 2 MiB; text/html(+text) only  
- Optional `BP_GITHUB_TOKEN` (or `GITHUB_TOKEN`) raises GitHub search rate limits for `web_search` — not required for default path  
- No mutation / admin-route tools  
- Multi `tool_calls` in one LLM response: **sequential** Execute; all results feed the next round

## Persistence (JSONL)

Under `BP_STORAGE_ROOT` (default `storage/webchat`):

```
session_index.jsonl
threads/{id}.jsonl      # append-only transcript items + seq
threads/{id}.seq
threads/{id}.lock
interrupt/{thread}/{turn}.flag
attachments/{threadId}/{attId}.meta.json
attachments/{threadId}/{attId}.data
```

- **seq** — monotonic per thread; SSE cursor = last seen seq  
- **session index** — conversation sidebar meta (title, active turn, floor holder, …)  
- **lock** — single active turn worker per thread  
- **interrupt flags** — cooperative stop between LLM rounds  
- **attachments** — multipart uploads; `POST /api/webchat/threads/{id}/attachments`; StartTurn accepts `attachment_ids`  
- Speak-floor + rate-limit state also under storage (see jsonl / ratelimit packages)  

## Frontend

- Dual-driver: `web/js/api/mock` + `web/js/api/real`, bound in `web/js/api/index.js`  
- Default **`mockMode=false`** (real BE); mock via `?mock=1`  
- UI (`web/js/ui/*`) must not call `fetch` / `EventSource` — only api exports  
- Mount: `bootChat({ root })` (full page or widget host with required DOM ids)  
- Bubbles: **thinking / tools / message** are separate articles — never merge reasoning + tool + agent_message into one turn card (`ensureActionBubble` by kind)  
- `make fe` on `:5173` (npx live-server, auto-refresh) auto-targets Go API on `:8080` (override `?api=`); Go-served `:8080` has no FE livereload  

## AI core vs product shell

| Layer | Portable AI kit | Product shell (this repo) |
|---|---|---|
| Domain + usecase + infra adapters | ✅ copy | — |
| `delivery` webchat handler + presenter | ✅ copy | — |
| `resources/webchat` prompts/tools | ✅ copy (edit voice) | — |
| `cmd/app` DI + listen | — | ✅ keep / rewrite |
| `web/` chrome (`shell.css`, `index.html`) | — | ✅ keep |
| `web/js/api` + `ui/chat\|render` | ✅ widget kit | branding in shell |
| `docs/webchat` knowledge MD | — | ✅ swap per product |

Full copy checklist: [`portable-ai-kit.md`](portable-ai-kit.md).

## Status

1. ✅ Delivery HTTP + usecase orchestration  
2. ✅ JSONL store / lock / interrupt / speak-floor / rate-limit / redact  
3. ✅ DocsIndex + ToolRegistry  
4. ✅ TurnWorker + LLM router + EventStreamer wired in `cmd/app`  
5. FE default is **real** (`mockMode=false`); mock via `?mock=1`  
6. ✅ Portable kit boundary documented; API mount separable from static FE  

## Compile & run

```bash
make be
# GET /healthz → {"ok":true}
# static FE at http://localhost:8080/ (real by default)
# or make fe + http://localhost:5173/ (real driver → :8080 API)
# mock: ?mock=1
```

Full env matrix and stub vs real: [`../operations/runbook.md`](../operations/runbook.md).

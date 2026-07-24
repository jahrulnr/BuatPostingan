# Architecture — BuatPostingan (Go)

Clean Architecture **ringan** (tanpa Wire / logmanager / salt-pkg). Dependency rule:

```
delivery → usecase → domain ← infrastructure
```

**Resep dapur** ada di **`internal/`**. **Delivery** (ruang depan) di root — adapter HTTP/SSE, bukan business rules.

**Portability:** AI core is **partially** copy-paste ready — see [`portable-ai-kit.md`](portable-ai-kit.md) for exact dirs to copy vs product shell to leave. HTTP: `MountWebchatAPI` does not require static `web/`.

## Layout

```
cmd/app/                         entrypoint + manual DI
delivery/
  http/                          net/http handlers (thin)
  presenter/                     JSON / SSE DTO helpers
internal/                        resep dapur (private)
  config/                        env loader (BP_* / WEBCHAT_*)
  usecase/webchat/   orchestration
  domain/
    entity/ valueobject/ enum/
    repository/                  ThreadStore, ThreadLock, InterruptFlag
    service/                     SpeakFloor, DocsIndex, LLM*, TurnWorker, EventStreamer
  infrastructure/
    repository/jsonl/            durable JSONL + lock + interrupt + floor
    ratelimit/                   turn rate limiter
    service/docs|tools|llm       RAG index, host tools, LLM client/router
    worker/                      in-process agent loop (ProcessChatTurnJob port)
    sse/                         EventStreamer (poll JSONL → SSE events)
    stub/                        501 adapters (tests / fallback)
  pkg/apperr|idgen|redact
web/                             vanilla FE (dual-driver mock|real)
resources/webchat/               prompts + *.tool.json
storage/webchat/                 runtime JSONL + docs_index + interrupt/rl/llm
docs/webchat/                    knowledge MD corpus
```

## Ports ↔ FE real driver

| FE export | HTTP | Usecase |
|---|---|---|
| `listConversations` | `GET /api/webchat/conversations` | `ListConversations` |
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
4. `write_enabled = false` — no mutation tools.
5. Soft tool failures return envelopes; do not crash the turn.

## Runtime wiring (`cmd/app`)

1. Ensure storage dirs; `docs.NewIndex` (+ Reindex) + Gate log
2. `tools.NewRegistry` (allowlist: search_docs, list_dir, read_file, grep)
3. `llm.NewClient` + `llm.NewRouter` (failover / circuit)
4. `worker.New` + `sse.NewStreamer`
5. `webchat.NewService(Deps{…})` → HTTP server

**LLM stub:** when `BP_LLM_STUB=true` (default if no API key), worker appends canned `agent_message` + `turn.completed` without calling providers.

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
# GET /api/webchat/conversations → docs gate + sidebar
# static FE at http://localhost:8080/ (real by default)
# or make fe + http://localhost:5173/ (real driver → :8080 API; CORS on)
# mock: ?mock=1
```


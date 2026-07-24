# Architecture — BuatPostingan (Go)

Clean Architecture **ringan** (tanpa Wire / logmanager / salt-pkg). Dependency rule:

```
delivery → application → domain ← infrastructure
```

**Resep dapur** ada di **`internal/`**. **Delivery** (ruang depan) di root — adapter HTTP/SSE, bukan business rules.

## Layout

```
cmd/app/                         entrypoint + manual DI
delivery/
  http/                          net/http handlers (thin)
  presenter/                     JSON / SSE DTO helpers
internal/                        resep dapur (private)
  config/                        env loader
  application/usecase/webchat/   orchestration
  domain/
    entity/ valueobject/ enum/
    repository/                  ports: ThreadStore, ThreadLock, …
    service/                     ports: SpeakFloor, DocsIndex, LLM*, …
  infrastructure/
    stub/                        501 adapters (temporary wiring)
    repository/jsonl/            (next) durable JSONL store
    service/llm|docs|tools       (next)
    worker/                      (next) in-process agent loop
  pkg/apperr|idgen               technical helpers (private)
web/                             vanilla FE (dual-driver mock|real)
storage/webchat/                 runtime JSONL
docs/webchat/                    knowledge MD corpus (later)
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
3. SSE tails durable seq — not LLM token streaming.
4. `write_enabled = false` — no mutation tools.
5. Soft tool failures return envelopes; do not crash the turn.

## Implementation order (suggested)

1. `internal/infrastructure/repository/jsonl` + locks / interrupt flags
2. SpeakFloor + TurnRateLimit + DocsIndex gate
3. Usecase Create/List/Get/Rename (no LLM yet)
4. TurnWorker + LLM router + ToolRegistry (`search_docs` first)
5. SSE EventStreamer
6. Flip FE default `mockMode` only after happy path works

## Compile & run scaffold

```bash
make run
# GET /healthz → {"ok":true}
# GET /api/webchat/conversations → 501 not_implemented
# static FE at http://localhost:8080/?mock=0
```

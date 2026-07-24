# Runbook — BuatPostingan

Local ops for humans and agents. Product locks and layer rules: [`AGENTS.md`](../../AGENTS.md), [Architecture](../architecture/README.md).

## Prerequisites

- Go **1.22+**
- Optional: provider API key for real LLM turns
- Optional: host `rg` for faster `grep` tool (Go RE2 fallback if missing)

```bash
cp .env.example .env   # optional; Load also reads .env if present
```

## Backend (`make be`)

```bash
make be                # alias: make run-be / make run — Go + reflex reload
# http://localhost:8080/
# GET /healthz → {"ok":true}
# Static FE served from BP_WEB_ROOT (default web/)
```

| Target | Purpose |
|---|---|
| `make be` / `make run-be` / `make run` | App + reload on `:8080` |
| `make build` | Binary → `bin/buatpostingan` |
| `make test` | `go test ./...` |
| `make tidy` | `go mod tidy` |

Useful env (defaults in parentheses):

| Var | Default |
|---|---|
| `BP_HTTP_ADDR` | `:8080` |
| `BP_WEB_ROOT` | `web` |
| `BP_STORAGE_ROOT` | `storage/webchat` |
| `BP_DOCS_ROOT` | `docs/webchat` |
| `BP_PROMPTS_ROOT` | `resources/webchat/prompts` |
| `BP_TOOLS_ROOT` | `resources/webchat/tools` |
| `BP_LLM_STUB` | `true` if no provider API key |

Portable aliases: same names with `WEBCHAT_*` prefix (see [LLM providers](../architecture/llm-providers.md)).

## Frontend (`make fe`)

```bash
make fe                # python http.server → http://localhost:5173/
# FE_PORT=3000 make fe
```

| Mode | How |
|---|---|
| **Real (default)** | omit or `?mock=0` — needs Go BE |
| Real on `:5173` | `make be` + open FE; API auto → `http://localhost:8080/api/webchat` |
| Override API | `?api=http://host:port` or `?api=…/api/webchat` |
| **Mock** | `?mock=1` — no Go required |

Same-origin real UI: use Go-served page at http://localhost:8080/ (no CORS dance).

> Python `http.server` returns **HTTP 501** on `POST /api/*`. That is the static FE process, not a missing Go route. Real mode must not post to `:5173`.

## Stub vs real LLM

**Stub (default without keys):**

```bash
make be
# send a message → agent_message "(stub) received: …" + turn.completed
```

**Real:**

```bash
# .env
BP_LLM_STUB=false
BP_LLM_PROVIDERS=OPENROUTER
BP_LLM_OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
BP_LLM_OPENROUTER_API_KEY=sk-or-...
BP_LLM_OPENROUTER_MODEL=openai/gpt-4o-mini
BP_LLM_OPENROUTER_API=responses   # or chat
```

Restart `make be`. Expect tool rounds (search_docs / list_dir / read_file / grep) when the model calls them. Logs: `webchat.turn_start`, `webchat.tool`, `webchat.reasoning`, `webchat.turn_completed`.

Details: [LLM providers](../architecture/llm-providers.md).

## Smoke checks

1. `GET /healthz` → ok  
2. `GET /api/webchat/conversations` → sidebar + docs gate  
3. Create thread → start turn → SSE `/events` shows `item.completed` / `turn.completed`  
4. Inspect `storage/webchat/threads/thr_*.jsonl` for durable seq  

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| POST 501 from `:5173` | Hitting Python static server — run `make be`, use auto API base or `?api=` |
| Docs / turns refused | Docs index not usable — check `BP_DOCS_ROOT` + startup Reindex/Gate |
| Empty model replies on Responses | Prefer `API=responses` + SSE streaming; avoid forcing non-stream |
| Mock UI when you wanted real | Remove `?mock=1` / clear `localStorage bp.mockMode` |
| Tool path errors | Tools are sandboxed to `BP_DOCS_ROOT` only |

## Related

- [Turn loop](../architecture/turn-loop.md)
- [Architecture](../architecture/README.md)
- [Docs index](../README.md)

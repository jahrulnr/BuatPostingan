# BuatPostingan

AI assistant for drafting posts — webchat UI + Go Clean Architecture backend.

## Layout

| Path | Role |
|---|---|
| [`web/`](web/) | Vanilla JS chat UI (mock \| real dual driver) |
| [`cmd/app`](cmd/app) | Go entrypoint + DI |
| [`delivery/`](delivery/) | HTTP/SSE adapters (ruang depan) |
| [`internal/`](internal/) | Resep dapur: domain, usecase, infrastructure, config, pkg |
| [`resources/webchat/`](resources/webchat/) | Prompts + tool JSON |
| [`docs/webchat/`](docs/webchat/) | Knowledge Markdown corpus |
| [`docs/architecture/`](docs/architecture/README.md) | CA map |
| [`docs/architecture/portable-ai-kit.md`](docs/architecture/portable-ai-kit.md) | What to copy for reuse in other products |

## Frontend

```bash
make fe                 # http://localhost:5173/  (default real → :8080 API)
# FE_PORT=3000 make fe
```

| Mode | How |
|---|---|
| Real (default) | omit or `?mock=0` — needs `make be`; Go-served UI: http://localhost:8080/ |
| Real from `make fe` | `make be` + http://localhost:5173/ — API auto-targets `http://localhost:8080/api/webchat` (override with `?api=http://host:port`) |
| Mock | `?mock=1` — no Go required |

> Python `http.server` on `:5173` cannot handle `POST /api/*` (HTTP 501). Real mode never posts to that process; it calls the Go backend (CORS enabled).

## Backend

```bash
cp .env.example .env   # optional
make be                # :8080 + reflex
make build && make test
```

Without API keys the app starts in **LLM stub** mode (canned replies). For a real model:

```bash
# .env
BP_LLM_STUB=false
BP_LLM_OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
BP_LLM_OPENROUTER_API_KEY=sk-or-...
BP_LLM_OPENROUTER_MODEL=openai/gpt-4o-mini
BP_LLM_OPENROUTER_API=responses   # or chat; SSE streaming is supported for both
```

| Var | Default |
|---|---|
| `BP_HTTP_ADDR` | `:8080` |
| `BP_WEB_ROOT` | `web` |
| `BP_STORAGE_ROOT` | `storage/webchat` |
| `BP_DOCS_ROOT` | `docs/webchat` |
| `BP_PROMPTS_ROOT` | `resources/webchat/prompts` |
| `BP_TOOLS_ROOT` | `resources/webchat/tools` |
| `BP_LLM_STUB` | `true` if no provider API key |

See [`.env.example`](.env.example) and [`docs/architecture/README.md`](docs/architecture/README.md).

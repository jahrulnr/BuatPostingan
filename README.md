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
| [`resources/webchat/docs/`](resources/webchat/docs/) | Knowledge Markdown corpus |
| [`docs/`](docs/README.md) | Docs index (architecture, turn loop, LLM, runbook) |
| [`docs/architecture/`](docs/architecture/README.md) | CA map + tools / persistence / FE |
| [`docs/architecture/portable-ai-kit.md`](docs/architecture/portable-ai-kit.md) | What to copy for reuse in other products |

## Frontend

```bash
make fe                 # http://localhost:5173/  (live-reload; needs Node/npx; default real → :8080 API)
# FE_PORT=3000 make fe
```

| Mode | How |
|---|---|
| Real (default) | omit or `?mock=0` — needs `make be`; Go-served UI: http://localhost:8080/ |
| Real from `make fe` | `make be` + http://localhost:5173/ — API auto-targets `http://localhost:8080/api/webchat` (override with `?api=http://host:port`) |
| Mock | `?mock=1` — no Go required |

> `make fe` uses `npx live-server` (auto-refresh on HTML/CSS/JS edits). Static `:5173` cannot handle `POST /api/*` — real mode calls the Go backend (CORS enabled). Go-served UI at `:8080` has no FE livereload (edit via `make fe` for that).

## Backend

```bash
cp .env.example .env   # optional
make be                # :8080 + reflex
make build && make test
```

Without API keys the app starts in **LLM stub** mode (canned replies). For a real model, configure providers in `storage/config.json`:

```bash
cp storage/config.example.json storage/config.json
# Edit llm.providers[] → set api_key, base_url, models
# Set BP_LLM_STUB=false in .env (or leave stub for ready-to-use local turns)
```

| Var | Default |
|---|---|
| `BP_HTTP_ADDR` | `:8080` |
| `BP_WEB_ROOT` | `web` |
| `BP_STORAGE_ROOT` | `storage/webchat` |
| `BP_DOCS_ROOT` | `resources/webchat/docs` |
| `BP_PROMPTS_ROOT` | `resources/webchat/prompts` |
| `BP_TOOLS_ROOT` | `resources/webchat/tools` |
| `BP_SKILLS_ROOT` | `resources/webchat/skills` |
| `BP_LLM_STUB` | `true` (set `false` after configuring providers in `config.json`) |

See [`.env.example`](.env.example), [`docs/operations/runbook.md`](docs/operations/runbook.md), and [`docs/architecture/llm-providers.md`](docs/architecture/llm-providers.md).

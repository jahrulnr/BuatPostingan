# BuatPostingan

AI assistant for drafting posts — **frontend-first** webchat + Go Clean Architecture scaffold.

## Layout

| Path | Role |
|---|---|
| [`web/`](web/) | Vanilla JS chat UI (mock \| real dual driver) |
| [`cmd/app`](cmd/app) | Go entrypoint |
| [`delivery/`](delivery/) | HTTP/SSE adapters (ruang depan) |
| [`internal/`](internal/) | Resep dapur: domain, application, infrastructure, config, pkg |
| [`docs/architecture/`](docs/architecture/README.md) | CA map + implementation order |

## Frontend (mock, no Go needed)

```bash
cd web && python3 -m http.server 5173
# http://localhost:5173/?mock=1
```

| Mode | How |
|---|---|
| Mock (default) | `?mock=1` |
| Real | `?mock=0` → `/api/webchat` |

## Go scaffold

```bash
make run          # :8080 — serves web/ + /api/webchat (501 until impl)
make build
make test
```

Env (optional):

| Var | Default |
|---|---|
| `BP_HTTP_ADDR` | `:8080` |
| `BP_WEB_ROOT` | `web` |
| `BP_STORAGE_ROOT` | `storage/webchat` |
| `BP_DOCS_ROOT` | `docs/webchat` |

See [`docs/architecture/README.md`](docs/architecture/README.md) before implementing JSONL / worker / LLM.

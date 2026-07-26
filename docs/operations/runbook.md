# Runbook — BuatPostingan

Local ops for humans and agents. Product locks and layer rules: [`AGENTS.md`](../../AGENTS.md), [Architecture](../architecture/README.md).

## Prerequisites

- Go **1.22+**
- Node.js (`npx`) for `make fe` live-reload (no committed `node_modules`)
- Optional: provider API key for real LLM turns
- Optional: host `rg` for faster `grep` tool (Go RE2 fallback if missing)
- Authentication bootstrap password for the first admin user

```bash
cp .env.example .env        # optional; Load also reads .env if present
# storage/config.json is auto-generated from struct defaults on first boot.
# Edit it to add providers/keys, or use the Settings UI.
```

Most knobs (limits, llm globals, context, docs, web_search, skills_root, mcp, providers) live in `storage/config.json` — the JSON source of truth. Env (`BP_*`) is process-level only: paths/addr, `BP_LLM_RETRY_STATUSES`, `BP_LLM_STUB`. See [settings-config](../architecture/settings-config.md).

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

Useful env (defaults in parentheses). Most knobs now live in `storage/config.json` — these are the env-only bootstrap:

| Var | Default |
|---|---|
| `BP_HTTP_ADDR` | `:8080` |
| `BP_WEB_ROOT` | `web` |
| `BP_STORAGE_ROOT` | `storage/webchat` |
| `BP_CONFIG_PATH` | `{dirname(STORAGE)}/config.json` |
| `BP_DOCS_ROOT` | `resources/webchat/docs` |
| `BP_PROMPTS_ROOT` | `resources/webchat/prompts` |
| `BP_TOOLS_ROOT` | `resources/webchat/tools` |
| `BP_SKILLS_ROOT` | `resources/webchat/skills` |
| `BP_WORKSPACE_ROOT` | `.` (resolved to absolute process cwd at boot) — the agent's working dir: surfaced to the LLM as `{{cwd}}` in `developer.md`, AND used as the default base for relative `list_dir` / `read_file` / `grep` / `write_file` / `edit_file` paths when the turn doesn't override. Per-turn override (sent via StartTurn `workspace`) wins. |
| `BP_LLM_STUB` | `true` for canned responses; set `false` after configuring a usable provider (local gateways may be keyless) |
| `BP_LLM_RETRY_STATUSES` | `408,409,413,425,429,500–504` |
| `BP_AUTH_DB_PATH` | `{dirname(BP_STORAGE_ROOT)}/users.sqlite` |
| `BP_AUTH_ADMIN_USERNAME` | empty; set together with password for first boot |
| `BP_AUTH_ADMIN_PASSWORD` | empty; 8–128 chars, never commit it |
| `BP_AUTH_SESSION_TTL_HOURS` | `24` |
| `BP_CORS_ORIGIN` | `http://localhost:5173` |

Everything else (provider slots, stream/vision/effort, retry backoff, context, docs, skills root, mcp) is configurable via `storage/config.json` — see [settings-config](../architecture/settings-config.md) and [LLM providers](../architecture/llm-providers.md).

## Frontend (`make fe`)

```bash
make fe                # npx live-server → http://localhost:5173/ (auto-refresh on web/ edits)
# FE_PORT=3000 make fe
```

Requires **Node.js** (`npx`); downloads `live-server` on first run via `npx --yes` (no repo `node_modules`).

| Mode | How |
|---|---|
| **Real (default)** | omit or `?mock=0` — needs Go BE |
| Real on `:5173` | `make be` + open FE; API auto → `http://localhost:8080/api/webchat` |
| Override API | `?api=http://host:port` or `?api=…/api/webchat` |
| **Mock** | `?mock=1` — no Go required |

Same-origin real UI: use Go-served page at http://localhost:8080/ (no CORS dance; **no FE livereload** — use `make fe` when editing HTML/CSS/JS).

> Static `:5173` cannot handle `POST /api/*`. That is the live-server process, not a missing Go route. Real mode must not post to `:5173`.

## Docker (Nginx + Go in one image)

```bash
make docker-build
make docker-up
# Admin UI: http://localhost:1212/admin/
# Published pages: http://localhost:1212/<page-id>/

make docker-restart
make docker-down
```

Nginx is the only public listener on port `1212`. Go listens only inside the
container on `127.0.0.1:1313`; Nginx proxies `/api/` and `healthz` to it.
The root route serves only `storage/pages/.published/`, so an unpublished draft
cannot be reached from the public site. Override the host port with
`BP_PORT=1314 make docker-up`.

### Admin authentication

Chat JSONL remains under `storage/webchat/`. User accounts and hashed session
metadata are stored separately in SQLite at `storage/users.sqlite`. On an empty
database, the app creates one admin user only when both bootstrap variables are
provided:

```bash
BP_AUTH_ADMIN_USERNAME=owner \
BP_AUTH_ADMIN_PASSWORD='use-a-local-password-of-8-chars-or-more' \
make docker-up
```

Open `http://localhost:1212/admin/login.html`. The password is bcrypt-hashed;
the browser receives only an HttpOnly, SameSite session cookie. If the database
already contains a user, bootstrap variables do not overwrite it. Set
`BP_CORS_ORIGIN` when using `make fe` from a non-default origin.

## Stub vs real LLM

**Stub (default without keys):**

```bash
make be
# send a message → agent_message "(stub) received: …" + turn.completed
```

**Real:** edit `storage/config.json` (auto-generated on first boot):

```json
{
  "llm": {
    "stub": false,
    "providers": [
      {
        "id": "OPENROUTER",
        "api": "responses",
        "base_url": "https://openrouter.ai/api/v1",
        "api_key": "sk-or-...",
        "enabled": true,
        "models": [ { "id": "openai/gpt-4o-mini" } ]
      }
    ]
  }
}
```

Restart `make be`. Expect tool rounds (docs_search / list_dir / read_file / grep / read_attachment / read_image / list_skills / read_skill / web_*) when the model calls them. Logs: `webchat.turn_start`, `webchat.tool`, `webchat.reasoning`, `webchat.turn_completed`.

**Skills:** Ask the agent to use a skill (e.g. “use the writing-post skill to outline a post about X”). Expect `list_skills` and/or `read_skill` then `docs_search`. Details: [skills-tools.md](../architecture/skills-tools.md).

**MCP:** Edit the `mcp` block in `storage/config.json`, run `make mcp-echo`, restart `make be`. Ask to list/call MCP tools. If `list_mcp_tools` returns empty `tools` with a `hint`, servers are missing from the runtime config file. Details: [mcp-support.md](../architecture/mcp-support.md).

**Vision (image attach):** With `llm.vision=auto` (default) or `on` and a vision-capable model (e.g. `xiaomi/mimo-v2.5`), attach a PNG/JPEG and ask "what is in this image?". Worker injects `image_url` / `input_image` data-URL parts (cap 4 MiB / image). Text-only models under `auto` get metadata only. Details: [llm-vision.md](../architecture/llm-vision.md).

**Effort (reasoning):** `llm.effort=auto` (default) probes `/models` and only sends `reasoning.effort` / `reasoning_effort` when the model advertises support. Explicit levels (`medium`, `high`, …) are clamped/omitted the same way. Details: [llm-effort.md](../architecture/llm-effort.md).

Details: [LLM providers](../architecture/llm-providers.md).

## Smoke checks

1. `GET /healthz` → ok  
2. `GET /api/webchat/conversations` → sidebar + docs gate  
3. Create thread → start turn → SSE `/events` shows `item.completed` / `turn.completed`  
4. Inspect `storage/webchat/threads/thr_*.jsonl` for durable seq  
5. Settings: gear on left rail → `#/settings/models` → configure a provider card or add a custom compatible endpoint; file lands at `storage/config.json` (gitignored). See [settings-config](../architecture/settings-config.md).

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| POST fail from `:5173` | Hitting static live-server — run `make be`, use auto API base or `?api=` |
| `make fe` exits “requires Node.js” | Install Node (`npx`), or use `make be` + http://localhost:8080/ without FE livereload |
| Docs / turns refused | Docs index not usable — check `BP_DOCS_ROOT` + startup Reindex/Gate |
| Empty model replies on Responses | Prefer `api=responses` + SSE (`llm.stream=true` default); only set `llm.stream=false` if upstream cannot stream. Reasoning-only rounds log `webchat.empty_model_response` (WARN) and nudge once — see [observability](../architecture/observability.md) |
| Thinking text not in VS Code search | Reasoning is in `storage/webchat/threads/*.jsonl`, not source: `rg 'The user is asking' storage/webchat/threads/` |
| Mock UI when you wanted real | Remove `?mock=1` / clear `localStorage bp.mockMode` |
| Tool path errors | Check absolute/relative path exists; FS tools are unrestricted on the host (local-dev) |

## Related

- [Turn loop](../architecture/turn-loop.md)
- [Architecture](../architecture/README.md)
- [Docs index](../README.md)

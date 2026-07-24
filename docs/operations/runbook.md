# Runbook — BuatPostingan

Local ops for humans and agents. Product locks and layer rules: [`AGENTS.md`](../../AGENTS.md), [Architecture](../architecture/README.md).

## Prerequisites

- Go **1.22+**
- Node.js (`npx`) for `make fe` live-reload (no committed `node_modules`)
- Optional: provider API key for real LLM turns
- Optional: host `rg` for faster `grep` tool (Go RE2 fallback if missing)

```bash
cp .env.example .env        # optional; Load also reads .env if present
cp storage/config.example.json storage/config.json   # then edit providers/keys
```

Most knobs (limits, llm globals, context, docs, web_search, skills_root, mcp) live in `storage/config.json` — the JSON source of truth. Env (`BP_*`) is now bootstrap only: process paths/addr, `BP_LLM_RETRY_STATUSES`, and `BP_LLM_STUB`. See [settings-config](../architecture/settings-config.md).

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
| `BP_DOCS_ROOT` | `docs/webchat` |
| `BP_PROMPTS_ROOT` | `resources/webchat/prompts` |
| `BP_TOOLS_ROOT` | `resources/webchat/tools` |
| `BP_LLM_STUB` | `true` if no provider API key |
| `BP_LLM_RETRY_STATUSES` | `408,409,413,425,429,500–504` |

Everything else (provider slots, stream/vision/effort, circuit, retry backoff, context, docs, skills root, mcp) is configurable via `storage/config.json` — see [settings-config](../architecture/settings-config.md) and [LLM providers](../architecture/llm-providers.md).

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

## Stub vs real LLM

**Stub (default without keys):**

```bash
make be
# send a message → agent_message "(stub) received: …" + turn.completed
```

**Real:** edit `storage/config.json` (copy from `storage/config.example.json`):

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

(Or use the legacy env: `BP_LLM_STUB=false` + `BP_LLM_OPENROUTER_*` keys.) Restart `make be`. Expect tool rounds (search_docs / list_dir / read_file / grep / read_attachment / read_image / list_skills / read_skill / web_*) when the model calls them. Logs: `webchat.turn_start`, `webchat.tool`, `webchat.reasoning`, `webchat.turn_completed`.

**Skills:** Ask the agent to use a skill (e.g. “pakai skill writing-post untuk outline postingan tentang X”). Expect `list_skills` and/or `read_skill` then `search_docs`. Details: [skills-tools.md](../architecture/skills-tools.md).

**MCP:** Copy `mcp` from `storage/config.example.json` into `storage/config.json`, run `make mcp-echo`, restart `make be`. Ask to list/call MCP tools. If `list_mcp_tools` returns empty `tools` with a `hint`, servers are missing from the runtime config file. Details: [mcp-support.md](../architecture/mcp-support.md).

**Vision (image attach):** With `BP_LLM_VISION=auto` (default) or `on` and a vision-capable model (e.g. `xiaomi/mimo-v2.5`), attach a PNG/JPEG and ask “apa isi gambar ini?”. Worker injects `image_url` / `input_image` data-URL parts (cap 4 MiB / image). Text-only models under `auto` get metadata only. Details: [llm-vision.md](../architecture/llm-vision.md).

**Effort (reasoning):** `BP_LLM_EFFORT=auto` (default) probes `/models` and only sends `reasoning.effort` / `reasoning_effort` when the model advertises support. Explicit levels (`medium`, `high`, …) are clamped/omitted the same way. Details: [llm-effort.md](../architecture/llm-effort.md).

Details: [LLM providers](../architecture/llm-providers.md).

## Smoke checks

1. `GET /healthz` → ok  
2. `GET /api/webchat/conversations` → sidebar + docs gate  
3. Create thread → start turn → SSE `/events` shows `item.completed` / `turn.completed`  
4. Inspect `storage/webchat/threads/thr_*.jsonl` for durable seq  
5. Settings: gear on left rail → `#/settings/models` → add OpenAI-compatible provider; file lands at `storage/config.json` (gitignored). See [settings-config](../architecture/settings-config.md).  

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| POST fail from `:5173` | Hitting static live-server — run `make be`, use auto API base or `?api=` |
| `make fe` exits “requires Node.js” | Install Node (`npx`), or use `make be` + http://localhost:8080/ without FE livereload |
| Docs / turns refused | Docs index not usable — check `BP_DOCS_ROOT` + startup Reindex/Gate |
| Empty model replies on Responses | Prefer `API=responses` + SSE (`BP_LLM_STREAM=true` default); only set `BP_LLM_STREAM=false` if upstream cannot stream. Reasoning-only rounds log `webchat.empty_model_response` (WARN) and nudge once — see [observability](../architecture/observability.md) |
| Thinking text not in VS Code search | Reasoning is in `storage/webchat/threads/*.jsonl`, not source: `rg 'The user is asking' storage/webchat/threads/` |
| Mock UI when you wanted real | Remove `?mock=1` / clear `localStorage bp.mockMode` |
| Tool path errors | Check absolute/relative path exists; FS tools are unrestricted on the host (local-dev) |

## Related

- [Turn loop](../architecture/turn-loop.md)
- [Architecture](../architecture/README.md)
- [Docs index](../README.md)

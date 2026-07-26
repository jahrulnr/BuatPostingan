# Settings + JSON-backed config

Product settings UI and durable app config **without a database**. Chat history stays JSONL; settings live in a single JSON file.

## Goals

- Persist users and typed LLM provider connections so operators can manage them from the UI.
- Keep **env (`BP_*`)** for process-level paths and a few env-only toggles.
- Make **`storage/config.json`** the runtime source of truth once it has LLM providers.
- Use **hardcoded defaults** in `config.Load()` when JSON omits a key.
- Never leak raw API keys over the wire; mask on read; write-only key updates.
- Match the IDE shell (charcoal / light via `data-theme`); settings is a full-page view with hash deep-links.

## File location

| Priority | Path | Notes |
|---|---|---|
| 1 | `BP_CONFIG_PATH` | Absolute or cwd-relative override |
| 2 | default | `{dirname(BP_STORAGE_ROOT)}/config.json` |

With default `BP_STORAGE_ROOT=storage/webchat`, that resolves to **`storage/config.json`** (sibling of the webchat data dir — matches `./storage/config.json` from app cwd).

Permissions: create parent dirs `0755`; write file `0600` (secrets). Atomic save: write `config.json.tmp` → `fsync` → `rename`.

Committed template: none. `storage/config.json` is **auto-generated** from struct defaults (`config.DefaultSeedFile`) on first boot via `appconfig.Store.EnsureSeeded()` when the file is missing. Runtime file is **gitignored**.

## Schema (v1)

```json
{
  "version": 1,
  "users": [
    { "id": "usr_owner", "name": "Owner", "role": "owner" }
  ],
  "limits": {
    "max_tool_rounds": 8,
    "speak_floor_ttl_sec": 600,
    "lock_ttl_sec": 300,
    "turn_job_timeout_sec": 120
  },
  "llm": {
    "strategy": "failover",
    "active_provider": "OPENROUTER",
    "stream": true,
    "vision": "auto",
    "effort": "auto",
    "total_attempt_budget": 4,
    "retry_base_delay_ms": 250,
    "retry_max_delay_ms": 5000,
    "retry_jitter": 0.2,
    "providers": [
      {
        "type": "openrouter",
        "id": "OPENROUTER",
        "name": "OpenRouter",
        "prefix": "openrouter",
        "api": "responses",
        "base_url": "https://openrouter.ai/api/v1",
        "api_key": "",
        "api_keys": [],
        "enabled": true,
        "models": [
          { "id": "openai/gpt-4o-mini", "label": "GPT-4o mini" }
        ],
        "timeout_sec": 60,
        "max_attempts": 1,
        "weight": 1
      }
    ]
  },
  "context": {
    "compaction_enabled": true,
    "max_input_tokens": 12000,
    "reserve_tokens": 3000,
    "recent_turns": 4,
    "summary_max_chars": 12000
  },
  "docs": {
    "top_k": 5,
    "min_score": 0.5,
    "fuzzy_enabled": true,
    "app_id": "buatpostingan"
  },
  "web_search": {
    "github_token": ""
  },
  "mcp": { "...": "see mcp-support.md" }
}
```

### Field notes

| Area | Fields | Rules |
|---|---|---|
| **User** | `id`, `name`, `role` | `role` ∈ `owner` \| `admin` \| `member`. IDs opaque (`usr_…`). No passwords in v1. |
| **skills_root** | — | Env-only: `BP_SKILLS_ROOT`. Jailed root for `list_skills` / `read_skill`. |
| **limits** | `max_tool_rounds`, `speak_floor_ttl_sec`, `lock_ttl_sec`, `turn_job_timeout_sec` | Pointer fields — omit a key to keep the hardcoded default. |
| **llm.strategy / active_provider** | see provider notes | Strategy and active provider from JSON. |
| **llm globals** | `stream`, `vision` (`auto`\|`on`\|`off`), `effort` (`auto`\|`none`\|`minimal`\|`low`\|`medium`\|`high`\|`xhigh`\|`max`), `total_attempt_budget`, `retry_base_delay_ms`, `retry_max_delay_ms`, `retry_jitter` | Pointer fields — omit keeps hardcoded default. Invalid `vision` / `effort` fall back to `auto`. |
| **Provider** | `type?`, `id`, `name`, `prefix`, `api`, `base_url`, `api_key`, `api_key_optional`, `api_keys[]`, `enabled`, `models[]`, sizing | `type` selects an injected provider adapter; omitted legacy entries are inferred. `id` is the uppercased router slot. `api` ∈ `chat` \| `responses` \| `messages`. `api_key_optional` is registry-owned for local gateways. |
| **Model** | `id`, `label?` | First model id maps to runtime `LLMProvider.Model` (primary). Extra models are picker allowlist entries for the same provider slot. |
| **context** | `compaction_enabled`, `max_input_tokens`, `reserve_tokens`, `recent_turns`, `summary_max_chars` | Pointer fields — omit keeps hardcoded default. |
| **docs** | `top_k`, `min_score`, `fuzzy_enabled`, `app_id` | Pointer fields (except `app_id`) — omit keeps hardcoded default. |
| **web_search** | `github_token` | Optional GitHub rate-limit token. Empty string is valid. |
| **mcp** | `enabled`, `connect_timeout_sec`, `call_timeout_sec`, `servers[]` | See [mcp-support.md](mcp-support.md). |

## Env ↔ file merge

**Policy (JSON SoT for product knobs; env is process-level only):**

1. Always load env via `config.Load()` (paths, retry statuses, stub toggle, write toggle, and hardcoded defaults for every product knob).
2. `config.ApplySettingsFile(envCfg, doc)` overlays the JSON document:
   - **Pointer fields** (limits / context / docs / llm globals): set in JSON → override hardcoded default; **omitted** → hardcoded default wins.
   - **providers**: file providers replace the env map when non-empty. Strategy / active provider come from file when present. Stub is env-only.
   - **string fields** (`docs.app_id`, `web_search.github_token`): non-empty → override hardcoded default; empty → hardcoded default.
3. If `config.json` is **missing** or **unreadable** → hardcoded defaults + env-only vars. Old files without the new sections keep working because every new field is a pointer / omitempty string.
4. First successful Settings write creates the file (seeded with current runtime providers + sections).

Env-only (never in JSON): `BP_HTTP_ADDR`, `BP_WEB_ROOT`, `BP_STORAGE_ROOT`, `BP_DOCS_ROOT`, `BP_PROMPTS_ROOT`, `BP_TOOLS_ROOT`, `BP_SKILLS_ROOT`, `BP_WORKSPACE_ROOT`, `BP_CONFIG_PATH`, `BP_LLM_RETRY_STATUSES`, `BP_LLM_STUB`. Paths/addr are process-level; retry statuses are a fixed operational policy; stub stays in env as the development toggle; write toggle is a local-dev safety switch. Every other historical `BP_*` knob is now configurable via `storage/config.json`.

**MCP:** optional `mcp` object on the same file (`mcp.servers[]`, timeouts). Applied by `ApplySettingsFile` even when `llm.providers` is empty. Hardcoded default `MCPEnabled=true`, but with no `mcp.servers` the catalog is empty (`list_mcp_tools` returns a `hint`). The first-boot seed (`config.DefaultSeedFile` → `appconfig.Store.EnsureSeeded()`) already includes the sample echo server via `DefaultLocalDevMCP`; existing files without `mcp` are left alone — add the block and **restart** `make be`. See [mcp-support.md](mcp-support.md). MCP timeouts can also come from `mcp.connect_timeout_sec` / `mcp.call_timeout_sec` in the JSON; hardcoded defaults (15s / 30s) apply when JSON omits them.

```
env (process-level) + hardcoded defaults ──► runtime Config
                                        ▲
config.json ──────────────────────────────┘  (JSON wins when keys are present; omit → hardcoded default)
```

## API design

Base: **`/api/settings`** (product-level; separate from chat turns). Mount via `MountSettingsAPI`. CORS already allows `GET, POST, PATCH, DELETE`.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/settings` | Snapshot: users + masked providers + meta (`source`: `file`\|`env`) |
| `GET` | `/api/settings/users` | List users |
| `POST` | `/api/settings/users` | Create `{name, role}` → user |
| `PATCH` | `/api/settings/users/{id}` | Update name/role |
| `DELETE` | `/api/settings/users/{id}` | Delete (forbid deleting last owner) |
| `GET` | `/api/settings/llm/providers` | List providers (masked keys) |
| `GET` | `/api/settings/llm/provider-catalog` | List credential-free provider definitions and defaults |
| `POST` | `/api/settings/llm/providers` | Create a typed provider connection |
| `GET` | `/api/settings/llm/providers/{id}` | Detail + models (masked) |
| `PATCH` | `/api/settings/llm/providers/{id}` | Update fields; omit/`""` `api_key` = keep existing |
| `DELETE` | `/api/settings/llm/providers/{id}` | Delete provider |
| `POST` | `/api/settings/llm/providers/{id}/models` | Add model `{id, label?}` |
| `DELETE` | `/api/settings/llm/providers/{id}/models/{modelId}` | Remove model |
| `POST` | `/api/settings/llm/providers/{id}/import-models` | Stub: returns `{imported:0, message}` (future: GET `{base}/models`) |

### Security responses

- List/detail never return raw `api_key`. Shape:

```json
{
  "api_key_set": true,
  "api_key_masked": "••••…abcd"
}
```

- `PATCH`/`POST`: if `api_key` is non-empty, replace stored secret; if omitted or empty string, leave unchanged.
- Errors: same `apperr` envelope as webchat (`code`, `message`).

### Hot reload

After any LLM-mutating write, call `LLMRuntime.Reload(mergedConfig)` so Router / Client / Catalog pick up providers **without process restart**. Documented fallback: if reload fails, operator restarts `make be`.

## Frontend

### Navigation

- Left rail footer: profile card (**Owner** hardcoded display) + gear → `#/settings/models`.
- Settings = **full-page swap** of the main workspace. The provider page renders
  registry definitions as cards and overlays configured connection state; custom
  OpenAI-compatible slots remain available.
- Hash routes:
  - `#/settings` → redirect Models
  - `#/settings/general` — stub panel
  - `#/settings/users`
  - `#/settings/models`
  - `#/settings/models/{providerId}`
- Back / brand → `#/` (chat).

### Dual-driver

Settings API methods live in `web/js/api/{mock,real}/` and are bound in `index.js` like chat. UI never `fetch`es directly (`G-ui-no-fetch`). Settings base URL: same origin as webchat API, path `/api/settings` (derive from `resolveApiBase` by replacing `/api/webchat`).

### Logout

Button only: clear local prefs (`bp.theme`, `bp.modelId`, `bp.effort`, `bp.mockMode`, `bp.previewWidth`, …) and toast. **No** server session.

## Out of scope (v1)

- Real auth / sessions
- Multi-key round-robin UI (`api_keys` stored empty)
- Speed submenu / connection health probes beyond stub import

## Related

- [LLM providers](llm-providers.md) — env slots + router
- [LLM model picker](llm-model-picker.md) — `/api/webchat/models` consumes runtime providers
- [Runbook](../operations/runbook.md) — local try steps

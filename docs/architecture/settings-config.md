# Settings + JSON-backed config

Product settings UI and durable app config **without a database**. Chat history stays JSONL; settings live in a single JSON file.

## Goals

- Persist **users** (minimal) and **LLM providers** (OpenAI-compatible) so operators can manage them from the UI.
- Keep **env (`BP_*`)** as bootstrap / first-run defaults.
- Make **`storage/config.json`** the runtime source of truth once it has LLM providers.
- Never leak raw API keys over the wire; mask on read; write-only key updates.
- Match the IDE shell (charcoal / light via `data-theme`); settings is a full-page view with hash deep-links.

## File location

| Priority | Path | Notes |
|---|---|---|
| 1 | `BP_CONFIG_PATH` | Absolute or cwd-relative override |
| 2 | default | `{dirname(BP_STORAGE_ROOT)}/config.json` |

With default `BP_STORAGE_ROOT=storage/webchat`, that resolves to **`storage/config.json`** (sibling of the webchat data dir — matches `./storage/config.json` from app cwd).

Permissions: create parent dirs `0755`; write file `0600` (secrets). Atomic save: write `config.json.tmp` → `fsync` → `rename`.

Committed template: `storage/config.example.json`. Runtime file is **gitignored**.

## Schema (v1)

```json
{
  "version": 1,
  "users": [
    { "id": "usr_owner", "name": "Owner", "role": "owner" }
  ],
  "llm": {
    "strategy": "failover",
    "active_provider": "OPENROUTER",
    "stub": null,
    "providers": [
      {
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
  }
}
```

### Field notes

| Area | Fields | Rules |
|---|---|---|
| **User** | `id`, `name`, `role` | `role` ∈ `owner` \| `admin` \| `member`. IDs opaque (`usr_…`). No passwords in v1. |
| **Provider** | `id`, `name`, `prefix`, `api`, `base_url`, `api_key`, `api_keys[]`, `enabled`, `models[]`, sizing | `id` uppercased slot (router key). `api` ∈ `chat` \| `responses`. Phase 1 uses **one** `api_key`; `api_keys` reserved for future round-robin without schema break. |
| **Model** | `id`, `label?` | First model id maps to runtime `LLMProvider.Model` (primary). Extra models are picker allowlist entries for the same provider slot. |
| **llm.stub** | `bool` or omit/`null` | When set, overrides env stub default. When null, stub follows “no usable key → stub”. |

## Env ↔ file merge

**Recommended policy (implemented):**

1. Always load env via `config.Load()` (paths, timeouts, circuit, vision/effort globals, etc.).
2. If `config.json` is **missing** or **unreadable** → env providers only (today’s behavior).
3. If file exists and `llm.providers` is **non-empty** → file providers **replace** env provider map for runtime. Strategy / active provider / stub come from file when present.
4. If file exists but `llm.providers` is **empty** → keep env providers (bootstrap); users still load from file.
5. First successful Settings write creates the file (seeded with current runtime providers converted to schema, keys included on disk only).

Globals that stay env-only in v1: `BP_LLM_STREAM`, `BP_LLM_VISION`, `BP_LLM_EFFORT`, circuit thresholds, storage/docs roots. Strategy/active can move to file when providers come from file.

```
env (bootstrap) ──► runtime Config
                      ▲
config.json ──────────┘  (wins for providers when non-empty)
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
| `POST` | `/api/settings/llm/providers` | Create OpenAI-compatible provider |
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
- Settings = **full-page swap** of the main workspace (hide chat columns; show settings shell). Not a modal overlay — matches IDE “settings editor” feel.
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
- Moving vision/effort globals into the JSON file

## Related

- [LLM providers](llm-providers.md) — env slots + router
- [LLM model picker](llm-model-picker.md) — `/api/webchat/models` consumes runtime providers
- [Runbook](../operations/runbook.md) — local try steps

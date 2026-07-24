# LLM model picker

Preparation + vertical slice for a Cursor-like composer pill (model + reasoning effort).

Grounded in: `GET /api/webchat/models`, `StartTurn` overrides, `llm.Catalog`, FE `model-picker.js`, and existing [`llm-effort.md`](llm-effort.md) / [`llm-vision.md`](llm-vision.md) / [`llm-providers.md`](llm-providers.md).

## Goals

- Let the operator pick **which configured provider/model** and **reasoning effort** before send.
- Prefer a small allowlist from `BP_LLM_*` provider slots (security), optionally enriched with live OpenRouter-style `GET {base}/models` metadata.
- Persist preference in the browser; clamp when a model disappears or effort is unsupported.
- No new env vars; no Speed submenu.

## UX (composer)

```
┌─────────────────────────────────────────────┐
│ [📎]  [ Model · medium ▾ ]     input…  [➤] │
└─────────────────────────────────────────────┘
                 │
                 ▼
        ┌─ Reasoning ─────────┐
        │ auto low medium high│
        ├─ Model ─────────────┤
        │ gpt-4o-mini · OR    │
        │ o3-mini · OR        │
        └─────────────────────┘
```

- Pill shows short model name + current effort.
- Menu: **Reasoning** section only when the selected model advertises `supported_efforts` (otherwise keep effort as `auto` / server default).
- Keyboard: pill toggles menu; `Escape` closes; options are focusable buttons with `aria-expanded` / `aria-selected`.

Common chat UIs (Cursor Composer, ChatGPT model menu, Claude project model) use the same pattern: compact trigger + grouped dropdown (capability section + model list). Effort levels follow OpenRouter / OpenAI enums — see [OpenRouter Reasoning Tokens](https://openrouter.ai/docs/guides/best-practices/reasoning-tokens) and [OpenRouter List models](https://openrouter.ai/docs/api/api-reference/models/list-all-models-and-their-properties).

## API contract

### `GET /api/webchat/models`

```json
{
  "models": [
    {
      "id": "openai/o3-mini",
      "label": "o3-mini · OPENROUTER",
      "provider": "OPENROUTER",
      "supports_vision": false,
      "supported_efforts": ["none", "low", "medium", "high"],
      "default_effort": "medium",
      "disabled": false
    }
  ],
  "default_model_id": "openai/o3-mini",
  "stub": false,
  "effort": {
    "current": "auto",
    "options": ["auto", "none", "minimal", "low", "medium", "high", "xhigh", "max"]
  }
}
```

| Field | Meaning |
|---|---|
| `models[].id` | Picker id (configured `BP_LLM_<ID>_MODEL`, or stub id) |
| `models[].provider` | Provider slot id (`OPENROUTER`, …) — never includes API keys |
| `supports_vision` | Catalog/heuristic (for chips); independent of `BP_LLM_VISION` force mode |
| `supported_efforts` | Empty → model does not expose effort selection |
| `default_model_id` | Active provider’s model (`BP_LLM_ACTIVE_PROVIDER`) |
| `effort.current` | Server default from `BP_LLM_EFFORT` |
| `stub` | `true` when LLM stub / canned list |

**Source of truth**

1. Enabled entries in `BP_LLM_PROVIDERS` / `BP_LLM_<ID>_*` (allowlist).
2. Enrichment via the same `/models` probe used by `EffortPolicy` / `VisionPolicy` when `BaseURL` is reachable.
3. `BP_LLM_STUB=true` → canned stub models (`stub/default`, `stub/reasoning`, `stub/vision`).

### `POST /api/webchat/threads/{id}/turns`

Optional body fields (in addition to `message` / `attachment_ids`):

```json
{
  "message": "…",
  "model": "openai/o3-mini",
  "effort": "high"
}
```

| Field | Validation |
|---|---|
| `model` | Must match a configured model id **or** provider id (case-insensitive for provider). Else `422 validation`. Empty → no provider pin. |
| `effort` | One of `auto\|none\|minimal\|low\|medium\|high\|xhigh\|max` (+ aliases). Else `422`. Empty → server `BP_LLM_EFFORT`. |

Resolved overrides land on `TurnJob.ProviderID` / `TurnJob.Effort`. Worker pins the provider and attaches effort via `llm.WithEffortMode` → `EffortPolicy.ResolveWithMode` (still omits unsupported effort on the wire — [`G-llm-effort-omit-unsupported`](llm-effort.md)).

## FE persistence

| Key | Value |
|---|---|
| `bp.modelId` | Selected model id |
| `bp.effort` | Selected effort (`auto`…) |

- Loaded on boot; written on menu change.
- Thread-scoped override is **out of scope** for this slice (global preference only).
- On send: `startTurn({ model, effort })` from the picker selection.

## Gone-model / unsupported-effort policy

```mermaid
sequenceDiagram
  participant FE as Composer
  participant LS as localStorage
  participant API as GET /models
  participant BE as StartTurn

  FE->>API: listModels()
  API-->>FE: models + default_model_id
  FE->>LS: read bp.modelId / bp.effort
  alt model missing from list
    FE->>LS: write default_model_id
    FE->>FE: toast once ("Model tidak tersedia")
  end
  FE->>FE: clamp effort to supported / default
  FE->>BE: StartTurn model+effort
  Note over BE: allowlist validate; reject unknown
```

1. If `bp.modelId` is **gone** from the backend list → fall back to `default_model_id`, update storage, toast once per session.
2. If effort is unsupported for the newly selected model → clamp to `default_effort` or first supported / `auto`.
3. Backend is the security gate: FE clamp is UX only; invalid overrides still `422`.

## Best practices

| Topic | Choice |
|---|---|
| Security | Overrides validated against configured providers only — never accept arbitrary upstream model strings outside the allowlist |
| Per-turn vs global | localStorage = global preference; StartTurn body = per-turn override for that job |
| Mock parity | `listModelsMock` + mock `startTurn` reject unknown model/effort |
| Accessibility | Pill `aria-haspopup` / `aria-expanded`; options `role="option"` + `aria-selected`; Escape closes |
| Dual-driver | UI only talks to `listModels` / `startTurn` via `web/js/api` |

## Implementation map

| Layer | Files |
|---|---|
| Domain | `entity.ModelsCatalog`, `service.ModelCatalog`, `TurnJob.ProviderID/Effort` |
| Infra | `llm/catalog.go`, `effort_ctx.go`, `EffortPolicy.ResolveWithMode`, worker pin + context |
| Usecase | `ListModels`, `StartTurn` override validation |
| Delivery | `GET /api/webchat/models`, StartTurn body fields, presenter |
| FE | `model-picker.js`, composer pill in `index.html`, CSS, mock/real drivers |

## Out of scope (later)

- Thread-local model preference
- Full OpenRouter catalog browsing (only configured slots)
- Speed / temperature submenu
- Renaming env prefix (kit consumers still sed `BP_*`)

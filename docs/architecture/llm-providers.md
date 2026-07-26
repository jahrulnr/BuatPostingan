# LLM providers

Grounded in `internal/config`, `internal/infrastructure/provider/`, and `internal/infrastructure/service/llm/`.

Provider families are explicit registry entries injected at startup. OpenRouter,
OmniRoute, 9Router, OpenAI, OpenAI-compatible endpoints, and the official Claude
API are currently mapped. The transport supports Chat Completions (`chat`),
Responses (`responses`), and Anthropic Messages (`messages`).

## Stub vs real

| Mode | When | Behavior |
|---|---|---|
| **Stub** | `BP_LLM_STUB` = true (default), **or** no providers configured in `config.json` | Worker skips HTTP; appends canned `agent_message` + `turn.completed` |
| **Real** | `BP_LLM_STUB=false` and at least one enabled provider is usable | `llm.Router` → `llm.Client` HTTP |

Default stub is intentional (ready-to-use local turns). Configure providers in `storage/config.json` and set `BP_LLM_STUB=false` for real LLM.

## Provider slots (config)

Configured connections remain named slots under `llm.providers[]`. The optional
`type` maps a slot to one injected provider adapter. Old entries without `type`
remain valid: the registry infers known providers from ID/base URL and otherwise
uses `openai-compatible`.

Provider definitions are separated under `internal/infrastructure/provider/<type>/`.
`cmd/app` builds the registry and injects its domain interface into settings;
the use case does not import concrete provider packages.

For each `ID`:

| Knob | JSON (`llm.providers[]`) | Default |
|---|---|---|
| Base URL | `base_url` (no trailing slash) | — |
| Credential | `api_key` | Bearer for OpenAI-shaped APIs; `x-api-key` for Claude |
| Model id | first `models[].id` | — |
| Dialect | `api` | Provider default: `chat`, `responses`, or `messages` |
| HTTP timeout | `timeout_sec` | 60 |
| Per-provider attempts | `max_attempts` | 1 |
| Enable toggle | `enabled` | true |
| Weight / sizing | `weight` (sizing uses defaults in v1) | 1 |

Example (`storage/config.json`):

```json
{
  "llm": {
    "providers": [
      {
        "type": "openrouter",
        "id": "OPENROUTER",
        "name": "OpenRouter",
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

Add another slot through Settings or `providers[]`. The catalog is exposed at
`GET /api/settings/llm/provider-catalog`; it contains no credentials.
`openai-compatible` stays registered for normalize/infer but is **hidden from the catalog** (`hide_from_catalog`) — custom connections are added via the Settings UI "+ Custom provider" button.

## API dialects

### `API=chat` — Chat Completions

- POST `{base}/chat/completions`
- Body: `messages`, `tools` (function shape), `tool_choice=auto`, `max_tokens`, **`stream`** (from config)
- Parses final assistant text + `tool_calls` (+ optional `reasoning_content` / `reasoning` / `thinking`)

### `API=responses` — Responses API (preferred)

- POST `{base}/responses`
- Maps system/developer → `instructions`; user/assistant/tool history → `input`
- Tools as Responses `function` objects; **`stream`** (from config)
- Parses `output[]` (`message`, `function_call`, `reasoning`) and optional `output_text`
- If the proxy returns chat.completion-shaped JSON, client falls back to the chat parser

### `API=messages` — Anthropic Messages API

- POST `{base}/messages`
- Sends `x-api-key` and `anthropic-version`; never sends the key as Bearer auth
- Maps system/developer content to `system`, function tools to `input_schema`,
  assistant tool calls to `tool_use`, and tool results to `tool_result`
- Parses text, thinking blocks, usage, and native `tool_use`
- This first implementation is deliberately non-streaming. Anthropic SSE has a
  different event protocol and will be added with its own parser rather than
  being forced through the OpenAI SSE parser.

### Streaming

Controlled by **`llm.stream`** in `storage/config.json` (bool, **default `true`**). Global only (no per-provider override).

When enabled (default):

- Sets `stream=true` and `Accept: text/event-stream, application/json`
- If body looks like SSE (`text/event-stream` or `data:` / `event:` prefix), `stream.go` aggregates events into one final payload
- Non-stream JSON responses still parse when the proxy returns JSON instead of SSE

When disabled (`llm.stream=false`):

- Sets `stream=false` and `Accept: application/json`
- Uses the JSON unmarshal path only

**Auto-fallback:** if stream was requested and the provider rejects streaming (HTTP 4xx whose body mentions stream/streaming unsupported, or a JSON error object about stream — not auth/quota 401/402/403), the client **retries once** with `stream=false` and logs `webchat.llm stream_fallback …`.

**Transport retry (Codex-like):** Responses SSE that ends without `response.completed`, emits `response.incomplete`, or dies mid-read is marked **Transient** (`SSE_TRANSPORT` / `ErrSSETransport`). The existing router budget (`llm.total_attempt_budget` + per-provider `MaxAttempts`) retries the same LLM request — no parallel retry stack. This is separate from the worker empty-response nudge (semantic, after a completed round).

Prefer leaving stream on for local OpenAI-compatible proxies that degrade Responses when forced non-stream. Use `llm.stream=false` only when you know the upstream cannot SSE.

**Important:** application SSE to the browser (`/events`) is **not** LLM token streaming. It tails durable JSONL seq after the worker appends items. LLM SSE is internal to the client→provider hop.

## Multi-provider router

`llm.NewRouter` wraps the client. These knobs live in `storage/config.json` under `llm.*` — see [settings-config.md](settings-config.md).

| Setting | JSON | Default | Role |
|---|---|---|---|
| Strategy | `llm.strategy` | `failover` | `failover` \| `round_robin` \| `switch` |
| Active provider | `llm.active_provider` | first enabled (sorted) | Preferred head of candidate list |
| Attempt budget | `llm.total_attempt_budget` | 4 | Cap across providers |
| Retry HTTP statuses | — (env-only: `BP_LLM_RETRY_STATUSES`) | 408,409,413,425,429,500–504 | Transient → retry/failover |
| Retry base delay | `llm.retry_base_delay_ms` | 250 | First backoff step |
| Retry max delay | `llm.retry_max_delay_ms` | 5000 | Backoff ceiling |
| Retry jitter | `llm.retry_jitter` | 0.2 | ±fraction around each step |

- **failover:** try active first, then others; pin successful provider for later rounds in the same turn
- **round_robin:** rotate via `storage/.../llm/round_robin.cursor`
- **switch:** only `ActiveProvider`
- Non-transient errors stop immediately (no failover)
- Transient errors (retry HTTP statuses, connect/timeout, **SSE transport incomplete**) retry within the provider's `MaxAttempts`, then failover, until the total budget is spent
- Provider state is not persisted between turns.

### Retry backoff

Between transient attempts the router waits `base·2^(n-1)` (retry `n`, 1-based) perturbed by `±jitter`, capped at the max delay — never an immediate re-hit. A provider **`Retry-After`** header (delta-seconds *or* HTTP-date) overrides the computed delay, still capped by the max. Waits honor `context` cancellation/deadline: once the turn context is done the router never sleeps and returns the last provider error. Each wait logs `webchat.llm.retry_backoff` (WARN, `trace_id`) with `provider`, `attempt`, `delay_ms`, `retry_after_ms`, `status`, and `kind` (`sse_transport` \| `http_status` \| `connect`).

### Retry behavior

## Model ref

Successful results carry `ModelRef{Provider, ID, API}` (provider slot id + configured model string + dialect). Worker stores this on reasoning / tool_call / agent_message / turn.completed payloads for UI badges.

## What is not supported

- Claude Code OAuth/account import (use the official Claude API key provider)
- Non–OpenAI-compatible vendor protocols without a compatible proxy
- Mutation or admin tools in the LLM tools array (product lock — see [Architecture](README.md#tools))

## Vision

Image attachments and `llm.vision`: see [LLM vision](llm-vision.md).

## Reasoning effort

`llm.effort` in `storage/config.json` (default `auto`): see [LLM effort](llm-effort.md). Applied on both chat completions and Responses when the active model advertises support; omitted otherwise.

## Related

- [Turn loop](turn-loop.md) — when Chat is called inside the agent loop
- [XML / pipe tool calls](xml-tool-calls.md) — fallback parser for models that emit tool calls as text (fenced XML, Anthropic native, `<tool_use>`, Kimi K2 pipe)
- [LLM vision](llm-vision.md) — multimodal gate + request shapes
- [LLM effort](llm-effort.md) — reasoning_effort / reasoning.effort policy
- [Runbook](../operations/runbook.md) — env examples for local stub vs real
- [Portable AI kit](portable-ai-kit.md) — copy boundary; rename env prefix on embed

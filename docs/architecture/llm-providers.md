# LLM providers

Grounded in `internal/config` + `internal/infrastructure/service/llm/`.

There is **no Anthropic/Claude-native client**. Upstream must speak **OpenAI-compatible** HTTP: either Chat Completions (`chat`) or the Responses API (`responses`). OpenRouter, local OpenAI-shaped proxies, and similar gateways work via config slots — not via a separate Claude SDK.

## Stub vs real

| Mode | When | Behavior |
|---|---|---|
| **Stub** | `BP_LLM_STUB` = true, **or** unset and **no** provider `API_KEY` | Worker skips HTTP; appends canned `agent_message` + `turn.completed` |
| **Real** | stub false **and** at least one configured provider has an API key | `llm.Router` → `llm.Client` HTTP |

Default stub when no key is intentional (ready-to-use local turns). Set a key and `BP_LLM_STUB=false` (or omit stub once a key exists — `config.Load` defaults stub to `!anyKey`).

## Provider slots (config)

Providers are **named slots**, not hardcoded vendor packages. Configure them in `storage/config.json` under `llm.providers[]` (preferred — see [settings-config.md](settings-config.md)). Env (`BP_LLM_PROVIDERS` + `BP_LLM_<ID>_*`) still works as bootstrap and is what runs before `config.json` exists or has no providers.

For each `ID`:

| Knob | Env | JSON (`llm.providers[]`) |
|---|---|---|
| Base URL | `BP_LLM_<ID>_BASE_URL` | `base_url` (no trailing slash) |
| Bearer token | `BP_LLM_<ID>_API_KEY` | `api_key` |
| Model id | `BP_LLM_<ID>_MODEL` | first `models[].id` |
| Dialect | `BP_LLM_<ID>_API` | `api` (`responses` default, or `chat`) |
| HTTP timeout | `BP_LLM_<ID>_TIMEOUT_SEC` | `timeout_sec` (default 60) |
| Per-provider attempts | `BP_LLM_<ID>_MAX_ATTEMPTS` | `max_attempts` (default 1) |
| Enable toggle | `BP_LLM_<ID>_ENABLED` | `enabled` |
| Weight / sizing | `BP_LLM_<ID>_WEIGHT`, `…_CONTEXT_WINDOW`, `…_MAX_OUTPUT_TOKENS`, `…_MAX_INPUT_TOKENS` | `weight` (sizing uses defaults in v1) |

Example (`storage/config.json`):

```json
{
  "llm": {
    "stub": false,
    "providers": [
      {
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

Legacy env equivalent (still honored when no file providers are configured):

```bash
BP_LLM_STUB=false
BP_LLM_PROVIDERS=OPENROUTER
BP_LLM_OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
BP_LLM_OPENROUTER_API_KEY=sk-or-...
BP_LLM_OPENROUTER_MODEL=openai/gpt-4o-mini
BP_LLM_OPENROUTER_API=responses
```

Add another slot by adding another `providers[]` entry (or listing the ID and defining the same `BP_LLM_<ID>_*` keys — still OpenAI-shaped). Invalid `api` values fall back to `responses`.

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

### Streaming

Controlled by **`BP_LLM_STREAM`** / **`llm.stream`** in `storage/config.json` (bool, **default `true`**). Global only (no per-provider override).

When enabled (default):

- Sets `stream=true` and `Accept: text/event-stream, application/json`
- If body looks like SSE (`text/event-stream` or `data:` / `event:` prefix), `stream.go` aggregates events into one final payload
- Non-stream JSON responses still parse when the proxy returns JSON instead of SSE

When disabled (`BP_LLM_STREAM=false`):

- Sets `stream=false` and `Accept: application/json`
- Uses the JSON unmarshal path only

**Auto-fallback:** if stream was requested and the provider rejects streaming (HTTP 4xx whose body mentions stream/streaming unsupported, or a JSON error object about stream — not auth/quota 401/402/403), the client **retries once** with `stream=false` and logs `webchat.llm stream_fallback …`.

**Transport retry (Codex-like):** Responses SSE that ends without `response.completed`, emits `response.incomplete`, or dies mid-read is marked **Transient** (`SSE_TRANSPORT` / `ErrSSETransport`). The existing router budget (`BP_LLM_TOTAL_ATTEMPT_BUDGET` + per-provider `MaxAttempts`) retries the same LLM request — no parallel retry stack. This is separate from the worker empty-response nudge (semantic, after a completed round).

Prefer leaving stream on for local OpenAI-compatible proxies that degrade Responses when forced non-stream. Use `BP_LLM_STREAM=false` only when you know the upstream cannot SSE.

**Important:** application SSE to the browser (`/events`) is **not** LLM token streaming. It tails durable JSONL seq after the worker appends items. LLM SSE is internal to the client→provider hop.

## Router + circuit

`llm.NewRouter` wraps the client. These knobs live in `storage/config.json` under `llm.*` (preferred) with env (`BP_LLM_*`) as bootstrap fallback — see [settings-config.md](settings-config.md).

| Setting | Env | JSON | Default | Role |
|---|---|---|---|---|
| Strategy | `BP_LLM_STRATEGY` | `llm.strategy` | `failover` | `failover` \| `round_robin` \| `switch` |
| Active provider | `BP_LLM_ACTIVE_PROVIDER` | `llm.active_provider` | first enabled (sorted) | Preferred head of candidate list |
| Attempt budget | `BP_LLM_TOTAL_ATTEMPT_BUDGET` | `llm.total_attempt_budget` | 4 | Cap across providers |
| Circuit threshold | `BP_LLM_CIRCUIT_FAILURE_THRESHOLD` | `llm.circuit_failure_threshold` | 3 | Open after N failures |
| Circuit cooldown | `BP_LLM_CIRCUIT_COOLDOWN_SEC` | `llm.circuit_cooldown_sec` | 60 | Skip open providers until cool |
| Retry HTTP statuses | `BP_LLM_RETRY_STATUSES` | — (env-only) | 408,409,413,425,429,500–504 | Transient → retry/failover |
| Retry base delay | `BP_LLM_RETRY_BASE_DELAY_MS` | `llm.retry_base_delay_ms` | 250 | First backoff step |
| Retry max delay | `BP_LLM_RETRY_MAX_DELAY_MS` | `llm.retry_max_delay_ms` | 5000 | Backoff ceiling |
| Retry jitter | `BP_LLM_RETRY_JITTER` | `llm.retry_jitter` | 0.2 | ±fraction around each step |

- **failover:** try active first, then others; pin successful provider for later rounds in the same turn
- **round_robin:** rotate via `storage/.../llm/round_robin.cursor`
- **switch:** only `ActiveProvider`
- Non-transient errors stop immediately (no failover)
- Transient errors (retry HTTP statuses, connect/timeout, **SSE transport incomplete**) retry within the provider's `MaxAttempts`, then failover, until the total budget is spent
- Circuit state lives under `{StorageRoot}/llm/`

### Retry backoff

Between transient attempts the router waits `base·2^(n-1)` (retry `n`, 1-based) perturbed by `±jitter`, capped at the max delay — never an immediate re-hit. A provider **`Retry-After`** header (delta-seconds *or* HTTP-date) overrides the computed delay, still capped by the max. Waits honor `context` cancellation/deadline: once the turn context is done the router never sleeps and returns the last provider error. Each wait logs `webchat.llm.retry_backoff` (WARN, `trace_id`) with `provider`, `attempt`, `delay_ms`, `retry_after_ms`, `status`, and `kind` (`sse_transport` \| `http_status` \| `connect`).

### Circuit (half-open, cross-process)

The per-provider circuit is a **closed → open → half-open → closed/open** machine persisted to `{StorageRoot}/llm/provider_state.json`:

- **closed** — normal; transient failures accumulate. At `FailureThreshold` the circuit **opens** (records `opened_at`).
- **open** — within `CooldownSec` the provider is dropped from the candidate list (fail fast / fail over).
- **half-open** — after cooldown a single probe is leased (`probe_at`); concurrent turns fail fast on that provider and use an alternate. A stale probe lease (older than the cooldown-derived TTL) is reclaimable so a crashed probe never wedges the provider.
- **probe result** — success fully closes and resets failures; a transient failure reopens with a fresh cooldown.

Only transient/provider failures count; **auth/validation errors (401/402/403, 4xx) never trip the circuit**. Writes are atomic (temp file + rename) under an advisory file lock (`flock` on `provider_state.lock`, Linux/macOS) plus an in-process mutex, so goroutines and separate processes cannot corrupt or clobber state. A missing or corrupt file recovers safely as all-closed with a `webchat.llm.circuit … state=corrupt_reset` WARN. Transitions log `webchat.llm.circuit` (WARN/INFO, `trace_id`) with `state` (`open` \| `closed` \| `half_open_probe`) and `reason`. If all providers are open, the router falls back to trying them anyway (degraded last resort) rather than locking the user out.

## Model ref

Successful results carry `ModelRef{Provider, ID, API}` (provider slot id + configured model string + dialect). Worker stores this on reasoning / tool_call / agent_message / turn.completed payloads for UI badges.

## What is not supported

- Native Anthropic Messages API / Claude SDK
- Non–OpenAI-compatible vendor protocols without a compatible proxy
- Mutation or admin tools in the LLM tools array (product lock — see [Architecture](README.md#tools))

## Vision

Image attachments and `BP_LLM_VISION`: see [LLM vision](llm-vision.md).

## Reasoning effort

`BP_LLM_EFFORT` / `llm.effort` in `storage/config.json` (default `auto`): see [LLM effort](llm-effort.md). Applied on both chat completions and Responses when the active model advertises support; omitted otherwise.

## Related

- [Turn loop](turn-loop.md) — when Chat is called inside the agent loop
- [XML / pipe tool calls](xml-tool-calls.md) — fallback parser for models that emit tool calls as text (fenced XML, Anthropic native, `<tool_use>`, Kimi K2 pipe)
- [LLM vision](llm-vision.md) — multimodal gate + request shapes
- [LLM effort](llm-effort.md) — reasoning_effort / reasoning.effort policy
- [Runbook](../operations/runbook.md) — env examples for local stub vs real
- [Portable AI kit](portable-ai-kit.md) — copy boundary; rename env prefix on embed

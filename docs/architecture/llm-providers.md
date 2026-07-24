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

Providers are **named slots**, not hardcoded vendor packages. `BP_LLM_PROVIDERS` is a comma list of IDs (default `OPENROUTER`). For each `ID`:

| Env | Meaning |
|---|---|
| `BP_LLM_<ID>_BASE_URL` | Base URL without trailing slash (client appends path) |
| `…_API_KEY` | Bearer token |
| `…_MODEL` | Model id string (e.g. `openai/gpt-4o-mini`) |
| `…_API` | `responses` (default) or `chat` |
| `…_TIMEOUT_SEC` | Per-request HTTP timeout (default 60) |
| `…_MAX_ATTEMPTS` | Attempts on this provider before failover (default 1) |
| `…_ENABLED` | Include in candidate set (default true) |
| `…_WEIGHT`, `…_CONTEXT_WINDOW`, `…_MAX_OUTPUT_TOKENS`, `…_MAX_INPUT_TOKENS` | Sizing / optional weight |

Example:

```bash
BP_LLM_STUB=false
BP_LLM_PROVIDERS=OPENROUTER
BP_LLM_OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
BP_LLM_OPENROUTER_API_KEY=sk-or-...
BP_LLM_OPENROUTER_MODEL=openai/gpt-4o-mini
BP_LLM_OPENROUTER_API=responses
```

Add another slot by listing it and defining the same `BP_LLM_<ID>_*` keys (still OpenAI-shaped). Invalid `API` values fall back to `responses`.

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

Controlled by **`BP_LLM_STREAM`** (bool, **default `true`**). Global only (no per-provider override).

When enabled (default):

- Sets `stream=true` and `Accept: text/event-stream, application/json`
- If body looks like SSE (`text/event-stream` or `data:` / `event:` prefix), `stream.go` aggregates events into one final payload
- Non-stream JSON responses still parse when the proxy returns JSON instead of SSE

When disabled (`BP_LLM_STREAM=false`):

- Sets `stream=false` and `Accept: application/json`
- Uses the JSON unmarshal path only

**Auto-fallback:** if stream was requested and the provider rejects streaming (HTTP 4xx whose body mentions stream/streaming unsupported, or a JSON error object about stream — not auth/quota 401/402/403), the client **retries once** with `stream=false` and logs `webchat.llm stream_fallback …`.

Prefer leaving stream on for local OpenAI-compatible proxies that degrade Responses when forced non-stream. Use `BP_LLM_STREAM=false` only when you know the upstream cannot SSE.

**Important:** application SSE to the browser (`/events`) is **not** LLM token streaming. It tails durable JSONL seq after the worker appends items. LLM SSE is internal to the client→provider hop.

## Router + circuit

`llm.NewRouter` wraps the client:

| Setting | Env | Default | Role |
|---|---|---|---|
| Strategy | `BP_LLM_STRATEGY` | `failover` | `failover` \| `round_robin` \| `switch` |
| Active provider | `BP_LLM_ACTIVE_PROVIDER` | first enabled (sorted) | Preferred head of candidate list |
| Attempt budget | `BP_LLM_TOTAL_ATTEMPT_BUDGET` | 4 | Cap across providers |
| Circuit threshold | `BP_LLM_CIRCUIT_FAILURE_THRESHOLD` | 3 | Open after N failures |
| Circuit cooldown | `BP_LLM_CIRCUIT_COOLDOWN_SEC` | 60 | Skip open providers until cool |
| Retry HTTP statuses | `BP_LLM_RETRY_STATUSES` | 408,409,413,425,429,500–504 | Transient → retry/failover |

- **failover:** try active first, then others; pin successful provider for later rounds in the same turn
- **round_robin:** rotate via `storage/.../llm/round_robin.cursor`
- **switch:** only `ActiveProvider`
- Non-transient errors stop immediately (no failover)
- Circuit state lives under `{StorageRoot}/llm/`

## Model ref

Successful results carry `ModelRef{Provider, ID, API}` (provider slot id + configured model string + dialect). Worker stores this on reasoning / tool_call / agent_message / turn.completed payloads for UI badges.

## What is not supported

- Native Anthropic Messages API / Claude SDK
- Non–OpenAI-compatible vendor protocols without a compatible proxy
- Mutation or admin tools in the LLM tools array (product lock — see [Architecture](README.md#tools))

## Related

- [Turn loop](turn-loop.md) — when Chat is called inside the agent loop
- [Runbook](../operations/runbook.md) — env examples for local stub vs real
- [Portable AI kit](portable-ai-kit.md) — copy boundary; rename env prefix on embed

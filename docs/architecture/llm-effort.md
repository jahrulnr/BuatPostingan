# LLM reasoning effort

Grounded in `internal/infrastructure/service/llm/effort_policy.go`, request builders in `client.go`, and `BP_LLM_EFFORT`.

## Config

| Value | Behavior |
|---|---|
| `auto` (default) | Probe OpenRouter-style `GET {base}/models` for a `reasoning` object (or `supported_parameters` containing `reasoning` / `reasoning_effort`). If supported, send catalog `default_effort` (or `medium` if missing). If `default_effort` is `none`, **omit** (reasoning off by default). If probe fails, use conservative family heuristics; otherwise omit. |
| `none` / `minimal` / `low` / `medium` / `high` / `xhigh` / `max` | Send that level **only when** the model advertises effort support (catalog or heuristic). Clamp to nearest `supported_efforts` when the catalog lists them. Never send `none` to `mandatory` models. Unsupported models → omit (avoid 400s). |

```bash
BP_LLM_EFFORT=auto   # recommended
# BP_LLM_EFFORT=medium
# BP_LLM_EFFORT=high
# BP_LLM_EFFORT=none
```

Aliases: `off`→`none`, `min`→`minimal`, `med`→`medium`, `x-high`/`extra`→`xhigh`, `maximum`→`max`. Unknown values fall back to `auto`.

## How to use (providers)

### OpenAI Chat Completions

Top-level `reasoning_effort` on reasoning models (o-series, GPT-5 family, etc.). Supported values are **model-dependent** and can include `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`.

Source: [OpenAI Reasoning models](https://developers.openai.com/api/docs/guides/reasoning)

### OpenAI Responses API

Nested object (preferred for `BP_LLM_*_API=responses`):

```json
{ "reasoning": { "effort": "medium" } }
```

Same effort enum; higher effort → more reasoning tokens, latency, and cost. Streaming still works — effort only changes compute budget, not the SSE transport.

Sources: [Reasoning guide](https://developers.openai.com/api/docs/guides/reasoning), [Deployment checklist](https://developers.openai.com/api/docs/guides/deployment-checklist)

### OpenRouter (unified)

Prefer the `reasoning` object on chat/completions (and compatible Responses proxies):

```json
{
  "reasoning": {
    "effort": "high",
    "exclude": false
  }
}
```

Also accepts OpenAI-style top-level `reasoning_effort`. Effort values: `max`, `xhigh`, `high`, `medium`, `low`, `minimal`, `none`. Anthropic-style models may use `reasoning.max_tokens` (token budget) instead of or alongside effort — this repo currently sends **effort only** (not max_tokens).

Legacy `include_reasoning` is deprecated in favor of `reasoning.exclude`.

Source: [OpenRouter Reasoning Tokens](https://openrouter.ai/docs/guides/best-practices/reasoning-tokens), [API parameters](https://openrouter.ai/docs/api/reference/parameters)

### What is real vs marketing

| Level | Real? | Notes |
|---|---|---|
| `none` | Yes | Disables reasoning when the model allows it |
| `minimal` | Yes | OpenAI + OpenRouter; ~10% budget on OR mapping |
| `low` / `medium` / `high` | Yes | Common across OpenAI + OpenRouter |
| `xhigh` | Yes | Newer GPT-5.x / OR gateway; not every model |
| `max` | Yes on some | OR treats similarly to `xhigh` (~95% of max_tokens); OpenAI model-dependent |

Always treat the catalog / model card as source of truth — sending an unsupported value can 400.

## Detecting support before send

OpenRouter `GET /api/v1/models` entries may include:

```json
{
  "id": "openai/o3-mini",
  "supported_parameters": ["reasoning", "tools"],
  "reasoning": {
    "supported_efforts": ["high", "medium", "low", "minimal"],
    "default_effort": "medium",
    "mandatory": false,
    "supports_max_tokens": false
  }
}
```

| Field | Meaning |
|---|---|
| `reasoning` object present | Model exposes effort (or related) controls |
| `supported_efforts` | Allowed levels (descending). `null` → all gateway values; **omitted** → no effort selection |
| `default_effort` | Catalog default; `"none"` means off-by-default |
| `mandatory` | Do not send `effort: "none"` |
| `supported_parameters` | Fallback signal if `reasoning` / `reasoning_effort` listed |

Non-reasoning models omit `reasoning`. Probe result is cached per `baseURL + model` (same pattern as vision).

**Gaps:** Some OpenAI-compatible proxies do not return OpenRouter’s `reasoning` metadata. Then we fall back to heuristics (`o1`/`o3`/`gpt-5`/`deepseek-r1`/`claude-*-4`/`gemini-2.5`/`qwq`/…). Plain chat models (e.g. `gpt-4o`, `deepseek-chat`) stay omit-by-default. If a proxy accepts effort without advertising it, set an explicit `BP_LLM_EFFORT` only after verifying — or extend the heuristic list carefully.

**Error behavior:** Unsupported params typically return HTTP 400 from OpenAI/OpenRouter. This policy omits the field when support is unknown/false to avoid that.

## Request shapes in this repo

**Chat** (`API=chat`) — both shapes for OpenRouter + OpenAI chat:

```json
{
  "reasoning_effort": "medium",
  "reasoning": { "effort": "medium" }
}
```

**Responses** (`API=responses`) — nested only:

```json
{
  "reasoning": { "effort": "medium" }
}
```

Effort is applied in `Client.chatViaCompletions` / `chatViaResponses` after the base body is built. Streaming (`BP_LLM_STREAM`) is orthogonal — the same effort object is sent with `stream: true|false`.

## Worker / tools guidance

- Higher effort can improve multi-step tool planning but increases latency and reasoning token cost on every LLM round (including tool-call rounds).
- Prefer `auto` or `medium` for interactive webchat; reserve `high`/`xhigh` for eval-proven hard tasks.
- Do not put effort instructions in prompts as a substitute for the API param — use the config knob.
- Vision (`BP_LLM_VISION`) is independent; effort does not change multimodal gating.

## Related

- [LLM providers](llm-providers.md)
- [LLM vision](llm-vision.md) — same probe/cache style
- [Turn loop](turn-loop.md)
- OpenAI reasoning: https://developers.openai.com/api/docs/guides/reasoning
- OpenRouter reasoning tokens: https://openrouter.ai/docs/guides/best-practices/reasoning-tokens

# Docs — BuatPostingan

Human + agent documentation grounded in **current code**. Start here, then dive by topic.

## Index

| Doc | What it covers |
|---|---|
| [Architecture](architecture/README.md) | Clean Architecture layers, foldering, tools, persistence, frontend overview |
| [Turn loop](architecture/turn-loop.md) | StartTurn → worker → LLM/tools → JSONL → SSE end-to-end |
| [LLM providers](architecture/llm-providers.md) | OpenAI-compatible `chat` / `responses`, stub, env, router/circuit |
| [Portable AI kit](architecture/portable-ai-kit.md) | What to copy vs leave when reusing the webchat stack |
| [Runbook](operations/runbook.md) | `make be` / `make fe`, stub vs real LLM, `?mock=1` |
| [API service notes](api-service/README.md) | Pointer to `/api/webchat` surface |
| [Knowledge corpus](webchat/) | Markdown docs the agent can search (`docs/webchat`) |

## Quick start

```bash
cp .env.example .env   # optional
make be                # Go + static FE → http://localhost:8080/
# or: make fe          # static only :5173; needs make be for real API
```

Without provider API keys the backend runs **LLM stub** (canned replies). Details: [Runbook](operations/runbook.md), [LLM providers](architecture/llm-providers.md).

## Agent entry

Coding agents should also read root [`AGENTS.md`](../AGENTS.md). Architecture non-negotiables and the portable kit boundary live under [`docs/architecture/`](architecture/README.md).

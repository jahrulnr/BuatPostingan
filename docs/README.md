# Docs — BuatPostingan

Human + agent documentation grounded in **current code**. Start here, then dive by topic.

## Index

| Doc | What it covers |
|---|---|
| [Architecture](architecture/README.md) | Clean Architecture layers, foldering, tools, persistence, frontend overview |
| [Turn loop](architecture/turn-loop.md) | StartTurn → worker → LLM/tools → JSONL → SSE end-to-end |
| [Realtime streaming](architecture/realtime-streaming.md) | FE↔BE perceived realtime vs ChatGPT/Claude/Cursor patterns; P0/P1 gaps |
| [LLM providers](architecture/llm-providers.md) | OpenAI-compatible `chat` / `responses`, stub, env, router/circuit |
| [LLM vision](architecture/llm-vision.md) | Multimodal image parts, `BP_LLM_VISION`, capability gate |
| [LLM effort](architecture/llm-effort.md) | Reasoning effort (`BP_LLM_EFFORT`), catalog probe, request shapes |
| [LLM model picker](architecture/llm-model-picker.md) | Composer model + effort UI, `GET /models`, StartTurn overrides |
| [XML / pipe tool calls](architecture/xml-tool-calls.md) | Fenced, Anthropic native, `<tool_use>`, Kimi K2 pipe — parsing + stream recovery |
| [Settings + JSON config](architecture/settings-config.md) | UI settings, `storage/config.json`, env merge, `/api/settings` |
| [Observability](architecture/observability.md) | `trace_id` middleware, worker propagation, grep failing turns |
| [Portable AI kit](architecture/portable-ai-kit.md) | What to copy vs leave when reusing the webchat stack |
| [Codex gap analysis](architecture/codex-gap-analysis.md) | Codex vs BP prioritized gaps (compaction, title, retry/circuit, N/A coding features) |
| [Skills tools](architecture/skills-tools.md) | Progressive skill discovery (`list_skills` / `read_skill`) |
| [MCP support](architecture/mcp-support.md) | MCP client (meta-tools), `mcp.servers` config, mutation deny |
| [Runbook](operations/runbook.md) | `make be` / `make fe`, stub vs real LLM, `?mock=1` |
| [API service notes](api-service/README.md) | Pointer to `/api/webchat` surface |
| [Knowledge corpus](webchat/) | Markdown docs the agent can search (`docs/webchat`) |

## Quick start

```bash
cp .env.example .env   # optional
make be                # Go + static FE → http://localhost:8080/
# or: make fe          # static + livereload :5173 (needs Node/npx); needs make be for real API
```

Without provider API keys the backend runs **LLM stub** (canned replies). Details: [Runbook](operations/runbook.md), [LLM providers](architecture/llm-providers.md).

## Agent entry

Coding agents should also read root [`AGENTS.md`](../AGENTS.md). Architecture non-negotiables and the portable kit boundary live under [`docs/architecture/`](architecture/README.md).

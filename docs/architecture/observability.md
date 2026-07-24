# Observability — trace IDs + structured logs

Lightweight correlation for local and kit embeds: **`log/slog` + `context.Context`**, no Wire/logmanager.

Package: [`internal/pkg/logging`](../../internal/pkg/logging/logging.go).

## Trace ID rules

| Path | Trace ID |
|---|---|
| HTTP request | Middleware accepts `X-Trace-Id`, else W3C `traceparent` (32-hex id), else generates `tr_<32hex>`. Echoes `X-Trace-Id` on the response. |
| HTTP → worker (StartTurn / RetryTurn) | **Same** request id stored on `TurnJob.TraceID` and planted into the worker goroutine context. |
| System-initiated (startup, reindex, orphan recovery, enqueue with no ctx id) | Literal **`system`** (`logging.TraceSystem` / `logging.SystemContext`). |

Choice for system work: a single literal `system` (not `sys_<uuid>` / `system:<op>`), so grepping startup noise is trivial and HTTP turns never collide with it.

## Middleware

`delivery/http.TraceMiddleware` wraps the mux in `NewServer` (after CORS so embeds that only call `MountWebchatAPI` should wrap themselves):

```go
mux := http.NewServeMux()
httpdelivery.MountWebchatAPI(mux, uc)
handler := httpdelivery.TraceMiddleware(mux)
```

## Logging policy

- Prefer **`logging.Logger(ctx)`** / `Info` / `Warn` / `Error` so `trace_id=` appears on every line.
- **ERROR** at boundaries only:
  - HTTP adapter: `writeErr` → `BoundaryHTTPError` (5xx / unknown; skips 4xx noise)
  - Worker: `webchat.turn_failed` / `webchat.turn_panic`
  - Important infra (LLM hard failures bubble to turn_failed; `stream_fallback`, `retry_backoff`, and circuit transitions are WARN)
- Do not re-log the same error on every wrap layer.

Example line shape (text handler on stderr):

```
time=... level=ERROR msg=error trace_id=tr_… op=webchat.turn_failed err="…" thread=thr_… turn=trn_…
```

## Grep a failing turn

1. From the Failed UI (real driver) or response header, copy `X-Trace-Id` / `trace: tr_…`.
2. On the BE terminal (`make be`):

```bash
# match one turn end-to-end (HTTP enqueue + worker + LLM)
rg 'trace_id=tr_YOUR_ID'   # or: grep -F 'trace_id=tr_YOUR_ID'

# system startup / reindex only
rg 'trace_id=system'
```

3. Useful ops once you have the id:

```bash
rg 'trace_id=tr_…' | rg 'webchat\.(turn_failed|turn_start|tool|llm|empty_model_response|reasoning|retry_backoff|stream_fallback|circuit)'
```

**LLM reliability signals:**

```bash
rg 'webchat.llm.retry_backoff'   # WARN per transient retry: provider, attempt, delay_ms, retry_after_ms, status, kind
rg 'webchat.llm.circuit'         # provider circuit transitions: state=open|closed|half_open_probe|corrupt_reset, reason
```

## Finding model “Thinking” / reasoning text

Reasoning is **not** in Go/JS source. It is durable transcript data:

```bash
# thoughts live in JSONL under storage (gitignored runtime data)
rg 'The user is asking' storage/webchat/threads/

# or open the thread file the UI was on
rg '"type":"reasoning"' storage/webchat/threads/thr_….jsonl
```

Workspace search of the repo (default exclude of `storage/`) will miss them. Empty assistant bubbles after Thinking usually mean a **reasoning-only** LLM round (no `output_text`, no tool call). Look for:

```bash
rg 'webchat.empty_model_response'   # WARN with trace_id, provider, model, has_reasoning
rg '\(empty model response\)' storage/webchat/threads/
```

## Frontend

Real driver sends a fresh `X-Trace-Id` per request (`web/js/api/real/driver.js`). On HTTP failure, Failed bubbles show `trace: …` when the response exposes `X-Trace-Id` (CORS: `Access-Control-Expose-Headers`). Durable `turn.failed` / runtime empty-agent items also carry `trace_id` for the same UI.

## Related

- [Turn loop](turn-loop.md)
- [Runbook](../operations/runbook.md)

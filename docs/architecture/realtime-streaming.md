# Realtime streaming

How FE↔BE feel realtime on BuatPostingan’s Go worker + JSONL + SSE + vanilla JS stack.

**Related:** [`turn-loop.md`](turn-loop.md), [`llm-providers.md`](llm-providers.md).

---

## What landed (P0 + P1)

| Piece | Behavior |
|---|---|
| LLM deltas | `llm.WithStreamHooks` / `OnTextDelta` while provider SSE is parsed (`stream.go`); final `Chat` payload unchanged for the tool loop |
| Ephemeral SSE | Worker publishes `item.delta` via in-process hub — **not** written to JSONL, **no** durable `seq` / SSE `id` |
| Durable confirm | Full `agent_message` JSONL append + `item.completed` as before; FE reconciles the live draft (same `item_id` when streamed) |
| Pub/sub hub | `sse.Hub` + `NotifyingStore` wake on every durable append; streamer prefers notify + ephemeral channels |
| Safety poll | Streamer ticker **1.5s** (not 50ms) for missed wakes |
| FE | `item.delta` → live message bubble (rAF-batched plain text); `item.completed` confirms markdown; mock emits fake chunk deltas |
| Durable resume | FE keeps `lastAppliedSeq` per thread, drops duplicate durable frames, and reconnects with `after_seq=<lastAppliedSeq>` |
| Reconnect state | `connecting → open → reconnecting → exhausted`; exponential backoff starts near 400ms, adds jitter, caps at 8s, and stops after six consecutive failures |
| Stream ownership | Every subscription has a generation token; thread switch/completion/interrupt invalidates old callbacks and pending reconnect timers |
| Perceived latency | Optimistic send and `turn.started` create one accessible “Thinking…” placeholder; the first reasoning/tool/delta/message adopts or removes it |
| Scroll anchoring | New activity follows only near the bottom; readers who scroll up get a keyboard-accessible “New activity ↓” affordance |

### Delta event contract

```text
event: item.delta
data: {
  "type": "agent_message",
  "turn_id": "trn_…",
  "item_id": "itm_…",   // provisional; reused on durable append when streamed
  "field": "text",
  "delta": "…"            // incremental fragment (not cumulative)
}
```

Rules for FE:

- Treat `item.delta` as **ephemeral** — do not advance a durable seq cursor from it (payload has no `seq`; SSE `id:` is omitted).
- On first delta for a `turn_id`, open one live agent bubble; append fragments (rAF budget).
- On durable `item.completed` / `agent_message`, replace/confirm that draft (no second bubble).
- If `tool_call` arrives for the same turn after a partial stream, discard the draft (tool-only round).
- Never replay `item.delta`: reconnect only from the last durable seq and wait for the durable final item to reconcile any draft.

### Durable cursor and reconnect

`lastAppliedSeq` is runtime state keyed by `thread_id`. A full thread hydration seeds it from `seq_head`; every later durable SSE frame advances it before rendering. Frames at or below the cursor are ignored, so an EventSource reconnect cannot duplicate bubbles. Ephemeral deltas have no seq and never move the cursor.

On disconnect while a turn remains active, the UI closes the browser EventSource and creates a new one with `after_seq=<lastAppliedSeq>`. Retry uses exponential backoff with jitter (about 0.4s → 0.8s → 1.6s, capped at 8s) and a six-attempt budget. Returning to a visible tab forces an immediate resume when the stream is disconnected or has had no visible event activity for 25 seconds. The backend’s ~15-second SSE comment heartbeat keeps idle proxy connections open; comments are transport keepalives and do not affect the durable cursor.

Each stream captures the active thread and a generation number. Completion, failure/interruption, new chat, and thread switch invalidate that generation, which prevents queued callbacks from an old stream from painting into the new thread. After the retry budget is exhausted, the UI emits one traceable connection error (`sse:<thread>:seq:<cursor>`) and waits for an explicit visibility/open-thread recovery instead of looping alerts.

### Placeholder and scroll behavior

The assistant placeholder is structural feedback, not generated content. It appears as “Thinking…” plus pulsing dots, uses `role=status`, and disables animation under `prefers-reduced-motion`. The first reasoning or tool item removes it; the first text delta or final agent message adopts the same bubble. Completion, interruption, request failure, or exhausted reconnect removes it cleanly.

Delta painting remains rAF-batched plain text. Markdown parsing and sanitization happen once on the durable final message, avoiding repeated expensive markdown work without adding a Web Worker.

### Pub/sub sketch

```text
AppendItem (JSONL) → NotifyingStore → Hub.Notify(threadID)
LLM OnTextDelta     → Worker       → Hub.PublishEphemeral(threadID, "item.delta", …)

Streamer.Subscribe:
  select {
    Hub.Notify      → GetThread(afterSeq) → emit durable events (seq / id)
    Hub.Ephemeral   → emit item.delta (no seq)
    safety ticker   → GetThread (1.5s)
    ctx.Done
  }
```

Single-process only. Multi-instance would need sticky SSE or a shared bus (P2).

---

## Still open (P1+)

| Item | Priority |
|---|---|
| Optional `reasoning.delta` into think bubble | P1 |
| Web Workers / virtualization / tool-arg JSON stream | P2 |

---

## Industry context (research)

Public API proxies (OpenAI Responses / Anthropic Messages) stream typed `*.delta` events over SSE. BuatPostingan already uses browser **SSE + EventSource**; the former gap was granularity (whole JSONL items only) and wake path (50ms poll). WebSocket is not required for reader/instructor chat.

Sources: [OpenAI streaming](https://developers.openai.com/api/docs/guides/streaming-responses), [Anthropic streaming](https://platform.claude.com/docs/en/build-with-claude/streaming).

---

## Verify live typing

1. `make be` with a real provider key and `BP_LLM_STUB=false` (or open `?mock=1` for fake chunk deltas).
2. Open a thread, send a message that yields a long answer (not tool-only).
3. Watch the agent bubble grow before `item.completed`; status may show `Streaming…`.
4. Confirm one bubble after completion (markdown replaces plain draft).
5. Optional: DevTools → EventStream → `item.delta` frames without `id:`, then `item.completed` with `id: <seq>`.
6. Real mode: disconnect the browser network briefly during a turn, restore it, and confirm status moves through `Reconnecting…` without duplicate bubbles.
7. Mock mode: open `?mock=1&mock_disconnect=1`; the first subscription drops once, then resumes from the durable cursor.
8. Scroll upward during a long turn; confirm the timeline stays anchored and “New activity ↓” follows the latest event on activation.

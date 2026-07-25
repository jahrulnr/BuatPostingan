# Codex ↔ BuatPostingan — prioritized gap analysis

Analysis-only. Compares [OpenAI Codex](https://github.com/openai/codex) (`codex-rs/`, local checkout) to BuatPostingan’s **webchat** (Go + JSONL + vanilla FE). Priorities are for *this product*, not for becoming a coding agent. Current dev phase runs with `write_enabled=true` (local full-FS mutation tools on); relock via `BP_WRITE_ENABLED=false`.

**Baseline locks (relock for reader/instructor mode):** `write_enabled=false`; no mutation tools in the LLM tools array; allowlist readers only — see [`AGENTS.md`](../../AGENTS.md) and [Architecture](README.md#tools).

**Sources (grounded):**

| Area | Codex | BuatPostingan |
|---|---|---|
| Compaction | `codex-rs/core/src/compact.rs`, `compact_remote*.rs`, `prompts/src/compact.rs`, `state/auto_compact_window.rs` | **have** — `BP_CONTEXT_COMPACTION_ENABLED` + worker LLM/extractive writer; `context_compacted` read+write in `internal/infrastructure/worker` |
| Title | `preview_from_rollout_items` in `app-server/.../thread_processor.rs`; manual rename / thread `name` | **have** — stub sync truncate; real path async LLM title job (`Worker.maybeAutoTitle`) + manual `RenameThread` |
| Retry / transport | `core/src/responses_retry.rs`, `util::backoff` (exp + jitter) | Router attempt budget + SSE transport retry + **exp backoff + jitter + Retry-After** ([llm-providers.md](llm-providers.md)) |
| Circuit | Provider failover + guardian CB (coding-path) | `llm/circuit.go` closed→open→half-open state machine, **flock + atomic temp/rename**, single-probe |
| Skills / MCP | Flatten MCP + rich skills (`ext/skills`, core session inject) | Progressive meta-tools ([skills-tools.md](skills-tools.md), [mcp-support.md](mcp-support.md)) — intentional |
| Streaming | App-server + TUI events | Worker hub `item.delta` + JSONL seq ([realtime-streaming.md](realtime-streaming.md)) |
| Historical PHP parity | — | AIPedia: live `WebchatContextCompactor` + async `ProcessThreadTitleJob` |

---

## Feature comparison

Status: **have** | **partial** | **missing**. Priority: product value for reader/instructor (P0 = next builds that unblock long sessions / reliability).

| Feature | Codex (cite) | BP status | Priority | Notes |
|---|---|---|---|---|
| Context compaction / summarization | Inline + remote compact; pre/mid-turn; `SUMMARIZATION_PROMPT`; token budget (`compact.rs`, `auto_compact_window.rs`) | **have** | — | Flag default off. When enabled (non-stub), worker estimates tokens before the agent loop; on overflow summarizes older turns via `llm.Router` (`resources/webchat/prompts/compact.md`), persists `context_compacted` with `compacted_through_seq`, keeps recent N turns raw. LLM failure → extractive fallback (turn continues). |
| Conversation auto-title | Preview from first user message; optional manual `name` | **have** | — | Stub: sync truncate of first user text → `title_source=auto`. Real: async goroutine after first completed turn (inherits `trace_id`), LLM ≤6-word title with truncate fallback; skips `manual`. FE polls list briefly while pending + optional `conversation.updated` ephemeral. |
| Interrupt / cancel turn | `turn_interrupt` (TUI/app-server) | **have** | — | Flag file + per-round check ([turn-loop.md](turn-loop.md)). |
| Turn retry (user recovery) | Re-submit / restore compose payload (TUI) | **have** | — | `RetryTurn` + FE retry CTA; interrupted turns not retryable. |
| LLM transport retry | Exp backoff + jitter; WS→HTTPS fallback (`responses_retry.rs`) | **have** | — | Transient (retry statuses, connect, `SSE_TRANSPORT`) retries within budget with exponential backoff + bounded jitter, capped; honors `Retry-After`; respects ctx cancel. Config: `BP_LLM_RETRY_BASE_DELAY_MS`/`_MAX_DELAY_MS`/`_JITTER`. WARN `webchat.llm.retry_backoff`. |
| Provider circuit / failover | Failover + analytics; guardian CB for denials | **have** | — | Failover/round-robin/switch + closed→open→half-open→closed/open. Cross-process safe (`flock` + atomic temp/rename on `provider_state.json`); single half-open probe (concurrent → fail fast / alternate); auth/validation errors don't trip; corrupt/stale state recovers with WARN; transitions log `webchat.llm.circuit`. |
| Turn rate limit | Account quota UI + snapshots (`GetAccountRateLimits`) | **partial** | **P2** | BP has per-admin turn RL (`TurnRateLimitPerMin` → 429). Codex **account** quota UX is ChatGPT-product — not required for local kit. |
| Model + effort picker | Model/effort in session/turn settings | **have** | — | [llm-model-picker.md](llm-model-picker.md), [llm-effort.md](llm-effort.md). |
| Attachments / vision | Images in user input; `view_image` tool | **have** | — | Uploads + `read_image` / multimodal gate ([llm-vision.md](llm-vision.md)). |
| Skills | Rich catalog, inject, installers (`ext/skills`) | **have** (simpler) | **P2** | Progressive `list_skills` / `read_skill` is enough for instructor; Codex installers/dynamic selectors are coding-agent scale. |
| MCP | Flattened tools + connectors | **have** (different shape) | — | Meta-tools by design ([mcp-support.md](mcp-support.md)). Do not flatten for this product. |
| Token / text streaming | Live deltas to UI | **have** | — | Ephemeral `item.delta` + durable confirm. |
| SSE resume / reconnect polish | Session resume APIs | **partial** | **P1** | Durable seq + EventSource exist; open P1: `lastAppliedSeq`, visibility reconnect ([realtime-streaming.md](realtime-streaming.md)). |
| Thread list / rename / CRUD | Thread store + search by title | **have** | — | JSONL session index + rename. |
| Thread fork / rollback | `fork_thread`, `thread_rollback` | **missing** | **P2** | Useful for “branch this lesson”; not blocking instructor MVP. |
| Speak floor / multi-admin | — (single-user TUI) | **have** | — | BP-only product control. |
| Observability | Analytics + tracing crates | **have** (light) | **P2** | `trace_id` + slog ([observability.md](observability.md)). No metrics/dashboards yet. |
| Streaming UX polish | Rich TUI status | **partial** | **P1** | Skeleton on `turn.started`, scroll-lock, reasoning.delta — listed open in realtime-streaming. |
| User-facing error copy / recovery | Warnings + rate-limit menus | **partial** | **P1** | FE toast for 429/retry; could map provider/circuit failures to clearer next actions. |

---

## Skip / N/A (coding-agent Codex)

Do **not** schedule for BuatPostingan under current locks:

| Codex capability | Why N/A |
|---|---|
| `apply_patch`, `exec_command` / shell, sandbox modes | Host write/shell — BP mutation tools (`write_file`/`edit_file`/`delete_file`) cover file changes in dev phase; shell exec and `apply_patch` remain out of scope |
| Approval queues, guardian review CB, permission profiles | Coding safety UX for mutating tools |
| Image generation / write plugins | Mutation / generative side-effects |
| Workspace trust, git SHA/branch session metadata | Coding workspace identity |
| Elevated Windows sandbox / provisioning | Desktop coding runtime |
| Flattened MCP as primary tool array | Product chose meta-tools; flatten blows tool budget |
| Multi-agent / subagent orchestration | Coding parallel agents |
| Account rate-limit **credits** / ChatGPT billing reset | SaaS account product, not local kit |
| Remote compaction API as mandatory dependency | Optional later; extractive/local summary sufficient for P0 |

---

## Top 5 recommended next builds

1. ~~**Add exponential backoff (+ honor Retry-After) on transient LLM retries**~~ — **Done**: backoff + jitter + `Retry-After`, ctx-aware ([llm-providers.md](llm-providers.md#retry-backoff)).  
2. ~~**Harden provider circuit (file lock + half-open probe)**~~ — **Done**: closed→open→half-open with `flock` + atomic writes ([llm-providers.md](llm-providers.md#circuit-half-open-cross-process)).  
3. **FE SSE reconnect + turn-started skeleton** — Perceived realtime and recovery after tab sleep; already scoped P1 in [realtime-streaming.md](realtime-streaming.md).  
4. **Thread fork / rollback (optional)** — Useful for “branch this lesson”; not blocking instructor MVP.  
5. **User-facing error copy / recovery** — Map provider/circuit failures to clearer next actions beyond toast/retry.  

---

## Related

- [Turn loop](turn-loop.md)  
- [Realtime streaming](realtime-streaming.md)  
- [LLM providers](llm-providers.md)  
- [MCP support](mcp-support.md)  
- [Skills tools](skills-tools.md)  
- [Portable AI kit](portable-ai-kit.md)  

# Portable AI kit — copy boundary

**Verdict: Partial.** The Go webchat stack is structured for reuse (ports + adapters), but it is not a separate Go module yet. Copy-paste into another repo works if you rename the module path and supply product shell + config + knowledge docs.

Do **not** rename `buatpostingan` → a new shared module in this PR. Prefer this kit list + injected config until a second consumer proves the boundary.

## What to copy (AI core)

| Path | Role |
|---|---|
| `internal/domain/` | Entities, ports (`repository`, `service`), value objects, enums |
| `internal/usecase/webchat/` | Orchestration (`Usecase` + `Service`) |
| `internal/infrastructure/repository/jsonl/` | Durable JSONL + lock + interrupt + speak-floor |
| `internal/infrastructure/ratelimit/` | Turn rate limiter |
| `internal/infrastructure/service/docs/` | Lexical docs index |
| `internal/infrastructure/service/tools/` | Allowlisted host tools |
| `internal/infrastructure/service/llm/` | Client, router, circuit, SSE parse |
| `internal/infrastructure/worker/` | Agent turn loop |
| `internal/infrastructure/sse/` | EventStreamer (JSONL seq → SSE) |
| `internal/infrastructure/stub/` | Optional 501 stubs for tests |
| `internal/pkg/apperr`, `idgen`, `redact` | Shared kitchen helpers |
| `internal/config/` | Env loader (`WEBCHAT_*` portable; `BP_*` product alias) |
| `delivery/http/webchat_handler.go` (+ helpers used by it) | Thin `/api/webchat` adapter |
| `delivery/presenter/` | Contract-stable JSON/SSE shapes |
| `resources/webchat/prompts/` | system / developer / user templates *(edit product voice)* |
| `resources/webchat/tools/` | `*.tool.json` schemas |

Optional FE widget kit (same API, no product chrome):

| Path | Role |
|---|---|
| `web/js/api/` | Dual-driver mock \| real |
| `web/js/ui/chat.js`, `web/js/ui/render.js` | Mount via `bootChat({ root })` |
| `web/css/webchat.css`, `web/css/webchat-conversations.css` | Chat surface styles |

## What to leave (product shell)

| Path | Why |
|---|---|
| `web/index.html`, `web/css/shell.css`, `web/js/app.js` | BuatPostingan branding / full-page chrome |
| `cmd/app/` | Product DI, listen addr, static mount |
| `docs/webchat/` | Product knowledge corpus (swap for consumer docs) |
| `docs/architecture/` (except this kit doc) | This repo’s narrative |
| `Makefile`, `bin/`, `storage/` runtime | Host-specific |
| Product welcome copy / `__WC_PRODUCT_NAME__` | Branding |

## Consumer must provide

1. **Go module rename** — replace import prefix `buatpostingan/...` (or use `replace` / go.work later).
2. **DI entry** — mirror `cmd/app/main.go`: mkdir storage subs, `docs.NewIndex` + Reindex, `tools.NewRegistry`, `llm` client/router, `worker.New`, `sse.NewStreamer`, `webchat.NewService`, then mount HTTP.
3. **Config** — set roots via env (prefer portable `WEBCHAT_*`; `BP_*` also works here):
   - `WEBCHAT_STORAGE_ROOT`, `WEBCHAT_DOCS_ROOT`, `WEBCHAT_PROMPTS_ROOT`, `WEBCHAT_TOOLS_ROOT`
   - LLM: `WEBCHAT_LLM_*` / provider keys; stub when no key
4. **Knowledge** — consumer’s own Markdown under docs root (not BuatPostingan writing guides unless wanted).
5. **HTTP mount** — call `httpdelivery.MountWebchatAPI(mux, uc)` (+ optional `MountHealthz` / `MountStaticWeb`). Do not require product `web/` static files.
6. **FE shell or widget** — either embed full page, or a floating host that includes the required DOM ids and calls `bootChat({ root })` against the same `/api/webchat`.

## Coupling that blocks “blind copas”

| Coupling | Severity | Mitigation |
|---|---|---|
| Module path `buatpostingan/...` | High | sed / go mod rename on copy |
| `BP_*` env names | Low | `WEBCHAT_*` aliases already accepted |
| Product prompts / docs content | Medium | Replace after copy |
| FE shell branding | Medium | Leave shell; copy api + ui only |
| Static FE served from `NewServer` | Low | Use `MountWebchatAPI` alone |

## Suggested next consumers

| Consumer | Approach |
|---|---|
| Floating widget | Thin host page/component + same API + `bootChat({ root })`; hide rail if unused |
| AIPedia Go | Copy AI core dirs; swap Laravel PHP loop for this worker; keep AIPedia docs/prompts |
| Other Go app | Mount API on existing mux; inject storage/docs paths |

## Non-negotiables (stay in the kit)

- Chat history = JSONL (no SQL chat tables)
- Agent loop in worker, not HTTP handler
- SSE tails durable seq
- `write_enabled = false` / no mutation tools

## Tool dialect notes

- **`grep`**: prefers host `rg` (`exec.Command`, no shell) with `--json -e <pattern> -- <sandbox-path>`; falls back to Go `regexp` (RE2) when `rg` is missing. Both dialects are linear-time regex (no backrefs/lookaround). Pattern is never passed through a shell. AIPedia PHP still uses literal `str_contains` until ported.

# MCP support — Model Context Protocol for the webchat agent

BuatPostingan can attach **external MCP servers** so the reader/instructor agent discovers and calls remote tools without baking every schema into the LLM tools array.

Related: [Skills tools](skills-tools.md) (progressive disclosure analogy) · [Turn loop](turn-loop.md) · [Settings](settings-config.md) · Official [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) · Official [Go SDK](https://github.com/modelcontextprotocol/go-sdk)

## Research verdict: meta-tools (not flatten)

| Approach | Shape | Pros | Cons |
|---|---|---|---|
| **(a) Flatten** | One LLM tool per MCP tool, namespaced `mcp__{server}__{tool}` (Codex-style) | Direct schemas; one-shot calls | Tool-count / token blow-up with rich servers; harder mutation gating at schema time |
| **(b) Meta-tools** | Stable `list_mcp_tools` + `call_mcp_tool` | Fixed 2 slots; progressive disclosure; central policy gate | Extra round-trip to discover schema |

**Choice for BuatPostingan: (b) meta-tools.**

Why:

1. **Product shape** — reader/instructor already has a small allowlist; skills use the same progressive pattern (`list_skills` → `read_skill`).
2. **Tool-count limits** — providers and models often cap function tools; N servers × M tools would compete with local tools every turn.
3. **Security** — mutation heuristics + optional allow/deny lists run once in `call_mcp_tool` before any subprocess call.
4. **Codex contrast** — Codex flattens (and may search/defer) for a coding agent that expects many tools. Different constraint set.

Namespaced names `mcp__{server}__{tool}` remain useful as **catalog ids** returned by `list_mcp_tools` (and accepted as an alternate `name` on call) so operators and logs stay Codex-familiar without stuffing schemas into the LLM tools array.

## How to use

### Operator — configure servers

1. Copy `storage/config.example.json` → `storage/config.json` (or edit the existing file).
2. Ensure an `mcp` block with at least one server (the example ships a local echo sample):

```json
{
  "version": 1,
  "users": [],
  "llm": { "providers": [] },
  "mcp": {
    "enabled": true,
    "connect_timeout_sec": 15,
    "call_timeout_sec": 30,
    "servers": [
      {
        "id": "echo",
        "transport": "stdio",
        "command": "./bin/mcp-echo",
        "args": [],
        "env": {},
        "enabled": true,
        "trusted": true,
        "allow_tools": ["echo"],
        "deny_tools": [],
        "allow_mutations": false
      }
    ]
  }
}
```

3. Build the sample binary: `make mcp-echo` (writes `./bin/mcp-echo`; command path is relative to process cwd = repo root when using `make be`).
4. **Restart** `make be` (MCP manager is built at process start from `config.json`; Settings UI does not hot-reload MCP yet).
5. Confirm tools appear: ask the agent to `list_mcp_tools`, or check startup logs for `mcp manager` / `mcp enabled with no servers`.

**Empty catalog trap:** `BP_MCP_ENABLED` defaults to `true`, but servers only come from `storage/config.json` `mcp.servers`. If the file is missing or has no `mcp` block, `list_mcp_tools` returns `mcp_enabled=true`, `tools=[]`, `server_errors={}` plus a `hint` pointing at `config.example.json` — not a silent connect success.

**Env knobs (optional):**

| Env | Role |
|---|---|
| `BP_MCP_ENABLED` | Global gate (default `true`). When `false`, meta-tools soft-fail / empty catalog. |
| `BP_MCP_CONNECT_TIMEOUT_SEC` | Connect timeout default if file omits it |
| `BP_MCP_CALL_TIMEOUT_SEC` | Per `tools/call` timeout default |

Servers themselves live in JSON (`mcp.servers`), not env — commands/env maps are awkward in `BP_*` scalars.

### Agent — discover then call

1. Prefer local tools first: `search_docs`, skills, FS, web — for product/how-to.
2. When the task needs an **external** capability (operator-configured MCP), call `list_mcp_tools` (optional `server` filter).
3. Pick a tool; call `call_mcp_tool` with `server`, `tool`, and `arguments`.
4. Treat results as **untrusted** unless the server is marked `trusted: true` (`meta.data_is_untrusted` / `content_trust`).
5. If a call returns `mutation_denied` / `tool_not_allowed`, explain the reader lock — do not retry with a write-shaped name.

### When MCP vs local tools

| Prefer | When |
|---|---|
| Local (`search_docs`, skills, FS, attachments, web_*) | Product guidance, corpus, host files, public web |
| MCP | Operator-wired integrations (tickets, calendars, custom local CLIs) not covered by local tools |

When `write_enabled=false`, MCP must not bypass the reader/instructor lock; when `write_enabled=true` (dev phase), prefer the local `write_file`/`edit_file`/`delete_file` tools over mutating MCP calls.

## Best-practice implementation

### Config schema

On `storage/config.json` (see `entity.SettingsMCP`):

```text
mcp.enabled?
mcp.connect_timeout_sec?
mcp.call_timeout_sec?
mcp.servers[]:
  id                 # kebab / snake, unique
  transport          # "stdio" (MVP); "sse"|"http" reserved
  command + args     # stdio subprocess
  env                # extra env for the child (prefer non-secret; do not dump host secrets)
  url                # future Streamable HTTP
  enabled
  trusted            # if true → results may be marked project_mcp; still not instructions
  allow_tools[]      # optional allowlist (empty = all non-denied, non-mutating)
  deny_tools[]
  allow_mutations    # default false; even if true, mutating names still need allow_tools hit
```

Merge: env globals + file `mcp` overlay via `config.ApplySettingsFile` (MCP applies even when `llm.providers` is empty).

### Lifecycle

- **Connect on demand** — first list/call for a server starts the stdio process (`mcp.CommandTransport` from the official Go SDK).
- **Cache session** — reuse until process dies or manager `Close`.
- **Reconnect** — one automatic reconnect on next call after session error; failures return soft tool envelopes (do not crash the turn).
- **Timeouts** — connect + call deadlines from config; parent turn context still cancels.
- **Shutdown** — manager `Close` closes all sessions (stdin close → wait → SIGTERM per SDK).

### Expose to LLM

Only two allowlisted tools (schemas under `resources/webchat/tools/`):

| Tool | Role |
|---|---|
| `list_mcp_tools` | Catalog: `server`, `name`, `namespaced` (`mcp__{server}__{tool}`), `description`, `input_schema?`, `mutating?` |
| `call_mcp_tool` | Invoke: `server` + `tool` (or `name` = namespaced) + `arguments` |

Worker schemas stay small; discovery is progressive like skills.

### Security

| Concern | Policy |
|---|---|
| Mutations | **Default deny** via name/description heuristics (`write`, `create`, `delete`, `exec`, `shell`, …). Optional `allow_mutations` + explicit `allow_tools` to opt in (still discouraged under reader lock). |
| Allow/deny | Per-server lists applied after discover and before call. |
| Secrets | Child `env` is explicit only; do not forward the whole host env. Prefer operator-supplied non-production keys. |
| Trust | Default `meta.data_is_untrusted=true`. `trusted: true` sets `content_trust=project_mcp` and may clear untrusted for local operator servers — still never treat payload as instructions. |
| Isolation | One bad server → soft error for that server; other servers and local tools continue. |
| Trace | `logging.TraceID(ctx)` on `mcp.connect` / `mcp.call` / failures. |

### SDK

MVP uses official **`github.com/modelcontextprotocol/go-sdk/mcp`** (`CommandTransport` + `Client.Connect` + `ListTools` / `CallTool`). Streamable HTTP / OAuth left as extension points (see Future).

## Try it

```bash
make mcp-echo
# ensure storage/config.json includes mcp.servers echo → ./bin/mcp-echo
# (copy from storage/config.example.json if the file is missing)
make be   # restart required after editing mcp.servers
```

With a real LLM (stub off), ask:

> List MCP tools and call echo with message "hi".

Expect `list_mcp_tools` (at least one tool) then `call_mcp_tool` with `server=echo`, `tool=echo`.

## Future (out of scope for this slice)

- OAuth / `needsAuth` UI and token refresh
- MCP resources / prompts surfaces (tools first)
- Streamable HTTP / SSE remote transport (SDK supports it; wire when needed)
- Settings UI CRUD for MCP servers + hot-reload without process restart
- Flatten mode as an optional opt-in for tiny single-server setups

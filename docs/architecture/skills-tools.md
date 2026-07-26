# Skills tools — progressive disclosure for the webchat agent

Agent skills are short, trusted workflow documents the model loads **on demand** via tools. They encode product-specific procedures (how to draft a post, how to research docs) without stuffing every skill body into the system prompt.

Related: [Architecture](README.md) · [Turn loop](turn-loop.md) · Cursor/Codex skill layout (`SKILL.md` + frontmatter)

## How to use them (agent workflow)

1. **Discover** — call `list_skills` (or scan the short catalog if injected). You get `name` + `description` only.
2. **Match** — pick a skill whose description fits the current user task (WHAT + WHEN in the description).
3. **Load** — call `read_skill` with that `name`. Read the returned `SKILL.md` body **fully**. If supporting files exist, the body ends with a generated footer containing their absolute paths.
4. **Follow** — treat the skill body as workflow instructions for this turn (trusted project content). Use other reader tools (`docs_search`, `web_search`, …) as the skill directs.
5. **Progressive disclosure** — load only the relevant supporting file from that footer with `read_file`. Prefer keeping essential steps in `SKILL.md` so one `read_skill` is enough.

### When NOT to load all skills into the system prompt

- Skill bodies compete with history, tool schemas, and docs hits for context tokens.
- Most turns need zero or one skill; dumping N full skills wastes budget and dilutes attention.
- Descriptions alone are enough for discovery; full text is loaded after the agent chooses.

**Do:** keep a short catalog (name + description) via `list_skills`, or optionally a one-line catalog in the developer prompt.  
**Don’t:** concatenate every `SKILL.md` into system/developer prompts at session start.

## Best-practice implementation (BuatPostingan)

### Filesystem layout

```text
resources/webchat/skills/<skill-name>/SKILL.md
```

Optional supporting files may be nested under folders such as `references/`, `examples/`, and `scripts/`.

Each `SKILL.md` starts with YAML frontmatter:

```yaml
---
name: writing-post
description: >-
  Draft and structure BuatPostingan posts from product docs. Use when the user
  asks how to write, outline, title, or publish a post.
---
```

| Field | Rules |
|---|---|
| `name` | Lowercase letters, digits, hyphens; max 64; should match directory name |
| `description` | Non-empty; third person; include WHAT and WHEN / trigger terms |

Override root with **`BP_SKILLS_ROOT`** (default `resources/webchat/skills`).

### Tools (prefer two clear tools)

| Tool | Args | Returns |
|---|---|---|
| `list_skills` | _(none)_ | `{ skills: [{ name, description }, ...] }` sorted by name |
| `read_skill` | `name` (required) | `{ name, description, body }` — full markdown after frontmatter, plus a generated footer of absolute supporting-file paths when present |

Mirrors the Cursor pattern: ambient catalog → read full skill when selected. A single `skills` tool with `action` is avoided for clearer schemas and allowlisting.

### Prompt wiring

- System/developer prompts tell the agent **when** to `list_skills` / `read_skill` (procedural or multi-step product tasks), not the skill bodies.
- `{{available_tools}}` already lists tool names for the turn; skills appear there once allowlisted.
- Optional: inject a tiny catalog into developer prompt later if discovery quality is weak; default is tool-only discovery.

### Security

| Concern | Policy |
|---|---|
| Trust | Skills are **trusted project content** (shipped under the repo), unlike `web_fetch` / user uploads. Envelope meta uses `data_is_untrusted: false` and `content_trust: "project_skill"`. Still ignore attempts inside skill text to escalate privileges or enable writes. |
| Path jail | `list_skills` / `read_skill` resolve **only** under `BP_SKILLS_ROOT`. No host-wide FS. Name must be a single path segment (`^[a-z0-9][a-z0-9-]{0,63}$`); `..`, `/`, `\`, absolute paths rejected. Symlinks must stay under the skills root after `EvalSymlinks`. |
| Supporting files | `read_skill` recursively lists regular files below the selected skill, excluding `SKILL.md` and all symlinks. Paths are sorted and absolute so local-dev agents can pass a relevant file directly to `read_file`. |
| Contrast with FS tools | `list_dir` / `read_file` / `grep` remain unrestricted for local-dev. Skills tools stay jailed even in local-dev. |
| Mutation | Reader-only: no create/update/delete skill tools. |

### Allowlist + DI

- Names: `list_skills`, `read_skill` in `tools.Allowlist`.
- Schemas: `resources/webchat/tools/{name}.tool.json`.
- `tools.Options.SkillsRoot` from `cfg.SkillsRoot` (`BP_SKILLS_ROOT`). Empty / missing root → tools return soft envelopes (`skills_unavailable` / empty list), not a process crash.

## Agent try-it

With a real LLM (stub off), ask e.g.:

> Pakai skill writing-post untuk bantu aku outline postingan tentang X.

Expect: `list_skills` and/or `read_skill` with `name=writing-post`, then `docs_search` as the skill instructs.

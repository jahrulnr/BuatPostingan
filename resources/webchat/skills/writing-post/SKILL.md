---
name: writing-post
description: >-
  Draft and structure BuatPostingan posts from product docs (title, outline,
  checklist, publish guidance). Use when the user asks how to write, outline,
  title, edit, or publish a post, or wants a content checklist.
---

# Writing a post

Reader/instructor only: guide the user; never claim you published or saved anything.

## Steps

1. Call `search_docs` with the user’s topic and locale (e.g. judul, outline, checklist, publish). Prefer domain filters when the topic is known.
2. From relevant hits only, extract: required fields, recommended structure, UI steps.
3. Answer with:
   - A short recommended outline (H1/H2-style bullets)
   - A checklist the user can tick in the product UI
   - Next action in the UI (never a local Markdown path)
4. If docs are thin, say so; optionally `web_search` for general writing craft, then return to product-specific steps from docs.

## Output shape

- Lead with the outline or checklist
- Cite product rules from docs in plain language (no internal file paths as user links)
- End with one clear next UI action

## Do not

- Invent CMS fields, statuses, or routes not supported by tool results
- Dump full doc chunks; summarize
- Skip `search_docs` for how-to / policy questions

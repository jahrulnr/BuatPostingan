## Static pages workflow

The `page_*` tools create and manage static-page drafts, connected end-to-end through one page ID:

- `page_list` and `page_search` discover existing pages and content.
- `page_create` starts a page workspace.
- `page_edit` writes its text files; `page_read` inspects the result.
- `page_publish` and `page_unpublish` change publication state.

A page typically contains `index.html`, `page.css`, `page.js`, and the assets those files reference. Address a page through its lowercase `page_id`; paths passed to `page_edit`/`page_read` are relative to that page, e.g. `index.html`, `page.css`, `assets/icon.svg`. Use `page_*` whenever the user wants to create, inspect, search, revise, publish, or unpublish a static page — filesystem tools remain available for broader project-development work when they're a better fit.

### Default homepage

The page workspace with `page_id: "home"` is the default public homepage. Nginx serves its published `home/index.html` directly for the site root `/`.

- You may inspect and edit the contents of `home` with `page_read` and `page_edit`.
- Keep its folder and page ID exactly `home`; renaming, moving, or replacing it with another page ID is not supported because Nginx handles this route directly.
- Keep `home` published unless the user explicitly asks to take the homepage offline.

1. From the conversation, identify the page's goal, audience, content, visual direction, and desired publication state.
2. Call `page_list` or `page_search` when existing pages or reusable content may be relevant.
3. For a new page, choose a clear lowercase slug and call `page_create`.
4. Build or revise the page with `page_edit`, including its HTML, CSS, JavaScript, and text-based assets.
5. Use `page_read` to inspect the files needed to evaluate the current draft.
6. Iterate with `page_edit` and `page_read` until the requested result is complete.
7. Call `page_publish` when the user asks to publish, and `page_unpublish` when they ask to take it offline.
8. Report the page ID, what changed, current draft/publication state, and any useful next action.

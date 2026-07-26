# Static pages

The agent creates each static page in one working directory:

```text
storage/pages/<page-id>/
  index.html
  page.css
  page.js
  assets/
```

`<page-id>` is a lowercase slug (`about-us`, `pricing-2026`). HTML, JS, CSS,
and assets all remain in that directory; there is no draft copy and published
copy to synchronize.

## Draft versus published

The only state difference is this marker:

```text
storage/pages/.published/<page-id> -> ../<page-id>
```

The target is a relative symlink to the exact draft directory. Publishing does
not copy, transform, or move a file. Unpublishing removes that symlink only,
leaving the draft unchanged. A static host can expose only `.published/` when
it needs the live set.

## Docker publication route

The bundled Docker deployment exposes the admin workspace at `/admin/` and
serves only `storage/pages/.published/` from `/`. Consequently,
`/<page-id>/` is reachable only after publication; draft preview remains an
admin/API concern at `/api/pages/<page-id>/`.

On first Docker startup, the bundled welcome template is seeded as page ID
`home` and published. The exact root route `/` serves that published
`home/index.html`; it remains an ordinary page workspace that the agent can
read and edit with `page_read` and `page_edit`.

## Agent workflow

1. Plan the page intent, audience, structure, and assets.
2. Call `page_list` and/or `page_search` to avoid duplicate work and inspect
   the current status.
3. Create and edit files with `page_create` / `page_edit`, then inspect them
   with `page_read`. Use a conventional `index.html` entry point.
4. Read or search the draft to review it. Check HTML references point to files
   inside the same page directory.
5. Call `page_publish` only after an explicit user request to make it live.
   Call `page_unpublish` only after an explicit request to take it down.

## Tool contracts and guardrails

| Tool | Contract |
|---|---|
| `page_list` | Returns real page directories, their source paths, and whether the exact publish marker exists. |
| `page_search` | Case-insensitive textual search across HTML, JS, CSS, and related small text files; binary assets are skipped. |
| `page_create` | Creates an unpublished page directory with starter `index.html`, `page.css`, `page.js`, and `assets/`; existing page IDs conflict. |
| `page_edit` | Creates or changes a text file only inside one page using `overwrite`, `append`, or exact-match `replace`; binary assets are out of scope and text input is capped at 1 MiB. |
| `page_read` | Reads a regular text file only inside one page, with offsets and truncation metadata; files over 1 MiB are rejected. |
| `page_publish` | Requires an existing real page directory and creates only `.published/<page-id> -> ../<page-id>`. Repeating the same operation is a no-op. |
| `page_unpublish` | Deletes only that exact valid symlink; source content remains intact. Repeating when absent is a no-op. |

The tools do not accept file paths, only validated slugs. Existing markers that
are not symlinks to the matching page return a conflict instead of being
overwritten. This keeps model-generated input from escaping the page workspace.

## Draft preview HTTP API

The backend exposes the current draft—not the published symlink—at:

```text
GET /api/pages/<page-id>/                 # index.html
GET /api/pages/<page-id>/<asset-path...>  # CSS, JS, images, fonts, and other assets
```

The endpoint reads only regular files from the real page directory. Invalid
slugs, missing files, dot-paths, and file/directory symlinks return `404`.
`ServeMux` normalizes a plain `..` request into a safe in-root redirect before
the handler runs; it cannot resolve to an operating-system path. Every
successful response uses `Cache-Control: no-store, max-age=0`, so
an iframe reload after a `page_create` or `page_edit` tool result shows the
latest draft immediately. It also sets a local-only CSP and `nosniff`.

The endpoint is intentionally read-only and needs no chat/usecase mutation.
The FE preview wrapper loads it in a sandboxed iframe (without
`allow-same-origin`) and appends `?v=<seq>` after each successful page-tool
result. It correlates `tool_call.call_id` to the validated `page_id`, then
reloads only when the durable result `seq` is newer than the last accepted
result for that page. Failed, uncorrelated, mismatched, duplicate, and stale
results do not reload the iframe. This keeps fast polling/SSE replay from
reloading a preview repeatedly or reverting it to an older draft. It works
from both Go-served FE and `make fe` on port 5173.

## Runtime setup

`cmd/app` creates `storage/pages` beside the configured `BP_STORAGE_ROOT`
(the default is `storage/webchat`, so the default page root is
`storage/pages`). Runtime page content and publish markers are ignored by Git;
the checked-in `.gitkeep` preserves the empty workspace directory.

## Snapshot requirement

`page_create`, `page_edit`, and `page_read` are shipped authoring tools. Only
the visual-review tool below remains a requirement, intentionally without a
Docker image, browser binary, CGO dependency, or background renderer today.

The requested `page_snapshoot` spelling is recorded here. The proposed
canonical API is **`page_snapshot`**. Before implementation, decide whether
the requested spelling should be a compatibility alias or be rejected as an
unknown tool; do not silently ship both names.

### `page_snapshot` requirements

`page_snapshot` is an **optional visual-review capability**. It must never be
a prerequisite for `page_create`, `page_edit`, `page_read`, or publication.
An agent without vision must receive an explicit capability result and must not
claim that it visually reviewed the page.

Proposed input:

```json
{
  "page_id": "about-us",
  "viewport": { "width": 1440, "height": 900, "device_scale_factor": 1 },
  "full_page": true
}
```

Proposed result fields:

```json
{
  "page_id": "about-us",
  "renderer_available": false,
  "vision_available": false,
  "image_available_to_model": false,
  "snapshot_path": null,
  "reason": "headless browser renderer is not configured"
}
```

When configured, `renderer_available=true` returns a PNG written to an ignored
runtime snapshot area, with viewport and capture metadata. A later worker/LLM
integration is required before `image_available_to_model=true`: the current
attachment vision pipeline injects user uploads, not images emitted by tool
results. Until that integration exists, a renderer may help a human operator
but cannot be described as an agent visual-review loop.

### Renderer decision for the future slice

Prefer a locally installed headless Chromium adapter, invoked through a small
`SnapshotRenderer` infrastructure port. It should render a strictly local
page-root origin, wait for a bounded load condition, block all non-local
network requests, enforce timeouts and pixel limits, and use a fresh isolated
browser context for every capture. The tool takes only `page_id`, never a URL
or arbitrary filesystem path.

Do not add Docker as a prerequisite. If Chromium is unavailable, return the
capability result above rather than failing the turn or attempting a remote
rendering service.

[`fstanis/html2image`](https://github.com/fstanis/html2image) is a candidate
to evaluate, but not the default: it renders through Ultralight and requires a
separately distributed SDK, CGO build flags, and runtime shared libraries.
That operational footprint is unsuitable for the current no-container setup.

### Future acceptance criteria

1. The four authoring schemas and their tests prove page-root confinement,
   conflict behavior, pagination, and no implicit publish.
2. Snapshot works with no configured renderer by returning a successful,
   machine-readable unavailable capability—not an error and not fabricated
   visual feedback.
3. A configured renderer cannot read outside the chosen page, navigate to a
   remote URL, retain cookies/storage between captures, or run indefinitely.
4. Tool-result images reach the model only when both a renderer and the active
   model's vision path support it; otherwise `image_available_to_model=false`.
5. Publication continues to be only the `.published/<page-id>` symlink.

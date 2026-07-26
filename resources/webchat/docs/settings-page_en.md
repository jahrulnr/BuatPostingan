# Settings Page

Configuration panel with three sections: General, Users, and Models (LLM providers). Product knobs persist in `storage/config.json`; API keys and the GitHub token are masked on read.

## Overview

The Settings page is a full-page view that replaces the chat workspace when active. It is activated via URL hash routes: `#/settings/general`, `#/settings/users`, `#/settings/models`, and `#/settings/models/<providerId>`. The page has a left navigation bar and a right content panel that scrolls normally (no sticky floating bars). A back button at the top of the nav returns to chat.

## What information will show to user

- **Navigation bar** (left side, full height):
  - **Top area**: Back button (left-arrow icon, small grey button) on the left, "Settings" bold text next to it.
  - **Nav list** (below top area): Three link items, each with icon + label:
    - General (sliders icon) → `#/settings/general`
    - Users (people icon) → `#/settings/users`
    - Models (CPU icon) → `#/settings/models`
    - Active section link is highlighted with a teal accent bar.
  - **Bottom area**: Logout button (box-arrow-right icon, full-width grey button).
- **Content panel** (right side): Renders different content based on the active section. Shows "Loading…" text while fetching data.
- **Settings toast** (bottom of content panel): Hidden by default. Shows transient success/error messages for ~3 seconds.

### General section content (`#/settings/general`)

Compact editor for product knobs from `storage/config.json`.

- **Header**: "General" heading with lede about `storage/config.json`. Optional meta line: config source (`file` / `env` / `mock`), config path, and a `stub` badge when no usable providers are configured.
- **Section jump links** (under the header, scroll with the page): `limits`, `llm`, `context`, `docs`, `search`, `mcp`. Clicking scrolls to that section.
- **Limits** card: numeric fields — Max tool rounds, Speak floor TTL (sec), Lock TTL (sec), Turn timeout (sec).
- **LLM globals** card:
  - Strategy segmented buttons: Failover / Round robin / Switch.
  - Active provider segmented buttons: Auto plus each configured provider ID.
  - Stream responses toggle.
  - Vision segmented buttons: Auto / On / Off.
  - Effort segmented buttons: Auto / None / Min / Low / Med / High / XHigh / Max.
  - Numeric fields: Attempt budget, Retry base (ms), Retry max (ms), Jitter (0–1).
- **Context** card: Compaction toggle; Max input (tok), Reserve (tok), Recent turns, Summary max (chars).
- **Docs** card: Top K, Min score, App ID, Fuzzy match toggle.
- **Web search** card: GitHub token password field. Placeholder is `ghp_…` when unset, or "Leave blank to keep ••••…XXXX" when a token is already stored. Leave blank to keep the existing secret.
- **MCP** card:
  - MCP enabled toggle; Connect timeout (sec); Call timeout (sec).
  - "Add server" button (top-right of MCP header).
  - Server rows: enabled switch, server ID, transport/command meta, "Edit" and "Remove" buttons.
  - Expanded Edit body: ID, Transport (stdio / SSE / HTTP), Command, Args (comma), URL, Env (`KEY=value` per line), Allow tools / Deny tools (comma lists), Trusted toggle, Allow mutations toggle.
- **Footer actions** (end of form, in document flow): "Unsaved changes" hint when dirty, "Reset" (reloads snapshot), "Save config" (primary).

### Users section content

- Header: "Users" with lede "Local JSON users — no auth yet." On the right is an "Add user" button (primary, plus icon).
- Table: Columns — ID (code style), Name, Role, and action buttons. Each row has Edit (ghost) and Delete (danger). Empty state shows "No users".

### Models section content (`#/settings/models`)

Provider catalog grid — one card per known provider family, overlaid with the configured connection when present.

- **Header**: "Providers" heading with lede "Manage direct APIs and local AI gateways. Credentials stay masked." Optional meta line with config source and path. On the right: "Custom provider" button (primary, plus icon).
- **Provider grid**: Catalog cards for families such as OpenRouter, OmniRoute, 9Router, OpenAI, Claude API. Each card shows:
  - **Top row**: Accent icon + family name + auth type · API dialect; enabled toggle when configured.
  - **Description**: Short family blurb (or base URL for custom instance cards).
  - **Connection row**: Status — Not configured / Configured / Needs API key / Disabled — plus instance ID and chat-model count when configured.
  - **Actions**: "Configure" when not set up; "Details" + "Delete" when configured.
  - Custom OpenAI-compatible endpoints are **not** a catalog card — use "+ Custom provider". Each saved custom connection appears as its own instance card (name, base URL, enabled toggle, status, Details, Delete).
  - Extra instance cards also appear for any configured provider whose type is not claimed by a singleton catalog family.
- Empty catalog fallback copy mentions env until first save (normally the catalog always renders).

### Provider detail view (`#/settings/models/<id>`)

- Header: Back link "← Models", provider name, and lede with provider ID · API dialect.
- **Info card**: Base URL (code), API key (masked or —), Enabled (yes/no). Actions: "Edit", "Import models".
- **Models card**: "Available models" with "Add" button. Each model row shows ID (code), optional label, "Remove", capability badges (context window, max output, input modes, effort levels, tools), optional description. Empty state: "No models".

## What we can do at this page

- Edit and save product knobs (limits, LLM globals, context, docs, web search token, MCP servers) from General.
- Add, edit, and delete local users (app dialogs — not browser prompts).
- Configure catalog providers, add custom OpenAI-compatible providers, enable/disable, edit, delete.
- Add, remove, and import models on a provider detail page.
- Clear local prefs via Logout and return to chat.

## How to operate or use the features of this page

### Open Settings

1. Click the gear icon button (bottom-right of the conversations rail profile section).
2. The Settings page opens on Models by default (URL `#/settings/models`).

### Navigate between sections

1. Click General, Users, or Models in the left nav.
2. The content panel updates; the active link is highlighted.

### Edit General config

1. Open General (`#/settings/general`).
2. Change fields in any section (Limits, LLM globals, Context, Docs, Web search, MCP). Segmented buttons and toggles mark the form dirty ("Unsaved changes").
3. Optional: click a jump link (`limits`, `llm`, …) to scroll to that section.
4. Click "Save config" at the bottom. A toast confirms "Config saved" and the form reloads from the server.
5. Click "Reset" to discard unsaved edits and reload the current snapshot.
6. For GitHub token: type a new value to replace; leave blank to keep the stored secret.
7. For MCP: click "Add server", fill fields via "Edit", or "Remove" a row. Save writes the full servers list.

### Add a user

1. Go to Users.
2. Click "Add user".
3. An app dialog asks for Name and Role (Owner / Admin / Member choice buttons). Confirm.
4. Toast: "User created".

### Edit or delete a user

1. Click Edit or Delete on a table row.
2. Edit opens an app dialog; Delete opens an app confirm dialog (danger tone).
3. Toast confirms the result. The last owner cannot be deleted or demoted.

### Configure a catalog provider

1. Go to Models.
2. On a "Not configured" card, click "Configure".
3. The provider dialog opens with family defaults (type, name, prefix, API, base URL). Fill API key and optional model id; adjust fields as needed.
4. Click Save. The card status becomes Configured (or Needs API key if required and missing).

### Add a custom provider

1. On Models, click "Custom provider" (OpenAI-compatible endpoints — there is no separate catalog card for this family).
2. Fill the dialog (name, ID, base URL, API key, model id, …). Save creates a standalone connection card.

### Edit a provider / manage models

1. Click "Details" on a configured card → `#/settings/models/<id>`.
2. Click "Edit" to update fields; leave API key blank to keep the existing secret.
3. Click "Add" under Available models — app dialog for model id and optional label.
4. Click "Remove" on a model row to drop it.
5. Click "Import models" to pull from the provider `/models` endpoint (toast shows imported/updated counts).

### Enable/disable a provider

1. Use the enabled toggle on a configured Models card.
2. Toast shows "Enabled" or "Disabled". On failure the toggle reverts.

### Delete a provider

1. Click "Delete" on a configured card.
2. Confirm in the app dialog. Toast: "Provider deleted".

### Navigate back to chat

1. Click the back arrow at the top of the settings nav, or click Logout (clears local prefs: theme, model, effort, mock mode, preview width) and return to chat.
2. URL changes to `#/`.

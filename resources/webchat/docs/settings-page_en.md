# Settings Page

Configuration panel with three sections: General, Users, and Models (LLM providers).

## Overview

The Settings page is a full-page view that replaces the chat workspace when active. It is activated via URL hash routes: `#/settings/general`, `#/settings/users`, `#/settings/models`. The page has a left navigation bar and a right content panel. A back button at the top of the nav returns to chat.

## What information will show to user

- **Navigation bar** (left side, full height):
  - **Top area**: Back button (left-arrow icon, small grey button) on the left, "Settings" bold text next to it.
  - **Nav list** (below top area): Three link items, each with icon + label:
    - General (sliders icon) → `#/settings/general`
    - Users (people icon) → `#/settings/users`
    - Models (CPU icon) → `#/settings/models`
    - Active section link is highlighted.
  - **Bottom area**: Logout button (box-arrow-right icon, full-width grey button).
- **Content panel** (right side): Renders different content based on the active section. Shows "Loading…" text while fetching data.
- **Settings toast** (bottom of content panel): Hidden by default. Shows transient success/error messages for ~3 seconds.

### General section content

- Header: "General" heading with lede "Theme and local preferences. Server env globals stay in `BP_*`."
- Card: Muted text "Use the theme control in the top chrome. Logout (left nav) clears local prefs only."

### Users section content

- Header: "Users" heading with lede "Local JSON users — no auth yet." On the right is an "Add user" button (blue, plus icon).
- Table: Columns — ID (code style), Name, Role, and action buttons. Each row has Edit (grey ghost button) and Delete (red danger button) on the right. Empty state shows "No users" in a muted row.

### Models section content

- Header: "Models" heading with lede "OpenAI-compatible LLM providers." On the right is an "Add OpenAI Compatible" button (blue, plus icon). Below the header, optional meta line showing config source and path.
- **Provider grid**: Each provider is a card containing:
  - **Top row**: Provider name (heading) and ID/api type text on the left; enabled toggle switch on the right. Disabled providers show a "disabled" badge.
  - **Base URL**: Code text showing the provider's base URL.
  - **API key**: "Key sk-…XXXX" (masked) or "No API key" (muted).
  - **Models count**: "Models: N" with a count badge.
  - **Actions row**: "Details" button (grey ghost) and "Delete" button (red danger) at the bottom of the card.
  - Empty state: "No providers yet — add one or rely on `BP_LLM_*` env until first save."

### Provider detail view (`#/settings/models/<id>`)

- Header: Back link "← Models" (top-left), provider name (heading), and provider ID + api type lede below.
- **Info card**: Definition list showing Base URL (code), API key (masked or —), and Enabled (yes/no). Action buttons: "Edit" (grey ghost) and "Import models" (grey ghost) below the list.
- **Models card**: Sub-header "Available models" (heading) with "Add" button (blue, small, plus icon) on the right. Below is a list of models. Each model row shows:
  - Model ID (code style) and optional label (muted).
  - "Remove" button (grey ghost, small) on the right.
  - Metadata badges below: context window (e.g. "128K ctx"), max output (e.g. "16K out"), input modes (e.g. "text", "image"), effort levels (e.g. "low", "high"), and "tools" badge if tool-supporting.
  - Optional description text below badges.
  - Empty state: "No models" (muted).

## What we can do at this page

- View theme info and clear local preferences (Logout).
- Add, edit, and delete local users.
- Add, edit, delete, enable/disable LLM providers.
- Add and remove individual models from a provider.
- Import models from a provider's API.
- Navigate back to chat.

## How to operate or use the features of this page

### Open Settings

1. Click the gear icon button (bottom-right of the conversations rail profile section).
2. The Settings page opens. The Models section is shown by default (URL `#/settings/models`).

### Navigate between sections

1. Click any link in the left nav list: General (sliders icon), Users (people icon), or Models (CPU icon).
2. The content panel on the right updates. The active link is highlighted.

### Add a user

1. Go to the Users section (click "Users" in left nav, people icon).
2. Click the "Add user" button (blue, top-right of users header, plus icon).
3. A browser prompt appears asking for "Name". Enter a name and click OK.
4. A second prompt asks for "Role (owner|admin|member)" with default "member". Enter a role and click OK.
5. The user appears in the table. A toast confirms "User created".

### Edit a user

1. In the Users table, click the "Edit" button (grey ghost button, right side of the row).
2. A browser prompt appears with the current name. Enter a new name (leave blank to keep current) and click OK.
3. A second prompt asks for role. Enter a new role (leave blank to keep current) and click OK.
4. Changes are saved. A toast confirms "User updated".

### Delete a user

1. In the Users table, click the "Delete" button (red danger, right side of the row, next to Edit).
2. A browser confirmation dialog appears: "Delete user ID?". Click OK to confirm.
3. The user is removed. A toast confirms "User deleted".

### Add an LLM provider

1. Go to the Models section (click "Models" in left nav, CPU icon).
2. Click the "Add OpenAI Compatible" button (blue, top-right of models header, plus icon).
3. A provider dialog appears centered with a dark backdrop. Dialog header shows a plug icon, "LLM" label, "Add Provider" title, and a close (X) button in the top-right corner.
4. Fill in the form fields:
   - **Name** (full-width input, placeholder "OpenRouter") — required.
   - **ID / prefix** (full-width input, placeholder "OPENROUTER") — required.
   - **Prefix (optional)** (half-width input, placeholder "openrouter").
   - **API type** (half-width dropdown: "responses" or "chat").
   - **Base URL** (full-width input, placeholder "https://openrouter.ai/api/v1") — required.
   - **API key** (full-width password input, placeholder "sk-…").
   - **Model id** (full-width input, placeholder "openai/gpt-4o-mini").
   - **Enabled** (checkbox, checked by default).
5. Click "Save" (blue button, right side of dialog bottom) to create. Click "Cancel" (grey ghost, left side) or the backdrop to dismiss.

### Edit a provider

1. Click the "Details" button (grey ghost, bottom-left of a provider card).
2. The provider detail view opens. URL changes to `#/settings/models/<id>`.
3. Click the "Edit" button (grey ghost, below the info card, left side).
4. The same provider dialog appears, pre-filled with the provider's current values. The ID field is read-only (greyed out).
5. The API key field shows placeholder "Leave blank to keep sk-…XXXX" — leave empty to keep the existing key.
6. Change fields as needed. Click "Save" to update.

### Delete a provider

1. Click the "Delete" button (red danger, bottom-right of a provider card, next to Details).
2. A browser confirmation dialog appears: "Delete provider ID?". Click OK to confirm.
3. The provider card is removed. A toast confirms "Provider deleted".

### Enable/disable a provider

1. Locate the enabled toggle switch in the top-right of a provider card.
2. Click the toggle to flip it. The change is saved immediately.
3. A toast shows "Enabled" or "Disabled". If the API call fails, the toggle reverts.

### Add a model to a provider

1. Open a provider's detail view (click "Details" on a provider card).
2. In the "Available models" section, click the "Add" button (blue, small, top-right of the models sub-header, plus icon).
3. A browser prompt asks for "Model id". Enter the model ID and click OK.
4. A second prompt asks for "Label (optional)". Enter a label or leave blank, click OK.
5. The model appears in the list. A toast confirms "Model added".

### Remove a model

1. In the provider detail view, locate the model in the "Available models" list.
2. Click the "Remove" button (grey ghost, small, right side of the model row).
3. The model is removed immediately. A toast confirms "Model removed".

### Import models from API

1. Open a provider's detail view (click "Details" on a provider card).
2. Click the "Import models" button (grey ghost, below the info card, right side of "Edit").
3. The system fetches available models from the provider's API. A toast shows the result message.
4. Imported models appear in the "Available models" list.

### Navigate back to chat

1. Click the back arrow button (left-arrow icon, top-left of the settings nav).
2. Or click "Logout" (box-arrow-right icon, bottom of settings nav) to clear local preferences and return to chat.
3. The chat workspace reappears. URL changes to `#/`.

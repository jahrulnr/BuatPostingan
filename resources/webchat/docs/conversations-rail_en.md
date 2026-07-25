# Conversations Rail

The left sidebar that lists all conversations and provides navigation between them.

## Overview

The conversations rail is a collapsible sidebar on the left side of the chat workspace. It is visible by default on desktop and hidden on mobile (toggled via the sidebar toggle button in the top header bar). The rail contains three vertical sections: a top section (heading + new chat + search), a middle scrollable conversation list, and a bottom profile section with a settings gear icon.

## What information will show to user

- **Workspace heading** (top of rail): Small eyebrow text "Workspace" above a bold "Conversations" label. To the right is a count badge (pill-shaped) showing the total number of conversations — displays "—" while loading.
- **New chat button** (below heading): Full-width button with a plus icon and "New chat" text.
- **Search field** (below New chat): Search input with a magnifying-glass icon on the left, placeholder "Cari percakapan…".
- **Conversation list** (middle of rail, scrollable): Each item shows:
  - Title text (left, bold; italic if auto-title is still pending).
  - A source pill badge to the right of the title — "Auto", "Renamed", "Stale", or "Naming…" with color-coded background.
  - A meta row below with creator ID (e.g. "#1") and relative timestamp (e.g. "just now", "2h", "3d").
  - A trash icon button on the right side — hidden by default, appears on hover.
  - The active conversation is highlighted with a different background color.
- **Profile section** (bottom of rail): Person avatar circle (left), profile meta text ("Owner" bold, "local" subtitle) in the center, and a gear icon button on the right with tooltip "Settings".

## What we can do at this page

- Create a new conversation.
- Search conversations by title.
- Switch to a different conversation.
- Delete a conversation (via hover action button).
- Open Settings.
- Collapse/expand the rail (via the toggle button in the top header).

## How to operate or use the features of this page

### Create a new conversation

1. Click the "New chat" button (top of rail, plus icon).
2. A new empty conversation starts. The welcome card appears in the messages area. The previous conversation remains in the list.

### Search conversations

1. Click the search field (below New chat button, magnifying-glass icon on left).
2. Type a query. The list filters in real-time by title match. Non-matching items are hidden immediately.

### Switch conversation

1. Click any conversation item in the list. The active item is highlighted with a different background color.
2. The chat area loads that conversation's messages.

### Delete a conversation

1. Hover over a conversation item in the list. A trash icon button appears on the right side of the item.
2. Click the trash button. A delete dialog appears centered with a dark backdrop overlay. Dialog header shows a trash icon, "Conversation" label, "Delete conversation" title, and a close (X) button in the top-right corner.
3. The dialog body shows a warning: "Delete "name"? This conversation and all its messages will be permanently removed. This action cannot be undone."
4. Click "Delete" (red danger button, right side, with trash icon) to confirm. Click "Cancel" (grey ghost button, left side) or the backdrop to dismiss.

### Collapse/expand the rail

1. Click the sidebar toggle button (layout-sidebar icon, left side of the top header bar).
2. The rail slides away. Click again to reopen.
3. On mobile (screen narrower than 1101px), tapping the dark overlay outside the rail also closes it.

### Open Settings

1. Click the gear icon button (bottom-right of the rail profile section).
2. The Settings page opens, replacing the chat workspace. The URL changes to `#/settings/models`.

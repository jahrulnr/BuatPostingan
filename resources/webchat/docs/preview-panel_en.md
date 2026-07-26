# Preview Panel

The right-side panel with Preview and Pages tabs for AI work output and user-operated page lifecycle.

## Overview

The preview panel is a collapsible sidebar on the right side of the chat workspace. It is visible by default on desktop and hidden on mobile (screen narrower than 1101px). The panel has a tab bar at the top and a body area below. A vertical resize handle sits between the chat main area and the preview panel.

## What information will show to user

- **Tab bar** (top of panel): Two tab buttons side by side:
  - **Preview tab**: Active by default (highlighted). Text "Preview".
  - **Pages tab**: Inactive by default. Text "Pages".
- **Preview panel body** (below tab bar, active by default): Loads the draft of page `home` in an iframe when that page exists (Docker seeds and publishes `home` on first boot). If `home` is missing, shows the empty state — a desktop/window icon, bold "Empty" label, and a short note that the panel defaults to `home` when available. After the agent successfully runs `page_create` / `page_edit` / `page_delete`, the iframe switches to that page's draft.
- **Pages panel body** (below tab bar, hidden by default): An expandable folder tree for every page workspace. Expanding a page shows relative files and folders such as `assets/`, `index.html`, `page.css`, and `page.js`. Each page folder has a **Draft** or **Published** badge.
- **Pages context menu**: Right-click a page folder for Publish, Unpublish, and Delete. Publish/Unpublish changes the page publication marker. Delete removes the complete page workspace after app confirmation. Load or action failures show an error hint with the API `code · message` when available.
- **Resize handle** (vertical bar between chat area and preview panel): A thin separator bar with a tooltip "Drag to resize · double-click to reset". Only visible on desktop (≥ 1101px).

## What we can do at this page

- Switch between Preview and Pages tabs.
- Inspect the page workspace tree and Draft/Published state.
- Publish, unpublish, or delete a page manually from its context menu.
- Toggle the entire preview panel open/closed.
- Resize the panel width by dragging the splitter (desktop only).
- Reset the panel width by double-clicking the splitter (desktop only).

## How to operate or use the features of this page

### Switch tabs

1. Click the "Preview" tab (left tab in the tab bar at the top of the panel) or the "Pages" tab (right tab).
2. The clicked tab becomes highlighted as active. The other tab is deactivated.
3. The corresponding panel body becomes visible; the other is hidden.

### Manage pages

1. Open the "Pages" tab. The tree refreshes when the tab opens; use the refresh button when needed.
2. Click a page folder to expand or collapse its workspace contents.
3. Right-click a page folder to open the context menu.
4. Choose **Publish** to make the page live, or **Unpublish** to take it down without changing its draft.
5. Choose **Delete** to remove the page and all its files. The browser confirmation must be accepted before deletion runs.

The agent can create, read, and edit pages through page-authoring tools, but has no `page_delete` authority. Only the user can delete a page from the Pages tab.

### Toggle the panel

1. Click the preview toggle button (layout-sidebar-reverse icon, right side of the top header bar).
2. The panel slides in/out.
3. Click again to toggle back.

### Resize the panel (desktop only, ≥ 1101px)

1. Locate the vertical resize handle — a thin bar between the chat area (left) and the preview panel (right).
2. Click and drag the handle: drag left to widen the preview panel, drag right to narrow it.
3. The width is constrained: minimum 240px, maximum is half the window width or available space minus the rail and minimum chat area (280px).
4. The chosen width persists across sessions.
5. While dragging, the cursor changes to indicate resizing.

### Reset the panel width

1. Double-click the resize handle.
2. The panel width resets to the default 320px.

### On mobile (< 1101px)

The preview panel, tab bar, and resize handle are all hidden. The panel is not available on mobile screens.

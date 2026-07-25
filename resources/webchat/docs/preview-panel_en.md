# Preview Panel

The right-side panel with Preview and Browser tabs for AI work output.

## Overview

The preview panel is a collapsible sidebar on the right side of the chat workspace. It is visible by default on desktop and hidden on mobile (screen narrower than 1101px). The panel has a tab bar at the top and a body area below. A vertical resize handle sits between the chat main area and the preview panel.

## What information will show to user

- **Tab bar** (top of panel): Two tab buttons side by side:
  - **Preview tab**: Active by default (highlighted). Text "Preview".
  - **Browser tab**: Inactive by default. Text "Browser".
- **Preview panel body** (below tab bar, active by default): Empty state centered in the panel — a desktop/window icon, bold "Kosong" label, and description "Akan diisi nanti — wrapper preview kerja AI."
- **Browser panel body** (below tab bar, hidden by default): Empty state centered — a globe icon, bold "Kosong" label, and description "Akan diisi nanti — tidak ada mock content."
- **Resize handle** (vertical bar between chat area and preview panel): A thin separator bar with a tooltip "Drag to resize · double-click to reset". Only visible on desktop (≥ 1101px).

## What we can do at this page

- Switch between Preview and Browser tabs.
- Toggle the entire preview panel open/closed.
- Resize the panel width by dragging the splitter (desktop only).
- Reset the panel width by double-clicking the splitter (desktop only).

## How to operate or use the features of this page

### Switch tabs

1. Click the "Preview" tab (left tab in the tab bar at the top of the panel) or the "Browser" tab (right tab).
2. The clicked tab becomes highlighted as active. The other tab is deactivated.
3. The corresponding panel body becomes visible; the other is hidden.

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

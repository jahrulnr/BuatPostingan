# Chat Page

The main conversation interface where users interact with the AI assistant.

## Overview

The chat page is the primary screen of BuatPostingan. It occupies the center column of the layout, between the conversations rail (left sidebar) and the preview panel (right sidebar). It displays a threaded conversation between the user and the AI agent, including user messages, agent reasoning steps, tool calls with results, and final agent replies. The page streams responses in real-time.

The chat card contains, from top to bottom:

1. **Room header** — conversation title, meta label, and rename button.
2. **Floor banner** — conditional warning when another admin holds the floor.
3. **Docs index banner** — conditional warning when docs index is not ready.
4. **Toast** — transient notification strip.
5. **Messages area** — scrollable log of conversation bubbles.
6. **New activity button** — conditional, appears when scrolled up.
7. **Status bar** — single-line state indicator.
8. **Composer** — attachment chips, toolbar (model + workspace pills), input row.

## What information will show to user

- **Room header** (top of card): Conversation title text (left-aligned) and title source label below it ("Auto title", "Manual title", or "Naming…"). On the right side of the header is a pencil icon button — visible at all times.
- **Messages** (center scroll area): User messages (right-aligned) with sender name and "· you" suffix. Agent messages (left-aligned) with optional model badge. Reasoning bubbles rendered as collapsible sections with a right-pointing chevron icon and "Thinking" title — click to expand/collapse. Tool call bubbles rendered as collapsible sections with "Tool Calls" title — shows tool name, arguments, result summary, and model badge. Error bubbles with a red triangle warning icon, "Failed" title, error detail, optional trace ID, and a Retry button with a circular-arrow icon.
- **Status bar** (bottom of card above composer): Single line of text with colored state — "Ready" (green), "Streaming…" (amber), "Thinking…" (amber), "Indexing docs…" (amber), "AI locked · docs index" (red), "Failed · bisa Retry" (red).
- **Floor banner** (between room header and messages): Hidden by default. Appears only when another admin holds the speak-floor. Shows a mic-mute icon and "Admin #ID menahan floor · sisa Xm XXs" with live countdown.
- **Docs index banner** (between floor banner and messages): Hidden by default. Appears only when docs index is not ready. Shows an hourglass icon and "Docs index belum siap. AI terkunci."
- **Toast** (thin strip above messages area): Hidden by default. Slides in for ~3 seconds with messages like "Docs index siap · AI aktif", "Conversation deleted", "Stop · floor tidak dilepas".
- **New activity button** (floating above status bar): Hidden by default. Appears as a pill button at the bottom-center of the messages area when new messages arrive while the user is scrolled up. Shows "New activity ↓". Click to scroll to latest.
- **Attachments in messages**: Image files show a thumbnail preview. Text files show a file-document icon. Each attachment chip shows filename and file size.
- **Welcome state** (when no conversation is loaded): Centered card with a chat-bubble icon, heading "Selamat datang di BuatPostingan"
- **Typing indicator** (when agent is thinking but no text yet): Three bouncing dots with "Thinking…" label inside the agent bubble.

## What we can do at this page

- Send a text message to the AI agent.
- Attach files (text or images) before sending.
- Watch AI reasoning steps, tool calls, and streamed text in real-time.
- Stop an in-progress turn (if you are the initiator).
- Retry a failed turn.
- Rename the conversation.
- Delete the conversation.
- Switch between conversations (via the rail).
- Select an LLM model and effort level (via the composer model picker).
- Select a workspace folder (via the workspace picker).
- Toggle the preview panel.
- Open settings.

## How to operate or use the features of this page

### Send a message

1. Locate the text input field at the bottom of the card (placeholder "Ketik pertanyaan…") — it sits between the paperclip button (left) and the send button (right).
2. Type your message. The send button (paper-plane icon, far right of input row) becomes active when there is text or pending attachments.
3. Press Enter or click the send button. Your message appears immediately on the right side. The AI response streams in as bubbles on the left side.

### Stop a turn

1. While the AI is responding, the send button hides and a Stop button (stop-square icon only) appears in its place — same slot, far right of the input row.
2. Click Stop to interrupt. The Stop button is only clickable if you are the turn initiator. Non-initiators see it greyed out.
3. After stopping, status shows "Interrupted · floor tetap Anda" and a toast appears.

### Retry a failed turn

1. When a turn fails, an error bubble appears in the messages area (left-aligned, red-tinted, triangle warning icon).
2. At the bottom of the error bubble is a Retry button (circular-arrow icon + "Retry" text).
3. Click Retry to re-send the same message. The error bubble is cleared and a new turn begins.

### Rename a conversation

1. Click the pencil icon button (top-right corner of the room header).
2. A rename dialog appears centered on screen with a dark backdrop overlay. Dialog header shows a pencil-square icon, "Conversation" label, "Rename conversation" title, and a close (X) button in the top-right corner.
3. Enter a new title in the text input (max 60 characters). A character counter shows "N/60" to the right of the input.
4. Click "Save title" button (blue, right side of dialog bottom, with a right-arrow icon) or press Enter to submit.
5. Click "Cancel" (grey ghost button, left side) or click the backdrop or the X icon to dismiss without saving.

### Delete a conversation

1. In the conversations rail (left sidebar), hover over a conversation item to reveal a trash icon button on its right side.
2. Click the trash button. A delete dialog appears centered with backdrop. Dialog header shows a trash icon, "Conversation" label, "Delete conversation" title, and a close (X) button.
3. The dialog body shows a warning message with the conversation name. Click "Delete" (red danger button, right side, with trash icon) to confirm, or "Cancel" (grey ghost, left side) to dismiss.

### Attach files

1. Click the paperclip button (left side of the composer input row).
2. An attachment dialog appears centered with backdrop. Dialog header shows a paperclip icon, "Attachments" label, "Lampirkan file" title, and a close (X) button.
3. The dialog body is a drop zone with a cloud-upload icon and "Drop file di sini" text. Either drag files onto the drop zone or click the "Pilih file" button (blue, with a folder-open icon) to open the file picker.
4. Accepted types: text (md, txt, json, csv, xml, yaml, html) and images (png, jpg, gif, webp). Max 8 MB per file — oversized files show a toast warning.
5. Selected files appear as chips above the composer input. Image chips show a thumbnail preview; text chips show a file-document icon. Each chip has a remove (X) button on its right.
6. Click "Selesai" (grey ghost button, right side of dialog bottom) to close the dialog.
7. Files are uploaded when you send the message.

### Switch conversations

1. Click any conversation item in the left rail. The active conversation is highlighted with a different background color.
2. The messages area clears and loads the selected conversation's messages.

### Expand/collapse reasoning steps

1. Agent reasoning appears as a collapsible section with a right-pointing chevron icon and "Thinking" label.
2. Click the section header to expand or collapse the reasoning steps. When expanded, a numbered list of thinking steps is shown.

### Expand/collapse tool calls

1. Tool calls appear as a collapsible section with a right-pointing chevron icon and "Tool Calls" label, showing a count (e.g. "2 tool calls").
2. Click the section header to expand or collapse. Each tool row shows a compact signature `tool_name(arg=value, …)` (arguments truncated at 80 characters), a result summary, and an optional model badge.

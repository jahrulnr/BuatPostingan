# Composer Toolbar

The bottom input area with model picker, workspace picker, attachments, and send controls.

## Overview

The composer toolbar is the bottom section of the chat card. It is the primary interaction point for sending messages to the AI agent. The composer has two rows:

1. **Toolbar row** (top): Model picker pill (left) and workspace picker pill (right).
2. **Input row** (bottom): Attach button (left), text input (center), send/stop button (right).

Above both rows is a conditional attachment chips area.

## What information will show to user

- **Attachment chips** (above toolbar row): Hidden by default. Appears only when files are pending. Each chip shows: thumbnail preview (images) or file-document icon (text) on the left, file name and formatted size (e.g. "1.2 KB") in the middle, and a remove (X) button on the right.
- **Model picker pill** (left side of toolbar row): A button with a CPU icon on the left, label text in the center (e.g. "gpt-4o-mini · auto"), and a chevron-down icon on the right. Tooltip: "Model & effort".
- **Workspace picker pill** (right side of toolbar row): A button with a folder-open icon on the left, label text in the center (e.g. "my-project" or "Workspace"), and a chevron-down icon on the right. Tooltip: "Workspace folder".
- **Attach button** (left side of input row): Paperclip icon. Tooltip: "Attach file". Greyed out when AI is busy or floor is blocked.
- **Text input** (center of input row): Text field with placeholder "Ketik pertanyaan…". Greyed out when AI is busy or floor is blocked.
- **Send button** (right side of input row): Paper-plane icon only. Greyed out when input is empty and no attachments pending. Hidden while the AI is responding (replaced by Stop in the same slot).
- **Stop button** (right side of input row, same slot as Send): Stop-square icon only (no label). Hidden by default. Appears only while the AI is responding. Greyed out for non-initiators.

### Model picker dropdown

Hidden by default. Opens when the model picker pill is clicked. Contains:

- **Search field** (top of dropdown): Text input with a magnifying-glass icon on the left, placeholder "Search models…". Auto-focuses when opened.
- **Model list** (below search): Each model row shows:
  - A model button with model name (left) and metadata tags (right, e.g. "vision · reasoning"). Selected model is highlighted.
  - If the model supports reasoning effort: a row of effort buttons below the model entry, showing options like "auto", "none", "minimal", "low", "medium", "high", "xhigh", "max". Selected effort is highlighted.
- Empty state: "No models match "query"" or "Models unavailable" (on error).

### Workspace picker dialog

Hidden by default. Opens when the workspace picker pill is clicked. See below for details.

## What we can do at this page

- Select an LLM model and effort level.
- Select a workspace folder for file operations.
- Attach files (text or images) to the message.
- Type and send a message.
- Stop an in-progress AI turn.

## How to operate or use the features of this page

### Select a model

1. Click the model picker pill (left side of toolbar row, CPU icon).
2. A dropdown opens below the pill. A search field (magnifying-glass icon) is at the top — it auto-focuses.
3. Type in the search field to filter models by name, ID, or provider. The list updates in real-time.
4. Click a model button to select it. The dropdown closes and the pill label updates.
5. If the model supports reasoning effort, effort level buttons appear below the model entry in the dropdown. Click an effort button (e.g. "auto", "low", "high") to set it — the dropdown closes.
6. Selections persist across sessions.
7. Press Escape or click outside the dropdown to close without selecting.

### Select a workspace folder

1. Click the workspace picker pill (right side of toolbar row, folder-open icon).
2. A workspace dialog appears centered with a dark backdrop. Dialog header shows a folder-open icon, "Workspace" label, "Pilih folder workspace" title, and a close (X) button in the top-right corner.
3. The dialog body is a folder browser:
   - **Path bar** (top): Up button (up-arrow icon, left) and current path text (e.g. "/") next to it. Up button is greyed out when at root.
   - **Directory list** (below path bar): Scrollable list of subdirectories. Each entry is a button with a folder icon and folder name. Click to navigate into it. Selected directory is highlighted.
   - **Help text** below list: "Pilih folder untuk mengatur working directory. Path absolut digunakan tanpa restriction."
   - **Error text** (red, hidden by default): Shows browse errors.
4. Click "Select folder" (blue button, right side of dialog bottom, check icon) to set the current directory as workspace.
5. Click "Clear (use default)" (grey ghost button, left side) to reset to config default workspace.
6. Click "Cancel" (grey ghost button, center) or the backdrop or the X icon to dismiss.
7. The workspace persists per conversation across sessions.

### Attach files

1. Click the paperclip button (left side of the input row).
2. An attachment dialog appears centered with backdrop. Dialog header shows a paperclip icon, "Attachments" label, "Lampirkan file" title, and a close (X) button.
3. The dialog body is a drop zone with a cloud-upload icon and "Drop file di sini" text. Either drag files onto the drop zone or click the "Pilih file" button (blue, folder-open icon) to open the OS file picker.
4. Accepted types: text (md, txt, json, csv, xml, yaml, html) and images (png, jpg, gif, webp). Max 8 MB per file — oversized files show a toast warning.
5. Selected files appear as chips above the composer toolbar. Image chips show thumbnail preview; text chips show a file-document icon. Each chip has a remove (X) button on its right — click to remove.
6. Click "Selesai" (grey ghost button, right side of dialog bottom) to close the dialog.
7. Files are uploaded when you send the message.

### Send a message

1. Type text in the input field (center of input row, placeholder "Ketik pertanyaan…").
2. The send button (paper-plane icon, far right) becomes active (no longer greyed out) when there is text or pending attachments.
3. Press Enter or click the send button. If attachments are pending, they are uploaded first, then the message is sent with attachment IDs.
4. The send button and input are greyed out while the AI is responding.

### Stop an AI turn

1. While the AI is responding, the send button is hidden and a Stop button (stop-square icon only) appears in its place — same slot, far right of the input row.
2. Click Stop to interrupt. The Stop button is only clickable if you are the turn initiator. Non-initiators see it greyed out.
3. After stopping, the send button reappears and the input is re-enabled.

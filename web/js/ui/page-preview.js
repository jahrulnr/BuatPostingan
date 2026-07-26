const pageMutationTools = new Set(['page_create', 'page_edit', 'page_delete']);
const pageIDPattern = /^[a-z0-9](?:[a-z0-9-]{0,78}[a-z0-9])?$/;

function itemSeq(item) {
    const seq = Number(item && item.seq);
    return Number.isFinite(seq) && seq > 0 ? seq : 0;
}

function pageID(value) {
    const id = String(value || '').trim();
    return pageIDPattern.test(id) ? id : '';
}

/**
 * Correlates durable tool calls/results and rejects replayed or stale results.
 * A result is actionable only once, after its matching successful page mutation.
 */
export function createPagePreviewTracker() {
    const calls = new Map();
    const latestSeqByPage = new Map();

    function observeToolCall(item) {
        const tool = String(item && item.name || '');
        const callID = String(item && item.call_id || '');
        const id = pageID(item && item.arguments && item.arguments.page_id);
        if (!callID || !id || !pageMutationTools.has(tool)) return;
        calls.set(callID, { tool: tool, pageID: id });
    }

    function observeToolResult(item) {
        const callID = String(item && item.call_id || '');
        const call = calls.get(callID);
        const envelope = item && item.envelope;
        const seq = itemSeq(item);
        if (!call || !envelope || envelope.ok !== true || !seq) return null;
        if (envelope.tool && envelope.tool !== call.tool) return null;
        if (seq <= Number(latestSeqByPage.get(call.pageID) || 0)) return null;

        latestSeqByPage.set(call.pageID, seq);
        return { pageID: call.pageID, seq: seq };
    }

    function reset() {
        calls.clear();
        latestSeqByPage.clear();
    }

    return { observeToolCall: observeToolCall, observeToolResult: observeToolResult, reset: reset };
}

/** Mount the existing preview panel and reload it only for a fresh page revision. */
export function bootPagePreview(options) {
    const opts = options || {};
    const panel = opts.panelEl;
    const previewURL = opts.previewURL;
    const tracker = createPagePreviewTracker();
    let frame = null;
    const emptyMarkup = panel ? panel.innerHTML : '';

    function ensureFrame() {
        if (frame || !panel) return frame;
        panel.textContent = '';
        frame = document.createElement('iframe');
        frame.className = 'wc-preview__frame';
        frame.title = 'Static page draft preview';
        frame.setAttribute('sandbox', 'allow-forms allow-scripts');
        frame.setAttribute('referrerpolicy', 'no-referrer');
        panel.appendChild(frame);
        return frame;
    }

    function observeToolCall(item) {
        tracker.observeToolCall(item);
    }

    function observeToolResult(item) {
        const change = tracker.observeToolResult(item);
        if (!change || typeof previewURL !== 'function') return;
        const url = previewURL({ pageID: change.pageID, version: change.seq });
        if (!url) return;
        const currentFrame = ensureFrame();
        if (currentFrame) currentFrame.src = url;
    }

    function reset() {
        tracker.reset();
        frame = null;
        if (panel) panel.innerHTML = emptyMarkup;
    }

    return {
        observeToolCall: observeToolCall,
        observeToolResult: observeToolResult,
        reset: reset,
    };
}

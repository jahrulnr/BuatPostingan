import assert from 'node:assert/strict';
import test from 'node:test';

import { createPagePreviewTracker } from './page-preview.js';

function pageCall(callID, name, pageID) {
    return { type: 'tool_call', call_id: callID, name: name, arguments: { page_id: pageID } };
}

function pageResult(callID, name, seq, ok) {
    return { type: 'tool_result', call_id: callID, seq: seq, envelope: { tool: name, ok: ok } };
}

test('page preview only accepts the fresh successful result of its matching call', function () {
    const tracker = createPagePreviewTracker();
    tracker.observeToolCall(pageCall('create-1', 'page_create', 'about-us'));

    assert.deepEqual(tracker.observeToolResult(pageResult('create-1', 'page_create', 14, true)), {
        pageID: 'about-us', seq: 14,
    });
    assert.equal(tracker.observeToolResult(pageResult('create-1', 'page_create', 14, true)), null);
    assert.equal(tracker.observeToolResult(pageResult('create-1', 'page_create', 13, true)), null);
});

test('page preview rejects failed, uncorrelated, and mismatched tool results', function () {
    const tracker = createPagePreviewTracker();
    tracker.observeToolCall(pageCall('edit-1', 'page_edit', 'about-us'));

    assert.equal(tracker.observeToolResult(pageResult('edit-1', 'page_edit', 20, false)), null);
    assert.equal(tracker.observeToolResult(pageResult('unknown', 'page_edit', 21, true)), null);
    assert.equal(tracker.observeToolResult(pageResult('edit-1', 'page_create', 22, true)), null);
});

test('page preview recognises a future page_delete mutation but ignores non-page tools', function () {
    const tracker = createPagePreviewTracker();
    tracker.observeToolCall(pageCall('delete-1', 'page_delete', 'about-us'));
    tracker.observeToolCall(pageCall('docs-1', 'docs_search', 'about-us'));

    assert.deepEqual(tracker.observeToolResult(pageResult('delete-1', 'page_delete', 24, true)), {
        pageID: 'about-us', seq: 24,
    });
    assert.equal(tracker.observeToolResult(pageResult('docs-1', 'docs_search', 25, true)), null);
});

test('page preview keeps stale detection per page and resets for another conversation', function () {
    const tracker = createPagePreviewTracker();
    tracker.observeToolCall(pageCall('a', 'page_edit', 'about-us'));
    tracker.observeToolCall(pageCall('b', 'page_edit', 'pricing'));

    assert.deepEqual(tracker.observeToolResult(pageResult('a', 'page_edit', 30, true)), { pageID: 'about-us', seq: 30 });
    assert.deepEqual(tracker.observeToolResult(pageResult('b', 'page_edit', 29, true)), { pageID: 'pricing', seq: 29 });

    tracker.reset();
    tracker.observeToolCall(pageCall('c', 'page_edit', 'about-us'));
    assert.deepEqual(tracker.observeToolResult(pageResult('c', 'page_edit', 1, true)), { pageID: 'about-us', seq: 1 });
});

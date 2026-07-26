import assert from 'node:assert/strict';
import test from 'node:test';

import { bootPagePreview, createPagePreviewTracker } from './page-preview.js';

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

function fakePanel() {
    const kids = [];
    const panel = {
        innerHTML: '<div class="wc-preview__empty">empty</div>',
        textContent: '',
        appendChild: function (node) { kids.push(node); return node; },
        _kids: kids,
    };
    Object.defineProperty(panel, 'textContent', {
        set: function () { kids.length = 0; panel.innerHTML = ''; },
        get: function () { return ''; },
    });
    return panel;
}

function installDocumentStub() {
    const prev = globalThis.document;
    globalThis.document = {
        createElement: function (tag) {
            const attrs = {};
            return {
                tagName: String(tag).toUpperCase(),
                className: '',
                title: '',
                src: '',
                setAttribute: function (k, v) { attrs[k] = v; },
                getAttribute: function (k) { return attrs[k]; },
            };
        },
    };
    return function restore() {
        if (prev === undefined) delete globalThis.document;
        else globalThis.document = prev;
    };
}

test('page preview loads home by default when the draft exists', async function () {
    const restore = installDocumentStub();
    try {
        const panel = fakePanel();
        const urls = [];
        const preview = bootPagePreview({
            panelEl: panel,
            previewURL: function (req) {
                urls.push(req);
                return '/api/pages/' + req.pageID + '/?v=' + req.version;
            },
            fetch: async function () { return { ok: true }; },
        });
        await Promise.resolve();
        await Promise.resolve();
        assert.equal(urls[0].pageID, 'home');
        assert.equal(panel._kids.length, 1);
        assert.equal(panel._kids[0].tagName, 'IFRAME');
        assert.match(panel._kids[0].src, /\/api\/pages\/home\/\?v=/);
        preview.reset();
    } finally {
        restore();
    }
});

test('page preview keeps empty state when home draft is missing', async function () {
    const restore = installDocumentStub();
    try {
        const panel = fakePanel();
        bootPagePreview({
            panelEl: panel,
            previewURL: function (req) { return '/api/pages/' + req.pageID + '/?v=' + req.version; },
            fetch: async function () { return { ok: false, status: 404 }; },
        });
        await Promise.resolve();
        await Promise.resolve();
        assert.equal(panel._kids.length, 0);
        assert.match(panel.innerHTML, /wc-preview__empty/);
    } finally {
        restore();
    }
});

test('agent page mutation wins over an in-flight home default', async function () {
    const restore = installDocumentStub();
    try {
        let resolveHome;
        const homePending = new Promise(function (resolve) { resolveHome = resolve; });
        const panel = fakePanel();
        const preview = bootPagePreview({
            panelEl: panel,
            previewURL: function (req) {
                return '/api/pages/' + req.pageID + '/?v=' + req.version;
            },
            fetch: function () { return homePending; },
        });
        preview.observeToolCall(pageCall('edit-1', 'page_edit', 'about-us'));
        preview.observeToolResult(pageResult('edit-1', 'page_edit', 9, true));
        assert.equal(panel._kids[0].src, '/api/pages/about-us/?v=9');
        resolveHome({ ok: true });
        await Promise.resolve();
        await Promise.resolve();
        assert.equal(panel._kids[0].src, '/api/pages/about-us/?v=9');
    } finally {
        restore();
    }
});

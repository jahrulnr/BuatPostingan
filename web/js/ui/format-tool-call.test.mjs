import assert from 'node:assert/strict';
import test from 'node:test';

import { formatToolCall } from './render.js';

test('formatToolCall shows page tool arguments', function () {
    assert.equal(
        formatToolCall('page_list', {}),
        'page_list()'
    );
    assert.equal(
        formatToolCall('page_read', { page_id: 'home', path: 'index.html' }),
        'page_read(page_id="home", path="index.html")'
    );
});

test('formatToolCall truncates long argument strings to 80 chars', function () {
    const long = 'x'.repeat(120);
    const out = formatToolCall('docs_search', { query: long });
    const inner = out.slice('docs_search('.length, -1);
    assert.equal(inner.length, 80);
    assert.ok(inner.endsWith('…'));
    assert.match(out, /^docs_search\(query="/);
});

test('formatToolCall accepts JSON string arguments', function () {
    assert.equal(
        formatToolCall('page_list', '{"limit":10}'),
        'page_list(limit=10)'
    );
});

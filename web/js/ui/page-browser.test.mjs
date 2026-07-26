import assert from 'node:assert/strict';
import test from 'node:test';

import { normalizePages } from './page-browser.js';

test('normalizePages sorts workspaces and preserves tree entries', function () {
    const pages = normalizePages({
        pages: [
            { id: 'zeta', published: false, entries: [{ path: 'index.html', type: 'file' }] },
            { id: 'alpha', published: true, entries: [{ path: 'assets', type: 'directory' }] },
        ],
    });
    assert.deepEqual(pages.map(function (page) { return page.id; }), ['alpha', 'zeta']);
    assert.equal(pages[0].published, true);
    assert.deepEqual(pages[0].entries, [{ path: 'assets', type: 'directory' }]);
});

test('normalizePages ignores incomplete page records', function () {
    assert.deepEqual(normalizePages({ pages: [{ id: '' }, null] }), []);
    assert.deepEqual(normalizePages(null), []);
});

import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

test('provider enabled checkbox has an explicit checked indicator', async function () {
    const css = await readFile(new URL('../../css/settings.css', import.meta.url), 'utf8');

    assert.match(css, /\.bp-provider-form \.bp-check input\[type="checkbox"\]:checked\s*\{/);
    assert.match(css, /\.bp-provider-form \.bp-check input\[type="checkbox"\]:checked::after\s*\{/);
    assert.match(css, /var\(--bp-accent-ink\)/);
});

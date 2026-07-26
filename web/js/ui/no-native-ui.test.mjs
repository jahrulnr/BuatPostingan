import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const sources = [
    new URL('./settings.js', import.meta.url),
    new URL('./page-browser.js', import.meta.url),
];

test('interactive app views do not delegate prompts or confirmations to the browser', async function () {
    const contents = await Promise.all(sources.map(function (url) {
        return readFile(url, 'utf8');
    }));

    contents.forEach(function (source) {
        assert.doesNotMatch(source, /window\.(?:alert|confirm|prompt)\s*\(/);
    });
});

test('provider configuration uses an app picker instead of a native select', async function () {
    const html = await readFile(new URL('../../index.html', import.meta.url), 'utf8');
    assert.doesNotMatch(html, /<select\b/i);
    assert.match(html, /data-ui-select/);
});

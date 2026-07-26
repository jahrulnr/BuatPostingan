import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import { registerPWA } from './pwa.js';

const webRoot = new URL('../', import.meta.url);

async function read(path) {
    return readFile(new URL(path, webRoot), 'utf8');
}

test('PWA manifest declares standalone display and raster install icons', async function () {
    const manifest = JSON.parse(await read('app.webmanifest'));

    assert.equal(manifest.display, 'standalone');
    assert.equal(manifest.start_url, './');
    assert.ok(manifest.icons.some(function (icon) { return icon.sizes === '192x192'; }));
    assert.ok(manifest.icons.some(function (icon) { return icon.sizes === '512x512'; }));
});

test('admin and login pages both opt into the PWA', async function () {
    const [admin, login] = await Promise.all([read('index.html'), read('login.html')]);

    [admin, login].forEach(function (html) {
        assert.match(html, /rel="manifest" href="\.\/app\.webmanifest"/);
        assert.match(html, /\.\/js\/pwa\.js/);
    });
});

test('offline fallback is a standalone, accessible page', async function () {
    const offline = await read('offline.html');

    assert.match(offline, /<title>You're offline · BuatPostingan<\/title>/);
    assert.match(offline, /\.\/css\/offline\.css/);
    assert.match(offline, /\.\/assets\/offline-robot\.svg/);
    assert.match(offline, /Try again/);
});

test('service worker caches only static assets plus the offline fallback', async function () {
    const worker = await read('sw.js');

    assert.match(worker, /request\.method !== 'GET'/);
    assert.match(worker, /url\.origin !== self\.location\.origin/);
    assert.match(worker, /STATIC_DIRECTORIES/);
    assert.match(worker, /OFFLINE_PATH/);
    assert.match(worker, /caches\.match\(offlineURL\(\)\)/);
    assert.doesNotMatch(worker, /caches\.match\(request\).*\/api\//s);
});

test('PWA registration scopes the worker to its containing app directory', async function () {
    const calls = [];
    const result = await registerPWA({
        serviceWorker: {
            register: function (url, options) {
                calls.push({ url: String(url), options: options });
                return Promise.resolve('registered');
            },
        },
    }, 'https://example.test/admin/js/pwa.js');

    assert.equal(result, 'registered');
    assert.equal(calls.length, 1);
    assert.match(calls[0].url, /\/sw\.js$/);
    assert.equal(calls[0].options.scope, '/admin/');
    assert.equal(calls[0].options.updateViaCache, 'none');
});

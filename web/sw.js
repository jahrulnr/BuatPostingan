const CACHE_NAME = 'buatpostingan-static-v2';
const STATIC_DIRECTORIES = ['assets/', 'css/', 'js/', 'vendor/'];
const OFFLINE_PATH = 'offline.html';

function scopePath() {
    return new URL(self.registration.scope).pathname.replace(/\/$/, '') + '/';
}

function offlineURL() {
    return new URL(OFFLINE_PATH, self.registration.scope).toString();
}

function isCacheableStaticAsset(request) {
    if (request.method !== 'GET' || request.mode === 'navigate') return false;
    const url = new URL(request.url);
    if (url.origin !== self.location.origin || url.pathname.includes('/api/')) return false;
    const root = scopePath();
    return STATIC_DIRECTORIES.some(function (directory) {
        return url.pathname.startsWith(root + directory);
    });
}

self.addEventListener('install', function (event) {
    self.skipWaiting();
    event.waitUntil(caches.open(CACHE_NAME).then(function (cache) {
        return cache.addAll([
            offlineURL(),
            new URL('css/offline.css', self.registration.scope).toString(),
            new URL('assets/offline-robot.svg', self.registration.scope).toString(),
            new URL('assets/buatpostingan-mark.svg', self.registration.scope).toString(),
        ]);
    }));
});

self.addEventListener('activate', function (event) {
    event.waitUntil(caches.keys().then(function (keys) {
        return Promise.all(keys.filter(function (key) {
            return key.startsWith('buatpostingan-static-') && key !== CACHE_NAME;
        }).map(function (key) {
            return caches.delete(key);
        }));
    }).then(function () {
        return self.clients.claim();
    }));
});

self.addEventListener('fetch', function (event) {
    const request = event.request;
    const url = new URL(request.url);

    if (request.method === 'GET' && request.mode === 'navigate' && url.origin === self.location.origin) {
        event.respondWith(fetch(request).catch(function () {
            return caches.match(offlineURL());
        }));
        return;
    }

    if (!isCacheableStaticAsset(request)) return;

    event.respondWith(fetch(request).then(function (response) {
        if (response.ok && response.type === 'basic') {
            const copy = response.clone();
            event.waitUntil(caches.open(CACHE_NAME).then(function (cache) {
                return cache.put(request, copy);
            }));
        }
        return response;
    }).catch(function () {
        return caches.match(request);
    }));
});

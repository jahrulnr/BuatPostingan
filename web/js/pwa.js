/** Register the admin-scoped service worker when the browser supports it. */
export function registerPWA(navigatorRef, moduleURL) {
    const nav = navigatorRef || (typeof navigator !== 'undefined' ? navigator : null);
    if (!nav || !('serviceWorker' in nav)) return Promise.resolve(null);

    const workerURL = new URL('../sw.js', moduleURL || import.meta.url);
    const scopeURL = new URL('./', workerURL);
    return nav.serviceWorker.register(workerURL, {
        scope: scopeURL.pathname,
        updateViaCache: 'none',
    }).catch(function () {
        // PWA support is progressive; an unavailable worker must not block the app.
        return null;
    });
}

if (typeof window !== 'undefined') {
    window.addEventListener('load', function () {
        void registerPWA();
    }, { once: true });
}

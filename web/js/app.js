import { api, authLogout, authMe, listPages, publishPage, unpublishPage, deletePage } from './api/index.js';
import { bootChat } from './ui/chat.js';
import { bootTheme } from './ui/theme.js';
import { bootPreviewResize } from './ui/preview-resize.js';
import { bootSettings } from './ui/settings.js';
import { bootPageBrowser } from './ui/page-browser.js';

function paintModeBadge() {
    const badge = document.getElementById('bpModeBadge');
    if (!badge) return;
    badge.dataset.mode = api.mockMode ? 'mock' : 'real';
    badge.textContent = 'mode: ' + (api.mockMode ? 'mock' : 'real');
    badge.title = api.mockMode
        ? 'Mock driver aktif (?mock=0 untuk kembali ke real)'
        : 'Real driver → ' + api.baseUrl + ' (?mock=1 untuk mock)';
}

function bootChrome() {
    const shell = document.getElementById('bpShell');
    const scrim = document.getElementById('bpScrim');
    const railBtn = document.getElementById('btnToggleRail');
    const previewBtn = document.getElementById('btnTogglePreview');
    const chromeRoom = document.getElementById('chromeRoomTitle');
    const roomTitle = document.getElementById('roomTitle');

    function setOpen(kind, open) {
        if (!shell) return;
        const cls = kind === 'rail' ? 'is-rail-open' : 'is-preview-open';
        const other = kind === 'rail' ? 'is-preview-open' : 'is-rail-open';
        shell.classList.toggle(cls, open);
        if (open) shell.classList.remove(other);
        const anyOpen = shell.classList.contains('is-rail-open') || shell.classList.contains('is-preview-open');
        if (scrim) scrim.hidden = !anyOpen;
        if (railBtn) railBtn.setAttribute('aria-expanded', shell.classList.contains('is-rail-open') ? 'true' : 'false');
        if (previewBtn) previewBtn.setAttribute('aria-expanded', shell.classList.contains('is-preview-open') ? 'true' : 'false');
    }

    function closeDrawers() {
        setOpen('rail', false);
        setOpen('preview', false);
    }

    if (railBtn) {
        railBtn.addEventListener('click', function () {
            const open = !(shell && shell.classList.contains('is-rail-open'));
            setOpen('rail', open);
        });
    }
    if (previewBtn) {
        previewBtn.addEventListener('click', function () {
            const open = !(shell && shell.classList.contains('is-preview-open'));
            setOpen('preview', open);
        });
    }
    if (scrim) {
        scrim.addEventListener('click', closeDrawers);
    }
    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') closeDrawers();
    });

    // Mirror room title into chrome (chat.js owns #roomTitle text)
    if (roomTitle && chromeRoom && typeof MutationObserver !== 'undefined') {
        const sync = function () {
            chromeRoom.textContent = roomTitle.textContent || 'New chat';
        };
        sync();
        new MutationObserver(sync).observe(roomTitle, { characterData: true, childList: true, subtree: true });
    }

    const pageBrowser = bootPageBrowser({
        panelEl: document.getElementById('panelPages'),
        api: {
            listPages: function () { return listPages(api); },
            publishPage: function (req) { return publishPage(api, req); },
            unpublishPage: function (req) { return unpublishPage(api, req); },
            deletePage: function (req) { return deletePage(api, req); },
        },
    });

    // Preview and Pages tabs.
    const tabs = document.querySelectorAll('[data-preview-tab]');
    tabs.forEach(function (tab) {
        tab.addEventListener('click', function () {
            const name = tab.getAttribute('data-preview-tab');
            tabs.forEach(function (t) {
                const on = t === tab;
                t.classList.toggle('is-active', on);
                t.setAttribute('aria-selected', on ? 'true' : 'false');
            });
            document.querySelectorAll('[data-preview-panel]').forEach(function (panel) {
                const on = panel.getAttribute('data-preview-panel') === name;
                panel.classList.toggle('is-active', on);
                panel.hidden = !on;
            });
            if (name === 'pages') pageBrowser.refresh();
        });
    });
}

bootTheme();

function bootAuthenticated(user) {
    if (user && user.user) {
        window.__BP_ADMIN_USER_ID__ = user.user.id;
        window.__BP_ADMIN_DISPLAY_NAME__ = user.user.displayName;
    }
    const logoutButton = document.getElementById('btnLogout');
    if (logoutButton && !api.mockMode) {
        logoutButton.addEventListener('click', function () {
            logoutButton.disabled = true;
            authLogout(api).finally(function () {
                window.location.replace('./login.html');
            });
        });
    }
    paintModeBadge();
    bootChrome();
    bootPreviewResize();
    bootSettings();
    bootChat();
}

if (api.mockMode) {
    bootAuthenticated(null);
} else {
    authMe(api).then(bootAuthenticated).catch(function () {
        const next = encodeURIComponent(window.location.pathname + window.location.search);
        window.location.replace('./login.html?next=' + next);
    });
}

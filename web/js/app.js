import { api } from './api/index.js';
import { bootChat } from './ui/chat.js';
import { bootTheme } from './ui/theme.js';
import { bootPreviewResize } from './ui/preview-resize.js';

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

    // Preview tabs — empty panels only
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
        });
    });
}

bootTheme();
paintModeBadge();
bootChrome();
bootPreviewResize();
bootChat();

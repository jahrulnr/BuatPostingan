import { api } from './api/index.js';
import { bootChat } from './ui/chat.js';

function paintModeBadge() {
    const badge = document.getElementById('bpModeBadge');
    if (!badge) return;
    badge.dataset.mode = api.mockMode ? 'mock' : 'real';
    badge.textContent = 'mode: ' + (api.mockMode ? 'mock' : 'real');
    badge.title = api.mockMode
        ? 'Mock driver aktif (?mock=0 untuk real)'
        : 'Real driver → ' + api.baseUrl + ' (?mock=1 untuk mock)';
}

paintModeBadge();
bootChat();

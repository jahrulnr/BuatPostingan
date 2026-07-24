import * as mock from './mock/driver.js';
import * as real from './real/driver.js';

/**
 * Resolve mockMode:
 * 1. ?mock=0|1|true|false
 * 2. window.__BP_MOCK__
 * 3. localStorage bp.mockMode
 * 4. default true
 */
export function resolveMockMode() {
    try {
        const params = new URLSearchParams(window.location.search);
        if (params.has('mock')) {
            const v = String(params.get('mock') || '').toLowerCase();
            return !(v === '0' || v === 'false' || v === 'no' || v === 'real');
        }
    } catch (e) { /* ignore */ }

    if (typeof window.__BP_MOCK__ === 'boolean') {
        return window.__BP_MOCK__;
    }

    try {
        const stored = localStorage.getItem('bp.mockMode');
        if (stored === '0' || stored === 'false') return false;
        if (stored === '1' || stored === 'true') return true;
    } catch (e) { /* ignore */ }

    return true;
}

const mockMode = resolveMockMode();

export const listConversations = mockMode
    ? mock.listConversationsMock
    : real.listConversationsImpl;

export const createThread = mockMode
    ? mock.createThreadMock
    : real.createThreadImpl;

export const getThread = mockMode
    ? mock.getThreadMock
    : real.getThreadImpl;

export const renameThread = mockMode
    ? mock.renameThreadMock
    : real.renameThreadImpl;

export const startTurn = mockMode
    ? mock.startTurnMock
    : real.startTurnImpl;

export const retryTurn = mockMode
    ? mock.retryTurnMock
    : real.retryTurnImpl;

export const interruptTurn = mockMode
    ? mock.interruptTurnMock
    : real.interruptTurnImpl;

export const subscribeEvents = mockMode
    ? mock.subscribeEventsMock
    : real.subscribeEventsImpl;

/** @type {import('./types.js').ApiContext} */
export const api = {
    baseUrl: window.__BP_API_BASE__ || '/api/webchat',
    mockMode: mockMode,
    adminUserId: Number(window.__BP_ADMIN_USER_ID__ || 1),
    adminDisplayName: String(window.__BP_ADMIN_DISPLAY_NAME__ || 'Admin User'),
    csrf: window.__BP_CSRF__ || '',
};

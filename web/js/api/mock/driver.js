import { createStore } from './store.js';
import { resolveInitialDocsIndex } from './fixtures.js';

const store = createStore({ docsIndexReady: resolveInitialDocsIndex() });

function adminId(api) {
    return Number((api && api.adminUserId) || window.__BP_ADMIN_USER_ID__ || 1);
}

function adminName(api) {
    return String((api && api.adminDisplayName) || window.__BP_ADMIN_DISPLAY_NAME__ || 'Admin User');
}

/** @param {import('../types.js').ApiContext} api */
export function listConversationsMock(api, _req) {
    return Promise.resolve(store.listConversations());
}

/** @param {import('../types.js').ApiContext} api */
export function createThreadMock(api, _req) {
    try {
        return Promise.resolve(store.createThread(adminId(api)));
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function getThreadMock(api, req) {
    try {
        return Promise.resolve(store.getThread(req.threadId));
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function renameThreadMock(api, req) {
    try {
        return Promise.resolve(store.renameThread(req.threadId, req.title));
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function startTurnMock(api, req) {
    try {
        return Promise.resolve(
            store.startTurn(req.threadId, req.message, adminId(api), adminName(api))
        );
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function retryTurnMock(api, req) {
    try {
        return Promise.resolve(store.retryTurn(req.threadId, req.turnId, adminId(api)));
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function interruptTurnMock(api, req) {
    try {
        return Promise.resolve(store.interruptTurn(req.threadId, req.turnId, adminId(api)));
    } catch (err) {
        return Promise.reject(err);
    }
}

/**
 * Fake SSE: callback-based subscription with same event names as EventSource.
 * @param {import('../types.js').ApiContext} api
 * @param {import('../types.js').SubscribeEventsRequest} req
 * @returns {import('../types.js').EventSubscription}
 */
export function subscribeEventsMock(api, req) {
    try {
        return store.subscribe(req.threadId, req.afterSeq || 0, function (eventName, data) {
            req.onEvent(eventName, data);
        });
    } catch (err) {
        if (typeof req.onError === 'function') req.onError(err);
        return { close: function () {} };
    }
}

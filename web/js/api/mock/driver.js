import { createStore } from './store.js';
import { resolveInitialDocsIndex } from './fixtures.js';
import { createSettingsStore } from './settings-store.js';

const store = createStore({ docsIndexReady: resolveInitialDocsIndex() });
const settingsStore = createSettingsStore();
let disconnectInjected = false;

function shouldInjectDisconnect() {
    if (disconnectInjected) return false;
    return new URLSearchParams(window.location.search).get('mock_disconnect') === '1';
}

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

export function deleteThreadMock(api, req) {
    try {
        return Promise.resolve(store.deleteThread(req.threadId));
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function startTurnMock(api, req) {
    try {
        return Promise.resolve(
            store.startTurn(
                req.threadId,
                req.message,
                adminId(api),
                adminName(api),
                req.attachmentIds || [],
                { model: req.model, effort: req.effort }
            )
        );
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function listModelsMock(_api, _req) {
    return Promise.resolve({
        models: [
            {
                id: 'stub/default',
                label: 'Stub default',
                provider: 'STUB',
                supports_vision: false,
                supported_efforts: [],
                default_effort: 'auto',
                disabled: false,
            },
            {
                id: 'stub/reasoning',
                label: 'Stub reasoning',
                provider: 'STUB',
                supports_vision: false,
                supported_efforts: ['none', 'low', 'medium', 'high'],
                default_effort: 'medium',
                disabled: false,
            },
            {
                id: 'stub/vision',
                label: 'Stub vision',
                provider: 'STUB',
                supports_vision: true,
                supported_efforts: [],
                default_effort: 'auto',
                disabled: false,
            },
        ],
        default_model_id: 'stub/default',
        stub: true,
        effort: {
            current: 'auto',
            options: ['auto', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'],
        },
    });
}

/** @param {import('../types.js').ApiContext} api */
export function uploadAttachmentMock(api, req) {
    try {
        return Promise.resolve(
            store.uploadAttachment(req.threadId, {
                filename: req.filename,
                mime: req.mime,
                size: req.size,
                kind: req.kind,
                content: req.content,
                width: req.width,
                height: req.height,
                adminUserId: adminId(api),
            })
        );
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function listAttachmentsMock(api, req) {
    try {
        return Promise.resolve(store.listAttachments(req.threadId));
    } catch (err) {
        return Promise.reject(err);
    }
}

/** @param {import('../types.js').ApiContext} api */
export function retryTurnMock(api, req) {
    try {
        return Promise.resolve(store.retryTurn(req.threadId, req.turnId, adminId(api), req.model, req.effort));
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
        const subscription = store.subscribe(req.threadId, req.afterSeq || 0, function (eventName, data) {
            req.onEvent(eventName, data);
        });
        if (typeof req.onOpen === 'function') req.onOpen();
        if (shouldInjectDisconnect()) {
            disconnectInjected = true;
            setTimeout(function () {
                subscription.close();
                if (typeof req.onError === 'function') {
                    req.onError(new Error('mock_sse_disconnect'));
                }
            }, 250);
        }
        return subscription;
    } catch (err) {
        if (typeof req.onError === 'function') req.onError(err);
        return { close: function () {} };
    }
}

function wrapSettings(fn) {
    try {
        return Promise.resolve(fn());
    } catch (err) {
        return Promise.reject(err);
    }
}

export function getSettingsSnapshotMock() {
    return wrapSettings(function () { return settingsStore.snapshot(); });
}
export function listSettingsUsersMock() {
    return wrapSettings(function () { return settingsStore.listUsers(); });
}
export function createSettingsUserMock(_api, body) {
    return wrapSettings(function () { return settingsStore.createUser(body); });
}
export function updateSettingsUserMock(_api, id, body) {
    return wrapSettings(function () { return settingsStore.updateUser(id, body); });
}
export function deleteSettingsUserMock(_api, id) {
    return wrapSettings(function () { settingsStore.deleteUser(id); return null; });
}
export function listLLMProvidersMock() {
    return wrapSettings(function () { return settingsStore.listProviders(); });
}
export function getLLMProviderMock(_api, id) {
    return wrapSettings(function () { return settingsStore.getProvider(id); });
}
export function createLLMProviderMock(_api, body) {
    return wrapSettings(function () { return settingsStore.createProvider(body); });
}
export function updateLLMProviderMock(_api, id, body) {
    return wrapSettings(function () { return settingsStore.updateProvider(id, body); });
}
export function deleteLLMProviderMock(_api, id) {
    return wrapSettings(function () { settingsStore.deleteProvider(id); return null; });
}
export function addLLMModelMock(_api, id, model) {
    return wrapSettings(function () { return settingsStore.addModel(id, model); });
}
export function removeLLMModelMock(_api, id, modelId) {
    return wrapSettings(function () { return settingsStore.removeModel(id, modelId); });
}
export function importLLMModelsMock(_api, id) {
    return wrapSettings(function () { return settingsStore.importModels(id); });
}

export function browseDirMock(_api, req) {
    var path = (req && req.path) || '/';
    return Promise.resolve({
        path: path,
        parent: path !== '/' ? path.replace(/\/[^/]+\/?$/, '') || '/' : '',
        entries: [
            { name: 'mock-folder', path: path.replace(/\/+$/, '') + '/mock-folder' },
            { name: 'project', path: path.replace(/\/+$/, '') + '/project' },
        ],
    });
}

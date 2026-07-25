function csrfHeader(api) {
    const token = (api && api.csrf) || window.__BP_CSRF__ || '';
    return token ? { 'X-CSRF-TOKEN': token } : {};
}

function newTraceId() {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
        return 'tr_' + crypto.randomUUID().replace(/-/g, '');
    }
    return 'tr_' + Date.now().toString(16) + Math.random().toString(16).slice(2, 10);
}

function traceHeader() {
    return { 'X-Trace-Id': newTraceId() };
}

function jsonHeaders(api) {
    return Object.assign(
        {
            Accept: 'application/json',
            'Content-Type': 'application/json',
        },
        csrfHeader(api),
        traceHeader()
    );
}

function acceptHeaders(api) {
    return Object.assign({ Accept: 'application/json' }, csrfHeader(api), traceHeader());
}

async function parseJson(res) {
    const body = await res.json().catch(function () { return {}; });
    const traceId = res.headers.get('X-Trace-Id') || res.headers.get('x-trace-id') || '';
    if (!res.ok) {
        const err = new Error(
            (body && (body.code || body.error || (body.error && body.error.code))) ||
            ('HTTP ' + res.status)
        );
        err.status = res.status;
        err.body = body;
        err.retryAfter = res.headers.get('Retry-After');
        err.traceId = traceId;
        throw err;
    }
    if (traceId) body._traceId = traceId;
    return body;
}

/** @param {import('../types.js').ApiContext} api */
export function listConversationsImpl(api, _req) {
    return fetch(api.baseUrl + '/conversations', {
        headers: acceptHeaders(api),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function createThreadImpl(api, _req) {
    return fetch(api.baseUrl + '/threads', {
        method: 'POST',
        headers: acceptHeaders(api),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function getThreadImpl(api, req) {
    const after = req.afterSeq || 0;
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '?after_seq=' + after, {
        headers: acceptHeaders(api),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function renameThreadImpl(api, req) {
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId), {
        method: 'PATCH',
        headers: jsonHeaders(api),
        body: JSON.stringify({ title: req.title }),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function deleteThreadImpl(api, req) {
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId), {
        method: 'DELETE',
        headers: acceptHeaders(api),
    }).then(function (res) {
        if (!res.ok) return parseJson(res);
        return null;
    });
}

/** @param {import('../types.js').ApiContext} api */
export function startTurnImpl(api, req) {
    const body = {
        message: req.message,
        attachment_ids: req.attachmentIds || [],
        ui_path: window.location.href,
    };
    if (req.model) body.model = req.model;
    if (req.effort) body.effort = req.effort;
    if (req.workspace) body.workspace = req.workspace;
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '/turns', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify(body),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function listModelsImpl(api, _req) {
    return fetch(api.baseUrl + '/models', {
        headers: acceptHeaders(api),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function uploadAttachmentImpl(api, req) {
    const form = new FormData();
    form.append('file', req.file, req.file && req.file.name);
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '/attachments', {
        method: 'POST',
        headers: acceptHeaders(api),
        body: form,
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function listAttachmentsImpl(api, req) {
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '/attachments', {
        headers: acceptHeaders(api),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function retryTurnImpl(api, req) {
    const body = {
        turn_id: req.turnId,
        ui_path: window.location.href,
    };
    if (req.model) body.model = req.model;
    if (req.effort) body.effort = req.effort;
    if (req.workspace) body.workspace = req.workspace;
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '/retry', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify(body),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function interruptTurnImpl(api, req) {
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '/interrupt', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify({ turn_id: req.turnId }),
    }).then(parseJson);
}

/**
 * @param {import('../types.js').ApiContext} api
 * @param {import('../types.js').SubscribeEventsRequest} req
 * @returns {import('../types.js').EventSubscription}
 */
export function subscribeEventsImpl(api, req) {
    const url =
        api.baseUrl +
        '/threads/' +
        encodeURIComponent(req.threadId) +
        '/events?after_seq=' +
        (req.afterSeq || 0);
    const es = new EventSource(url);
    const names = [
        'thread.started',
        'turn.started',
        'turn.resumed',
        'turn.completed',
        'turn.failed',
        'item.completed',
        'item.delta',
        'item.updated',
        'conversation.updated',
    ];
    es.onopen = function () {
        if (typeof req.onOpen === 'function') {
            req.onOpen();
        }
    };
    names.forEach(function (name) {
        es.addEventListener(name, function (e) {
            let data = {};
            try {
                data = typeof e.data === 'string' ? JSON.parse(e.data) : e.data;
            } catch (err) {
                data = { raw: e.data };
            }
            req.onEvent(name, data);
        });
    });
    es.onerror = function () {
        if (typeof req.onError === 'function') {
            req.onError(new Error('sse_error'));
        }
    };
    return {
        close: function () {
            es.close();
        },
    };
}

function settingsBase(api) {
    const base = String((api && api.baseUrl) || '').replace(/\/api\/webchat\/?$/, '');
    return (base || '') + '/api/settings';
}

/** @param {import('../types.js').ApiContext} api */
export function getSettingsSnapshotImpl(api) {
    return fetch(settingsBase(api), { headers: { Accept: 'application/json' } }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function listSettingsUsersImpl(api) {
    return fetch(settingsBase(api) + '/users', { headers: { Accept: 'application/json' } }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function createSettingsUserImpl(api, body) {
    return fetch(settingsBase(api) + '/users', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify(body || {}),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function updateSettingsUserImpl(api, id, body) {
    return fetch(settingsBase(api) + '/users/' + encodeURIComponent(id), {
        method: 'PATCH',
        headers: jsonHeaders(api),
        body: JSON.stringify(body || {}),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function deleteSettingsUserImpl(api, id) {
    return fetch(settingsBase(api) + '/users/' + encodeURIComponent(id), {
        method: 'DELETE',
        headers: acceptHeaders(api),
    }).then(function (res) {
        if (!res.ok) return parseJson(res);
        return null;
    });
}

/** @param {import('../types.js').ApiContext} api */
export function listLLMProvidersImpl(api) {
    return fetch(settingsBase(api) + '/llm/providers', { headers: { Accept: 'application/json' } }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function getLLMProviderImpl(api, id) {
    return fetch(settingsBase(api) + '/llm/providers/' + encodeURIComponent(id), {
        headers: acceptHeaders(api),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function createLLMProviderImpl(api, body) {
    return fetch(settingsBase(api) + '/llm/providers', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify(body || {}),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function updateLLMProviderImpl(api, id, body) {
    return fetch(settingsBase(api) + '/llm/providers/' + encodeURIComponent(id), {
        method: 'PATCH',
        headers: jsonHeaders(api),
        body: JSON.stringify(body || {}),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function deleteLLMProviderImpl(api, id) {
    return fetch(settingsBase(api) + '/llm/providers/' + encodeURIComponent(id), {
        method: 'DELETE',
        headers: acceptHeaders(api),
    }).then(function (res) {
        if (!res.ok) return parseJson(res);
        return null;
    });
}

/** @param {import('../types.js').ApiContext} api */
export function addLLMModelImpl(api, id, model) {
    return fetch(settingsBase(api) + '/llm/providers/' + encodeURIComponent(id) + '/models', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify(model || {}),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function removeLLMModelImpl(api, id, modelId) {
    return fetch(
        settingsBase(api) +
            '/llm/providers/' +
            encodeURIComponent(id) +
            '/models/' +
            encodeURIComponent(modelId),
        {
            method: 'DELETE',
            headers: acceptHeaders(api),
        }
    ).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function importLLMModelsImpl(api, id) {
    return fetch(settingsBase(api) + '/llm/providers/' + encodeURIComponent(id) + '/import-models', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: '{}',
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function browseDirImpl(api, req) {
    const body = {};
    if (req && req.path) body.path = req.path;
    return fetch(api.baseUrl + '/browse', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify(body),
    }).then(parseJson);
}

function csrfHeader(api) {
    const token = (api && api.csrf) || window.__BP_CSRF__ || '';
    return token ? { 'X-CSRF-TOKEN': token } : {};
}

function jsonHeaders(api) {
    return Object.assign(
        {
            Accept: 'application/json',
            'Content-Type': 'application/json',
        },
        csrfHeader(api)
    );
}

async function parseJson(res) {
    const body = await res.json().catch(function () { return {}; });
    if (!res.ok) {
        const err = new Error(
            (body && (body.code || body.error || (body.error && body.error.code))) ||
            ('HTTP ' + res.status)
        );
        err.status = res.status;
        err.body = body;
        err.retryAfter = res.headers.get('Retry-After');
        throw err;
    }
    return body;
}

/** @param {import('../types.js').ApiContext} api */
export function listConversationsImpl(api, _req) {
    return fetch(api.baseUrl + '/conversations', {
        headers: { Accept: 'application/json' },
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function createThreadImpl(api, _req) {
    return fetch(api.baseUrl + '/threads', {
        method: 'POST',
        headers: Object.assign({ Accept: 'application/json' }, csrfHeader(api)),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function getThreadImpl(api, req) {
    const after = req.afterSeq || 0;
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '?after_seq=' + after, {
        headers: { Accept: 'application/json' },
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
export function startTurnImpl(api, req) {
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '/turns', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify({ message: req.message }),
    }).then(parseJson);
}

/** @param {import('../types.js').ApiContext} api */
export function retryTurnImpl(api, req) {
    return fetch(api.baseUrl + '/threads/' + encodeURIComponent(req.threadId) + '/retry', {
        method: 'POST',
        headers: jsonHeaders(api),
        body: JSON.stringify({ turn_id: req.turnId }),
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
    ];
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

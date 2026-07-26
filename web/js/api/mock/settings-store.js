/** In-memory settings for mock driver (mirrors /api/settings). */

function uid(prefix) {
    return prefix + '_' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}

function maskKey(key) {
    const k = String(key || '').trim();
    if (!k) return { set: false, masked: '' };
    if (k.length <= 4) return { set: true, masked: '••••' };
    return { set: true, masked: '••••…' + k.slice(-4) };
}

function toPublic(p) {
    const m = maskKey(p.api_key);
    return {
        type: p.type || 'openai-compatible',
        id: p.id,
        name: p.name,
        prefix: p.prefix || '',
        api: p.api,
        base_url: p.base_url,
        api_key_set: m.set,
        api_key_masked: m.masked,
        api_key_optional: !!p.api_key_optional,
        enabled: !!p.enabled,
        models: (p.models || []).map(function (x) {
            return Object.assign({}, x, { id: x.id, label: x.label || '' });
        }),
        timeout_sec: p.timeout_sec || 0,
        max_attempts: p.max_attempts || 0,
        weight: p.weight || 0,
    };
}

export function createSettingsStore() {
    const providerCatalog = [
        { type: 'openrouter', name: 'OpenRouter', description: 'One API for models from many upstream providers.', auth_type: 'api_key', api: 'chat', base_url: 'https://openrouter.ai/api/v1', prefix: 'openrouter', icon: 'OR', accent: '#7c5cff', configurable: true },
        { type: 'omniroute', name: 'OmniRoute', description: 'Local multi-provider AI gateway with OpenAI-compatible endpoints.', auth_type: 'local', api: 'responses', base_url: 'http://127.0.0.1:20128/v1', prefix: 'omniroute', icon: 'OM', accent: '#ef476f', configurable: true, api_key_optional: true },
        { type: '9router', name: '9Router', description: 'Local 9Router gateway using its OpenAI-compatible endpoint.', auth_type: 'local', api: 'chat', base_url: 'http://127.0.0.1:20128/v1', prefix: '9router', icon: '9R', accent: '#16c784', configurable: true, api_key_optional: true },
        { type: 'openai', name: 'OpenAI', description: 'Official OpenAI API using the Responses API by default.', auth_type: 'api_key', api: 'responses', base_url: 'https://api.openai.com/v1', prefix: 'openai', icon: 'OA', accent: '#10a37f', configurable: true },
        { type: 'openai-compatible', name: 'OpenAI Compatible', description: 'Custom OpenAI-compatible endpoint hosted by you or another vendor.', auth_type: 'api_key', api: 'chat', base_url: '', prefix: 'compatible', icon: 'OC', accent: '#64748b', configurable: true },
        { type: 'claude', name: 'Claude API', description: 'Official Anthropic Messages API for Claude models.', auth_type: 'api_key', api: 'messages', base_url: 'https://api.anthropic.com/v1', prefix: 'claude', icon: 'AI', accent: '#d97757', configurable: true },
    ];
    /** @type {{users: Object[], llm: {strategy:string, active_provider:string, stub:boolean, providers: Object[]}}} */
    const state = {
        users: [{ id: 'usr_owner', name: 'Owner', role: 'owner' }],
        llm: {
            strategy: 'failover',
            active_provider: '',
            stub: true,
            providers: [],
        },
    };

    function ApiError(status, code, message) {
        const err = new Error(code || message || 'error');
        err.status = status;
        err.code = code;
        err.body = { code: code, message: message, error: code };
        return err;
    }

    function snapshot() {
        return {
            source: 'mock',
            config_path: 'storage/config.json',
            users: state.users.slice(),
            llm: {
                strategy: state.llm.strategy,
                active_provider: state.llm.active_provider,
                stub: state.llm.stub,
                providers: state.llm.providers.map(toPublic),
            },
        };
    }

    function listUsers() {
        return { users: state.users.slice() };
    }

    function createUser(body) {
        const name = String(body && body.name || '').trim();
        const role = String(body && body.role || '').toLowerCase();
        if (!name) throw ApiError(422, 'validation', 'name required');
        if (['owner', 'admin', 'member'].indexOf(role) < 0) {
            throw ApiError(422, 'validation', 'role must be owner|admin|member');
        }
        const u = { id: uid('usr'), name: name, role: role };
        state.users.push(u);
        return u;
    }

    function updateUser(id, body) {
        const idx = state.users.findIndex(function (u) { return u.id === id; });
        if (idx < 0) throw ApiError(404, 'not_found', 'user not found');
        const u = Object.assign({}, state.users[idx]);
        if (body && body.name) u.name = String(body.name).trim();
        if (body && body.role) {
            const role = String(body.role).toLowerCase();
            if (['owner', 'admin', 'member'].indexOf(role) < 0) {
                throw ApiError(422, 'validation', 'role must be owner|admin|member');
            }
            const owners = state.users.filter(function (x) { return x.role === 'owner'; }).length;
            if (u.role === 'owner' && role !== 'owner' && owners <= 1) {
                throw ApiError(422, 'validation', 'cannot demote the last owner');
            }
            u.role = role;
        }
        state.users[idx] = u;
        return u;
    }

    function deleteUser(id) {
        const idx = state.users.findIndex(function (u) { return u.id === id; });
        if (idx < 0) throw ApiError(404, 'not_found', 'user not found');
        const owners = state.users.filter(function (x) { return x.role === 'owner'; }).length;
        if (state.users[idx].role === 'owner' && owners <= 1) {
            throw ApiError(422, 'validation', 'cannot delete the last owner');
        }
        state.users.splice(idx, 1);
    }

    function listProviders() {
        return { providers: state.llm.providers.map(toPublic) };
    }

    function listProviderCatalog() {
        return { providers: providerCatalog.map(function (p) { return Object.assign({}, p); }) };
    }

    function getProvider(id) {
        const p = state.llm.providers.find(function (x) {
            return String(x.id).toUpperCase() === String(id || '').toUpperCase();
        });
        if (!p) throw ApiError(404, 'not_found', 'provider not found');
        return toPublic(p);
    }

    function createProvider(body) {
        let id = String(body && (body.id || body.prefix) || '').trim().toUpperCase();
        if (!id) throw ApiError(422, 'validation', 'id or prefix required');
        if (state.llm.providers.some(function (p) { return p.id === id; })) {
            throw ApiError(422, 'validation', 'provider id already exists');
        }
        const base = String(body.base_url || '').replace(/\/+$/, '');
        if (!base) throw ApiError(422, 'validation', 'base_url required');
        let api = String(body.api || 'responses').toLowerCase();
        if (api !== 'chat' && api !== 'responses' && api !== 'messages') {
            throw ApiError(422, 'validation', 'api must be chat|responses|messages');
        }
        const models = Array.isArray(body.models) ? body.models.slice() : [];
        if (!models.length && body.model_id) {
            models.push({ id: String(body.model_id), label: '' });
        }
        const p = {
            type: String(body.type || 'openai-compatible'),
            id: id,
            name: String(body.name || id).trim(),
            prefix: String(body.prefix || id.toLowerCase()).trim(),
            api: api,
            base_url: base,
            api_key: String(body.api_key || '').trim(),
            enabled: body.enabled !== false,
            models: models,
            timeout_sec: body.timeout_sec || 60,
            max_attempts: body.max_attempts || 1,
            weight: body.weight || 1,
        };
        state.llm.providers.push(p);
        if (!state.llm.active_provider) state.llm.active_provider = id;
        state.llm.stub = !state.llm.providers.some(function (x) { return !!x.api_key; });
        return toPublic(p);
    }

    function updateProvider(id, body) {
        const idx = state.llm.providers.findIndex(function (x) {
            return x.id === String(id || '').toUpperCase();
        });
        if (idx < 0) throw ApiError(404, 'not_found', 'provider not found');
        const p = Object.assign({}, state.llm.providers[idx]);
        if (body.name) p.name = String(body.name).trim();
        if (body.prefix) p.prefix = String(body.prefix).trim();
        if (body.api) {
            const api = String(body.api).toLowerCase();
            if (api !== 'chat' && api !== 'responses' && api !== 'messages') {
                throw ApiError(422, 'validation', 'api must be chat|responses|messages');
            }
            p.api = api;
        }
        if (body.base_url) p.base_url = String(body.base_url).replace(/\/+$/, '');
        if (body.api_key != null && String(body.api_key).trim() !== '') {
            p.api_key = String(body.api_key).trim();
        }
        if (typeof body.enabled === 'boolean') p.enabled = body.enabled;
        if (Array.isArray(body.models)) p.models = body.models;
        else if (body.model_id) {
            if (!p.models.length) p.models = [{ id: String(body.model_id), label: '' }];
            else p.models[0] = Object.assign({}, p.models[0], { id: String(body.model_id) });
        }
        state.llm.providers[idx] = p;
        state.llm.stub = !state.llm.providers.some(function (x) { return !!x.api_key; });
        return toPublic(p);
    }

    function deleteProvider(id) {
        const up = String(id || '').toUpperCase();
        const idx = state.llm.providers.findIndex(function (x) { return x.id === up; });
        if (idx < 0) throw ApiError(404, 'not_found', 'provider not found');
        state.llm.providers.splice(idx, 1);
        if (state.llm.active_provider === up) {
            state.llm.active_provider = state.llm.providers[0] ? state.llm.providers[0].id : '';
        }
    }

    function addModel(id, model) {
        const p = state.llm.providers.find(function (x) {
            return x.id === String(id || '').toUpperCase();
        });
        if (!p) throw ApiError(404, 'not_found', 'provider not found');
        const mid = String(model && model.id || '').trim();
        if (!mid) throw ApiError(422, 'validation', 'model id required');
        if (p.models.some(function (m) { return m.id === mid; })) {
            throw ApiError(422, 'validation', 'model already listed');
        }
        p.models.push({ id: mid, label: String(model.label || '').trim() });
        return toPublic(p);
    }

    function removeModel(id, modelId) {
        const p = state.llm.providers.find(function (x) {
            return x.id === String(id || '').toUpperCase();
        });
        if (!p) throw ApiError(404, 'not_found', 'provider not found');
        const before = p.models.length;
        p.models = p.models.filter(function (m) { return m.id !== modelId; });
        if (p.models.length === before) throw ApiError(404, 'not_found', 'model not found');
        return toPublic(p);
    }

    function importModels() {
        return {
            imported: 0,
            message: 'Import from provider /models is not implemented yet — add model ids manually.',
        };
    }

    return {
        snapshot,
        listUsers,
        createUser,
        updateUser,
        deleteUser,
        listProviders,
        listProviderCatalog,
        getProvider,
        createProvider,
        updateProvider,
        deleteProvider,
        addModel,
        removeModel,
        importModels,
    };
}

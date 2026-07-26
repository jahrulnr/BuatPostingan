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

function cloneServers(servers) {
    return (servers || []).map(function (s) {
        return {
            id: s.id,
            transport: s.transport || 'stdio',
            command: s.command || '',
            args: Array.isArray(s.args) ? s.args.slice() : [],
            env: Object.assign({}, s.env || {}),
            url: s.url || '',
            enabled: !!s.enabled,
            trusted: !!s.trusted,
            allow_tools: Array.isArray(s.allow_tools) ? s.allow_tools.slice() : [],
            deny_tools: Array.isArray(s.deny_tools) ? s.deny_tools.slice() : [],
            allow_mutations: !!s.allow_mutations,
        };
    });
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
    const state = {
        users: [{ id: 'usr_owner', name: 'Owner', role: 'owner' }],
        limits: {
            max_tool_rounds: 8,
            speak_floor_ttl_sec: 600,
            lock_ttl_sec: 300,
            turn_job_timeout_sec: 120,
        },
        llm: {
            strategy: 'failover',
            active_provider: '',
            stub: true,
            stream: true,
            vision: 'auto',
            effort: 'auto',
            total_attempt_budget: 4,
            retry_base_delay_ms: 250,
            retry_max_delay_ms: 5000,
            retry_jitter: 0.2,
            providers: [],
        },
        context: {
            compaction_enabled: true,
            max_input_tokens: 12000,
            reserve_tokens: 3000,
            recent_turns: 4,
            summary_max_chars: 12000,
        },
        docs: {
            top_k: 5,
            min_score: 0.5,
            fuzzy_enabled: true,
            app_id: 'buatpostingan',
        },
        web_search: {
            github_token: '',
        },
        mcp: {
            enabled: true,
            connect_timeout_sec: 15,
            call_timeout_sec: 30,
            servers: [{
                id: 'echo',
                transport: 'stdio',
                command: './bin/mcp-echo',
                args: [],
                env: {},
                url: '',
                enabled: true,
                trusted: true,
                allow_tools: ['echo'],
                deny_tools: [],
                allow_mutations: false,
            }],
        },
    };

    function ApiError(status, code, message) {
        const err = new Error(code || message || 'error');
        err.status = status;
        err.code = code;
        err.body = { code: code, message: message, error: code };
        return err;
    }

    function requirePositive(n, label) {
        if (!(n >= 1)) throw ApiError(422, 'validation', label + ' must be >= 1');
    }

    function snapshot() {
        const gh = maskKey(state.web_search.github_token);
        return {
            source: 'mock',
            config_path: 'storage/config.json',
            users: state.users.slice(),
            llm: {
                strategy: state.llm.strategy,
                active_provider: state.llm.active_provider,
                stub: state.llm.stub,
                stream: !!state.llm.stream,
                vision: state.llm.vision,
                effort: state.llm.effort,
                total_attempt_budget: state.llm.total_attempt_budget,
                retry_base_delay_ms: state.llm.retry_base_delay_ms,
                retry_max_delay_ms: state.llm.retry_max_delay_ms,
                retry_jitter: state.llm.retry_jitter,
                providers: state.llm.providers.map(toPublic),
            },
            limits: Object.assign({}, state.limits),
            context: Object.assign({}, state.context),
            docs: Object.assign({}, state.docs),
            web_search: {
                github_token_set: gh.set,
                github_token_masked: gh.masked,
            },
            mcp: {
                enabled: !!state.mcp.enabled,
                connect_timeout_sec: state.mcp.connect_timeout_sec,
                call_timeout_sec: state.mcp.call_timeout_sec,
                servers: cloneServers(state.mcp.servers),
            },
        };
    }

    function patchConfig(body) {
        const patch = body || {};
        if (patch.limits) {
            const l = patch.limits;
            if (l.max_tool_rounds != null) {
                requirePositive(Number(l.max_tool_rounds), 'limits.max_tool_rounds');
                state.limits.max_tool_rounds = Number(l.max_tool_rounds);
            }
            if (l.speak_floor_ttl_sec != null) {
                requirePositive(Number(l.speak_floor_ttl_sec), 'limits.speak_floor_ttl_sec');
                state.limits.speak_floor_ttl_sec = Number(l.speak_floor_ttl_sec);
            }
            if (l.lock_ttl_sec != null) {
                requirePositive(Number(l.lock_ttl_sec), 'limits.lock_ttl_sec');
                state.limits.lock_ttl_sec = Number(l.lock_ttl_sec);
            }
            if (l.turn_job_timeout_sec != null) {
                requirePositive(Number(l.turn_job_timeout_sec), 'limits.turn_job_timeout_sec');
                state.limits.turn_job_timeout_sec = Number(l.turn_job_timeout_sec);
            }
        }
        if (patch.llm) {
            const llm = patch.llm;
            if (llm.strategy != null) {
                const s = String(llm.strategy).toLowerCase();
                if (['failover', 'round_robin', 'switch'].indexOf(s) < 0) {
                    throw ApiError(422, 'validation', 'llm.strategy must be failover|round_robin|switch');
                }
                state.llm.strategy = s;
            }
            if (llm.active_provider != null) {
                state.llm.active_provider = String(llm.active_provider || '').trim().toUpperCase();
            }
            if (typeof llm.stream === 'boolean') state.llm.stream = llm.stream;
            if (llm.vision != null) {
                const v = String(llm.vision).toLowerCase();
                state.llm.vision = v === 'on' || v === 'off' ? v : 'auto';
            }
            if (llm.effort != null) {
                const e = String(llm.effort).toLowerCase();
                const ok = ['auto', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'];
                state.llm.effort = ok.indexOf(e) >= 0 ? e : 'auto';
            }
            if (llm.total_attempt_budget != null) {
                requirePositive(Number(llm.total_attempt_budget), 'llm.total_attempt_budget');
                state.llm.total_attempt_budget = Number(llm.total_attempt_budget);
            }
            if (llm.retry_base_delay_ms != null) {
                requirePositive(Number(llm.retry_base_delay_ms), 'llm.retry_base_delay_ms');
                state.llm.retry_base_delay_ms = Number(llm.retry_base_delay_ms);
            }
            if (llm.retry_max_delay_ms != null) {
                requirePositive(Number(llm.retry_max_delay_ms), 'llm.retry_max_delay_ms');
                state.llm.retry_max_delay_ms = Number(llm.retry_max_delay_ms);
            }
            if (llm.retry_jitter != null) {
                const j = Number(llm.retry_jitter);
                if (j < 0 || j > 1) throw ApiError(422, 'validation', 'llm.retry_jitter must be between 0 and 1');
                state.llm.retry_jitter = j;
            }
        }
        if (patch.context) {
            const c = patch.context;
            if (typeof c.compaction_enabled === 'boolean') state.context.compaction_enabled = c.compaction_enabled;
            if (c.max_input_tokens != null) {
                requirePositive(Number(c.max_input_tokens), 'context.max_input_tokens');
                state.context.max_input_tokens = Number(c.max_input_tokens);
            }
            if (c.reserve_tokens != null) {
                if (Number(c.reserve_tokens) < 0) throw ApiError(422, 'validation', 'context.reserve_tokens must be >= 0');
                state.context.reserve_tokens = Number(c.reserve_tokens);
            }
            if (c.recent_turns != null) {
                requirePositive(Number(c.recent_turns), 'context.recent_turns');
                state.context.recent_turns = Number(c.recent_turns);
            }
            if (c.summary_max_chars != null) {
                requirePositive(Number(c.summary_max_chars), 'context.summary_max_chars');
                state.context.summary_max_chars = Number(c.summary_max_chars);
            }
        }
        if (patch.docs) {
            const d = patch.docs;
            if (d.top_k != null) {
                requirePositive(Number(d.top_k), 'docs.top_k');
                state.docs.top_k = Number(d.top_k);
            }
            if (d.min_score != null) {
                const s = Number(d.min_score);
                if (s < 0 || s > 1) throw ApiError(422, 'validation', 'docs.min_score must be between 0 and 1');
                state.docs.min_score = s;
            }
            if (typeof d.fuzzy_enabled === 'boolean') state.docs.fuzzy_enabled = d.fuzzy_enabled;
            if (d.app_id != null) state.docs.app_id = String(d.app_id || '').trim();
        }
        if (patch.web_search && Object.prototype.hasOwnProperty.call(patch.web_search, 'github_token')) {
            state.web_search.github_token = String(patch.web_search.github_token || '').trim();
        }
        if (patch.mcp) {
            const m = patch.mcp;
            if (typeof m.enabled === 'boolean') state.mcp.enabled = m.enabled;
            if (m.connect_timeout_sec != null) {
                requirePositive(Number(m.connect_timeout_sec), 'mcp.connect_timeout_sec');
                state.mcp.connect_timeout_sec = Number(m.connect_timeout_sec);
            }
            if (m.call_timeout_sec != null) {
                requirePositive(Number(m.call_timeout_sec), 'mcp.call_timeout_sec');
                state.mcp.call_timeout_sec = Number(m.call_timeout_sec);
            }
            if (Array.isArray(m.servers)) {
                state.mcp.servers = cloneServers(m.servers);
            }
        }
        return snapshot();
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
        patchConfig,
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

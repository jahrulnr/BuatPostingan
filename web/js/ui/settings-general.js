/**
 * Settings → General — product knobs from storage/config.json.
 * Compact control-panel layout; app-owned choice controls (no native select).
 */
import { api, getSettingsSnapshot, patchSettingsConfig } from '../api/index.js';
import { formatApiError } from '../api/error.js';

const STRATEGY_OPTS = [
    { value: 'failover', label: 'Failover' },
    { value: 'round_robin', label: 'Round robin' },
    { value: 'switch', label: 'Switch' },
];
const VISION_OPTS = [
    { value: 'auto', label: 'Auto' },
    { value: 'on', label: 'On' },
    { value: 'off', label: 'Off' },
];
const EFFORT_OPTS = [
    { value: 'auto', label: 'Auto' },
    { value: 'none', label: 'None' },
    { value: 'minimal', label: 'Min' },
    { value: 'low', label: 'Low' },
    { value: 'medium', label: 'Med' },
    { value: 'high', label: 'High' },
    { value: 'xhigh', label: 'XHigh' },
    { value: 'max', label: 'Max' },
];
const TRANSPORT_OPTS = [
    { value: 'stdio', label: 'stdio' },
    { value: 'sse', label: 'SSE' },
    { value: 'http', label: 'HTTP' },
];

function escapeHtml(s) {
    return String(s == null ? '' : s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function errMsg(err) {
    return formatApiError(err, 'Unknown error');
}

function num(v, fallback) {
    const n = Number(v);
    return Number.isFinite(n) ? n : fallback;
}

function field(opts) {
    const id = opts.name;
    const hint = opts.hint
        ? '<span class="bp-cfg__hint">' + escapeHtml(opts.hint) + '</span>'
        : '';
    return (
        '<label class="bp-cfg__field' +
        (opts.span ? ' bp-cfg__field--' + opts.span : '') +
        '" for="' +
        escapeHtml(id) +
        '">' +
        '<span class="bp-cfg__label">' +
        escapeHtml(opts.label) +
        hint +
        '</span>' +
        '<input class="bp-cfg__input" id="' +
        escapeHtml(id) +
        '" name="' +
        escapeHtml(opts.name) +
        '" type="' +
        escapeHtml(opts.type || 'text') +
        '"' +
        (opts.step != null ? ' step="' + opts.step + '"' : '') +
        (opts.min != null ? ' min="' + opts.min + '"' : '') +
        (opts.max != null ? ' max="' + opts.max + '"' : '') +
        (opts.placeholder ? ' placeholder="' + escapeHtml(opts.placeholder) + '"' : '') +
        (opts.autocomplete === false ? ' autocomplete="off"' : '') +
        ' value="' +
        escapeHtml(opts.value == null ? '' : opts.value) +
        '">' +
        '</label>'
    );
}

function toggle(opts) {
    return (
        '<label class="bp-cfg__toggle' +
        (opts.span ? ' bp-cfg__field--' + opts.span : '') +
        '">' +
        '<input type="checkbox" name="' +
        escapeHtml(opts.name) +
        '"' +
        (opts.checked ? ' checked' : '') +
        '>' +
        '<span class="bp-cfg__toggle-ui" aria-hidden="true"></span>' +
        '<span class="bp-cfg__toggle-copy">' +
        '<strong>' +
        escapeHtml(opts.label) +
        '</strong>' +
        (opts.hint ? '<small>' + escapeHtml(opts.hint) + '</small>' : '') +
        '</span></label>'
    );
}

function segment(opts) {
    const buttons = (opts.options || [])
        .map(function (o) {
            const on = o.value === opts.value;
            return (
                '<button type="button" class="bp-cfg__seg' +
                (on ? ' is-on' : '') +
                '" data-seg="' +
                escapeHtml(opts.name) +
                '" data-value="' +
                escapeHtml(o.value) +
                '" aria-pressed="' +
                (on ? 'true' : 'false') +
                '">' +
                escapeHtml(o.label) +
                '</button>'
            );
        })
        .join('');
    return (
        '<div class="bp-cfg__field' +
        (opts.span ? ' bp-cfg__field--' + opts.span : '') +
        '">' +
        '<span class="bp-cfg__label">' +
        escapeHtml(opts.label) +
        '</span>' +
        '<input type="hidden" name="' +
        escapeHtml(opts.name) +
        '" value="' +
        escapeHtml(opts.value || '') +
        '">' +
        '<div class="bp-cfg__segs" role="group" aria-label="' +
        escapeHtml(opts.label) +
        '">' +
        buttons +
        '</div></div>'
    );
}

function section(id, title, lede, body) {
    return (
        '<section class="bp-cfg__section" id="cfg-' +
        escapeHtml(id) +
        '" data-cfg-section="' +
        escapeHtml(id) +
        '">' +
        '<header class="bp-cfg__section-head">' +
        '<h3>' +
        escapeHtml(title) +
        '</h3>' +
        (lede ? '<p>' + escapeHtml(lede) + '</p>' : '') +
        '</header>' +
        '<div class="bp-cfg__grid">' +
        body +
        '</div></section>'
    );
}

function envToText(env) {
    const rows = [];
    Object.keys(env || {}).sort().forEach(function (k) {
        rows.push(k + '=' + env[k]);
    });
    return rows.join('\n');
}

function textToEnv(raw) {
    const out = {};
    String(raw || '')
        .split(/\n/)
        .forEach(function (line) {
            const s = line.trim();
            if (!s || s.charAt(0) === '#') return;
            const eq = s.indexOf('=');
            if (eq <= 0) return;
            out[s.slice(0, eq).trim()] = s.slice(eq + 1);
        });
    return out;
}

function csvToList(raw) {
    return String(raw || '')
        .split(',')
        .map(function (s) { return s.trim(); })
        .filter(Boolean);
}

function renderServerRow(server, index) {
    const s = server || {};
    return (
        '<article class="bp-mcp-row" data-mcp-index="' +
        index +
        '">' +
        '<div class="bp-mcp-row__bar">' +
        '<label class="bp-switch" title="Enabled"><input type="checkbox" data-mcp-enabled' +
        (s.enabled ? ' checked' : '') +
        '><span class="bp-sr-only">Enabled</span></label>' +
        '<code class="bp-mcp-row__id">' +
        escapeHtml(s.id || 'server') +
        '</code>' +
        '<span class="bp-mcp-row__meta">' +
        escapeHtml(s.transport || 'stdio') +
        (s.command ? ' · ' + escapeHtml(s.command) : '') +
        (s.url ? ' · ' + escapeHtml(s.url) : '') +
        '</span>' +
        '<button type="button" class="bp-settings__btn bp-settings__btn--ghost bp-settings__btn--sm" data-mcp-toggle>Edit</button>' +
        '<button type="button" class="bp-settings__btn bp-settings__btn--danger bp-settings__btn--sm" data-mcp-remove>Remove</button>' +
        '</div>' +
        '<div class="bp-mcp-row__body" hidden>' +
        '<div class="bp-cfg__grid">' +
        field({ name: 'mcp_id_' + index, label: 'ID', value: s.id || '', span: 'half' }) +
        segment({
            name: 'mcp_transport_' + index,
            label: 'Transport',
            value: s.transport || 'stdio',
            options: TRANSPORT_OPTS,
            span: 'half',
        }) +
        field({ name: 'mcp_command_' + index, label: 'Command', value: s.command || '', span: 'full', placeholder: './bin/mcp-echo' }) +
        field({ name: 'mcp_args_' + index, label: 'Args (comma)', value: (s.args || []).join(', '), span: 'full', hint: 'csv' }) +
        field({ name: 'mcp_url_' + index, label: 'URL', value: s.url || '', span: 'full', placeholder: 'http://…' }) +
        '<label class="bp-cfg__field bp-cfg__field--full" for="mcp_env_' +
        index +
        '"><span class="bp-cfg__label">Env <span class="bp-cfg__hint">KEY=value per line</span></span>' +
        '<textarea class="bp-cfg__input bp-cfg__textarea" id="mcp_env_' +
        index +
        '" name="mcp_env_' +
        index +
        '" rows="3">' +
        escapeHtml(envToText(s.env)) +
        '</textarea></label>' +
        field({
            name: 'mcp_allow_' + index,
            label: 'Allow tools',
            value: (s.allow_tools || []).join(', '),
            span: 'half',
            hint: 'csv',
        }) +
        field({
            name: 'mcp_deny_' + index,
            label: 'Deny tools',
            value: (s.deny_tools || []).join(', '),
            span: 'half',
            hint: 'csv',
        }) +
        toggle({ name: 'mcp_trusted_' + index, label: 'Trusted', checked: !!s.trusted, span: 'half' }) +
        toggle({
            name: 'mcp_mutations_' + index,
            label: 'Allow mutations',
            checked: !!s.allow_mutations,
            hint: 'Default denied',
            span: 'half',
        }) +
        '</div></div></article>'
    );
}

export function renderGeneral(snap) {
    const limits = (snap && snap.limits) || {};
    const llm = (snap && snap.llm) || {};
    const ctx = (snap && snap.context) || {};
    const docs = (snap && snap.docs) || {};
    const ws = (snap && snap.web_search) || {};
    const mcp = (snap && snap.mcp) || {};
    const servers = Array.isArray(mcp.servers) ? mcp.servers : [];
    const providerIds = (llm.providers || []).map(function (p) { return p.id; });
    const activeOpts = [{ value: '', label: 'Auto' }].concat(
        providerIds.map(function (id) { return { value: id, label: id }; })
    );

    const meta =
        snap && snap.source
            ? '<p class="bp-settings__meta">Source <strong>' +
              escapeHtml(snap.source) +
              '</strong> · <code>' +
              escapeHtml(snap.config_path || '') +
              '</code>' +
              (llm.stub ? ' · <span class="bp-cfg__stub">stub</span>' : '') +
              '</p>'
            : '';

    return (
        '<form class="bp-cfg" data-general-form novalidate>' +
        '<header class="bp-settings__head">' +
        '<div><h2>General</h2>' +
        meta +
        '</div></header>' +
        section(
            'limits',
            'Limits',
            'Turn-loop ceilings and lock TTLs.',
            field({ name: 'max_tool_rounds', label: 'Max tool rounds', type: 'number', min: 1, value: limits.max_tool_rounds, span: 'quarter' }) +
                field({ name: 'speak_floor_ttl_sec', label: 'Speak floor TTL', type: 'number', min: 1, value: limits.speak_floor_ttl_sec, span: 'quarter', hint: 'sec' }) +
                field({ name: 'lock_ttl_sec', label: 'Lock TTL', type: 'number', min: 1, value: limits.lock_ttl_sec, span: 'quarter', hint: 'sec' }) +
                field({ name: 'turn_job_timeout_sec', label: 'Turn timeout', type: 'number', min: 1, value: limits.turn_job_timeout_sec, span: 'quarter', hint: 'sec' })
        ) +
        section(
            'llm',
            'LLM globals',
            'Routing strategy, streaming, vision, effort, retry budget.',
            segment({ name: 'strategy', label: 'Strategy', value: llm.strategy || 'failover', options: STRATEGY_OPTS, span: 'full' }) +
                segment({ name: 'active_provider', label: 'Active provider', value: llm.active_provider || '', options: activeOpts, span: 'full' }) +
                toggle({ name: 'stream', label: 'Stream responses', checked: llm.stream !== false, hint: 'OpenAI-shaped SSE', span: 'half' }) +
                segment({ name: 'vision', label: 'Vision', value: llm.vision || 'auto', options: VISION_OPTS, span: 'half' }) +
                segment({ name: 'effort', label: 'Effort', value: llm.effort || 'auto', options: EFFORT_OPTS, span: 'full' }) +
                field({ name: 'total_attempt_budget', label: 'Attempt budget', type: 'number', min: 1, value: llm.total_attempt_budget, span: 'quarter' }) +
                field({ name: 'retry_base_delay_ms', label: 'Retry base', type: 'number', min: 1, value: llm.retry_base_delay_ms, span: 'quarter', hint: 'ms' }) +
                field({ name: 'retry_max_delay_ms', label: 'Retry max', type: 'number', min: 1, value: llm.retry_max_delay_ms, span: 'quarter', hint: 'ms' }) +
                field({ name: 'retry_jitter', label: 'Jitter', type: 'number', min: 0, max: 1, step: 0.05, value: llm.retry_jitter, span: 'quarter' })
        ) +
        section(
            'context',
            'Context',
            'Compaction window before the next LLM call.',
            toggle({ name: 'compaction_enabled', label: 'Compaction', checked: ctx.compaction_enabled !== false, span: 'full' }) +
                field({ name: 'max_input_tokens', label: 'Max input', type: 'number', min: 1, value: ctx.max_input_tokens, span: 'quarter', hint: 'tok' }) +
                field({ name: 'reserve_tokens', label: 'Reserve', type: 'number', min: 0, value: ctx.reserve_tokens, span: 'quarter', hint: 'tok' }) +
                field({ name: 'recent_turns', label: 'Recent turns', type: 'number', min: 1, value: ctx.recent_turns, span: 'quarter' }) +
                field({ name: 'summary_max_chars', label: 'Summary max', type: 'number', min: 1, value: ctx.summary_max_chars, span: 'quarter', hint: 'chars' })
        ) +
        section(
            'docs',
            'Docs',
            'Knowledge retrieval defaults for doc tools.',
            field({ name: 'docs_top_k', label: 'Top K', type: 'number', min: 1, value: docs.top_k, span: 'quarter' }) +
                field({ name: 'docs_min_score', label: 'Min score', type: 'number', min: 0, max: 1, step: 0.05, value: docs.min_score, span: 'quarter' }) +
                field({ name: 'docs_app_id', label: 'App ID', value: docs.app_id || '', span: 'half' }) +
                toggle({ name: 'docs_fuzzy', label: 'Fuzzy match', checked: docs.fuzzy_enabled !== false, span: 'full' })
        ) +
        section(
            'search',
            'Web search',
            'Optional GitHub token for rate limits. Leave blank to keep the stored secret.',
            field({
                name: 'github_token',
                label: 'GitHub token',
                type: 'password',
                value: '',
                span: 'full',
                placeholder: ws.github_token_set
                    ? 'Leave blank to keep ' + (ws.github_token_masked || '••••')
                    : 'ghp_…',
                autocomplete: false,
            })
        ) +
        '<section class="bp-cfg__section" id="cfg-mcp" data-cfg-section="mcp">' +
        '<header class="bp-cfg__section-head bp-cfg__section-head--row">' +
        '<div><h3>MCP</h3><p>stdio MVP servers — mutations default-denied.</p></div>' +
        '<button type="button" class="bp-settings__btn bp-settings__btn--ghost bp-settings__btn--sm" data-mcp-add>' +
        '<i class="bi bi-plus-lg" aria-hidden="true"></i> Add server</button></header>' +
        '<div class="bp-cfg__grid">' +
        toggle({ name: 'mcp_enabled', label: 'MCP enabled', checked: mcp.enabled !== false, span: 'full' }) +
        field({ name: 'mcp_connect_timeout_sec', label: 'Connect timeout', type: 'number', min: 1, value: mcp.connect_timeout_sec, span: 'half', hint: 'sec' }) +
        field({ name: 'mcp_call_timeout_sec', label: 'Call timeout', type: 'number', min: 1, value: mcp.call_timeout_sec, span: 'half', hint: 'sec' }) +
        '</div>' +
        '<div class="bp-mcp-list" data-mcp-list>' +
        (servers.length
            ? servers.map(renderServerRow).join('')
            : '<p class="bp-muted bp-cfg__empty">No MCP servers configured.</p>') +
        '</div></section>' +
        '<footer class="bp-cfg__dock">' +
        '<span class="bp-cfg__dirty" data-cfg-dirty hidden>Unsaved changes</span>' +
        '<button type="button" class="bp-settings__btn bp-settings__btn--ghost" data-cfg-reset>Reset</button>' +
        '<button type="submit" class="bp-settings__btn bp-settings__btn--primary" data-cfg-save>Save config</button>' +
        '</footer></form>'
    );
}

function readSeg(form, name) {
    const el = form.elements.namedItem(name);
    return el ? String(el.value || '') : '';
}

function collectServers(form) {
    const rows = form.querySelectorAll('[data-mcp-index]');
    const out = [];
    rows.forEach(function (row) {
        const i = row.getAttribute('data-mcp-index');
        const id = String(form.elements.namedItem('mcp_id_' + i).value || '').trim();
        if (!id) return;
        const enabled = !!row.querySelector('[data-mcp-enabled]').checked;
        out.push({
            id: id,
            transport: readSeg(form, 'mcp_transport_' + i) || 'stdio',
            command: String(form.elements.namedItem('mcp_command_' + i).value || '').trim(),
            args: csvToList(form.elements.namedItem('mcp_args_' + i).value),
            url: String(form.elements.namedItem('mcp_url_' + i).value || '').trim(),
            env: textToEnv(form.elements.namedItem('mcp_env_' + i).value),
            enabled: enabled,
            trusted: !!form.elements.namedItem('mcp_trusted_' + i).checked,
            allow_tools: csvToList(form.elements.namedItem('mcp_allow_' + i).value),
            deny_tools: csvToList(form.elements.namedItem('mcp_deny_' + i).value),
            allow_mutations: !!form.elements.namedItem('mcp_mutations_' + i).checked,
        });
    });
    return out;
}

function collectPatch(form) {
    const patch = {
        limits: {
            max_tool_rounds: num(form.elements.namedItem('max_tool_rounds').value, 8),
            speak_floor_ttl_sec: num(form.elements.namedItem('speak_floor_ttl_sec').value, 600),
            lock_ttl_sec: num(form.elements.namedItem('lock_ttl_sec').value, 300),
            turn_job_timeout_sec: num(form.elements.namedItem('turn_job_timeout_sec').value, 120),
        },
        llm: {
            strategy: readSeg(form, 'strategy') || 'failover',
            active_provider: readSeg(form, 'active_provider'),
            stream: !!form.elements.namedItem('stream').checked,
            vision: readSeg(form, 'vision') || 'auto',
            effort: readSeg(form, 'effort') || 'auto',
            total_attempt_budget: num(form.elements.namedItem('total_attempt_budget').value, 4),
            retry_base_delay_ms: num(form.elements.namedItem('retry_base_delay_ms').value, 250),
            retry_max_delay_ms: num(form.elements.namedItem('retry_max_delay_ms').value, 5000),
            retry_jitter: num(form.elements.namedItem('retry_jitter').value, 0.2),
        },
        context: {
            compaction_enabled: !!form.elements.namedItem('compaction_enabled').checked,
            max_input_tokens: num(form.elements.namedItem('max_input_tokens').value, 12000),
            reserve_tokens: num(form.elements.namedItem('reserve_tokens').value, 3000),
            recent_turns: num(form.elements.namedItem('recent_turns').value, 4),
            summary_max_chars: num(form.elements.namedItem('summary_max_chars').value, 12000),
        },
        docs: {
            top_k: num(form.elements.namedItem('docs_top_k').value, 5),
            min_score: num(form.elements.namedItem('docs_min_score').value, 0.5),
            fuzzy_enabled: !!form.elements.namedItem('docs_fuzzy').checked,
            app_id: String(form.elements.namedItem('docs_app_id').value || '').trim(),
        },
        mcp: {
            enabled: !!form.elements.namedItem('mcp_enabled').checked,
            connect_timeout_sec: num(form.elements.namedItem('mcp_connect_timeout_sec').value, 15),
            call_timeout_sec: num(form.elements.namedItem('mcp_call_timeout_sec').value, 30),
            servers: collectServers(form),
        },
    };
    const token = String(form.elements.namedItem('github_token').value || '').trim();
    if (token) {
        patch.web_search = { github_token: token };
    }
    return patch;
}

export function wireGeneral(panel, snap, helpers) {
    const form = panel.querySelector('[data-general-form]');
    if (!form) return;
    const toast = helpers && helpers.toast ? helpers.toast : function () {};
    const reload = helpers && helpers.reload ? helpers.reload : async function () {};
    const dirtyEl = form.querySelector('[data-cfg-dirty]');
    let dirty = false;
    let mcpSeq = (snap && snap.mcp && snap.mcp.servers ? snap.mcp.servers.length : 0) + 1000;

    function setDirty(on) {
        dirty = !!on;
        if (dirtyEl) dirtyEl.hidden = !dirty;
        form.classList.toggle('is-dirty', dirty);
    }

    form.addEventListener('input', function () { setDirty(true); });
    form.addEventListener('change', function () { setDirty(true); });

    form.querySelectorAll('[data-seg]').forEach(function (btn) {
        btn.addEventListener('click', function () {
            const name = btn.getAttribute('data-seg');
            const value = btn.getAttribute('data-value') || '';
            const hidden = form.elements.namedItem(name);
            if (hidden) hidden.value = value;
            form.querySelectorAll('[data-seg="' + name + '"]').forEach(function (peer) {
                const on = peer === btn;
                peer.classList.toggle('is-on', on);
                peer.setAttribute('aria-pressed', on ? 'true' : 'false');
            });
            setDirty(true);
        });
    });

    form.querySelectorAll('[data-cfg-jump]').forEach(function (a) {
        a.addEventListener('click', function (e) {
            e.preventDefault();
            const id = a.getAttribute('data-cfg-jump');
            const target = form.querySelector('#cfg-' + id);
            if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
            form.querySelectorAll('[data-cfg-jump]').forEach(function (peer) {
                peer.classList.toggle('is-active', peer === a);
            });
        });
    });

    form.querySelectorAll('[data-mcp-toggle]').forEach(function (btn) {
        btn.addEventListener('click', function () {
            const row = btn.closest('.bp-mcp-row');
            const body = row && row.querySelector('.bp-mcp-row__body');
            if (!body) return;
            body.hidden = !body.hidden;
            btn.textContent = body.hidden ? 'Edit' : 'Collapse';
        });
    });

    form.querySelectorAll('[data-mcp-remove]').forEach(function (btn) {
        btn.addEventListener('click', function () {
            const row = btn.closest('.bp-mcp-row');
            if (row) row.remove();
            setDirty(true);
        });
    });

    const addBtn = form.querySelector('[data-mcp-add]');
    if (addBtn) {
        addBtn.addEventListener('click', function () {
            const list = form.querySelector('[data-mcp-list]');
            if (!list) return;
            const empty = list.querySelector('.bp-cfg__empty');
            if (empty) empty.remove();
            const idx = mcpSeq++;
            list.insertAdjacentHTML(
                'beforeend',
                renderServerRow(
                    {
                        id: 'mcp_' + idx,
                        transport: 'stdio',
                        command: '',
                        args: [],
                        env: {},
                        enabled: true,
                        trusted: false,
                        allow_tools: [],
                        deny_tools: [],
                        allow_mutations: false,
                    },
                    idx
                )
            );
            const row = list.querySelector('[data-mcp-index="' + idx + '"]');
            if (row) {
                const body = row.querySelector('.bp-mcp-row__body');
                if (body) body.hidden = false;
                row.querySelectorAll('[data-seg]').forEach(function (btn) {
                    btn.addEventListener('click', function () {
                        const name = btn.getAttribute('data-seg');
                        const value = btn.getAttribute('data-value') || '';
                        const hidden = form.elements.namedItem(name);
                        if (hidden) hidden.value = value;
                        form.querySelectorAll('[data-seg="' + name + '"]').forEach(function (peer) {
                            const on = peer === btn;
                            peer.classList.toggle('is-on', on);
                            peer.setAttribute('aria-pressed', on ? 'true' : 'false');
                        });
                        setDirty(true);
                    });
                });
                const toggleBtn = row.querySelector('[data-mcp-toggle]');
                if (toggleBtn) {
                    toggleBtn.textContent = 'Collapse';
                    toggleBtn.addEventListener('click', function () {
                        const bodyEl = row.querySelector('.bp-mcp-row__body');
                        if (!bodyEl) return;
                        bodyEl.hidden = !bodyEl.hidden;
                        toggleBtn.textContent = bodyEl.hidden ? 'Edit' : 'Collapse';
                    });
                }
                const removeBtn = row.querySelector('[data-mcp-remove]');
                if (removeBtn) {
                    removeBtn.addEventListener('click', function () {
                        row.remove();
                        setDirty(true);
                    });
                }
            }
            setDirty(true);
        });
    }

    const resetBtn = form.querySelector('[data-cfg-reset]');
    if (resetBtn) {
        resetBtn.addEventListener('click', function () {
            reload();
        });
    }

    form.addEventListener('submit', async function (e) {
        e.preventDefault();
        const saveBtn = form.querySelector('[data-cfg-save]');
        if (saveBtn) saveBtn.disabled = true;
        try {
            await patchSettingsConfig(api, collectPatch(form));
            toast('Config saved');
            setDirty(false);
            await reload();
        } catch (err) {
            toast(errMsg(err), { error: true });
        } finally {
            if (saveBtn) saveBtn.disabled = false;
        }
    });
}

export async function loadGeneralPanel(panel, helpers) {
    panel.innerHTML = '<div class="bp-settings__loading">Loading…</div>';
    try {
        const snap = await getSettingsSnapshot(api);
        panel.innerHTML = renderGeneral(snap);
        wireGeneral(panel, snap, helpers);
    } catch (err) {
        panel.innerHTML =
            '<div class="bp-settings__error" role="alert">' + escapeHtml(errMsg(err)) + '</div>';
    }
}

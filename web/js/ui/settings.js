/**
 * Settings full-page view — hash routes #/settings/...
 * Dual-driver via js/api only (no direct fetch).
 */
import {
    api,
    getSettingsSnapshot,
    listSettingsUsers,
    createSettingsUser,
    updateSettingsUser,
    deleteSettingsUser,
    listLLMProviders,
    listLLMProviderCatalog,
    getLLMProvider,
    createLLMProvider,
    updateLLMProvider,
    deleteLLMProvider,
    addLLMModel,
    removeLLMModel,
    importLLMModels,
} from '../api/index.js';
import { bootAppSelects, confirmAppDialog, openAppDialog, setAppSelectValue } from './dialogs.js';
import { chatModels } from './model-capabilities.js';
import { loadGeneralPanel } from './settings-general.js';

const PREF_KEYS = [
    'bp.theme',
    'bp.modelId',
    'bp.effort',
    'bp.mockMode',
    'bp.previewWidth',
];

function escapeHtml(s) {
    return String(s == null ? '' : s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function parseHash() {
    const raw = String(window.location.hash || '').replace(/^#/, '');
    const parts = raw.split('/').filter(Boolean);
    if (parts[0] !== 'settings') {
        return { view: 'chat', section: '', providerId: '' };
    }
    const section = parts[1] || 'models';
    const providerId = section === 'models' && parts[2] ? parts[2] : '';
    return { view: 'settings', section: section, providerId: providerId };
}

function navigate(hash) {
    window.location.hash = hash;
}

function errMsg(err) {
    if (!err) return 'Unknown error';
    if (err.body && (err.body.message || err.body.error)) {
        return err.body.message || err.body.error;
    }
    return err.message || String(err);
}

function userFields(user) {
    const current = user || {};
    return [
        { name: 'name', label: 'Name', required: true, value: current.name || '', placeholder: 'Display name' },
        {
            name: 'role',
            label: 'Role',
            type: 'choice',
            required: true,
            value: current.role || 'member',
            options: [
                { value: 'owner', label: 'Owner' },
                { value: 'admin', label: 'Admin' },
                { value: 'member', label: 'Member' },
            ],
        },
    ];
}

export function bootSettings() {
    const shell = document.getElementById('bpShell');
    const workspace = document.getElementById('bpWorkspace');
    const settingsRoot = document.getElementById('bpSettings');
    const chromeRoom = document.getElementById('chromeRoomTitle');
    const profileGear = document.getElementById('btnOpenSettings');
    const toastEl = document.getElementById('settingsToast');

    if (!shell || !settingsRoot) return;
    bootAppSelects();

    let route = parseHash();

    function toast(msg) {
        if (!toastEl) return;
        toastEl.textContent = msg;
        toastEl.hidden = false;
        clearTimeout(toastEl._t);
        toastEl._t = setTimeout(function () {
            toastEl.hidden = true;
        }, 2800);
    }

    function applyView() {
        route = parseHash();
        const onSettings = route.view === 'settings';
        shell.classList.toggle('is-settings', onSettings);
        if (workspace) workspace.hidden = onSettings;
        settingsRoot.hidden = !onSettings;
        if (chromeRoom && onSettings) {
            chromeRoom.textContent = 'Settings';
        }
        if (onSettings) {
            renderSettings();
        }
    }

    async function renderSettings() {
        const nav = settingsRoot.querySelector('[data-settings-nav]');
        const panel = settingsRoot.querySelector('[data-settings-panel]');
        if (!nav || !panel) return;

        const section = route.section === 'general' || route.section === 'users' || route.section === 'models'
            ? route.section
            : 'models';

        nav.querySelectorAll('[data-settings-link]').forEach(function (a) {
            const on = a.getAttribute('data-settings-link') === section;
            a.classList.toggle('is-active', on);
            a.setAttribute('aria-current', on ? 'page' : 'false');
        });

        panel.innerHTML = '<div class="bp-settings__loading">Loading…</div>';
        try {
            if (section === 'general') {
                await loadGeneralPanel(panel, {
                    toast: toast,
                    reload: renderSettings,
                });
            } else if (section === 'users') {
                const data = await listSettingsUsers(api);
                panel.innerHTML = renderUsers(data.users || []);
                wireUsers(panel, data.users || []);
            } else if (route.providerId) {
                const p = await getLLMProvider(api, route.providerId);
                panel.innerHTML = renderProviderDetail(p);
                wireProviderDetail(panel, p);
            } else {
                const results = await Promise.all([
                    listLLMProviders(api),
                    listLLMProviderCatalog(api),
                ]);
                const data = results[0];
                const catalog = results[1];
                let snap = null;
                try {
                    snap = await getSettingsSnapshot(api);
                } catch (e) { /* optional */ }
                panel.innerHTML = renderProviders(data.providers || [], catalog.providers || [], snap);
                wireProviders(panel, catalog.providers || []);
            }
        } catch (err) {
            panel.innerHTML =
                '<div class="bp-settings__error" role="alert">' +
                escapeHtml(errMsg(err)) +
                '</div>';
        }
    }

    function renderUsers(users) {
        const rows = users
            .map(function (u) {
                return (
                    '<tr data-user-id="' +
                    escapeHtml(u.id) +
                    '">' +
                    '<td><code>' +
                    escapeHtml(u.id) +
                    '</code></td>' +
                    '<td>' +
                    escapeHtml(u.name) +
                    '</td>' +
                    '<td>' +
                    escapeHtml(u.role) +
                    '</td>' +
                    '<td class="bp-settings__row-actions">' +
                    '<button type="button" class="bp-settings__btn bp-settings__btn--ghost" data-edit-user>Edit</button>' +
                    '<button type="button" class="bp-settings__btn bp-settings__btn--danger" data-del-user>Delete</button>' +
                    '</td></tr>'
                );
            })
            .join('');
        return (
            '<header class="bp-settings__head">' +
            '<button type="button" class="bp-settings__btn bp-settings__btn--primary" data-add-user>' +
            '<i class="bi bi-plus-lg" aria-hidden="true"></i> Add user</button>' +
            '</header>' +
            '<div class="bp-settings__card bp-settings__table-wrap">' +
            '<table class="bp-settings__table"><thead><tr><th>ID</th><th>Name</th><th>Role</th><th></th></tr></thead>' +
            '<tbody>' +
            (rows || '<tr><td colspan="4" class="bp-muted">No users</td></tr>') +
            '</tbody></table></div>'
        );
    }

    function wireUsers(panel, users) {
        const addBtn = panel.querySelector('[data-add-user]');
        if (addBtn) {
            addBtn.addEventListener('click', async function () {
                const values = await openAppDialog({
                    eyebrow: 'Settings',
                    title: 'Add user',
                    message: 'Add a local workspace user and choose their role.',
                    icon: 'bi-person-plus',
                    confirmLabel: 'Add user',
                    fields: userFields({ role: 'member' }),
                });
                if (!values) return;
                try {
                    await createSettingsUser(api, values);
                    toast('User created');
                    renderSettings();
                } catch (err) {
                    toast(errMsg(err));
                }
            });
        }
        panel.querySelectorAll('[data-edit-user]').forEach(function (btn) {
            btn.addEventListener('click', async function () {
                const tr = btn.closest('tr');
                const id = tr && tr.getAttribute('data-user-id');
                if (!id) return;
                const user = (users || []).find(function (item) { return item.id === id; });
                const values = await openAppDialog({
                    eyebrow: 'Settings',
                    title: 'Edit user',
                    message: 'Update this local workspace user.',
                    icon: 'bi-person-gear',
                    confirmLabel: 'Save changes',
                    fields: userFields(user || {}),
                });
                if (!values) return;
                try {
                    await updateSettingsUser(api, id, {
                        name: values.name || undefined,
                        role: values.role || undefined,
                    });
                    toast('User updated');
                    renderSettings();
                } catch (err) {
                    toast(errMsg(err));
                }
            });
        });
        panel.querySelectorAll('[data-del-user]').forEach(function (btn) {
            btn.addEventListener('click', async function () {
                const tr = btn.closest('tr');
                const id = tr && tr.getAttribute('data-user-id');
                if (!id) return;
                const approved = await confirmAppDialog({
                    eyebrow: 'Settings',
                    title: 'Delete user?',
                    message: 'User “' + id + '” will be removed from this workspace.',
                    icon: 'bi-person-x',
                    confirmLabel: 'Delete user',
                    tone: 'danger',
                });
                if (!approved) return;
                try {
                    await deleteSettingsUser(api, id);
                    toast('User deleted');
                    renderSettings();
                } catch (err) {
                    toast(errMsg(err));
                }
            });
        });
    }

    function renderProviders(providers, catalog, snap) {
        const meta =
            snap && snap.source
                ? '<p class="bp-settings__meta">Source: <strong>' +
                  escapeHtml(snap.source) +
                  '</strong> · <code>' +
                  escapeHtml(snap.config_path || '') +
                  '</code></p>'
                : '';
        const byType = {};
        providers.forEach(function (p) {
            const type = p.type || 'openai-compatible';
            if (!byType[type]) byType[type] = [];
            byType[type].push(p);
        });
        const seen = {};
        const cards = catalog
            .map(function (definition) {
                const matches = byType[definition.type] || [];
                const p = matches[0] || null;
                if (p) seen[p.id] = true;
                const modelCount = p ? chatModels(p.models).length : 0;
                const ready = p && p.enabled && (p.api_key_set || p.api_key_optional);
                const status = !p
                    ? '<span class="bp-provider-status">Not configured</span>'
                    : !p.enabled
                        ? '<span class="bp-provider-status is-off">Disabled</span>'
                        : ready
                            ? '<span class="bp-provider-status is-ready">Configured</span>'
                            : '<span class="bp-provider-status is-warn">Needs API key</span>';
                return (
                    '<article class="bp-provider-card" data-provider-id="' +
                    escapeHtml(p ? p.id : '') +
                    '" data-provider-type="' +
                    escapeHtml(definition.type) +
                    '" style="--provider-accent:' +
                    escapeHtml(definition.accent || '#64748b') +
                    '">' +
                    '<div class="bp-provider-card__top">' +
                    '<div class="bp-provider-card__identity">' +
                    '<span class="bp-provider-card__icon">' + escapeHtml(definition.icon || 'AI') + '</span>' +
                    '<div><h3>' + escapeHtml(definition.name) + '</h3>' +
                    '<p class="bp-provider-card__kind">' + escapeHtml(definition.auth_type) + ' · ' + escapeHtml(definition.api) + '</p></div></div>' +
                    (p ? '<label class="bp-switch" title="Enabled"><input type="checkbox" data-toggle-enabled ' +
                    (p.enabled ? 'checked' : '') + '><span class="bp-sr-only">Enabled</span></label>' : '') +
                    '</div>' +
                    '<p class="bp-provider-card__description">' + escapeHtml(definition.description) + '</p>' +
                    '<div class="bp-provider-card__connection">' + status +
                    (p ? '<span class="bp-provider-card__instance"><code>' + escapeHtml(p.id) + '</code> · ' + modelCount + ' model</span>' : '') +
                    '</div>' +
                    '<div class="bp-provider-card__actions">' +
                    (p
                        ? '<button type="button" class="bp-settings__btn bp-settings__btn--ghost" data-open-provider>Details</button>' +
                          '<button type="button" class="bp-settings__btn bp-settings__btn--danger" data-del-provider>Delete</button>'
                        : '<button type="button" class="bp-settings__btn bp-settings__btn--ghost" data-configure-provider>Configure</button>') +
                    '</div></article>'
                );
            })
            .join('') +
            providers.filter(function (p) { return !seen[p.id]; }).map(function (p) {
                return '<article class="bp-provider-card bp-provider-card--legacy" data-provider-id="' + escapeHtml(p.id) +
                    '" data-provider-type="' + escapeHtml(p.type || 'openai-compatible') + '">' +
                    '<div class="bp-provider-card__top"><div class="bp-provider-card__identity"><span class="bp-provider-card__icon">OC</span>' +
                    '<div><h3>' + escapeHtml(p.name || p.id) + '</h3><p class="bp-provider-card__kind">custom connection</p></div></div></div>' +
                    '<p class="bp-provider-card__description"><code>' + escapeHtml(p.base_url) + '</code></p>' +
                    '<div class="bp-provider-card__actions"><button type="button" class="bp-settings__btn bp-settings__btn--ghost" data-open-provider>Details</button>' +
                    '<button type="button" class="bp-settings__btn bp-settings__btn--danger" data-del-provider>Delete</button></div></article>';
            }).join('');
        return (
            '<header class="bp-settings__head">' +
            '<div><h2>Providers</h2><p class="bp-settings__lede">Manage direct APIs and local AI gateways. Credentials stay masked.</p>' +
            meta +
            '</div>' +
            '<button type="button" class="bp-settings__btn bp-settings__btn--primary" data-add-provider>' +
            '<i class="bi bi-plus-lg" aria-hidden="true"></i> Custom provider</button>' +
            '</header>' +
            '<div class="bp-provider-grid">' +
            (cards || '<p class="bp-muted">No providers yet — add one or rely on <code>BP_LLM_*</code> env until first save.</p>') +
            '</div>'
        );
    }

    function wireProviders(panel, catalog) {
        const add = panel.querySelector('[data-add-provider]');
        if (add) {
            add.addEventListener('click', function () {
                openProviderModal(null, {
                    type: 'openai-compatible', name: 'OpenAI Compatible',
                    prefix: 'compatible', api: 'chat', base_url: '',
                });
            });
        }
        panel.querySelectorAll('[data-configure-provider]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                const card = btn.closest('[data-provider-type]');
                const type = card && card.getAttribute('data-provider-type');
                const definition = catalog.find(function (d) { return d.type === type; });
                openProviderModal(null, definition || { type: type });
            });
        });
        panel.querySelectorAll('[data-open-provider]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                const id = btn.closest('[data-provider-id]').getAttribute('data-provider-id');
                navigate('#/settings/models/' + encodeURIComponent(id));
            });
        });
        panel.querySelectorAll('[data-del-provider]').forEach(function (btn) {
            btn.addEventListener('click', async function () {
                const id = btn.closest('[data-provider-id]').getAttribute('data-provider-id');
                const approved = await confirmAppDialog({
                    eyebrow: 'Providers',
                    title: 'Delete provider?',
                    message: 'Provider “' + id + '” and its configured models will be removed.',
                    icon: 'bi-plug',
                    confirmLabel: 'Delete provider',
                    tone: 'danger',
                });
                if (!approved) return;
                try {
                    await deleteLLMProvider(api, id);
                    toast('Provider deleted');
                    renderSettings();
                } catch (err) {
                    toast(errMsg(err));
                }
            });
        });
        panel.querySelectorAll('[data-toggle-enabled]').forEach(function (input) {
            input.addEventListener('change', async function () {
                const id = input.closest('[data-provider-id]').getAttribute('data-provider-id');
                try {
                    await updateLLMProvider(api, id, { enabled: !!input.checked });
                    toast(input.checked ? 'Enabled' : 'Disabled');
                } catch (err) {
                    input.checked = !input.checked;
                    toast(errMsg(err));
                }
            });
        });
    }

    function fmtTokens(n) {
        if (!n || n <= 0) return '';
        if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, '') + 'M';
        if (n >= 1000) return Math.round(n / 1000) + 'K';
        return String(n);
    }

    function renderModelBadges(m) {
        var badges = '';
        if (m.context_window > 0) {
            badges += '<span class="bp-meta-badge bp-meta-badge--ctx" title="Context window">' + fmtTokens(m.context_window) + ' ctx</span>';
        }
        if (m.max_output > 0) {
            badges += '<span class="bp-meta-badge bp-meta-badge--out" title="Max output tokens">' + fmtTokens(m.max_output) + ' out</span>';
        }
        (m.input_modes || []).forEach(function (mode) {
            badges += '<span class="bp-meta-badge bp-meta-badge--in">' + escapeHtml(mode) + '</span>';
        });
        (m.effort_levels || []).forEach(function (e) {
            badges += '<span class="bp-meta-badge bp-meta-badge--eff">' + escapeHtml(e) + '</span>';
        });
        if (m.supports_tools) {
            badges += '<span class="bp-meta-badge bp-meta-badge--tools">tools</span>';
        }
        return badges ? '<div class="bp-model-meta">' + badges + '</div>' : '';
    }

    function renderProviderDetail(p) {
        const models = chatModels(p.models)
            .map(function (m) {
                return (
                    '<li data-model-id="' +
                    escapeHtml(m.id) +
                    '">' +
                    '<div class="bp-model-row">' +
                    '<code>' +
                    escapeHtml(m.id) +
                    '</code>' +
                    (m.label && m.label !== m.id ? ' <span class="bp-muted">' + escapeHtml(m.label) + '</span>' : '') +
                    ' <button type="button" class="bp-settings__btn bp-settings__btn--ghost bp-settings__btn--sm" data-rm-model>Remove</button>' +
                    '</div>' +
                    renderModelBadges(m) +
                    (m.description ? '<p class="bp-model-desc">' + escapeHtml(m.description) + '</p>' : '') +
                    '</li>'
                );
            })
            .join('');
        return (
            '<header class="bp-settings__head">' +
            '<div><a href="#/settings/models" class="bp-settings__back"><i class="bi bi-arrow-left"></i> Models</a>' +
            '<h2>' +
            escapeHtml(p.name || p.id) +
            '</h2>' +
            '<p class="bp-settings__lede"><code>' +
            escapeHtml(p.id) +
            '</code> · ' +
            escapeHtml(p.api) +
            '</p></div></header>' +
            '<div class="bp-settings__card">' +
            '<dl class="bp-settings__dl">' +
            '<div><dt>Base URL</dt><dd><code>' +
            escapeHtml(p.base_url) +
            '</code></dd></div>' +
            '<div><dt>API key</dt><dd>' +
            (p.api_key_set ? escapeHtml(p.api_key_masked) : '—') +
            '</dd></div>' +
            '<div><dt>Enabled</dt><dd>' +
            (p.enabled ? 'yes' : 'no') +
            '</dd></div>' +
            '</dl>' +
            '<div class="bp-settings__row-actions" style="margin-top:1rem">' +
            '<button type="button" class="bp-settings__btn bp-settings__btn--ghost" data-edit-provider>Edit</button>' +
            '<button type="button" class="bp-settings__btn bp-settings__btn--ghost" data-import-models>Import models</button>' +
            '</div></div>' +
            '<div class="bp-settings__card">' +
            '<div class="bp-settings__head bp-settings__head--sm">' +
            '<h3>Available models</h3>' +
            '<button type="button" class="bp-settings__btn bp-settings__btn--primary bp-settings__btn--sm" data-add-model>Add</button>' +
            '</div>' +
            '<ul class="bp-model-list">' +
            (models || '<li class="bp-muted">No models</li>') +
            '</ul></div>'
        );
    }

    function wireProviderDetail(panel, p) {
        const edit = panel.querySelector('[data-edit-provider]');
        if (edit) {
            edit.addEventListener('click', function () {
                openProviderModal(p);
            });
        }
        const imp = panel.querySelector('[data-import-models]');
        if (imp) {
            imp.addEventListener('click', async function () {
                try {
                    const out = await importLLMModels(api, p.id);
                    toast(out.message || 'Import complete');
                    document.dispatchEvent(new CustomEvent('bp:models-changed'));
                    renderSettings();
                } catch (err) {
                    toast(errMsg(err));
                }
            });
        }
        const add = panel.querySelector('[data-add-model]');
        if (add) {
            add.addEventListener('click', async function () {
                const values = await openAppDialog({
                    eyebrow: 'Models',
                    title: 'Add model',
                    message: 'Add a model identifier for this provider.',
                    icon: 'bi-cpu',
                    confirmLabel: 'Add model',
                    fields: [
                        { name: 'id', label: 'Model id', required: true, placeholder: 'provider/model-name' },
                        { name: 'label', label: 'Display label (optional)', placeholder: 'Friendly model name' },
                    ],
                });
                if (!values) return;
                try {
                    await addLLMModel(api, p.id, values);
                    toast('Model added');
                    document.dispatchEvent(new CustomEvent('bp:models-changed'));
                    renderSettings();
                } catch (err) {
                    toast(errMsg(err));
                }
            });
        }
        panel.querySelectorAll('[data-rm-model]').forEach(function (btn) {
            btn.addEventListener('click', async function () {
                const mid = btn.closest('[data-model-id]').getAttribute('data-model-id');
                try {
                    await removeLLMModel(api, p.id, mid);
                    toast('Model removed');
                    document.dispatchEvent(new CustomEvent('bp:models-changed'));
                    renderSettings();
                } catch (err) {
                    toast(errMsg(err));
                }
            });
        });
    }

    function openProviderModal(existing, definition) {
        const dlg = document.getElementById('providerDialog');
        const form = document.getElementById('providerForm');
        const title = document.getElementById('providerDialogTitle');
        if (!dlg || !form) return;
        if (title) title.textContent = existing ? 'Edit Provider' : 'Add Provider';
        form.reset();
        const preset = definition || {};
        form.elements.namedItem('type').value = existing ? existing.type || 'openai-compatible' : preset.type || 'openai-compatible';
        form.elements.namedItem('id').value = existing ? existing.id : String(preset.type || '').toUpperCase().replace(/-/g, '_');
        form.elements.namedItem('id').readOnly = !!existing;
        form.elements.namedItem('name').value = existing ? existing.name || '' : preset.name || '';
        form.elements.namedItem('prefix').value = existing ? existing.prefix || '' : preset.prefix || '';
        setAppSelectValue(form.querySelector('[data-ui-select]'), existing ? existing.api || 'responses' : preset.api || 'chat');
        form.elements.namedItem('base_url').value = existing ? existing.base_url || '' : preset.base_url || '';
        form.elements.namedItem('api_key').value = '';
        form.elements.namedItem('api_key').placeholder = existing && existing.api_key_set
            ? 'Leave blank to keep ' + (existing.api_key_masked || 'existing key')
            : (existing && existing.api_key_optional) || preset.api_key_optional ? 'Optional for this local gateway' : 'sk-…';
        form.elements.namedItem('model_id').value =
            existing && existing.models && existing.models[0] ? existing.models[0].id : '';
        form.elements.namedItem('enabled').checked = existing ? !!existing.enabled : true;
        dlg.hidden = false;
        document.body.classList.add('has-wc-dialog');
        requestAnimationFrame(function () {
            dlg.classList.add('is-open');
        });
        form._editing = existing ? existing.id : '';
        const first = form.querySelector('input:not([readonly])');
        if (first) first.focus();
    }

    function closeProviderModal() {
        const dlg = document.getElementById('providerDialog');
        if (!dlg || dlg.hidden) return;
        dlg.classList.remove('is-open');
        document.body.classList.remove('has-wc-dialog');
        setTimeout(function () {
            dlg.hidden = true;
        }, 180);
    }

    const providerForm = document.getElementById('providerForm');
    if (providerForm) {
        providerForm.addEventListener('submit', async function (e) {
            e.preventDefault();
            const fd = new FormData(providerForm);
            const body = {
                type: String(fd.get('type') || 'openai-compatible').trim(),
                id: String(fd.get('id') || '').trim(),
                name: String(fd.get('name') || '').trim(),
                prefix: String(fd.get('prefix') || '').trim(),
                api: String(fd.get('api') || 'responses').trim(),
                base_url: String(fd.get('base_url') || '').trim(),
                model_id: String(fd.get('model_id') || '').trim(),
                enabled: !!providerForm.elements.namedItem('enabled').checked,
            };
            const key = String(fd.get('api_key') || '').trim();
            if (key) body.api_key = key;
            const editing = providerForm._editing;
            try {
                if (editing) {
                    if (!key) {
                        /* keep existing */
                    } else {
                        body.api_key = key;
                    }
                    await updateLLMProvider(api, editing, body);
                    toast('Provider updated');
                } else {
                    body.api_key = key;
                    await createLLMProvider(api, body);
                    toast('Provider created');
                }
                closeProviderModal();
                navigate('#/settings/models');
                renderSettings();
            } catch (err) {
                toast(errMsg(err));
            }
        });
    }
    document.querySelectorAll('[data-provider-close]').forEach(function (el) {
        el.addEventListener('click', closeProviderModal);
    });

    if (profileGear) {
        profileGear.addEventListener('click', function () {
            navigate('#/settings/models');
        });
    }

    const logoutBtn = document.getElementById('btnSettingsLogout');
    if (logoutBtn) {
        logoutBtn.addEventListener('click', function () {
            PREF_KEYS.forEach(function (k) {
                try {
                    localStorage.removeItem(k);
                } catch (e) { /* ignore */ }
            });
            toast('Local preferences cleared');
            navigate('#/');
        });
    }

    const backChat = document.getElementById('btnSettingsBack');
    if (backChat) {
        backChat.addEventListener('click', function () {
            navigate('#/');
        });
    }

    window.addEventListener('hashchange', applyView);
    applyView();
}

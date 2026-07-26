/**
 * Composer model + effort picker (Cursor-like pill + dropdown).
 * Persistence: localStorage bp.modelId / bp.effort
 */

const LS_MODEL = 'bp.modelId';
const LS_EFFORT = 'bp.effort';

/**
 * @param {{
 *   root?: ParentNode,
 *   listModels: Function,
 *   api: Object,
 *   showToast?: (msg: string) => void,
 * }} opts
 */
export function bootModelPicker(opts) {
    const root = opts.root || document;
    const byId = function (id) {
        if (root.nodeType === 9) return root.getElementById(id);
        return root.querySelector('#' + id);
    };

    const pill = byId('modelPill');
    const menu = byId('modelMenu');
    const labelEl = byId('modelPillLabel');
    if (!pill || !menu || !labelEl) {
        return {
            getSelection: function () { return { model: '', effort: '' }; },
            refresh: function () { return Promise.resolve(); },
        };
    }

    const showToast = typeof opts.showToast === 'function' ? opts.showToast : function () {};

    let catalog = null;
    /** @type {string} */
    let modelId = readLS(LS_MODEL);
    /** @type {string} */
    let effort = readLS(LS_EFFORT) || 'auto';
    let open = false;
    let goneToastShown = false;
    let searchQuery = '';

    function readLS(key) {
        try {
            return String(localStorage.getItem(key) || '').trim();
        } catch (e) {
            return '';
        }
    }

    function writeLS(key, value) {
        try {
            if (value) localStorage.setItem(key, value);
            else localStorage.removeItem(key);
        } catch (e) { /* ignore */ }
    }

    function findModel(id) {
        if (!catalog || !catalog.models) return null;
        for (let i = 0; i < catalog.models.length; i++) {
            if (catalog.models[i].id === id) return catalog.models[i];
        }
        return null;
    }

    function clampEffort(model, wanted) {
        const w = String(wanted || 'auto').toLowerCase();
        const options = (catalog && catalog.effort && catalog.effort.options) || [
            'auto', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max',
        ];
        if (options.indexOf(w) === -1) {
            return (catalog && catalog.effort && catalog.effort.current) || 'auto';
        }
        if (!model || !model.supported_efforts || !model.supported_efforts.length) {
            return w === 'auto' ? 'auto' : w;
        }
        if (w === 'auto') return 'auto';
        if (model.supported_efforts.indexOf(w) !== -1) return w;
        return model.default_effort || model.supported_efforts[0] || 'auto';
    }

    function modelPillLabel(m) {
        if (!m) return 'Model';
        const id = String(m.id || '');
        return id || m.label || 'Model';
    }

    function updatePill() {
        const m = findModel(modelId);
        const effortLabel = effort || 'auto';
        labelEl.textContent = modelPillLabel(m) + ' · ' + effortLabel;
        pill.setAttribute('aria-label', 'Model ' + modelPillLabel(m) + ', effort ' + effortLabel);
    }

    function setOpen(next) {
        open = !!next;
        menu.hidden = !open;
        pill.setAttribute('aria-expanded', open ? 'true' : 'false');
        if (open) {
            const searchEl = menu.querySelector('#modelSearchInput');
            if (searchEl) searchEl.focus();
            else {
                const focusable = menu.querySelector('[role="option"], [data-effort]');
                if (focusable) focusable.focus();
            }
        }
    }

    function applyGoneModelFallback() {
        if (!catalog) return;
        const m = findModel(modelId);
        if (modelId && !m) {
            const fallback = catalog.default_model_id || (catalog.models[0] && catalog.models[0].id) || '';
            modelId = fallback;
            writeLS(LS_MODEL, modelId);
            if (!goneToastShown) {
                goneToastShown = true;
                showToast('Model unavailable · using default');
            }
        }
        if (!modelId && catalog.default_model_id) {
            modelId = catalog.default_model_id;
            writeLS(LS_MODEL, modelId);
        }
        effort = clampEffort(findModel(modelId), effort || (catalog.effort && catalog.effort.current) || 'auto');
        writeLS(LS_EFFORT, effort);
        updatePill();
        renderMenu();
    }

    function renderMenu() {
        if (!catalog) {
            menu.innerHTML = '<div class="composer-model-empty">Loading models…</div>';
            return;
        }

        let html = '';
        html += '<div class="composer-model-section" role="group" aria-label="Models">';
        html += '<div class="composer-model-search">';
        html += '<i class="bi bi-search composer-model-search__icon" aria-hidden="true"></i>';
        html += '<input type="text" class="composer-model-search__input" id="modelSearchInput" placeholder="Search models…" autocomplete="off" value="' + escapeAttr(searchQuery) + '">';
        html += '</div>';

        const q = searchQuery.trim().toLowerCase();
        const filtered = [];
        for (let i = 0; i < catalog.models.length; i++) {
            const row = catalog.models[i];
            if (row.disabled) continue;
            if (q) {
                const haystack = ((row.label || '') + ' ' + (row.id || '') + ' ' + ((row.provider || '') )).toLowerCase();
                if (haystack.indexOf(q) === -1) continue;
            }
            filtered.push(row);
        }

        html += '<ul class="composer-model-list" role="listbox" aria-label="Models">';
        if (filtered.length === 0) {
            html += '<li class="composer-model-empty">No models match "' + escapeHtml(searchQuery) + '"</li>';
        } else {
            for (let i = 0; i < filtered.length; i++) {
                const row = filtered[i];
                const selected = row.id === modelId;
                const meta = [];
                if (row.supports_vision) meta.push('vision');
                const hasEffort = row.supported_efforts && row.supported_efforts.length > 0;
                if (hasEffort) meta.push('reasoning');
                html +=
                    '<li class="composer-model-row' + (selected ? ' is-selected' : '') + '">';
                html +=
                    '<button type="button" class="composer-model-item' +
                    (selected ? ' is-selected' : '') +
                    '" role="option" data-model="' +
                    escapeAttr(row.id) +
                    '"' +
                    (selected ? ' aria-selected="true"' : ' aria-selected="false"') +
                    '><span class="composer-model-item__name">' +
                    escapeHtml(row.label || row.id) +
                    '</span>' +
                    (meta.length
                        ? '<span class="composer-model-item__meta">' + escapeHtml(meta.join(' · ')) + '</span>'
                        : '') +
                    '</button>';
                if (hasEffort) {
                    const efforts = ['auto'].concat(row.supported_efforts.filter(function (e) { return e !== 'auto'; }));
                    html += '<div class="composer-model-efforts">';
                    for (let j = 0; j < efforts.length; j++) {
                        const e = efforts[j];
                        const isSel = selected && e === effort;
                        html +=
                            '<button type="button" class="composer-model-effort' +
                            (isSel ? ' is-selected' : '') +
                            '" data-effort="' + escapeAttr(e) +
                            '" data-model="' + escapeAttr(row.id) +
                            '"' + (isSel ? ' aria-selected="true"' : ' aria-selected="false"') + '>' +
                            escapeHtml(e) +
                            '</button>';
                    }
                    html += '</div>';
                }
                html += '</li>';
            }
        }
        html += '</ul></div>';
        menu.innerHTML = html;

        const searchEl = menu.querySelector('#modelSearchInput');
        if (searchEl) {
            searchEl.addEventListener('input', function (ev) {
                searchQuery = ev.target.value || '';
                renderMenu();
                const newEl = menu.querySelector('#modelSearchInput');
                if (newEl) {
                    newEl.focus();
                    var len = newEl.value.length;
                    newEl.setSelectionRange(len, len);
                }
            });
            if (open && !searchQuery) {
                searchEl.focus();
            }
        }
    }

    function onMenuClick(ev) {
        if (ev.target.id === 'modelSearchInput' || ev.target.closest('.composer-model-search')) return;
        const effortBtn = ev.target.closest('[data-effort]');
        if (effortBtn) {
            const targetModel = effortBtn.getAttribute('data-model') || '';
            if (targetModel && targetModel !== modelId) {
                modelId = targetModel;
                writeLS(LS_MODEL, modelId);
            }
            effort = clampEffort(findModel(modelId), effortBtn.getAttribute('data-effort'));
            writeLS(LS_EFFORT, effort);
            updatePill();
            renderMenu();
            setOpen(false);
            pill.focus();
            return;
        }
        const modelBtn = ev.target.closest('[data-model]');
        if (modelBtn) {
            modelId = modelBtn.getAttribute('data-model') || '';
            writeLS(LS_MODEL, modelId);
            effort = clampEffort(findModel(modelId), effort);
            writeLS(LS_EFFORT, effort);
            searchQuery = '';
            updatePill();
            renderMenu();
            setOpen(false);
            pill.focus();
        }
    }

    function onDocPointer(ev) {
        if (!open) return;
        if (pill.contains(ev.target) || menu.contains(ev.target)) return;
        setOpen(false);
    }

    function onKey(ev) {
        if (ev.key === 'Escape' && open) {
            ev.preventDefault();
            setOpen(false);
            pill.focus();
        }
    }

    pill.addEventListener('click', function () {
        setOpen(!open);
    });
    menu.addEventListener('click', onMenuClick);
    document.addEventListener('pointerdown', onDocPointer);
    document.addEventListener('keydown', onKey);

    function dedupeModels(models) {
        const seen = {};
        const out = [];
        for (let i = 0; i < models.length; i++) {
            const m = models[i];
            const key = (m.id || '') + '\x00' + (m.label || '');
            if (seen[key]) continue;
            seen[key] = true;
            out.push(m);
        }
        return out;
    }

    async function refresh() {
        try {
            catalog = await opts.listModels(opts.api, {});
            if (catalog && Array.isArray(catalog.models)) {
                catalog.models = dedupeModels(catalog.models);
            }
            applyGoneModelFallback();
        } catch (err) {
            menu.innerHTML = '<div class="composer-model-empty">Models unavailable</div>';
            updatePill();
        }
    }

    updatePill();
    refresh();

    return {
        getSelection: function () {
            return {
                model: modelId || '',
                effort: effort || '',
            };
        },
        refresh: refresh,
        destroy: function () {
            document.removeEventListener('pointerdown', onDocPointer);
            document.removeEventListener('keydown', onKey);
        },
    };
}

function escapeHtml(s) {
    return String(s || '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function escapeAttr(s) {
    return escapeHtml(s).replace(/'/g, '&#39;');
}

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

    function shortModelLabel(m) {
        if (!m) return 'Model';
        const id = String(m.id || '');
        const slash = id.lastIndexOf('/');
        return slash >= 0 ? id.slice(slash + 1) : (m.label || id || 'Model');
    }

    function updatePill() {
        const m = findModel(modelId);
        const effortLabel = effort || 'auto';
        labelEl.textContent = shortModelLabel(m) + ' · ' + effortLabel;
        pill.setAttribute('aria-label', 'Model ' + shortModelLabel(m) + ', effort ' + effortLabel);
    }

    function setOpen(next) {
        open = !!next;
        menu.hidden = !open;
        pill.setAttribute('aria-expanded', open ? 'true' : 'false');
        if (open) {
            const focusable = menu.querySelector('[role="option"], [data-effort]');
            if (focusable) focusable.focus();
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
                showToast('Model tidak tersedia · memakai default');
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
        const m = findModel(modelId);
        const showEffort = m && Array.isArray(m.supported_efforts) && m.supported_efforts.length > 0;
        const effortOpts = showEffort
            ? ['auto'].concat(m.supported_efforts.filter(function (e) { return e !== 'auto'; }))
            : ((catalog.effort && catalog.effort.options) || []);

        let html = '';
        if (showEffort || (effortOpts && effortOpts.length)) {
            html += '<div class="composer-model-section" role="group" aria-label="Reasoning effort">';
            html += '<div class="composer-model-section__title">Reasoning</div>';
            html += '<div class="composer-model-efforts">';
            const list = showEffort ? effortOpts : ['auto'];
            for (let i = 0; i < list.length; i++) {
                const e = list[i];
                const selected = e === effort ? ' aria-selected="true"' : ' aria-selected="false"';
                html +=
                    '<button type="button" class="composer-model-effort' +
                    (e === effort ? ' is-selected' : '') +
                    '" data-effort="' +
                    escapeAttr(e) +
                    '" role="option"' +
                    selected +
                    '>' +
                    escapeHtml(e) +
                    '</button>';
            }
            html += '</div></div>';
        }

        html += '<div class="composer-model-section" role="group" aria-label="Models">';
        html += '<div class="composer-model-section__title">Model</div>';
        html += '<ul class="composer-model-list" role="listbox" aria-label="Models">';
        for (let i = 0; i < catalog.models.length; i++) {
            const row = catalog.models[i];
            if (row.disabled) continue;
            const selected = row.id === modelId;
            const meta = [];
            if (row.supports_vision) meta.push('vision');
            if (row.supported_efforts && row.supported_efforts.length) meta.push('effort');
            html +=
                '<li><button type="button" class="composer-model-item' +
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
                '</button></li>';
        }
        html += '</ul></div>';
        menu.innerHTML = html;
    }

    function onMenuClick(ev) {
        const effortBtn = ev.target.closest('[data-effort]');
        if (effortBtn) {
            effort = clampEffort(findModel(modelId), effortBtn.getAttribute('data-effort'));
            writeLS(LS_EFFORT, effort);
            updatePill();
            renderMenu();
            return;
        }
        const modelBtn = ev.target.closest('[data-model]');
        if (modelBtn) {
            modelId = modelBtn.getAttribute('data-model') || '';
            writeLS(LS_MODEL, modelId);
            effort = clampEffort(findModel(modelId), effort);
            writeLS(LS_EFFORT, effort);
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

    async function refresh() {
        try {
            catalog = await opts.listModels(opts.api, {});
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

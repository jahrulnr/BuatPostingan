/**
 * Theme preference: auto | light | dark.
 * Persisted at localStorage key `bp.theme`.
 * Resolved scheme applied as html[data-theme="light"|"dark"].
 */

const STORAGE_KEY = 'bp.theme';
const PREFS = ['auto', 'light', 'dark'];

function readPref() {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (PREFS.indexOf(raw) !== -1) return raw;
    } catch (e) { /* ignore */ }
    return 'auto';
}

function writePref(pref) {
    try {
        localStorage.setItem(STORAGE_KEY, pref);
    } catch (e) { /* ignore */ }
}

function systemScheme() {
    try {
        return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    } catch (e) {
        return 'light';
    }
}

function resolveScheme(pref) {
    if (pref === 'light' || pref === 'dark') return pref;
    return systemScheme();
}

function applyTheme(pref) {
    const preference = PREFS.indexOf(pref) !== -1 ? pref : 'auto';
    const scheme = resolveScheme(preference);
    const root = document.documentElement;
    root.dataset.themePref = preference;
    root.dataset.theme = scheme;
    root.style.colorScheme = scheme;
    return { preference, scheme };
}

function labelFor(pref) {
    if (pref === 'light') return 'Light';
    if (pref === 'dark') return 'Dark';
    return 'Auto';
}

function iconFor(pref) {
    if (pref === 'light') return 'bi-sun';
    if (pref === 'dark') return 'bi-moon-stars';
    return 'bi-circle-half';
}

/**
 * Boot theme + wire toggle control.
 * @param {{ toggleEl?: HTMLElement|null }} [options]
 */
export function bootTheme(options) {
    const opts = options || {};
    let preference = readPref();
    let state = applyTheme(preference);

    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onSystemChange = function () {
        if (preference !== 'auto') return;
        state = applyTheme('auto');
        paintToggle();
    };
    if (typeof media.addEventListener === 'function') {
        media.addEventListener('change', onSystemChange);
    } else if (typeof media.addListener === 'function') {
        media.addListener(onSystemChange);
    }

    const toggleEl = opts.toggleEl || document.getElementById('bpThemeToggle');

    function paintToggle() {
        if (!toggleEl) return;
        toggleEl.dataset.themePref = state.preference;
        toggleEl.setAttribute('aria-label', 'Theme: ' + labelFor(state.preference) + ' (click to cycle)');
        toggleEl.title = 'Theme: ' + labelFor(state.preference);
        const icon = toggleEl.querySelector('[data-theme-icon]');
        const text = toggleEl.querySelector('[data-theme-label]');
        if (icon) {
            icon.className = 'bi ' + iconFor(state.preference);
            icon.setAttribute('data-theme-icon', '');
        }
        if (text) text.textContent = labelFor(state.preference);
    }

    function cycle() {
        const idx = PREFS.indexOf(preference);
        preference = PREFS[(idx + 1) % PREFS.length];
        writePref(preference);
        state = applyTheme(preference);
        paintToggle();
        return state;
    }

    if (toggleEl) {
        toggleEl.addEventListener('click', function () { cycle(); });
    }

    paintToggle();

    return {
        getPreference: function () { return preference; },
        getScheme: function () { return state.scheme; },
        setPreference: function (pref) {
            preference = PREFS.indexOf(pref) !== -1 ? pref : 'auto';
            writePref(preference);
            state = applyTheme(preference);
            paintToggle();
            return state;
        },
        cycle: cycle,
    };
}

// Apply before paint when imported as module (FOUC mitigation: also inline in <head>).
applyTheme(readPref());

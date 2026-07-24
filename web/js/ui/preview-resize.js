/**
 * Desktop-only resize for the right preview column.
 * Persists width at localStorage key `bp.previewWidth` (px number).
 */

const STORAGE_KEY = 'bp.previewWidth';
const DEFAULT_PX = 320;
const MIN_PX = 240;
const MIN_MAIN_PX = 280;
const SPLIT_PX = 5;
const DESKTOP_MQ = '(min-width: 1101px)';

function readStored() {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (raw == null || raw === '') return null;
        const n = Number(raw);
        if (Number.isFinite(n) && n > 0) return n;
    } catch (e) { /* ignore */ }
    return null;
}

function writeStored(px) {
    try {
        localStorage.setItem(STORAGE_KEY, String(Math.round(px)));
    } catch (e) { /* ignore */ }
}

function isDesktop() {
    try {
        return window.matchMedia(DESKTOP_MQ).matches;
    } catch (e) {
        return true;
    }
}

function maxPreviewPx(workspace) {
    const rail = document.getElementById('wcRail');
    const railW = rail ? rail.getBoundingClientRect().width : 264;
    const avail = workspace.getBoundingClientRect().width - railW - SPLIT_PX - MIN_MAIN_PX;
    const half = Math.floor(window.innerWidth * 0.5);
    return Math.max(MIN_PX, Math.min(half, Math.floor(avail)));
}

function clamp(px, workspace) {
    const max = maxPreviewPx(workspace);
    return Math.min(Math.max(Math.round(px), MIN_PX), max);
}

function applyWidth(px, workspace) {
    const w = clamp(px, workspace);
    workspace.style.setProperty('--bp-preview-w', w + 'px');
    return w;
}

/**
 * @param {{ workspaceEl?: HTMLElement|null, handleEl?: HTMLElement|null }} [options]
 */
export function bootPreviewResize(options) {
    const opts = options || {};
    const workspace = opts.workspaceEl || document.getElementById('bpWorkspace');
    const handle = opts.handleEl || document.getElementById('bpPreviewSplit');
    if (!workspace || !handle) return null;

    let width = readStored();
    if (width == null) width = DEFAULT_PX;

    function paint() {
        if (!isDesktop()) {
            workspace.style.removeProperty('--bp-preview-w');
            return;
        }
        width = applyWidth(width, workspace);
    }

    paint();

    let dragging = false;
    let startX = 0;
    let startW = 0;

    function onPointerDown(e) {
        if (!isDesktop()) return;
        if (e.button != null && e.button !== 0) return;
        dragging = true;
        startX = e.clientX;
        startW = workspace.getBoundingClientRect
            ? (() => {
                const preview = document.getElementById('wcPreview');
                return preview ? preview.getBoundingClientRect().width : width;
            })()
            : width;
        document.documentElement.classList.add('is-resizing-preview');
        handle.classList.add('is-active');
        try {
            if (e.pointerId != null) handle.setPointerCapture(e.pointerId);
        } catch (err) { /* synthetic/tests may lack a live pointer id */ }
        e.preventDefault();
    }

    function onPointerMove(e) {
        if (!dragging) return;
        // Dragging the left edge of preview: move left → wider preview
        const delta = startX - e.clientX;
        width = applyWidth(startW + delta, workspace);
    }

    function onPointerUp(e) {
        if (!dragging) return;
        dragging = false;
        document.documentElement.classList.remove('is-resizing-preview');
        handle.classList.remove('is-active');
        try { handle.releasePointerCapture?.(e.pointerId); } catch (err) { /* ignore */ }
        writeStored(width);
    }

    function onDblClick(e) {
        if (!isDesktop()) return;
        e.preventDefault();
        width = applyWidth(DEFAULT_PX, workspace);
        writeStored(width);
    }

    handle.addEventListener('pointerdown', onPointerDown);
    handle.addEventListener('pointermove', onPointerMove);
    handle.addEventListener('pointerup', onPointerUp);
    handle.addEventListener('pointercancel', onPointerUp);
    handle.addEventListener('dblclick', onDblClick);

    const mq = window.matchMedia(DESKTOP_MQ);
    const onMq = function () { paint(); };
    if (typeof mq.addEventListener === 'function') mq.addEventListener('change', onMq);
    else if (typeof mq.addListener === 'function') mq.addListener(onMq);

    let resizeTimer = null;
    window.addEventListener('resize', function () {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(paint, 80);
    });

    return {
        getWidth: function () { return width; },
        reset: function () {
            width = applyWidth(DEFAULT_PX, workspace);
            writeStored(width);
            return width;
        },
        setWidth: function (px) {
            width = applyWidth(px, workspace);
            writeStored(width);
            return width;
        },
    };
}

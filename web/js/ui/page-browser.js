import { confirmAppDialog } from './dialogs.js';

function normalizePages(raw) {
    const pages = raw && Array.isArray(raw.pages) ? raw.pages : [];
    return pages.map(function (page) {
        return {
            id: String(page && page.id || ''),
            published: !!(page && page.published),
            entries: Array.isArray(page && page.entries) ? page.entries : [],
        };
    }).filter(function (page) { return page.id !== ''; })
        .sort(function (a, b) { return a.id.localeCompare(b.id); });
}

function entryIcon(type, path) {
    if (type === 'directory') return 'bi-folder2';
    if (/\.html?$/i.test(path)) return 'bi-filetype-html';
    if (/\.css$/i.test(path)) return 'bi-filetype-css';
    if (/\.js$/i.test(path)) return 'bi-filetype-js';
    return 'bi-file-earmark';
}

function escapeHtml(value) {
    const div = document.createElement('div');
    div.textContent = String(value || '');
    return div.innerHTML;
}

export { normalizePages };

export function bootPageBrowser(options) {
    const opts = options || {};
    const panel = opts.panelEl;
    const tree = panel && panel.querySelector('#pagesTree');
    const hint = panel && panel.querySelector('#pagesHint');
    const refreshBtn = panel && panel.querySelector('#pagesRefresh');
    const menu = panel && panel.querySelector('#pagesContextMenu');
    const api = opts.api || {};
    const openPages = new Set();
    let pages = [];
    let menuPageID = '';

    if (!panel || !tree || !hint || !menu) return { refresh: function () {} };

    function setHint(text, state) {
        hint.textContent = text || '';
        hint.dataset.state = state || '';
    }

    function render() {
        tree.textContent = '';
        if (!pages.length) {
            setHint('No page workspaces yet.', 'empty');
            return;
        }
        setHint('Right-click a page folder for actions.', '');
        pages.forEach(function (page) {
            const expanded = openPages.has(page.id);
            const item = document.createElement('div');
            item.className = 'wc-pages__node wc-pages__node--page';
            item.setAttribute('role', 'treeitem');
            item.setAttribute('aria-expanded', expanded ? 'true' : 'false');
            item.dataset.pageId = page.id;

            const row = document.createElement('button');
            row.type = 'button';
            row.className = 'wc-pages__row';
            row.dataset.pageId = page.id;
            row.setAttribute('aria-expanded', expanded ? 'true' : 'false');
            row.innerHTML =
                '<i class="bi ' + (expanded ? 'bi-chevron-down' : 'bi-chevron-right') + ' wc-pages__chevron" aria-hidden="true"></i>' +
                '<i class="bi bi-folder-fill wc-pages__icon" aria-hidden="true"></i>' +
                '<span class="wc-pages__name">' + escapeHtml(page.id) + '</span>' +
                '<span class="wc-pages__state ' + (page.published ? 'is-published' : '') + '">' +
                (page.published ? 'Published' : 'Draft') + '</span>';
            item.appendChild(row);

            if (expanded) {
                const children = document.createElement('div');
                children.className = 'wc-pages__children';
                children.setAttribute('role', 'group');
                page.entries.forEach(function (entry) {
                    const path = String(entry && entry.path || '');
                    if (!path) return;
                    const child = document.createElement('div');
                    child.className = 'wc-pages__file';
                    child.setAttribute('role', 'treeitem');
                    child.innerHTML = '<i class="bi ' + entryIcon(entry.type, path) + ' wc-pages__file-icon" aria-hidden="true"></i><span>' + escapeHtml(path) + '</span>';
                    children.appendChild(child);
                });
                item.appendChild(children);
            }
            tree.appendChild(item);
        });
    }

    function closeMenu() {
        menu.hidden = true;
        menuPageID = '';
    }

    function openMenu(pageID, x, y) {
        menuPageID = pageID;
        menu.hidden = false;
        const rect = panel.getBoundingClientRect();
        const maxX = Math.max(0, panel.clientWidth - menu.offsetWidth - 8);
        const maxY = Math.max(0, panel.clientHeight - menu.offsetHeight - 8);
        menu.style.left = Math.min(maxX, Math.max(0, x - rect.left)) + 'px';
        menu.style.top = Math.min(maxY, Math.max(0, y - rect.top)) + 'px';
        const page = pages.find(function (item) { return item.id === pageID; });
        menu.querySelector('[data-page-action="publish"]').hidden = !!(page && page.published);
        menu.querySelector('[data-page-action="unpublish"]').hidden = !(page && page.published);
        const first = menu.querySelector('button:not([hidden])');
        if (first) first.focus();
    }

    function refresh() {
        if (typeof api.listPages !== 'function') {
            setHint('Pages API is not available.', 'error');
            return Promise.resolve();
        }
        if (refreshBtn) refreshBtn.disabled = true;
        setHint('Loading pages…', 'loading');
        return api.listPages().then(function (out) {
            pages = normalizePages(out);
            render();
        }).catch(function () {
            pages = [];
            render();
            setHint('Unable to load pages.', 'error');
        }).finally(function () {
            if (refreshBtn) refreshBtn.disabled = false;
        });
    }

    async function mutate(action) {
        const id = menuPageID;
        closeMenu();
        if (!id) return;
        if (action === 'delete') {
            const approved = await confirmAppDialog({
                eyebrow: 'Pages',
                title: 'Delete page?',
                message: 'Page “' + id + '” and every file within it will be deleted. This cannot be undone.',
                icon: 'bi-folder-x',
                confirmLabel: 'Delete page',
                tone: 'danger',
            });
            if (!approved) return;
        }
        const fn = action === 'publish' ? api.publishPage
            : (action === 'unpublish' ? api.unpublishPage : api.deletePage);
        if (typeof fn !== 'function') {
            setHint('This Pages action is not available.', 'error');
            return;
        }
        setHint(action === 'delete' ? 'Deleting page…' : 'Updating page status…', 'loading');
        Promise.resolve(fn({ pageId: id })).then(refresh).catch(function () {
            setHint('Page action failed.', 'error');
        });
    }

    tree.addEventListener('click', function (event) {
        const row = event.target.closest('.wc-pages__row');
        if (!row) return;
        const id = row.dataset.pageId;
        if (openPages.has(id)) openPages.delete(id);
        else openPages.add(id);
        render();
    });
    tree.addEventListener('contextmenu', function (event) {
        const row = event.target.closest('.wc-pages__row');
        if (!row) return;
        event.preventDefault();
        openMenu(row.dataset.pageId, event.clientX, event.clientY);
    });
    tree.addEventListener('keydown', function (event) {
        if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return;
        const row = event.target.closest('.wc-pages__row');
        if (!row) return;
        event.preventDefault();
        const rect = row.getBoundingClientRect();
        openMenu(row.dataset.pageId, rect.left, rect.bottom);
    });
    menu.addEventListener('click', function (event) {
        const btn = event.target.closest('[data-page-action]');
        if (btn) mutate(btn.dataset.pageAction);
    });
    document.addEventListener('pointerdown', function (event) {
        if (!menu.hidden && !menu.contains(event.target)) closeMenu();
    });
    document.addEventListener('keydown', function (event) {
        if (event.key === 'Escape') closeMenu();
    });
    if (refreshBtn) refreshBtn.addEventListener('click', refresh);

    refresh();
    return { refresh: refresh };
}

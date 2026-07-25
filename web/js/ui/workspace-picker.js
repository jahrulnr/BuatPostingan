/**
 * Workspace picker — folder browser dialog for selecting a working directory.
 * Persistence: per-thread via bp.workspace.<threadId> in localStorage.
 * When no workspace is set, empty string is returned (use config default).
 */

/**
 * @param {{
 *   root?: ParentNode,
 *   api: Object,
 *   browseDir: Function,
 *   threadId?: () => string,
 *   onChange?: (workspace: string) => void,
 * }} opts
 */
export function bootWorkspacePicker(opts) {
    var root = opts.root || document;
    var byId = function (id) {
        if (root.nodeType === 9) return root.getElementById(id);
        return root.querySelector('#' + id);
    };

    var pill = byId('workspacePill');
    var labelEl = byId('workspacePillLabel');
    var dialog = byId('workspaceDialog');
    if (!pill || !labelEl || !dialog) {
        return {
            getWorkspace: function () { return ''; },
            destroy: function () {},
        };
    }

    var cwdEl = byId('workspaceCwd');
    var listEl = byId('workspaceDirList');
    var upBtn = byId('workspaceUpBtn');
    var selectBtn = byId('workspaceSelectBtn');
    var clearBtn = byId('workspaceClearBtn');
    var errorEl = byId('workspaceDialogError');

    var currentPath = '/';
    var parentPath = '';
    var selectedPath = '';
    var workspace = '';
    var threadIdFn = opts.threadId || function () { return ''; };

    function lsKey() {
        var tid = threadIdFn();
        return tid ? 'bp.workspace.' + tid : 'bp.workspace';
    }

    function loadWorkspace() {
        try {
            workspace = localStorage.getItem(lsKey()) || '';
        } catch (e) {
            workspace = '';
        }
    }

    function saveWorkspace(ws) {
        workspace = ws;
        try {
            if (ws) {
                localStorage.setItem(lsKey(), ws);
            } else {
                localStorage.removeItem(lsKey());
            }
        } catch (e) { /* ignore */ }
    }

    function shortLabel(path) {
        if (!path) return 'Workspace';
        var parts = path.replace(/\/+$/, '').split('/');
        var last = parts[parts.length - 1];
        if (last) return last;
        return path;
    }

    function updatePill() {
        if (workspace) {
            labelEl.textContent = shortLabel(workspace);
            pill.setAttribute('data-wc-tooltip', workspace);
        } else {
            labelEl.textContent = 'Workspace';
            pill.setAttribute('data-wc-tooltip', 'Workspace folder (default)');
        }
    }

    function showError(msg) {
        if (errorEl) {
            errorEl.textContent = msg;
            errorEl.hidden = !msg;
        }
    }

    function renderList(entries) {
        listEl.innerHTML = '';
        selectedPath = currentPath;
        entries.forEach(function (entry) {
            var item = document.createElement('button');
            item.type = 'button';
            item.className = 'workspace-browser__item';
            if (entry.path === selectedPath) {
                item.classList.add('workspace-browser__item--selected');
            }
            item.innerHTML = '<i class="bi bi-folder" aria-hidden="true"></i><span>' +
                escapeHtml(entry.name) + '</span>';
            item.addEventListener('click', function () {
                selectedPath = entry.path;
                navigate(entry.path);
            });
            listEl.appendChild(item);
        });
    }

    function navigate(path) {
        showError('');
        opts.browseDir(opts.api, { path: path }).then(function (res) {
            currentPath = res.path || '/';
            parentPath = res.parent || '';
            cwdEl.textContent = currentPath;
            upBtn.disabled = !parentPath;
            selectedPath = currentPath;
            renderList(res.entries || []);
        }).catch(function (err) {
            showError('Cannot browse: ' + (err && err.message ? err.message : String(err)));
        });
    }

    function openDialog() {
        if (!dialog || !dialog.hidden) return;
        dialog.hidden = false;
        document.body.classList.add('has-wc-dialog');
        pill.setAttribute('aria-expanded', 'true');
        loadWorkspace();
        requestAnimationFrame(function () {
            dialog.classList.add('is-open');
            navigate(workspace || currentPath);
            setTimeout(function () {
                if (upBtn) upBtn.focus();
            }, 80);
        });
    }

    function closeDialog() {
        if (!dialog || dialog.hidden) return;
        dialog.classList.remove('is-open');
        document.body.classList.remove('has-wc-dialog');
        pill.setAttribute('aria-expanded', 'false');
        showError('');
        setTimeout(function () {
            dialog.hidden = true;
        }, 220);
    }

    pill.addEventListener('click', openDialog);

    dialog.querySelectorAll('[data-workspace-close]').forEach(function (el) {
        el.addEventListener('click', closeDialog);
    });

    upBtn.addEventListener('click', function () {
        if (parentPath) navigate(parentPath);
    });

    selectBtn.addEventListener('click', function () {
        saveWorkspace(selectedPath);
        updatePill();
        closeDialog();
        if (opts.onChange) opts.onChange(selectedPath);
    });

    clearBtn.addEventListener('click', function () {
        saveWorkspace('');
        updatePill();
        closeDialog();
        if (opts.onChange) opts.onChange('');
    });

    document.addEventListener('keydown', function (e) {
        if (!dialog.hidden && e.key === 'Escape') {
            e.preventDefault();
            closeDialog();
        }
    });

    loadWorkspace();
    updatePill();

    return {
        getWorkspace: function () { return workspace; },
        setWorkspace: function (ws) { saveWorkspace(ws || ''); updatePill(); },
        reload: function () { loadWorkspace(); updatePill(); },
        destroy: function () {},
    };
}

function escapeHtml(s) {
    return String(s)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

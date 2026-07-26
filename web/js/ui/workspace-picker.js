/**
 * Workspace picker — folder browser dialog for selecting a working directory.
 * Persistence: per-thread via bp.workspace.<threadId> in localStorage.
 * When no workspace is set, empty string is returned (use backend current dir).
 */

export function workspaceLabel(workspace, defaultPath) {
    return String(workspace || defaultPath || 'Workspace');
}

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

    var currentPath = '';
    var defaultPath = '';
    var parentPath = '';
    var selectedPath = '';
    var workspace = '';
    var browseGeneration = 0;
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

    function updatePill() {
        var label = workspaceLabel(workspace, defaultPath);
        labelEl.textContent = label;
        pill.setAttribute('data-wc-tooltip', workspace
            ? workspace
            : (defaultPath ? defaultPath + ' (default)' : 'Workspace folder (default)'));
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
        var generation = ++browseGeneration;
        showError('');
        opts.browseDir(opts.api, { path: path || '' }).then(function (res) {
            if (generation !== browseGeneration) return;
            currentPath = res.path || defaultPath || '/';
            if (!path && res.path) defaultPath = res.path;
            parentPath = res.parent || '';
            cwdEl.textContent = currentPath;
            upBtn.disabled = !parentPath;
            selectedPath = currentPath;
            renderList(res.entries || []);
            updatePill();
        }).catch(function (err) {
            if (generation !== browseGeneration) return;
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
            // Empty path asks the backend for its actual current directory.
            navigate(workspace || '');
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
        // Restore the explicit current directory label after a cleared selection.
        navigate('');
    });

    document.addEventListener('keydown', function (e) {
        if (!dialog.hidden && e.key === 'Escape') {
            e.preventDefault();
            closeDialog();
        }
    });

    loadWorkspace();
    updatePill();
    // Populate the full-path label before the dialog is first opened. This is
    // intentionally an empty path: the backend resolves it to os.Getwd().
    navigate('');

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

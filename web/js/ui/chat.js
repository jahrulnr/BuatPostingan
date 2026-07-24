import {
    api,
    listConversations,
    createThread,
    getThread,
    renameThread,
    startTurn,
    retryTurn,
    interruptTurn,
    subscribeEvents,
} from '../api/index.js';
import {
    escapeHtml,
    formatToolCall,
    summarizeToolResult,
    welcomeHtml,
    paintTurnBubble,
    appendUserMessage,
    appendError,
} from './render.js';

export function bootChat() {
    const messagesEl = document.getElementById('chatMessages');
    const inputEl = document.getElementById('chatInput');
    const sendBtn = document.getElementById('chatSend');
    const stopBtn = document.getElementById('chatStop');
    const statusEl = document.getElementById('chatStatus');
    const floorEl = document.getElementById('chatFloor');
    const indexBannerEl = document.getElementById('chatIndexBanner');
    const newBtn = document.getElementById('btnNewChat') || document.getElementById('chatNew');
    const toastEl = document.getElementById('chatToast');
    const listEl = document.getElementById('conversationList');
    const conversationCountEl = document.getElementById('conversationCount');
    const roomTitleEl = document.getElementById('roomTitle');
    const roomMetaEl = document.getElementById('roomMeta');
    const renameBtn = document.getElementById('btnRename');
    const renameDialog = document.getElementById('renameDialog');
    const renamePanel = renameDialog ? renameDialog.querySelector('.wc-dialog__panel') : null;
    const renameForm = document.getElementById('renameForm');
    const renameInput = document.getElementById('renameInput');
    const renameCount = document.getElementById('renameCount');
    const renameError = document.getElementById('renameDialogError');
    const renameSubmit = document.getElementById('renameSubmit');
    let renameReturnFocus = null;

    if (!messagesEl || !inputEl || !sendBtn || !statusEl) {
        return null;
    }

    const adminUserId = Number(api.adminUserId || 1);
    const adminDisplayName = String(api.adminDisplayName || 'Admin User');
    const storageKey = 'bp.webchat.thread_id.' + adminUserId;
    const hasRail = !!listEl;

    let threadId = null;
    let turnId = null;
    let sub = null;
    let busy = false;
    let isInitiator = false;
    let floorRemainingSec = 0;
    let floorHolderId = null;
    let floorTimer = null;
    let docsIndexUsable = true;
    let docsIndexPoll = null;
    let conversations = [];
    let pendingUserText = null;
    let pendingOptimisticEl = null;
    let seenItemIds = {};
    /** @type {Record<string, any>} */
    let turnUi = {};

    function setStatus(text) {
        statusEl.textContent = text;
        const normalized = String(text || '').toLowerCase();
        let state = 'neutral';
        if (/ready|hydrated|completed/.test(normalized)) state = 'ready';
        else if (/failed|denied|locked|429|423|error/.test(normalized)) state = 'danger';
        else if (/indexing|streaming|thinking|busy|stopping|sending|queued|retrying/.test(normalized)) state = 'busy';
        statusEl.dataset.state = state;
    }

    function showToast(text) {
        if (!toastEl) {
            setStatus(text);
            return;
        }
        toastEl.textContent = text;
        toastEl.hidden = false;
        clearTimeout(showToast._hide);
        requestAnimationFrame(function () { toastEl.classList.add('is-visible'); });
        clearTimeout(showToast._t);
        showToast._t = setTimeout(function () {
            toastEl.classList.remove('is-visible');
            showToast._hide = setTimeout(function () { toastEl.hidden = true; }, 180);
        }, 2800);
    }

    function updateComposer() {
        const floorBlocked = floorRemainingSec > 0 && floorHolderId !== adminUserId;
        const indexBlocked = !docsIndexUsable;
        sendBtn.disabled = busy || floorBlocked || indexBlocked;
        inputEl.disabled = busy || floorBlocked || indexBlocked;
        if (stopBtn) {
            stopBtn.hidden = !busy;
            stopBtn.disabled = !(busy && isInitiator);
        }
        sendBtn.hidden = !!busy;
    }

    function applyDocsIndexGate(gate) {
        if (!gate || typeof gate !== 'object') return;
        docsIndexUsable = !!gate.usable;
        if (indexBannerEl) {
            if (docsIndexUsable) {
                indexBannerEl.hidden = true;
                indexBannerEl.textContent = '';
            } else {
                indexBannerEl.hidden = false;
                indexBannerEl.innerHTML =
                    '<i class="bi bi-hourglass-split" aria-hidden="true"></i> ' +
                    escapeHtml(gate.message || 'Docs index belum siap. AI terkunci.');
            }
        }
        if (!docsIndexUsable) {
            setStatus(gate.status === 'building' ? 'Indexing docs…' : 'AI locked · docs index');
            startDocsIndexPoll();
        } else if (docsIndexPoll) {
            clearInterval(docsIndexPoll);
            docsIndexPoll = null;
        }
        updateComposer();
    }

    function startDocsIndexPoll() {
        if (docsIndexPoll) return;
        docsIndexPoll = setInterval(async function () {
            try {
                const list = await listConversations(api, {});
                if (list && list.docs_index) {
                    applyDocsIndexGate(list.docs_index);
                    if (docsIndexUsable) {
                        setStatus('Ready');
                        showToast('Docs index siap · AI aktif');
                    }
                }
            } catch (e) {
                // keep polling
            }
        }, 2000);
    }

    function refreshFloorBanner() {
        if (!floorEl) return;
        if (floorRemainingSec <= 0 || !floorHolderId || floorHolderId === adminUserId) {
            floorEl.hidden = true;
            floorEl.textContent = '';
            return;
        }
        const m = Math.floor(floorRemainingSec / 60);
        const s = floorRemainingSec % 60;
        floorEl.hidden = false;
        floorEl.innerHTML =
            '<i class="bi bi-mic-mute-fill" aria-hidden="true"></i> ' +
            '<strong>Admin #' + floorHolderId + '</strong> menahan floor · sisa ' +
            m + 'm ' + String(s).padStart(2, '0') + 's';
    }

    function startFloorTicker() {
        if (floorTimer) clearInterval(floorTimer);
        floorTimer = setInterval(function () {
            if (floorRemainingSec > 0 && floorHolderId !== adminUserId) {
                floorRemainingSec -= 1;
                if (floorRemainingSec <= 0) {
                    floorRemainingSec = 0;
                    floorHolderId = null;
                }
                refreshFloorBanner();
                updateComposer();
            }
        }, 1000);
    }

    function applyFloorFromPayload(data) {
        if (!data) return;
        if (data.floor_holder_admin_id != null) {
            floorHolderId = Number(data.floor_holder_admin_id);
        }
        if (typeof data.floor_remaining_sec === 'number') {
            floorRemainingSec = data.floor_remaining_sec;
        }
        refreshFloorBanner();
        updateComposer();
    }

    function clearMessages(welcome) {
        seenItemIds = {};
        pendingOptimisticEl = null;
        turnUi = {};
        messagesEl.innerHTML = welcome ? welcomeHtml() : '';
    }

    function clearTurnErrors(id) {
        messagesEl.querySelectorAll('.msg--error').forEach(function (el) {
            if (!id || el.dataset.turnId === id) el.remove();
        });
    }

    function ensureTurnUi(id) {
        const key = id || '_anon';
        if (turnUi[key]) return turnUi[key];
        const welcome = messagesEl.querySelector('.chat-welcome');
        if (welcome) welcome.remove();
        const art = document.createElement('article');
        art.className = 'msg msg--assistant';
        art.dataset.turnId = key;
        art.innerHTML = '<div class="msg__bubble"></div>';
        messagesEl.appendChild(art);
        turnUi[key] = { art: art, thinkingSteps: [], thinkingModel: null, tools: [], responseModel: null, text: '' };
        paintTurnBubble(messagesEl, turnUi[key]);
        return turnUi[key];
    }

    function applyReasoning(item) {
        const text = String(item.text || '').trim();
        if (!text) return;
        const state = ensureTurnUi(item.turn_id);
        const chunks = text.split(/\n+/).map(function (s) { return s.trim(); }).filter(Boolean);
        state.thinkingSteps = state.thinkingSteps.concat(chunks.length ? chunks : [text]);
        state.thinkingModel = item.model || state.thinkingModel;
        paintTurnBubble(messagesEl, state);
    }

    function applyToolCall(item) {
        const state = ensureTurnUi(item.turn_id);
        const callId = item.call_id || item.id || ('call_' + state.tools.length);
        const displayModel = item.model || (item.origin === 'host_preflight'
            ? { provider: 'BuatPostingan', id: 'docs preflight' }
            : null);
        const existing = state.tools.find(function (t) { return t.callId === callId; });
        if (existing) {
            existing.call = formatToolCall(item.name, item.arguments || {});
            existing.model = displayModel || existing.model || null;
        } else {
            state.tools.push({
                callId: callId,
                call: formatToolCall(item.name, item.arguments || {}),
                result: '…',
                ok: true,
                model: displayModel,
            });
        }
        paintTurnBubble(messagesEl, state);
    }

    function applyToolResult(item) {
        const state = ensureTurnUi(item.turn_id);
        const callId = item.call_id || '';
        const envelope = item.envelope || {};
        let row = state.tools.find(function (t) { return t.callId === callId; });
        if (!row) {
            row = {
                callId: callId || ('res_' + state.tools.length),
                call: 'tool',
                result: '—',
                ok: true,
            };
            state.tools.push(row);
        }
        row.ok = !!envelope.ok;
        row.result = summarizeToolResult(envelope);
        paintTurnBubble(messagesEl, state);
    }

    function applyAgentMessage(item) {
        const state = ensureTurnUi(item.turn_id);
        state.text = item.text || '';
        state.responseModel = item.model || null;
        if (item.id) state.art.dataset.id = item.id;
        paintTurnBubble(messagesEl, state);
    }

    function renderItem(item) {
        if (!item) return;
        const id = item.id || '';
        if (id && seenItemIds[id]) return;
        if (id) seenItemIds[id] = true;

        const type = item.type || '';
        if (type === 'user_message') {
            const text = item.text || '';
            if (pendingUserText !== null && text === pendingUserText && pendingOptimisticEl) {
                pendingOptimisticEl.dataset.id = id;
                if (id) seenItemIds[id] = true;
                pendingUserText = null;
                pendingOptimisticEl = null;
                return;
            }
            const aid = Number(item.admin_user_id || 0);
            const mine = aid === adminUserId || (aid === 0 && text && pendingUserText === text);
            const who = item.admin_display_name
                || (aid ? ('Admin #' + aid) : adminDisplayName);
            appendUserMessage(messagesEl, text, who, mine, id);
        } else if (type === 'reasoning') {
            applyReasoning(item);
        } else if (type === 'tool_call') {
            applyToolCall(item);
        } else if (type === 'tool_result') {
            applyToolResult(item);
        } else if (type === 'agent_message') {
            applyAgentMessage(item);
        } else if (type === 'turn.failed' && item.error) {
            const code = item.error.code || '';
            if (code !== 'interrupted') {
                appendError(messagesEl, (item.error.message || code || 'error'), item.turn_id || '', true);
            }
        } else if (type === 'turn.resumed') {
            clearTurnErrors(item.turn_id || '');
        }
    }

    function hydrateItems(items) {
        clearMessages(false);
        const lines = items || [];
        const completedTurns = {};
        const latestFailedSeq = {};
        lines.forEach(function (line) {
            if ((line.type || '') === 'turn.completed' && line.turn_id) {
                completedTurns[line.turn_id] = true;
            }
            if ((line.type || '') === 'turn.failed' && line.turn_id) {
                latestFailedSeq[line.turn_id] = Number(line.seq || 0);
            }
        });
        lines.forEach(function (line) {
            const type = line.type || '';
            if (type === 'thread.started' || type === 'turn.started' || type === 'turn.completed' || type === 'turn.resumed') {
                return;
            }
            if (type === 'turn.failed' && line.turn_id && completedTurns[line.turn_id]) {
                return;
            }
            if (type === 'turn.failed' && line.turn_id
                && Number(line.seq || 0) !== latestFailedSeq[line.turn_id]) {
                return;
            }
            renderItem(line);
        });
        if (!messagesEl.querySelector('.msg')) clearMessages(true);
    }

    function displayTitle(c) {
        if (!c) return 'New chat';
        if (c.title && String(c.title).trim()) return c.title;
        return 'New chat';
    }

    function relTime(ts) {
        if (!ts) return '';
        const ms = Number(ts) > 1e12 ? Number(ts) : Number(ts) * 1000;
        const diff = Date.now() - ms;
        const h = Math.floor(diff / 3600000);
        if (h < 1) return 'just now';
        if (h < 24) return h + 'h';
        return Math.floor(h / 24) + 'd';
    }

    function updateRoomHead(meta) {
        if (!roomTitleEl) return;
        roomTitleEl.textContent = displayTitle(meta || { title: null });
        if (roomMetaEl) {
            const src = (meta && meta.title_source) || 'pending';
            roomMetaEl.textContent = src === 'manual' ? 'Manual title' : (src === 'auto' ? 'Auto title' : 'Naming…');
        }
    }

    function sourceLabel(source) {
        if (source === 'manual') return 'Renamed';
        if (source === 'auto') return 'Auto';
        if (source === 'stale') return 'Stale';
        return 'Naming…';
    }

    function renderConversationList() {
        if (!listEl) return;
        if (conversationCountEl) conversationCountEl.textContent = String(conversations.length);
        listEl.innerHTML = '';
        conversations.forEach(function (c) {
            const li = document.createElement('li');
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'wc-conv' + (c.thread_id === threadId ? ' is-active' : '');
            const src = c.title_source || 'pending';
            const pending = !c.title;
            btn.innerHTML =
                '<span class="wc-conv__title' + (pending ? ' is-pending' : '') + '">' +
                escapeHtml(displayTitle(c)) + '</span>' +
                '<span class="wc-pill wc-pill--' + escapeHtml(src) + '">' + escapeHtml(sourceLabel(src)) + '</span>' +
                '<span class="wc-conv__meta">' +
                '<span class="wc-conv__by">#' + escapeHtml(String(c.created_by_admin_user_id || c.admin_user_id || '?')) + '</span>' +
                '<span>' + escapeHtml(relTime(c.last_activity_at || c.updated_at)) + '</span>' +
                '</span>';
            btn.addEventListener('click', function () {
                openConversation(c.thread_id);
            });
            li.appendChild(btn);
            listEl.appendChild(li);
        });
    }

    async function refreshConversationList() {
        try {
            const list = await listConversations(api, {});
            if (list && list.docs_index) {
                applyDocsIndexGate(list.docs_index);
            }
            if (!hasRail) return;
            conversations = (list && list.conversations) || [];
            renderConversationList();
            const active = conversations.find(function (c) { return c.thread_id === threadId; });
            if (active) updateRoomHead(active);
        } catch (e) {
            // ignore
        }
    }

    function onTurnFailed(data) {
        let detail = '';
        let code = '';
        let failedTurn = turnId;
        try {
            code = (data && data.error && data.error.code) ? String(data.error.code) : '';
            detail = (data && data.error && data.error.message) ? String(data.error.message) : '';
            if (data && data.turn_id) failedTurn = data.turn_id;
        } catch (err) {
            detail = '';
        }
        if (code === 'interrupted') {
            setStatus('Interrupted · floor tetap Anda');
            showToast('Stop · floor tidak dilepas');
        } else {
            setStatus('Failed · bisa Retry');
            appendError(messagesEl, detail || 'error', failedTurn, true);
        }
        busy = false;
        isInitiator = false;
        pendingUserText = null;
        pendingOptimisticEl = null;
        updateComposer();
        inputEl.focus();
    }

    function closeEvents() {
        if (sub) {
            sub.close();
            sub = null;
        }
    }

    function openEvents(id, afterSeq) {
        closeEvents();
        sub = subscribeEvents(api, {
            threadId: id,
            afterSeq: afterSeq || 0,
            onEvent: function (eventName, data) {
                if (eventName === 'item.completed') {
                    renderItem(data && data.item);
                    return;
                }
                if (eventName === 'turn.started') {
                    setStatus('Thinking…');
                    return;
                }
                if (eventName === 'turn.resumed') {
                    let tid = turnId;
                    if (data && data.turn_id) tid = data.turn_id;
                    clearTurnErrors(tid);
                    busy = true;
                    isInitiator = true;
                    setStatus('Retrying…');
                    updateComposer();
                    return;
                }
                if (eventName === 'turn.completed') {
                    setStatus('Ready');
                    busy = false;
                    isInitiator = false;
                    pendingUserText = null;
                    pendingOptimisticEl = null;
                    updateComposer();
                    inputEl.focus();
                    closeEvents();
                    refreshConversationList();
                    return;
                }
                if (eventName === 'turn.failed') {
                    onTurnFailed(data || {});
                    closeEvents();
                    refreshConversationList();
                }
            },
            onError: function () {
                if (busy) {
                    busy = false;
                    isInitiator = false;
                    updateComposer();
                }
            },
        });
    }

    async function openConversation(id) {
        closeEvents();
        try {
            const snap = await getThread(api, { threadId: id, afterSeq: 0 });
            threadId = id;
            localStorage.setItem(storageKey, id);
            applyFloorFromPayload(snap);
            hydrateItems(snap.items || []);
            setStatus('Hydrated ' + id);
            busy = false;
            isInitiator = false;
            updateComposer();
            await refreshConversationList();
            const active = conversations.find(function (c) { return c.thread_id === id; });
            updateRoomHead(active || { title: null, title_source: 'pending' });
        } catch (e) {
            localStorage.removeItem(storageKey);
            showToast('Thread unavailable');
        }
    }

    async function boot() {
        startFloorTicker();
        await refreshConversationList();
        const saved = localStorage.getItem(storageKey);
        if (saved) {
            try {
                await openConversation(saved);
                return;
            } catch (e) {
                localStorage.removeItem(storageKey);
            }
        }
        if (hasRail && conversations.length) {
            await openConversation(conversations[0].thread_id);
            return;
        }
        if (!hasRail) {
            try {
                const list = await listConversations(api, {});
                const rows = (list && list.conversations) || [];
                if (rows.length) {
                    await openConversation(rows[0].thread_id);
                    return;
                }
            } catch (e) {
                // empty
            }
        }
        threadId = null;
        clearMessages(true);
        updateRoomHead({ title: null, title_source: 'pending' });
        setStatus('Ready · kirim untuk lazy create');
        updateComposer();
        renderConversationList();
    }

    function newChat() {
        closeEvents();
        threadId = null;
        turnId = null;
        busy = false;
        isInitiator = false;
        localStorage.removeItem(storageKey);
        floorHolderId = null;
        floorRemainingSec = 0;
        pendingUserText = null;
        pendingOptimisticEl = null;
        clearMessages(true);
        updateRoomHead({ title: null, title_source: 'pending' });
        setStatus('New · lazy (create on send)');
        refreshFloorBanner();
        updateComposer();
        renderConversationList();
        inputEl.focus();
    }

    async function send() {
        const message = inputEl.value.trim();
        if (!message || busy) return;
        if (!docsIndexUsable) {
            showToast('503 docs index belum siap');
            return;
        }
        if (floorRemainingSec > 0 && floorHolderId !== adminUserId) {
            showToast('423 floor_locked · sisa ' + floorRemainingSec + 's');
            return;
        }

        busy = true;
        isInitiator = true;
        updateComposer();
        inputEl.value = '';
        pendingUserText = message;
        pendingOptimisticEl = appendUserMessage(messagesEl, message, adminDisplayName, true, null);

        try {
            if (!threadId) {
                const created = await createThread(api, {});
                threadId = created.thread_id;
                localStorage.setItem(storageKey, threadId);
                updateRoomHead({ title: null, title_source: 'pending' });
            }
            const started = await startTurn(api, { threadId: threadId, message: message });
            turnId = started.turn_id;
            applyFloorFromPayload(started);
            floorHolderId = adminUserId;
            floorRemainingSec = 0;
            refreshFloorBanner();
            setStatus('Queued');
            openEvents(threadId, started.seq_head || 0);
            refreshConversationList();
        } catch (err) {
            if (pendingOptimisticEl) {
                pendingOptimisticEl.remove();
                pendingOptimisticEl = null;
            }
            pendingUserText = null;
            if (err.status === 503) {
                if (err.body && err.body.docs_index) {
                    applyDocsIndexGate(err.body.docs_index);
                } else {
                    docsIndexUsable = false;
                    applyDocsIndexGate({
                        usable: false,
                        status: 'building',
                        message: 'Docs index sedang dibangun. AI sementara tidak tersedia.',
                    });
                }
                showToast('503 AI locked · indexing');
                setStatus('Indexing docs…');
            } else if (err.status === 429) {
                showToast('429 rate limited · retry ' + (err.retryAfter || '?') + 's');
                setStatus('429 rate limited');
            } else if (err.status === 423) {
                applyFloorFromPayload(err.body || {});
                floorRemainingSec = (err.body && err.body.remaining_sec) || floorRemainingSec;
                floorHolderId = (err.body && err.body.holder_admin_user_id) || floorHolderId;
                refreshFloorBanner();
                showToast('423 floor locked');
                setStatus('Floor locked');
            } else if (err.status === 409) {
                showToast('409 thread busy');
                setStatus('Busy');
            } else {
                setStatus('Failed: ' + (err.message || 'request error'));
                appendError(messagesEl, String(err.message || err));
            }
            busy = false;
            isInitiator = false;
            updateComposer();
        }
    }

    messagesEl.addEventListener('click', function (e) {
        const retryBtn = e.target.closest('[data-action="retry"]');
        if (retryBtn && messagesEl.contains(retryBtn)) {
            const tid = retryBtn.getAttribute('data-turn-id');
            if (!tid || busy) return;
            busy = true;
            isInitiator = true;
            updateComposer();
            setStatus('Retrying…');
            clearTurnErrors(tid);
            turnId = tid;
            retryTurn(api, { threadId: threadId, turnId: tid }).then(function (started) {
                applyFloorFromPayload(started);
                floorHolderId = adminUserId;
                floorRemainingSec = 0;
                refreshFloorBanner();
                openEvents(threadId, started.seq_head || 0);
            }).catch(function (err) {
                busy = false;
                isInitiator = false;
                updateComposer();
                if (err.status === 503) {
                    applyDocsIndexGate((err.body && err.body.docs_index) || { usable: false, status: 'building', message: 'Indexing…' });
                    showToast('503 docs index');
                } else if (err.status === 429) {
                    showToast('429 rate limited');
                } else if (err.status === 423) {
                    showToast('423 floor locked');
                } else if (err.status === 409) {
                    showToast(err.message || '409 not retryable / busy');
                } else {
                    showToast(err.message || 'Retry failed');
                    appendError(messagesEl, String(err.message || err), tid, true);
                }
                setStatus('Ready');
            });
            return;
        }
        const btn = e.target.closest('.disclosure__toggle');
        if (!btn || !messagesEl.contains(btn)) return;
        const panel = btn.closest('.disclosure');
        if (!panel) return;
        const open = !panel.classList.contains('is-open');
        panel.classList.toggle('is-open', open);
        btn.setAttribute('aria-expanded', open ? 'true' : 'false');
    });

    sendBtn.addEventListener('click', send);
    if (stopBtn) {
        stopBtn.addEventListener('click', function () {
            // Set before await: mock interrupt emits turn.failed synchronously,
            // so a then() status write would overwrite "Interrupted".
            setStatus('Stopping…');
            interruptTurn(api, { threadId: threadId, turnId: turnId }).catch(function (e) {
                showToast(e.message || 'Stop denied');
            });
        });
    }
    if (newBtn) newBtn.addEventListener('click', newChat);

    function setRenameError(message) {
        if (!renameError || !renameInput) return;
        renameError.textContent = message || '';
        renameError.hidden = !message;
        renameInput.setAttribute('aria-invalid', message ? 'true' : 'false');
        renameDialog.classList.toggle('has-error', !!message);
    }

    function updateRenameCount() {
        if (!renameInput || !renameCount) return;
        renameCount.textContent = renameInput.value.length + '/60';
    }

    function openRenameDialog() {
        if (!threadId || !renameDialog || !renameInput) {
            showToast('Kirim pesan dulu untuk membuat room');
            return;
        }
        renameReturnFocus = document.activeElement && document.activeElement !== document.body
            ? document.activeElement
            : renameBtn;
        const current = roomTitleEl ? roomTitleEl.textContent.trim() : '';
        renameInput.value = current === 'New chat' ? '' : current;
        updateRenameCount();
        setRenameError('');
        renameDialog.hidden = false;
        document.body.classList.add('has-wc-dialog');
        requestAnimationFrame(function () {
            renameDialog.classList.add('is-open');
            setTimeout(function () { renameInput.focus(); renameInput.select(); }, 120);
        });
    }

    function closeRenameDialog() {
        if (!renameDialog || renameDialog.hidden || (renameSubmit && renameSubmit.disabled)) return;
        renameDialog.classList.remove('is-open', 'has-error');
        document.body.classList.remove('has-wc-dialog');
        setTimeout(function () {
            renameDialog.hidden = true;
            if (renameReturnFocus && typeof renameReturnFocus.focus === 'function') renameReturnFocus.focus();
        }, 180);
    }

    if (renameBtn) renameBtn.addEventListener('click', openRenameDialog);
    if (renameDialog) {
        renameDialog.querySelectorAll('[data-rename-close]').forEach(function (button) {
            button.addEventListener('click', closeRenameDialog);
        });
        renameDialog.addEventListener('keydown', function (event) {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeRenameDialog();
                return;
            }
            if (event.key !== 'Tab' || !renamePanel) return;
            const focusable = Array.from(renamePanel.querySelectorAll('button:not([disabled]), input:not([disabled])'));
            if (!focusable.length) return;
            const first = focusable[0];
            const last = focusable[focusable.length - 1];
            if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
            else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
        });
    }
    if (renameInput) {
        renameInput.addEventListener('input', function () {
            updateRenameCount();
            if (renameInput.value.trim()) setRenameError('');
        });
    }
    if (renameForm) {
        renameForm.addEventListener('submit', function (event) {
            event.preventDefault();
            const title = renameInput ? renameInput.value.trim() : '';
            if (!title) {
                setRenameError('Enter a title before saving.');
                renameInput.focus();
                return;
            }
            if (title.length > 60) {
                setRenameError('Keep the title within 60 characters.');
                renameInput.focus();
                return;
            }
            if (roomTitleEl && title === roomTitleEl.textContent.trim()) {
                closeRenameDialog();
                return;
            }
            renameSubmit.disabled = true;
            renameSubmit.classList.add('is-loading');
            renameThread(api, { threadId: threadId, title: title }).then(function () {
                updateRoomHead({ title: title, title_source: 'manual' });
                refreshConversationList();
                renameSubmit.disabled = false;
                renameSubmit.classList.remove('is-loading');
                closeRenameDialog();
                showToast('Conversation title updated');
            }).catch(function (error) {
                renameSubmit.disabled = false;
                renameSubmit.classList.remove('is-loading');
                setRenameError(error.message || 'Could not rename this conversation.');
            });
        });
    }
    inputEl.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') send();
    });

    const apiSurface = {
        newChat: newChat,
        boot: boot,
        openConversation: openConversation,
    };
    window.webchatUi = apiSurface;
    boot();
    return apiSurface;
}

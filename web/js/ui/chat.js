import {
    api,
    listConversations,
    createThread,
    getThread,
    renameThread,
    deleteThread,
    startTurn,
    uploadAttachment,
    listModels,
    retryTurn,
    interruptTurn,
    subscribeEvents,
    browseDir,
    pagePreviewURL,
} from '../api/index.js';
import {
    escapeHtml,
    formatToolCall,
    summarizeToolResult,
    welcomeHtml,
    paintActionBubble,
    appendUserMessage,
    setUserMessageAttachments,
    appendError,
} from './render.js';
import { bootModelPicker } from './model-picker.js';
import { bootWorkspacePicker } from './workspace-picker.js';
import {
    durableSeq,
    isNearBottom,
    reconnectDelay,
} from './stream-reliability.js';
import { projectHistoricalItems } from './timeline-projection.js';
import { bootPagePreview } from './page-preview.js';

/**
 * Mount webchat UI. Pass { root } to scope lookups inside a widget host
 * (floating panel, embed). Omit root → document (full-page shell).
 * Required ids: chatMessages, chatInput, chatSend, chatStatus.
 * Optional rail/rename ids work when present.
 */
export function bootChat(options) {
    const opts = options || {};
    const root = opts.root || document;
    const byId = function (id) {
        if (!id) return null;
        if (root.nodeType === 9) return root.getElementById(id);
        return root.querySelector('#' + id);
    };

    const messagesEl = byId('chatMessages');
    const inputEl = byId('chatInput');
    const sendBtn = byId('chatSend');
    const stopBtn = byId('chatStop');
    const statusEl = byId('chatStatus');
    const newActivityBtn = byId('chatNewActivity');
    const floorEl = byId('chatFloor');
    const indexBannerEl = byId('chatIndexBanner');
    const newBtn = byId('btnNewChat') || byId('chatNew');
    const toastEl = byId('chatToast');
    const listEl = byId('conversationList');
    const conversationCountEl = byId('conversationCount');
    const conversationSearchEl = byId('conversationSearch');
    const roomTitleEl = byId('roomTitle');
    const roomMetaEl = byId('roomMeta');
    const previewPanelEl = byId('panelPreview');
    const renameBtn = byId('btnRename');
    const renameDialog = byId('renameDialog');
    const renamePanel = renameDialog ? renameDialog.querySelector('.wc-dialog__panel') : null;
    const renameForm = byId('renameForm');
    const renameInput = byId('renameInput');
    const renameCount = byId('renameCount');
    const renameError = byId('renameDialogError');
    const renameSubmit = byId('renameSubmit');
    let renameReturnFocus = null;
    let conversationQuery = '';

    const deleteDialog = byId('deleteDialog');
    const deleteSubmit = byId('deleteSubmit');
    let deleteReturnFocus = null;
    let pendingDeleteId = null;
    let pendingDeleteTitle = null;

    const attachBtn = byId('chatAttach');
    const attachChipsEl = byId('composerAttachments');
    const attachDialog = byId('attachDialog');
    const attachDropZone = byId('attachDropZone');
    const attachPickBtn = byId('attachPickBtn');
    const attachFileInput = byId('attachFileInput');
    let attachReturnFocus = null;
    /** @type {{ localId:string, file:File, name:string, size:number, mime:string, kind:string, previewUrl:string|null }[]} */
    let pendingAttachments = [];
    let pendingLocalSeq = 0;

    if (!messagesEl || !inputEl || !sendBtn || !statusEl) {
        return null;
    }

    const pagePreview = bootPagePreview({
        panelEl: previewPanelEl,
        previewURL: function (req) { return pagePreviewURL(api, req); },
    });

    const productName = opts.productName
        || (typeof window !== 'undefined' && window.__WC_PRODUCT_NAME__)
        || 'AI Assistant';
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
    /** Live agent_message drafts keyed by turn_id (ephemeral item.delta). */
    let liveDrafts = {};
    let turnsWithToolCalls = {};
    /** Assistant placeholders keyed by turn_id; `_pending` exists before StartTurn returns. */
    let assistantPlaceholders = {};
    let deltaFlushRaf = 0;
    /** Durable JSONL cursor per thread for duplicate suppression and resume. */
    let lastAppliedSeqByThread = {};
    let streamGeneration = 0;
    let streamState = 'idle';
    let reconnectAttempts = 0;
    let reconnectTimer = null;
    let lastStreamActivityAt = 0;
    let reconnectFailureShown = false;
    const reconnectBudget = 6;
    const staleStreamMs = 25000;

    function setStatus(text) {
        statusEl.textContent = text;
        const normalized = String(text || '').toLowerCase();
        let state = 'neutral';
        if (/ready|hydrated|completed/.test(normalized)) state = 'ready';
        else if (/failed|denied|locked|423|error/.test(normalized)) state = 'danger';
        else if (/indexing|streaming|thinking|busy|stopping|sending|queued|retrying|reconnecting|connecting/.test(normalized)) state = 'busy';
        statusEl.dataset.state = state;
    }

    function hideNewActivity() {
        if (newActivityBtn) newActivityBtn.hidden = true;
    }

    function scrollToLatest() {
        messagesEl.scrollTop = messagesEl.scrollHeight;
        hideNewActivity();
    }

    messagesEl.addEventListener('webchat:new-activity', function () {
        if (newActivityBtn) newActivityBtn.hidden = false;
    });
    messagesEl.addEventListener('scroll', function () {
        if (isNearBottom(messagesEl)) hideNewActivity();
    }, { passive: true });
    if (newActivityBtn) {
        newActivityBtn.addEventListener('click', scrollToLatest);
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

    const modelPicker = bootModelPicker({
        root: root,
        listModels: listModels,
        api: api,
        showToast: showToast,
    });

    const workspacePicker = bootWorkspacePicker({
        root: root,
        api: api,
        browseDir: browseDir,
        threadId: function () { return threadId; },
    });

    document.addEventListener('bp:models-changed', function () {
        modelPicker.refresh();
    });

    function updateComposer() {
        const floorBlocked = floorRemainingSec > 0 && floorHolderId !== adminUserId;
        const indexBlocked = !docsIndexUsable;
        const canCompose = !(busy || floorBlocked || indexBlocked);
        const hasPending = pendingAttachments.length > 0;
        sendBtn.disabled = !canCompose || (!inputEl.value.trim() && !hasPending);
        inputEl.disabled = !canCompose;
        if (attachBtn) attachBtn.disabled = !canCompose;
        if (stopBtn) {
            stopBtn.hidden = !busy;
            stopBtn.disabled = !(busy && isInitiator);
        }
        sendBtn.hidden = !!busy;
    }

    function formatBytes(n) {
        const size = Number(n) || 0;
        if (size < 1024) return size + ' B';
        if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KB';
        return (size / (1024 * 1024)).toFixed(1) + ' MB';
    }

    function inferKind(file) {
        const mime = String(file.type || '').toLowerCase();
        const name = String(file.name || '').toLowerCase();
        if (mime.indexOf('image/') === 0 || /\.(png|jpe?g|gif|webp)$/.test(name)) return 'image';
        return 'text';
    }

    function clearPendingAttachments() {
        pendingAttachments.forEach(function (a) {
            if (a.previewUrl) URL.revokeObjectURL(a.previewUrl);
        });
        pendingAttachments = [];
        renderAttachChips();
    }

    function renderAttachChips() {
        if (!attachChipsEl) return;
        attachChipsEl.innerHTML = '';
        if (!pendingAttachments.length) {
            attachChipsEl.hidden = true;
            updateComposer();
            return;
        }
        attachChipsEl.hidden = false;
        pendingAttachments.forEach(function (a) {
            const chip = document.createElement('div');
            chip.className = 'attach-chip';
            chip.dataset.localId = a.localId;
            let media = '';
            if (a.kind === 'image' && a.previewUrl) {
                media = '<img class="attach-chip__thumb" src="' + a.previewUrl + '" alt="">';
            } else {
                media = '<span class="attach-chip__icon" aria-hidden="true"><i class="bi bi-file-earmark-text"></i></span>';
            }
            chip.innerHTML =
                media +
                '<span class="attach-chip__meta">' +
                '<span class="attach-chip__name">' + escapeHtml(a.name) + '</span>' +
                '<span class="attach-chip__size">' + escapeHtml(formatBytes(a.size)) + '</span>' +
                '</span>' +
                '<button type="button" class="attach-chip__remove" data-remove-attach="' +
                escapeHtml(a.localId) + '" aria-label="Remove ' + escapeHtml(a.name) + '">' +
                '<i class="bi bi-x" aria-hidden="true"></i></button>';
            attachChipsEl.appendChild(chip);
        });
        updateComposer();
    }

    function addPendingFiles(fileList) {
        const files = Array.from(fileList || []);
        files.forEach(function (file) {
            if (!file || !file.size) return;
            if (file.size > 8 * 1024 * 1024) {
                showToast('File is too large (max 8 MB): ' + file.name);
                return;
            }
            const kind = inferKind(file);
            const localId = 'local_' + (++pendingLocalSeq);
            const previewUrl = kind === 'image' ? URL.createObjectURL(file) : null;
            pendingAttachments.push({
                localId: localId,
                file: file,
                name: file.name || 'upload.bin',
                size: file.size,
                mime: file.type || '',
                kind: kind,
                previewUrl: previewUrl,
            });
        });
        renderAttachChips();
    }

    function removePendingAttachment(localId) {
        const idx = pendingAttachments.findIndex(function (a) { return a.localId === localId; });
        if (idx < 0) return;
        const [removed] = pendingAttachments.splice(idx, 1);
        if (removed && removed.previewUrl) URL.revokeObjectURL(removed.previewUrl);
        renderAttachChips();
    }

    function openAttachDialog() {
        if (!attachDialog || (attachBtn && attachBtn.disabled)) return;
        attachReturnFocus = document.activeElement && document.activeElement !== document.body
            ? document.activeElement
            : attachBtn;
        attachDialog.hidden = false;
        document.body.classList.add('has-wc-dialog');
        requestAnimationFrame(function () {
            attachDialog.classList.add('is-open');
            if (attachDropZone) attachDropZone.focus();
        });
    }

    function closeAttachDialog() {
        if (!attachDialog || attachDialog.hidden) return;
        attachDialog.classList.remove('is-open');
        document.body.classList.remove('has-wc-dialog');
        setTimeout(function () {
            attachDialog.hidden = true;
            if (attachReturnFocus && typeof attachReturnFocus.focus === 'function') {
                attachReturnFocus.focus();
            }
        }, 160);
    }

    async function readFileAsText(file) {
        return new Promise(function (resolve) {
            const reader = new FileReader();
            reader.onload = function () { resolve(String(reader.result || '')); };
            reader.onerror = function () { resolve(''); };
            reader.readAsText(file);
        });
    }

    async function uploadPendingAttachments(targetThreadId) {
        const ids = [];
        for (let i = 0; i < pendingAttachments.length; i++) {
            const a = pendingAttachments[i];
            if (api.mockMode) {
                const content = a.kind === 'text' ? await readFileAsText(a.file) : '';
                const uploaded = await uploadAttachment(api, {
                    threadId: targetThreadId,
                    filename: a.name,
                    mime: a.mime,
                    size: a.size,
                    kind: a.kind,
                    content: content,
                });
                ids.push(uploaded.attachment_id);
            } else {
                const uploaded = await uploadAttachment(api, {
                    threadId: targetThreadId,
                    file: a.file,
                });
                ids.push(uploaded.attachment_id);
            }
        }
        return ids;
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
                    escapeHtml(gate.message || 'Docs index is not ready. AI is locked.');
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
                        showToast('Docs index ready · AI active');
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
            '<strong>Admin #' + floorHolderId + '</strong> holds the floor · ' +
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
        liveDrafts = {};
        turnsWithToolCalls = {};
        assistantPlaceholders = {};
        pagePreview.reset();
        if (deltaFlushRaf) {
            cancelAnimationFrame(deltaFlushRaf);
            deltaFlushRaf = 0;
        }
        messagesEl.innerHTML = welcome ? welcomeHtml(productName) : '';
        hideNewActivity();
    }

    function clearTurnErrors(id) {
        messagesEl.querySelectorAll('.msg--error').forEach(function (el) {
            if (!id || el.dataset.turnId === id) el.remove();
        });
    }

    /**
     * Per-turn action stream: one bubble per discrete phase.
     * Same kind while consecutive updates the current bubble;
     * kind change (think ↔ tools ↔ message) starts a new bubble.
     * agent_message always gets its own bubble.
     */
    function getTurnStream(id) {
        const key = id || '_anon';
        if (!turnUi[key]) {
            turnUi[key] = { kind: null, current: null, toolRows: {} };
        }
        return turnUi[key];
    }

    function startActionBubble(turnId, kind) {
        const key = turnId || '_anon';
        const stream = getTurnStream(key);
        const shouldFollow = isNearBottom(messagesEl);
        const welcome = messagesEl.querySelector('.chat-welcome');
        if (welcome) welcome.remove();
        const art = document.createElement('article');
        art.className = 'msg msg--assistant msg--' + kind;
        art.dataset.turnId = key;
        art.dataset.kind = kind;
        art.innerHTML = '<div class="msg__bubble"></div>';
        messagesEl.appendChild(art);
        const state = {
            art: art,
            kind: kind,
            thinkingSteps: [],
            thinkingModel: null,
            tools: [],
            responseModel: null,
            text: '',
            _followActivity: shouldFollow,
        };
        stream.kind = kind;
        stream.current = state;
        paintActionBubble(messagesEl, state);
        return state;
    }

    function removeAssistantPlaceholder(id) {
        const key = id || '';
        const placeholder = assistantPlaceholders[key];
        if (!placeholder) return;
        delete assistantPlaceholders[key];
        if (placeholder.art && placeholder.art.parentNode) placeholder.art.remove();
        const stream = turnUi[key || '_anon'];
        if (stream && stream.current === placeholder) {
            stream.current = null;
            stream.kind = null;
        }
    }

    function ensureAssistantPlaceholder(id) {
        const key = id || '_pending';
        if (assistantPlaceholders[key]) return assistantPlaceholders[key];
        const stream = getTurnStream(key);
        if (stream.current) return null;
        const state = startActionBubble(key, 'message');
        state.placeholder = true;
        assistantPlaceholders[key] = state;
        return state;
    }

    function adoptPendingPlaceholder(id) {
        if (!id || assistantPlaceholders[id]) return;
        const pending = assistantPlaceholders._pending;
        if (!pending) return;
        delete assistantPlaceholders._pending;
        delete turnUi._pending;
        pending.art.dataset.turnId = id;
        assistantPlaceholders[id] = pending;
        const stream = getTurnStream(id);
        stream.kind = 'message';
        stream.current = pending;
    }

    function takePlaceholderAsMessage(id) {
        const key = id || '';
        const placeholder = assistantPlaceholders[key];
        if (!placeholder) return null;
        delete assistantPlaceholders[key];
        placeholder.placeholder = false;
        placeholder.streaming = true;
        placeholder.text = '';
        const stream = getTurnStream(key);
        stream.kind = 'message';
        stream.current = placeholder;
        return placeholder;
    }

    function ensureActionBubble(turnId, kind) {
        const stream = getTurnStream(turnId);
        // Messages never merge — each agent_message is its own bubble.
        if (kind === 'message') {
            return startActionBubble(turnId, kind);
        }
        if (stream.current && stream.kind === kind) {
            return stream.current;
        }
        return startActionBubble(turnId, kind);
    }

    function discardLiveDraft(turnId) {
        const key = turnId || '';
        const draft = liveDrafts[key];
        if (!draft) return;
        delete liveDrafts[key];
        if (draft.art && draft.art.parentNode) {
            draft.art.remove();
        }
        const stream = turnUi[key || '_anon'];
        if (stream && stream.current === draft) {
            stream.current = null;
            stream.kind = null;
        }
    }

    function flushDeltaDrafts() {
        deltaFlushRaf = 0;
        Object.keys(liveDrafts).forEach(function (key) {
            const draft = liveDrafts[key];
            if (!draft || !draft._dirty) return;
            draft._dirty = false;
            paintActionBubble(messagesEl, draft);
        });
    }

    function applyTextDelta(data) {
        if (!data || data.field && data.field !== 'text') return;
        const turnId = data.turn_id || '';
        const delta = data.delta != null ? String(data.delta) : '';
        if (!delta || turnsWithToolCalls[turnId]) return;
        let draft = liveDrafts[turnId];
        if (!draft) {
            draft = takePlaceholderAsMessage(turnId) || ensureActionBubble(turnId, 'message');
            draft.streaming = true;
            draft.text = '';
            liveDrafts[turnId] = draft;
        }
        if (data.item_id && draft.art) {
            draft.art.dataset.id = String(data.item_id);
        }
        draft.text = (draft.text || '') + delta;

        // Weak models (e.g. openrouter/xiaomi/mimo-v2.5) sometimes stream raw
        // <tool_call> XML in output_text instead of using native function_call
        // arguments. Suppress that draft; the backend parses the XML and emits
        // the real tool_call item.
        const trimmed = draft.text.replace(/^\s+/, '');
        if (trimmed.indexOf('<tool_call') === 0 ||
            trimmed.indexOf('<function=') === 0 ||
            trimmed.indexOf('</function>') === 0 ||
            draft.text.indexOf('<tool_call') !== -1 ||
            draft.text.indexOf('<function=') !== -1 ||
            draft.text.indexOf('</function>') !== -1 ||
            draft.text.indexOf('</tool_call>') !== -1) {
            draft.isToolXML = true;
        }
        if (draft.isToolXML) {
            if (draft.text.indexOf('</tool_call>') !== -1) {
                if (draft.art && draft.art.parentNode) draft.art.remove();
                delete liveDrafts[turnId];
            } else if (draft.art && draft.art.parentNode) {
                draft.art.remove();
            }
            draft._dirty = false;
            return;
        }

        draft._dirty = true;
        if (!deltaFlushRaf) {
            deltaFlushRaf = requestAnimationFrame(flushDeltaDrafts);
        }
        setStatus('Streaming…');
    }

    function applyReasoning(item) {
        const text = String(item.text || '').trim();
        if (!text) return;
        // One JSONL reasoning item → one think bubble (phase change from tools/message).
        // Consecutive reasoning items without an intervening tool/message still share a bubble.
        // Do not discard live message drafts here: worker may append reasoning after text
        // deltas and before durable agent_message.
        removeAssistantPlaceholder(item.turn_id);
        const state = ensureActionBubble(item.turn_id, 'think');
        const chunks = text.split(/\n+/).map(function (s) { return s.trim(); }).filter(Boolean);
        state.thinkingSteps = state.thinkingSteps.concat(chunks.length ? chunks : [text]);
        state.thinkingModel = item.model || state.thinkingModel;
        if (item.id) state.art.dataset.id = item.id;
        const draft = liveDrafts[item.turn_id || ''];
        if (draft && draft.art && state.art && draft.art.parentNode === messagesEl) {
            messagesEl.insertBefore(state.art, draft.art);
        }
        paintActionBubble(messagesEl, state);
    }

    function applyToolCall(item) {
        pagePreview.observeToolCall(item);
        turnsWithToolCalls[item.turn_id || ''] = true;
        removeAssistantPlaceholder(item.turn_id);
        discardLiveDraft(item.turn_id);
        const state = ensureActionBubble(item.turn_id, 'tools');
        const stream = getTurnStream(item.turn_id);
        const callId = item.call_id || item.id || ('call_' + state.tools.length);
        const displayModel = item.model || (item.origin === 'host_preflight'
            ? { provider: productName, id: 'docs preflight' }
            : null);
        let row = stream.toolRows[callId];
        if (row) {
            row.call = formatToolCall(item.name, item.arguments || {});
            row.model = displayModel || row.model || null;
            row._state = state;
        } else {
            row = {
                callId: callId,
                call: formatToolCall(item.name, item.arguments || {}),
                result: '…',
                ok: true,
                model: displayModel,
                _state: state,
            };
            state.tools.push(row);
            stream.toolRows[callId] = row;
        }
        if (item.id) state.art.dataset.id = item.id;
        paintActionBubble(messagesEl, state);
    }

    function applyToolResult(item) {
        pagePreview.observeToolResult(item);
        removeAssistantPlaceholder(item.turn_id);
        const stream = getTurnStream(item.turn_id);
        const callId = item.call_id || '';
        const envelope = item.envelope || {};
        let row = callId ? stream.toolRows[callId] : null;
        if (!row) {
            const state = ensureActionBubble(item.turn_id, 'tools');
            row = {
                callId: callId || ('res_' + state.tools.length),
                call: 'tool',
                result: '—',
                ok: true,
                _state: state,
            };
            state.tools.push(row);
            if (row.callId) stream.toolRows[row.callId] = row;
        }
        row.ok = !!envelope.ok;
        row.result = summarizeToolResult(envelope);
        const paintTarget = row._state
            || (stream.current && stream.kind === 'tools' ? stream.current : null);
        if (paintTarget) paintActionBubble(messagesEl, paintTarget);
    }

    function applyAgentMessage(item, historical) {
        const text = item.text || '';
        const turnId = item.turn_id || '';
        if (item.origin === 'runtime' && text === '(empty model response)') {
            discardLiveDraft(turnId);
            appendError(
                messagesEl,
                historical
                    ? 'Previous attempt returned no answer. Retry to try again.'
                    : 'Model returned no answer (reasoning-only or truncated). Retry the turn.',
                turnId,
                true,
                historical ? '' : (item.trace_id || '')
            );
            return;
        }
        const draft = liveDrafts[turnId];
        if (draft) {
            delete liveDrafts[turnId];
            draft.streaming = false;
            draft._dirty = false;
            draft.text = text;
            draft.responseModel = item.model || null;
            if (item.id) draft.art.dataset.id = item.id;
            // Keep stream.current pointing at this durable bubble.
            const stream = getTurnStream(turnId);
            stream.kind = 'message';
            stream.current = draft;
            paintActionBubble(messagesEl, draft);
            return;
        }
        const state = takePlaceholderAsMessage(turnId) || ensureActionBubble(turnId, 'message');
        state.streaming = false;
        state.text = text;
        state.responseModel = item.model || null;
        if (item.id) state.art.dataset.id = item.id;
        paintActionBubble(messagesEl, state);
    }

    function renderItem(item, historical) {
        if (!item) return;
        const id = item.id || '';
        if (id && seenItemIds[id]) return;
        if (id) seenItemIds[id] = true;

        const type = item.type || '';
        if (type === 'user_message') {
            const text = item.text || '';
            const atts = Array.isArray(item.attachments) ? item.attachments : [];
            if (pendingUserText !== null && text === pendingUserText && pendingOptimisticEl) {
                pendingOptimisticEl.dataset.id = id;
                if (atts.length) {
                    setUserMessageAttachments(pendingOptimisticEl, atts);
                }
                if (id) seenItemIds[id] = true;
                pendingUserText = null;
                pendingOptimisticEl = null;
                return;
            }
            const aid = Number(item.admin_user_id || 0);
            const mine = aid === adminUserId || (aid === 0 && text && pendingUserText === text);
            const who = item.admin_display_name
                || (aid ? ('Admin #' + aid) : adminDisplayName);
            appendUserMessage(messagesEl, text, who, mine, id, atts);
        } else if (type === 'reasoning') {
            applyReasoning(item);
        } else if (type === 'tool_call') {
            applyToolCall(item);
        } else if (type === 'tool_result') {
            applyToolResult(item);
        } else if (type === 'agent_message') {
            applyAgentMessage(item, historical);
        } else if (type === 'turn.failed' && item.error) {
            const code = item.error.code || '';
            if (code !== 'interrupted') {
                appendError(
                    messagesEl,
                    historical
                        ? 'Previous attempt failed. Retry to try again.'
                        : (item.error.message || code || 'error'),
                    item.turn_id || '',
                    true,
                    historical ? '' : (item.trace_id || item.error.trace_id || '')
                );
            }
        } else if (type === 'turn.resumed') {
            clearTurnErrors(item.turn_id || '');
        }
    }

    function hydrateItems(items) {
        clearMessages(false);
        projectHistoricalItems(items).forEach(function (line) {
            renderItem(line, true);
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
        const q = conversationQuery.trim().toLowerCase();
        const visible = !q
            ? conversations
            : conversations.filter(function (c) {
                return displayTitle(c).toLowerCase().indexOf(q) !== -1;
            });
        visible.forEach(function (c) {
            const li = document.createElement('li');
            li.className = 'wc-conv-item';
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
            const delBtn = document.createElement('button');
            delBtn.type = 'button';
            delBtn.className = 'wc-conv__del';
            delBtn.title = 'Delete conversation';
            delBtn.setAttribute('aria-label', 'Delete conversation');
            delBtn.innerHTML = '<i class="bi bi-trash3"></i>';
            delBtn.addEventListener('click', function (ev) {
                ev.stopPropagation();
                handleDeleteConversation(c.thread_id, displayTitle(c));
            });
            li.appendChild(delBtn);
            listEl.appendChild(li);
        });
    }

    async function handleDeleteConversation(id, title) {
        pendingDeleteId = id;
        pendingDeleteTitle = title || 'this conversation';
        deleteReturnFocus = document.activeElement;
        openDeleteDialog();
    }

    function openDeleteDialog() {
        if (!deleteDialog) {
            confirmDeleteConversation();
            return;
        }
        var label = pendingDeleteTitle || 'this conversation';
        if (label.length > 40) label = label.slice(0, 37) + '…';
        var helpEl = byId('deleteDialogHelp');
        if (helpEl) helpEl.textContent = 'Delete "' + label + '"? This conversation and all its messages will be permanently removed. This action cannot be undone.';
        deleteDialog.hidden = false;
        document.body.classList.add('has-wc-dialog');
        requestAnimationFrame(function () {
            deleteDialog.classList.add('is-open');
            if (deleteSubmit) deleteSubmit.focus();
        });
    }

    function closeDeleteDialog() {
        if (!deleteDialog || deleteDialog.hidden) return;
        deleteDialog.classList.remove('is-open');
        document.body.classList.remove('has-wc-dialog');
        setTimeout(function () {
            deleteDialog.hidden = true;
            if (deleteReturnFocus && typeof deleteReturnFocus.focus === 'function') deleteReturnFocus.focus();
        }, 180);
    }

    async function confirmDeleteConversation() {
        var id = pendingDeleteId;
        if (!id) return;
        if (deleteSubmit) {
            deleteSubmit.disabled = true;
            deleteSubmit.classList.add('is-loading');
        }
        try {
            await deleteThread(api, { threadId: id });
        } catch (e) {
            if (deleteSubmit) {
                deleteSubmit.disabled = false;
                deleteSubmit.classList.remove('is-loading');
            }
            showToast('Delete failed: ' + (e && e.message ? e.message : 'error'));
            return;
        }
        if (deleteSubmit) {
            deleteSubmit.disabled = false;
            deleteSubmit.classList.remove('is-loading');
        }
        closeDeleteDialog();
        if (id === threadId) {
            closeEvents();
            threadId = null;
            turnId = null;
            busy = false;
            isInitiator = false;
            localStorage.removeItem(storageKey);
            clearPendingAttachments();
            clearMessages(true);
            updateRoomHead({ title: null, title_source: 'pending' });
            setStatus('Deleted');
        }
        await refreshConversationList();
        if (id === threadId || !threadId) {
            if (hasRail && conversations.length) {
                openConversation(conversations[0].thread_id);
            } else {
                newChat();
            }
        }
        showToast('Conversation deleted');
        pendingDeleteId = null;
        pendingDeleteTitle = null;
    }

    if (conversationSearchEl) {
        conversationSearchEl.addEventListener('input', function () {
            conversationQuery = conversationSearchEl.value || '';
            renderConversationList();
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

    let titleRefreshTimers = [];
    function clearTitleRefreshPoll() {
        titleRefreshTimers.forEach(function (id) { clearTimeout(id); });
        titleRefreshTimers = [];
    }
    function scheduleTitleRefreshPoll() {
        clearTitleRefreshPoll();
        const active = conversations.find(function (c) { return c.thread_id === threadId; });
        const pending = !active || !active.title || active.title_source === 'pending';
        if (!pending) return;
        [1200, 3500].forEach(function (ms) {
            titleRefreshTimers.push(setTimeout(function () {
                refreshConversationList();
            }, ms));
        });
    }

    function onTurnFailed(data) {
        let detail = '';
        let code = '';
        let failedTurn = turnId;
        let trace = '';
        try {
            code = (data && data.error && data.error.code) ? String(data.error.code) : '';
            detail = (data && data.error && data.error.message) ? String(data.error.message) : '';
            if (data && data.turn_id) failedTurn = data.turn_id;
            trace = (data && (data.trace_id || (data.error && data.error.trace_id)))
                ? String(data.trace_id || data.error.trace_id)
                : '';
        } catch (err) {
            detail = '';
        }
        removeAssistantPlaceholder(failedTurn);
        if (code === 'interrupted') {
            setStatus('Interrupted · your floor is retained');
            showToast('Stopped · floor retained');
        } else {
            setStatus('Failed · bisa Retry');
            appendError(messagesEl, detail || 'error', failedTurn, true, trace);
        }
        busy = false;
        isInitiator = false;
        pendingUserText = null;
        pendingOptimisticEl = null;
        updateComposer();
        inputEl.focus();
    }

    // Live SSE often delivers several item.completed frames in one EventSource
    // turn (poll burst or rapid worker appends). Queue + one paint per rAF so
    // think / tools / message bubbles appear progressively instead of clumping.
    let liveEventQueue = [];
    let liveFlushRaf = 0;

    function clearLiveEventQueue() {
        liveEventQueue = [];
        if (liveFlushRaf) {
            cancelAnimationFrame(liveFlushRaf);
            liveFlushRaf = 0;
        }
    }

    function dispatchLiveEvent(eventName, data) {
        if (eventName === 'item.delta') {
            applyTextDelta(data || {});
            return;
        }
        if (eventName === 'item.completed') {
            renderItem(data && data.item);
            return;
        }
        if (eventName === 'turn.started') {
            const startedTurnId = (data && data.turn_id) || turnId || '';
            if (startedTurnId) ensureAssistantPlaceholder(startedTurnId);
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
            if (data && data.turn_id) {
                removeAssistantPlaceholder(data.turn_id);
                discardLiveDraft(data.turn_id);
            }
            setStatus('Ready');
            busy = false;
            isInitiator = false;
            pendingUserText = null;
            pendingOptimisticEl = null;
            updateComposer();
            inputEl.focus();
            // Stop the socket but keep draining any already-queued paints.
            stopActiveStream();
            refreshConversationList().then(function () {
                // Async LLM auto-title may land after turn.completed.
                scheduleTitleRefreshPoll();
            });
            return;
        }
        if (eventName === 'conversation.updated') {
            refreshConversationList();
            return;
        }
        if (eventName === 'turn.failed') {
            if (data && data.turn_id) discardLiveDraft(data.turn_id);
            onTurnFailed(data || {});
            stopActiveStream();
            refreshConversationList();
        }
    }

    function flushLiveEventQueue() {
        liveFlushRaf = 0;
        const next = liveEventQueue.shift();
        if (!next) return;
        dispatchLiveEvent(next.eventName, next.data);
        if (liveEventQueue.length) {
            liveFlushRaf = requestAnimationFrame(flushLiveEventQueue);
        }
    }

    function enqueueLiveEvent(eventName, data) {
        liveEventQueue.push({ eventName: eventName, data: data });
        if (!liveFlushRaf) {
            liveFlushRaf = requestAnimationFrame(flushLiveEventQueue);
        }
    }

    function closeEventSource() {
        if (sub) {
            sub.close();
            sub = null;
        }
    }

    function cancelReconnect() {
        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
    }

    function stopActiveStream() {
        cancelReconnect();
        closeEventSource();
        streamGeneration += 1;
        streamState = 'idle';
        reconnectAttempts = 0;
    }

    function closeEvents() {
        clearLiveEventQueue();
        stopActiveStream();
    }

    function streamCursor(id) {
        return Number(lastAppliedSeqByThread[id] || 0);
    }

    function acceptDurableEvent(id, data) {
        const seq = durableSeq(data);
        if (!seq) return true;
        if (seq <= streamCursor(id)) return false;
        lastAppliedSeqByThread[id] = seq;
        return true;
    }

    function showReconnectFailure(id) {
        if (reconnectFailureShown) return;
        reconnectFailureShown = true;
        streamState = 'exhausted';
        const cursor = streamCursor(id);
        const trace = 'sse:' + id + ':seq:' + cursor;
        removeAssistantPlaceholder(turnId);
        setStatus('Reconnect failed · seq ' + cursor);
        appendError(
            messagesEl,
            'Realtime connection lost after ' + reconnectBudget + ' retries. Reopen this conversation to resume.',
            turnId || '',
            false,
            trace
        );
    }

    function scheduleReconnect(id, generation, immediate) {
        if (!busy || id !== threadId || generation !== streamGeneration) return;
        closeEventSource();
        cancelReconnect();
        if (reconnectAttempts >= reconnectBudget) {
            showReconnectFailure(id);
            return;
        }
        streamState = 'reconnecting';
        setStatus('Reconnecting…');
        const delay = immediate ? 0 : reconnectDelay(reconnectAttempts);
        reconnectAttempts += 1;
        reconnectTimer = setTimeout(function () {
            reconnectTimer = null;
            startSubscription(id, generation);
        }, delay);
    }

    function startSubscription(id, generation) {
        if (!busy || id !== threadId || generation !== streamGeneration) return;
        closeEventSource();
        streamState = reconnectAttempts ? 'reconnecting' : 'connecting';
        const nextSub = subscribeEvents(api, {
            threadId: id,
            afterSeq: streamCursor(id),
            onOpen: function () {
                if (id !== threadId || generation !== streamGeneration) return;
                streamState = 'open';
                lastStreamActivityAt = Date.now();
                setStatus(liveDrafts[turnId || ''] ? 'Streaming…' : 'Thinking…');
            },
            onEvent: function (eventName, data) {
                if (id !== threadId || generation !== streamGeneration) return;
                lastStreamActivityAt = Date.now();
                reconnectAttempts = 0;
                reconnectFailureShown = false;
                if (!acceptDurableEvent(id, data)) return;
                enqueueLiveEvent(eventName, data);
            },
            onError: function () {
                if (id !== threadId || generation !== streamGeneration || !busy) return;
                scheduleReconnect(id, generation, false);
            },
        });
        if (generation !== streamGeneration || id !== threadId || !busy) {
            nextSub.close();
            return;
        }
        sub = nextSub;
    }

    function openEvents(id, afterSeq) {
        closeEvents();
        lastAppliedSeqByThread[id] = Math.max(streamCursor(id), Number(afterSeq || 0));
        reconnectAttempts = 0;
        reconnectFailureShown = false;
        const generation = streamGeneration;
        startSubscription(id, generation);
    }

    document.addEventListener('visibilitychange', function () {
        if (document.visibilityState !== 'visible' || !busy || !threadId) return;
        const stale = !lastStreamActivityAt || (Date.now() - lastStreamActivityAt) > staleStreamMs;
        if (streamState !== 'open' || stale) {
            reconnectAttempts = 0;
            reconnectFailureShown = false;
            scheduleReconnect(threadId, streamGeneration, true);
        }
    });

    async function openConversation(id) {
        closeEvents();
        const requestGeneration = streamGeneration;
        try {
            const snap = await getThread(api, { threadId: id, afterSeq: 0 });
            if (requestGeneration !== streamGeneration) return;
            threadId = id;
            localStorage.setItem(storageKey, id);
            workspacePicker.reload();
            clearPendingAttachments();
            applyFloorFromPayload(snap);
            hydrateItems(snap.items || []);
            lastAppliedSeqByThread[id] = Math.max(
                Number(snap.seq_head || 0),
                (snap.items || []).reduce(function (max, item) {
                    return Math.max(max, Number(item && item.seq || 0));
                }, 0)
            );
            busy = !!snap.busy;
            turnId = snap.active_turn_id || null;
            isInitiator = busy
                && Number(snap.active_turn_initiator_admin_id || 0) === adminUserId;
            if (busy && turnId) {
                const stream = turnUi[turnId];
                if (!stream || !stream.current) ensureAssistantPlaceholder(turnId);
                setStatus('Connecting…');
                openEvents(id, streamCursor(id));
            } else {
                setStatus('Hydrated ' + id);
            }
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
        setStatus('Ready');
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
        clearPendingAttachments();
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
        const hasAttach = pendingAttachments.length > 0;
        if ((!message && !hasAttach) || busy) return;
        if (!docsIndexUsable) {
            showToast('503 docs index is not ready');
            return;
        }
        if (floorRemainingSec > 0 && floorHolderId !== adminUserId) {
            showToast('423 floor_locked · ' + floorRemainingSec + 's remaining');
            return;
        }

        busy = true;
        isInitiator = true;
        updateComposer();
        inputEl.value = '';
        pendingUserText = message || '(attached files)';
        const toUpload = pendingAttachments.slice();
        const optimisticAtts = toUpload.map(function (a) {
            return {
                id: a.localId,
                filename: a.name,
                kind: a.kind,
                size: a.size,
                previewUrl: a.previewUrl || null,
            };
        });
        pendingOptimisticEl = appendUserMessage(
            messagesEl,
            pendingUserText,
            adminDisplayName,
            true,
            null,
            optimisticAtts
        );
        ensureAssistantPlaceholder('_pending');

        try {
            if (!threadId) {
                const created = await createThread(api, {});
                threadId = created.thread_id;
                localStorage.setItem(storageKey, threadId);
                updateRoomHead({ title: null, title_source: 'pending' });
            }
            const attachmentIds = toUpload.length
                ? await uploadPendingAttachments(threadId)
                : [];
            clearPendingAttachments();
            const selection = modelPicker.getSelection();
            const ws = workspacePicker.getWorkspace();
            const started = await startTurn(api, {
                threadId: threadId,
                message: message,
                attachmentIds: attachmentIds,
                model: selection.model || undefined,
                effort: selection.effort || undefined,
                workspace: ws || undefined,
            });
            turnId = started.turn_id;
            adoptPendingPlaceholder(turnId);
            applyFloorFromPayload(started);
            floorHolderId = adminUserId;
            floorRemainingSec = 0;
            refreshFloorBanner();
            setStatus('Queued');
            openEvents(threadId, started.seq_head || 0);
            refreshConversationList();
        } catch (err) {
            removeAssistantPlaceholder('_pending');
            removeAssistantPlaceholder(turnId);
            if (pendingOptimisticEl) {
                pendingOptimisticEl.remove();
                pendingOptimisticEl = null;
            }
            pendingUserText = null;
            if (!pendingAttachments.length && toUpload.length) {
                pendingAttachments = toUpload;
                renderAttachChips();
            }
            if (err.status === 503) {
                if (err.body && err.body.docs_index) {
                    applyDocsIndexGate(err.body.docs_index);
                } else {
                    docsIndexUsable = false;
                    applyDocsIndexGate({
                        usable: false,
                        status: 'building',
                        message: 'The docs index is building. AI is temporarily unavailable.',
                    });
                }
                showToast('503 AI locked · indexing');
                setStatus('Indexing docs…');
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
                appendError(messagesEl, String(err.message || err), '', false, err.traceId);
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
            ensureAssistantPlaceholder(tid);
            var selection = modelPicker.getSelection();
            var ws = workspacePicker.getWorkspace();
            retryTurn(api, { threadId: threadId, turnId: tid, model: selection.model || undefined, effort: selection.effort || undefined, workspace: ws || undefined }).then(function (started) {
                applyFloorFromPayload(started);
                floorHolderId = adminUserId;
                floorRemainingSec = 0;
                refreshFloorBanner();
                openEvents(threadId, started.seq_head || 0);
            }).catch(function (err) {
                removeAssistantPlaceholder(tid);
                busy = false;
                isInitiator = false;
                updateComposer();
                if (err.status === 503) {
                    applyDocsIndexGate((err.body && err.body.docs_index) || { usable: false, status: 'building', message: 'Indexing…' });
                    showToast('503 docs index');
                } else if (err.status === 423) {
                    showToast('423 floor locked');
                } else if (err.status === 409) {
                    showToast(err.message || '409 not retryable / busy');
                } else {
                    showToast(err.message || 'Retry failed');
                    appendError(messagesEl, String(err.message || err), tid, true, err.traceId);
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
    inputEl.addEventListener('input', updateComposer);
    if (attachBtn) attachBtn.addEventListener('click', openAttachDialog);
    if (attachChipsEl) {
        attachChipsEl.addEventListener('click', function (e) {
            const btn = e.target.closest('[data-remove-attach]');
            if (!btn) return;
            removePendingAttachment(btn.getAttribute('data-remove-attach'));
        });
    }
    if (attachDialog) {
        attachDialog.querySelectorAll('[data-attach-close]').forEach(function (button) {
            button.addEventListener('click', closeAttachDialog);
        });
        attachDialog.addEventListener('keydown', function (event) {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeAttachDialog();
            }
        });
    }
    if (attachPickBtn && attachFileInput) {
        attachPickBtn.addEventListener('click', function () { attachFileInput.click(); });
        attachFileInput.addEventListener('change', function () {
            addPendingFiles(attachFileInput.files);
            attachFileInput.value = '';
            closeAttachDialog();
        });
    }
    if (attachDropZone) {
        ;['dragenter', 'dragover'].forEach(function (name) {
            attachDropZone.addEventListener(name, function (e) {
                e.preventDefault();
                e.stopPropagation();
                attachDropZone.classList.add('is-dragover');
            });
        });
        ;['dragleave', 'drop'].forEach(function (name) {
            attachDropZone.addEventListener(name, function (e) {
                e.preventDefault();
                e.stopPropagation();
                attachDropZone.classList.remove('is-dragover');
            });
        });
        attachDropZone.addEventListener('drop', function (e) {
            const dt = e.dataTransfer;
            if (dt && dt.files && dt.files.length) {
                addPendingFiles(dt.files);
                closeAttachDialog();
            }
        });
    }
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
            showToast('Send a message first to create a conversation');
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
    if (deleteDialog) {
        deleteDialog.querySelectorAll('[data-delete-close]').forEach(function (button) {
            button.addEventListener('click', closeDeleteDialog);
        });
        deleteDialog.addEventListener('keydown', function (event) {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeDeleteDialog();
            }
        });
    }
    if (deleteSubmit) {
        deleteSubmit.addEventListener('click', confirmDeleteConversation);
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

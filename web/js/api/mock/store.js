/** In-memory mock store for webchat dual-driver. */

function nowIso() {
    return new Date().toISOString();
}

function nowTs() {
    return Date.now();
}

function uid(prefix) {
    const rand = Math.random().toString(36).slice(2, 10);
    return prefix + '_' + Date.now().toString(36) + rand;
}

/**
 * @returns {import('./fixtures.js').MockState}
 */
export function createStore(fixtures) {
    /** @type {Map<string, {thread_id:string,seq_head:number,busy:boolean,items:Object[],title:string|null,title_source:string,created_by_admin_user_id:number,created_at:string,updated_at:number,last_activity_at:number,floor_holder_admin_id:number|null,floor_until:number,active_turn_id:string|null,active_turn_initiator_admin_id:number|null,deleted:boolean}>} */
    const threads = new Map();
    /** @type {Map<string, {timers:number[],subscribers:Set<Function>,interrupted:boolean}>} */
    const turns = new Map();
    /** @type {Map<string, Map<string, Object>>} threadId -> attachmentId -> meta+content */
    const attachments = new Map();

    let docsIndex = { ...fixtures.docsIndexReady };
    let rateWindow = [];

    function ApiError(status, code, message, extra) {
        const err = new Error(code || message || 'error');
        err.status = status;
        err.code = code;
        err.body = Object.assign({ error: code || message, code: code, message: message }, extra || {});
        return err;
    }

    function appendItem(thread, item) {
        thread.seq_head += 1;
        const line = Object.assign({}, item, {
            seq: thread.seq_head,
            id: item.id || uid('itm'),
            thread_id: thread.thread_id,
        });
        thread.items.push(line);
        thread.updated_at = nowTs();
        thread.last_activity_at = nowTs();
        return line;
    }

    function emitToSubscribers(threadId, eventName, data) {
        const thread = threads.get(threadId);
        if (!thread) return;
        turns.forEach(function (state) {
            // no-op marker; subscribers live per-turn key thr|after
        });
        const payload = Object.assign({ thread_id: threadId }, data || {});
        // Broadcast via per-thread subscriber registry on thread object
        if (!thread._subs) thread._subs = new Set();
        thread._subs.forEach(function (fn) {
            try { fn(eventName, payload); } catch (e) { /* ignore */ }
        });
    }

    function listConversations() {
        const conversations = Array.from(threads.values())
            .filter(function (t) { return !t.deleted; })
            .map(function (t) {
                const remaining = t.floor_until > Date.now()
                    ? Math.ceil((t.floor_until - Date.now()) / 1000)
                    : 0;
                return {
                    thread_id: t.thread_id,
                    title: t.title,
                    title_source: t.title_source,
                    status: 'active',
                    created_by_admin_user_id: t.created_by_admin_user_id,
                    updated_at: t.updated_at,
                    last_activity_at: t.last_activity_at,
                    floor_holder_admin_id: remaining > 0 ? t.floor_holder_admin_id : null,
                    floor_remaining_sec: remaining,
                };
            })
            .sort(function (a, b) { return b.last_activity_at - a.last_activity_at; });

        return {
            conversations: conversations,
            docs_index: { ...docsIndex },
        };
    }

    function createThread(adminUserId) {
        if (!docsIndex.usable) {
            throw ApiError(503, 'docs_index_not_ready', 'Docs index belum siap', {
                docs_index: { ...docsIndex },
            });
        }
        const threadId = uid('thr');
        const thread = {
            thread_id: threadId,
            seq_head: 0,
            busy: false,
            items: [],
            title: null,
            title_source: 'pending',
            created_by_admin_user_id: adminUserId || 1,
            created_at: nowIso(),
            updated_at: nowTs(),
            last_activity_at: nowTs(),
            floor_holder_admin_id: null,
            floor_until: 0,
            active_turn_id: null,
            active_turn_initiator_admin_id: null,
            deleted: false,
            _subs: new Set(),
        };
        const started = appendItem(thread, {
            type: 'thread.started',
            created_by_admin_user_id: thread.created_by_admin_user_id,
        });
        emitToSubscribers(threadId, 'thread.started', {
            type: 'thread.started',
            seq: started.seq,
            item: started,
        });
        threads.set(threadId, thread);
        return {
            thread_id: threadId,
            seq_head: thread.seq_head,
            created_by_admin_user_id: thread.created_by_admin_user_id,
            created_at: thread.created_at,
        };
    }

    function getThread(threadId) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        const remaining = thread.floor_until > Date.now()
            ? Math.ceil((thread.floor_until - Date.now()) / 1000)
            : 0;
        return {
            thread_id: thread.thread_id,
            seq_head: thread.seq_head,
            busy: !!thread.busy,
            floor_holder_admin_id: remaining > 0 ? thread.floor_holder_admin_id : null,
            floor_remaining_sec: remaining,
            active_turn_id: thread.active_turn_id,
            active_turn_initiator_admin_id: thread.active_turn_initiator_admin_id,
            items: thread.items.slice(),
        };
    }

    function renameThread(threadId, title) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        const clean = String(title || '').trim().slice(0, 60);
        if (!clean) throw ApiError(422, 'validation', 'Title required');
        thread.title = clean;
        thread.title_source = 'manual';
        thread.updated_at = nowTs();
        return { thread_id: threadId, title: clean };
    }

    function deleteThread(threadId) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        if (thread.busy) {
            throw ApiError(409, 'thread_busy', 'Thread busy');
        }
        thread.deleted = true;
        thread.items = [];
        thread.title = null;
        thread.title_source = 'pending';
        thread.floor_holder_admin_id = null;
        thread.floor_until = 0;
        thread.active_turn_id = null;
        thread.active_turn_initiator_admin_id = null;
        thread.updated_at = nowTs();
        attachments.delete(threadId);
        return null;
    }

    function assertCanSpeak(thread, adminUserId) {
        if (!docsIndex.usable) {
            throw ApiError(503, 'docs_index_not_ready', 'Docs index belum siap', {
                docs_index: { ...docsIndex },
            });
        }
        if (thread.busy) {
            throw ApiError(409, 'thread_busy', 'Thread busy');
        }
        const remaining = thread.floor_until > Date.now()
            ? Math.ceil((thread.floor_until - Date.now()) / 1000)
            : 0;
        if (remaining > 0 && thread.floor_holder_admin_id && thread.floor_holder_admin_id !== adminUserId) {
            throw ApiError(423, 'floor_locked', 'Speak floor locked', {
                holder_admin_user_id: thread.floor_holder_admin_id,
                remaining_sec: remaining,
            });
        }
        const now = Date.now();
        rateWindow = rateWindow.filter(function (t) { return now - t < 60000; });
        if (rateWindow.length >= 10) {
            const err = ApiError(429, 'rate_limited', 'Too many turns');
            err.retryAfter = '6';
            throw err;
        }
        rateWindow.push(now);
    }

    function clearTurnTimers(turnId) {
        const state = turns.get(turnId);
        if (!state) return;
        state.timers.forEach(function (id) { clearTimeout(id); });
        state.timers = [];
    }

    function schedule(turnId, delay, fn) {
        const state = turns.get(turnId);
        if (!state) return;
        const id = setTimeout(function () {
            if (state.interrupted) return;
            fn();
        }, delay);
        state.timers.push(id);
    }

    function pushEvent(thread, eventName, itemOrData) {
        const line = itemOrData.seq != null
            ? itemOrData
            : appendItem(thread, itemOrData);
        if (eventName === 'item.completed') {
            emitToSubscribers(thread.thread_id, 'item.completed', {
                type: 'item.completed',
                seq: line.seq,
                turn_id: line.turn_id,
                item: line,
            });
        } else {
            emitToSubscribers(thread.thread_id, eventName, Object.assign({
                type: eventName,
                seq: line.seq,
                turn_id: line.turn_id,
            }, typeof itemOrData === 'object' ? itemOrData : {}));
        }
        return line;
    }

    function runMockAgent(thread, turnId, adminUserId, userText, attachmentIds) {
        turns.set(turnId, { timers: [], interrupted: false });
        thread.busy = true;
        thread.active_turn_id = turnId;
        thread.active_turn_initiator_admin_id = adminUserId;
        thread.floor_holder_admin_id = adminUserId;
        thread.floor_until = Date.now() + 10 * 60 * 1000;

        pushEvent(thread, 'turn.started', {
            type: 'turn.started',
            turn_id: turnId,
        });

        const attIds = Array.isArray(attachmentIds) ? attachmentIds : [];
        const firstAtt = attIds.length
            ? (attachments.get(thread.thread_id) || new Map()).get(attIds[0])
            : null;

        schedule(turnId, 350, function () {
            pushEvent(thread, 'item.completed', {
                type: 'reasoning',
                turn_id: turnId,
                text: firstAtt
                    ? 'Membaca lampiran yang diunggah user.\nMenyusun jawaban singkat.'
                    : 'Mencari referensi relevan di knowledge base.\nMenyusun jawaban singkat.',
                model: { provider: 'mock', id: 'reasoner-sim' },
            });
        });

        const callId = uid('call');
        if (firstAtt) {
            const toolName = firstAtt.kind === 'image' ? 'read_image' : 'read_attachment';
            schedule(turnId, 700, function () {
                pushEvent(thread, 'item.completed', {
                    type: 'tool_call',
                    turn_id: turnId,
                    call_id: callId,
                    name: toolName,
                    arguments: { attachment_id: firstAtt.attachment_id },
                    model: { provider: 'mock', id: 'tool-router' },
                });
            });
            schedule(turnId, 1100, function () {
                const envelope = firstAtt.kind === 'image'
                    ? {
                        ok: true,
                        tool: 'read_image',
                        data: {
                            attachment_id: firstAtt.attachment_id,
                            filename: firstAtt.filename,
                            mime: firstAtt.mime,
                            kind: 'image',
                            size: firstAtt.size,
                            width: firstAtt.width || null,
                            height: firstAtt.height || null,
                            vision_available: true,
                            content_provided_to_model: true,
                            note: 'Mock: vision flagged available; real BE injects image bytes into the multimodal user message.',
                        },
                        error: null,
                        meta: { truncated: false, data_is_untrusted: true },
                    }
                    : {
                        ok: true,
                        tool: 'read_attachment',
                        data: {
                            attachment_id: firstAtt.attachment_id,
                            filename: firstAtt.filename,
                            mime: firstAtt.mime,
                            kind: 'text',
                            content: String(firstAtt.content || '').slice(0, 2000),
                        },
                        error: null,
                        meta: { truncated: false, data_is_untrusted: true },
                    };
                pushEvent(thread, 'item.completed', {
                    type: 'tool_result',
                    turn_id: turnId,
                    call_id: callId,
                    envelope: envelope,
                });
            });
        } else {
            schedule(turnId, 700, function () {
                pushEvent(thread, 'item.completed', {
                    type: 'tool_call',
                    turn_id: turnId,
                    call_id: callId,
                    name: 'search_docs',
                    arguments: { query: String(userText || '').slice(0, 80), top_k: 3 },
                    model: { provider: 'mock', id: 'tool-router' },
                });
            });
            schedule(turnId, 1100, function () {
                pushEvent(thread, 'item.completed', {
                    type: 'tool_result',
                    turn_id: turnId,
                    call_id: callId,
                    envelope: {
                        ok: true,
                        tool: 'search_docs',
                        data: {
                            chunks: [
                                { path: 'docs/getting-started.md', heading: 'Mulai', score: 0.92 },
                                { path: 'docs/writing-tips.md', heading: 'Tips menulis', score: 0.81 },
                            ],
                            count: 2,
                        },
                        error: null,
                        meta: { truncated: false, data_is_untrusted: true },
                    },
                });
            });
        }

        schedule(turnId, 1600, function () {
            const reply = firstAtt
                ? (
                    'Saya membaca lampiran **' + firstAtt.filename + '**.\n\n' +
                    (firstAtt.kind === 'image'
                        ? 'Ini gambar (' + (firstAtt.mime || 'image') + '). Vision belum aktif — hanya metadata yang tersedia.'
                        : 'Cuplikan isi:\n\n> ' + String(firstAtt.content || '').slice(0, 160).replace(/\n/g, ' ') + '…')
                )
                : (
                    'Berikut ringkasan mock untuk pertanyaan Anda:\n\n' +
                    '> **' + String(userText || '').slice(0, 120) + '**\n\n' +
                    '1. Tentukan sudut & audiens\n' +
                    '2. Susun outline singkat\n' +
                    '3. Tulis draft, lalu sunting\n\n' +
                    '_Sumber: search_docs (mock)_'
                );
            const draftId = uid('itm');
            // Fake token stream before durable item.completed (parity with real BE).
            const chunks = [];
            for (let i = 0; i < reply.length; i += 12) {
                chunks.push(reply.slice(i, i + 12));
            }
            chunks.forEach(function (chunk, idx) {
                schedule(turnId, 1600 + idx * 35, function () {
                    emitToSubscribers(thread.thread_id, 'item.delta', {
                        type: 'agent_message',
                        turn_id: turnId,
                        item_id: draftId,
                        field: 'text',
                        delta: chunk,
                    });
                });
            });
            schedule(turnId, 1600 + chunks.length * 35 + 40, function () {
                pushEvent(thread, 'item.completed', {
                    type: 'agent_message',
                    id: draftId,
                    turn_id: turnId,
                    text: reply,
                    model: { provider: 'mock', id: 'chat-sim' },
                });
                if (!thread.title) {
                    thread.title = String(userText || firstAtt && firstAtt.filename || 'Chat').trim().slice(0, 48) || 'New chat';
                    thread.title_source = 'auto';
                }
                schedule(turnId, 200, function () {
                    pushEvent(thread, 'turn.completed', {
                        type: 'turn.completed',
                        turn_id: turnId,
                        usage: { input_tokens: 120, output_tokens: 180 },
                        model: { provider: 'mock', id: 'chat-sim' },
                    });
                    thread.busy = false;
                    thread.active_turn_id = null;
                    thread.active_turn_initiator_admin_id = null;
                    clearTurnTimers(turnId);
                });
            });
        });
    }

    function uploadAttachment(threadId, fileMeta) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        const name = String((fileMeta && fileMeta.filename) || 'upload.bin');
        const mime = String((fileMeta && fileMeta.mime) || 'application/octet-stream');
        const size = Number((fileMeta && fileMeta.size) || 0);
        const kind = (fileMeta && fileMeta.kind) || (mime.indexOf('image/') === 0 ? 'image' : 'text');
        if (size > 8 * 1024 * 1024) {
            throw ApiError(422, 'validation', 'file too large');
        }
        const id = uid('att');
        const meta = {
            attachment_id: id,
            thread_id: threadId,
            filename: name,
            mime: mime,
            size: size,
            kind: kind,
            width: fileMeta && fileMeta.width,
            height: fileMeta && fileMeta.height,
            content: fileMeta && fileMeta.content,
            uploaded_at: nowIso(),
            uploaded_by_admin_user_id: fileMeta && fileMeta.adminUserId || 1,
        };
        if (!attachments.has(threadId)) attachments.set(threadId, new Map());
        attachments.get(threadId).set(id, meta);
        return {
            attachment_id: id,
            thread_id: threadId,
            filename: name,
            mime: mime,
            size: size,
            kind: kind,
            width: meta.width,
            height: meta.height,
            uploaded_at: meta.uploaded_at,
            uploaded_by_admin_user_id: meta.uploaded_by_admin_user_id,
        };
    }

    function listAttachments(threadId) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        const map = attachments.get(threadId) || new Map();
        return {
            attachments: Array.from(map.values()).map(function (m) {
                return {
                    attachment_id: m.attachment_id,
                    thread_id: m.thread_id,
                    filename: m.filename,
                    mime: m.mime,
                    size: m.size,
                    kind: m.kind,
                    width: m.width,
                    height: m.height,
                    uploaded_at: m.uploaded_at,
                    uploaded_by_admin_user_id: m.uploaded_by_admin_user_id,
                };
            }),
        };
    }

    const ALLOWED_MODELS = {
        'stub/default': true,
        'stub/reasoning': true,
        'stub/vision': true,
        STUB: true,
    };
    const ALLOWED_EFFORTS = {
        auto: true,
        none: true,
        minimal: true,
        low: true,
        medium: true,
        high: true,
        xhigh: true,
        max: true,
    };

    function startTurn(threadId, message, adminUserId, adminDisplayName, attachmentIds, opts) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        assertCanSpeak(thread, adminUserId);

        const text = String(message || '').trim();
        const attIds = Array.isArray(attachmentIds) ? attachmentIds.filter(Boolean) : [];
        if (!text && !attIds.length) throw ApiError(422, 'empty', 'Message empty');

        const overrides = opts || {};
        const model = String(overrides.model || '').trim();
        const effort = String(overrides.effort || '').trim().toLowerCase();
        if (model && !ALLOWED_MODELS[model]) {
            throw ApiError(422, 'validation', 'model not allowed');
        }
        if (effort && !ALLOWED_EFFORTS[effort]) {
            throw ApiError(422, 'validation', 'effort not allowed');
        }

        const attMap = attachments.get(threadId) || new Map();
        const refs = attIds.map(function (id) {
            const m = attMap.get(id);
            if (!m) throw ApiError(422, 'validation', 'unknown attachment_id: ' + id);
            return {
                id: m.attachment_id,
                attachment_id: m.attachment_id,
                filename: m.filename,
                mime: m.mime,
                size: m.size,
                kind: m.kind,
            };
        });

        const turnId = uid('trn');
        const userPayload = {
            type: 'user_message',
            turn_id: turnId,
            text: text || '(attached files)',
            admin_user_id: adminUserId,
            admin_display_name: adminDisplayName || ('Admin #' + adminUserId),
        };
        if (refs.length) userPayload.attachments = refs;
        if (model) userPayload.model_override = model;
        if (effort) userPayload.effort_override = effort;
        pushEvent(thread, 'item.completed', userPayload);

        setTimeout(function () {
            runMockAgent(thread, turnId, adminUserId, text || '(attached files)', attIds);
        }, 0);

        return {
            thread_id: threadId,
            turn_id: turnId,
            seq_head: thread.seq_head,
            status: 'queued',
            floor_holder_admin_id: adminUserId,
            floor_remaining_sec: 0,
        };
    }

    function retryTurn(threadId, turnId, adminUserId, model, effort) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        assertCanSpeak(thread, adminUserId);

        const failed = [...thread.items].reverse().find(function (it) {
            return it.type === 'turn.failed' && it.turn_id === turnId;
        });
        if (!failed || (failed.error && failed.error.code === 'interrupted')) {
            throw ApiError(409, 'not_retryable', 'Turn not retryable');
        }

        const userMsg = thread.items.find(function (it) {
            return it.type === 'user_message' && it.turn_id === turnId;
        });
        pushEvent(thread, 'turn.resumed', {
            type: 'turn.resumed',
            turn_id: turnId,
        });
        setTimeout(function () {
            runMockAgent(thread, turnId, adminUserId, (userMsg && userMsg.text) || 'retry');
        }, 0);

        return {
            thread_id: threadId,
            turn_id: turnId,
            seq_head: thread.seq_head,
            status: 'queued',
            floor_holder_admin_id: adminUserId,
            floor_remaining_sec: 0,
        };
    }

    function interruptTurn(threadId, turnId, adminUserId) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        if (!thread.busy || thread.active_turn_id !== turnId) {
            throw ApiError(409, 'not_busy', 'No active turn');
        }
        if (thread.active_turn_initiator_admin_id !== adminUserId) {
            throw ApiError(403, 'forbidden', 'Only initiator can interrupt');
        }
        const state = turns.get(turnId);
        if (state) {
            state.interrupted = true;
            clearTurnTimers(turnId);
        }
        pushEvent(thread, 'turn.failed', {
            type: 'turn.failed',
            turn_id: turnId,
            error: { code: 'interrupted', message: 'Interrupted by user' },
        });
        thread.busy = false;
        thread.active_turn_id = null;
        thread.active_turn_initiator_admin_id = null;
        return { ok: true, status: 'interrupt_requested' };
    }

    function subscribe(threadId, afterSeq, onEvent) {
        const thread = threads.get(threadId);
        if (!thread || thread.deleted) {
            throw ApiError(404, 'thread_not_found', 'Thread not found');
        }
        if (!thread._subs) thread._subs = new Set();

        // Replay backlog
        thread.items.forEach(function (line) {
            if (line.seq <= afterSeq) return;
            const type = line.type || '';
            if (type === 'turn.started' || type === 'turn.resumed' || type === 'turn.completed' || type === 'turn.failed') {
                onEvent(type, {
                    type: type,
                    seq: line.seq,
                    thread_id: threadId,
                    turn_id: line.turn_id,
                    error: line.error,
                    item: line,
                });
            } else if (type === 'thread.started') {
                onEvent('thread.started', {
                    type: 'thread.started',
                    seq: line.seq,
                    thread_id: threadId,
                    item: line,
                });
            } else {
                onEvent('item.completed', {
                    type: 'item.completed',
                    seq: line.seq,
                    thread_id: threadId,
                    turn_id: line.turn_id,
                    item: line,
                });
            }
        });

        const listener = function (eventName, data) {
            if (data && typeof data.seq === 'number' && data.seq <= afterSeq) return;
            onEvent(eventName, data);
        };
        thread._subs.add(listener);
        return {
            close: function () {
                thread._subs.delete(listener);
            },
        };
    }

    function setDocsIndex(gate) {
        docsIndex = { ...gate };
    }

    return {
        listConversations,
        createThread,
        getThread,
        renameThread,
        deleteThread,
        startTurn,
        uploadAttachment,
        listAttachments,
        retryTurn,
        interruptTurn,
        subscribe,
        setDocsIndex,
        ApiError,
    };
}

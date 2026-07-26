/** DOM rendering helpers for chat bubbles (markdown, disclosures, errors). */

import { isNearBottom } from './stream-reliability.js';

function finishActivity(messagesEl, shouldFollow) {
    if (shouldFollow) {
        messagesEl.scrollTop = messagesEl.scrollHeight;
        return;
    }
    messagesEl.dispatchEvent(new CustomEvent('webchat:new-activity'));
}

export function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text == null ? '' : String(text);
    return div.innerHTML;
}

export function formatMarkdown(text) {
    const source = String(text == null ? '' : text)
        .replace(/^[\u200B\u200C\u200D\u200E\u200F\uFEFF]+/, '');

    if (!window.marked || typeof window.marked.parse !== 'function' || !window.DOMPurify) {
        return escapeHtml(source).replace(/\n/g, '<br>');
    }

    const rendered = window.marked.parse(source, {
        gfm: true,
        breaks: true,
    });
    const clean = window.DOMPurify.sanitize(rendered, {
        USE_PROFILES: { html: true },
        FORBID_TAGS: ['style', 'form', 'button', 'textarea', 'select', 'option', 'iframe'],
        FORBID_ATTR: ['style'],
    });

    const template = document.createElement('template');
    template.innerHTML = clean;
    template.content.querySelectorAll('a[href]').forEach(function (link) {
        link.target = '_blank';
        link.rel = 'noopener noreferrer';
    });

    return template.innerHTML;
}

export function summarizeToolResult(envelope) {
    if (!envelope) return '—';
    if (!envelope.ok) {
        return (envelope.error && envelope.error.message) ? String(envelope.error.message) : 'fail';
    }
    const data = envelope.data || {};
    if (Array.isArray(data.chunks)) {
        const n = data.chunks.length;
        const trunc = envelope.meta && envelope.meta.truncated ? ' · truncated' : '';
        return n + ' hit' + (n === 1 ? '' : 's') + trunc;
    }
    if (Array.isArray(data.entries)) {
        const total = typeof data.total === 'number' ? data.total : data.entries.length;
        if (total === 0) {
            return 'empty (ls . ..)';
        }
        const names = data.entries.slice(0, 3).map(function (e) {
            const n = e && e.name != null ? String(e.name) : '';
            return e && e.type === 'directory' ? n + '/' : n;
        }).filter(Boolean);
        const more = total > names.length ? ' +' + (total - names.length) : '';
        return total + ' entr' + (total === 1 ? 'y' : 'ies') + (names.length ? ': ' + names.join(', ') + more : '');
    }
    if (typeof data.listing === 'string' && data.listing.trim()) {
        const lines = data.listing.trim().split('\n');
        return lines[0] + (lines.length > 1 ? ' · ' + (lines.length - 1) + ' lines' : '');
    }
    if (typeof data.count === 'number') return data.count + ' hits';
    if (envelope.meta && typeof envelope.meta.count === 'number') {
        return envelope.meta.count + ' items';
    }
    return 'ok';
}

export function formatToolCall(name, args) {
    const a = args && typeof args === 'object' ? args : {};
    if (a.query != null && String(a.query) !== '') {
        return String(name || 'tool') + '(' + JSON.stringify(String(a.query)) + ')';
    }
    if (name === 'list_dir' || name === 'read_file' || name === 'grep' ||
        name === 'write_file' || name === 'edit_file' || name === 'delete_file') {
        const path = a.path != null ? String(a.path) : '';
        return String(name || 'tool') + '(' + JSON.stringify(path) + ')';
    }
    return String(name || 'tool') + '()';
}

function modelLabel(model) {
    if (!model || !model.id) return '';
    return (model.provider ? String(model.provider) + ' · ' : '') + String(model.id);
}

export function modelBadge(model) {
    const label = modelLabel(model);
    return label ? '<span class="model-badge">' + escapeHtml(label) + '</span>' : '';
}

export function disclosureHtml(kind, block) {
    if (!block) return '';
    const isThink = kind === 'think';
    const steps = block.steps || [];
    const items = block.items || [];
    if (isThink && !steps.length) return '';
    if (!isThink && !items.length) return '';
    const open = !!block.open;
    const title = isThink ? 'Thinking' : 'Tool Calls';
    const summary = block.summary || (isThink
        ? (steps.length + ' step' + (steps.length === 1 ? '' : 's'))
        : (items.length + ' tool call' + (items.length === 1 ? '' : 's')));
    const cls = 'disclosure disclosure--' + (isThink ? 'think' : 'tools') + (open ? ' is-open' : '');
    let body = '';
    if (isThink) {
        body = '<ul class="think-list">' + steps.map(function (s) {
            return '<li>' + escapeHtml(s) + '</li>';
        }).join('') + '</ul>';
    } else {
        body = '<div class="tool-rows">' + items.map(function (row) {
            const badge = modelBadge(row.model);
            return (
                '<div class="tool-row">' +
                '<div class="tool-row__call">' + badge + (badge ? '<br/>' : '') + escapeHtml(row.call) + '</div>' +
                '<div class="tool-row__result' + (row.ok === false ? ' is-fail' : '') + '">' +
                escapeHtml(row.result) + '</div></div>'
            );
        }).join('') + '</div>';
    }
    return (
        '<div class="' + cls + '" data-kind="' + kind + '">' +
        '<button type="button" class="disclosure__toggle" aria-expanded="' + (open ? 'true' : 'false') + '">' +
        '<span class="disclosure__chevron" aria-hidden="true"><i class="bi bi-chevron-right"></i></span>' +
        '<span class="disclosure__title">' + title + '</span>' +
        modelBadge(block.model) +
        '</button>' +
        '<div class="disclosure__body">' + body + '</div></div>'
    );
}

export function welcomeHtml(productName) {
    const name = productName
        || (typeof window !== 'undefined' && window.__WC_PRODUCT_NAME__)
        || 'AI Assistant';
    return (
        '<div class="chat-welcome">' +
        '<div class="chat-welcome-icon"><i class="bi bi-chat-square-text"></i></div>' +
        '<h5>Welcome to ' + escapeHtml(String(name)) + '</h5>'
    );
}

/** Paint one action bubble (think | tools | message) — never merge kinds. */
export function paintActionBubble(messagesEl, state) {
    const bubble = state.art && state.art.querySelector('.msg__bubble');
    if (!bubble) return;
    const shouldFollow = state._followActivity == null
        ? isNearBottom(messagesEl)
        : !!state._followActivity;
    delete state._followActivity;

    let inner = '';
    if (state.kind === 'think') {
        const thinkSteps = state.thinkingSteps || [];
        const thinkPreview = thinkSteps.length
            ? (thinkSteps[0].length > 72 ? thinkSteps[0].slice(0, 69) + '…' : thinkSteps[0])
            : 'Reasoning hidden';
        inner = disclosureHtml('think', {
            open: thinkSteps.length > 0,
            model: state.thinkingModel,
            summary: thinkSteps.length
                ? (thinkSteps.length + ' step' + (thinkSteps.length === 1 ? '' : 's') + ' · ' + thinkPreview)
                : 'Reasoning hidden',
            steps: thinkSteps,
        });
    } else if (state.kind === 'tools') {
        const tools = state.tools || [];
        inner = disclosureHtml('tools', {
            open: true,
            summary: tools.length + ' tool call' + (tools.length === 1 ? '' : 's'),
            items: tools,
        });
    } else if (state.kind === 'message') {
        if (state.text) {
            const body = state.streaming
                ? escapeHtml(state.text).replace(/\n/g, '<br>')
                : formatMarkdown(state.text);
            inner = modelBadge(state.responseModel) + '<div class="msg__text">' + body + '</div>';
        } else {
            inner = '';
        }
    }

    bubble.innerHTML = inner || (
        '<div class="typing" role="status" aria-label="Thinking">' +
        '<span class="typing__label">Thinking…</span>' +
        '<span class="typing__dots" aria-hidden="true"><i></i><i></i><i></i></span>' +
        '</div>'
    );
    finishActivity(messagesEl, shouldFollow);
}

export function appendUserMessage(messagesEl, text, who, mine, id, attachments) {
    const shouldFollow = isNearBottom(messagesEl);
    const welcome = messagesEl.querySelector('.chat-welcome');
    if (welcome) welcome.remove();
    const art = document.createElement('article');
    art.className = 'msg msg--user ' + (mine ? 'is-mine' : 'is-peer');
    if (id) art.dataset.id = id;
    const bubbleBody = escapeHtml(text || '');
    const attachHtml = renderMessageAttachments(attachments);
    art.innerHTML =
        '<div class="msg__who">' + escapeHtml(who) + (mine ? ' · you' : '') + '</div>' +
        '<div class="msg__bubble">' +
        (bubbleBody ? '<div class="msg__text">' + bubbleBody + '</div>' : '') +
        attachHtml +
        '</div>';
    messagesEl.appendChild(art);
    finishActivity(messagesEl, shouldFollow);
    return art;
}

/** Paint attachment chips inside an existing user bubble (optimistic → SSE confirm). */
export function setUserMessageAttachments(art, attachments) {
    if (!art) return;
    const bubble = art.querySelector('.msg__bubble');
    if (!bubble) return;
    let host = bubble.querySelector('.msg__attachments');
    const html = renderMessageAttachments(attachments);
    if (!html) {
        if (host) host.remove();
        return;
    }
    if (host) {
        host.outerHTML = html;
    } else {
        bubble.insertAdjacentHTML('beforeend', html);
    }
}

function renderMessageAttachments(attachments) {
    const list = Array.isArray(attachments) ? attachments : [];
    if (!list.length) return '';
    const chips = list.map(function (a) {
        if (!a) return '';
        const name = a.filename || a.name || a.attachment_id || a.id || 'file';
        const kind = a.kind || '';
        const size = a.size != null ? formatBytes(a.size) : '';
        let media = '';
        if (kind === 'image' && a.previewUrl) {
            media = '<img class="attach-chip__thumb" src="' + escapeHtml(a.previewUrl) + '" alt="">';
        } else if (kind === 'image') {
            media = '<span class="attach-chip__icon" aria-hidden="true"><i class="bi bi-file-earmark-image"></i></span>';
        } else {
            media = '<span class="attach-chip__icon" aria-hidden="true"><i class="bi bi-file-earmark-text"></i></span>';
        }
        return (
            '<div class="attach-chip attach-chip--msg" title="' + escapeHtml(name) + '">' +
            media +
            '<span class="attach-chip__meta">' +
            '<span class="attach-chip__name">' + escapeHtml(name) + '</span>' +
            (size ? '<span class="attach-chip__size">' + escapeHtml(size) + '</span>' : '') +
            '</span></div>'
        );
    }).filter(Boolean).join('');
    if (!chips) return '';
    return '<div class="msg__attachments">' + chips + '</div>';
}

function formatBytes(n) {
    const num = Number(n) || 0;
    if (num < 1024) return num + ' B';
    if (num < 1024 * 1024) return (num / 1024).toFixed(1) + ' KB';
    return (num / (1024 * 1024)).toFixed(1) + ' MB';
}

export function appendError(messagesEl, detail, turnId, retryable, traceId) {
    const shouldFollow = isNearBottom(messagesEl);
    const welcome = messagesEl.querySelector('.chat-welcome');
    if (welcome) welcome.remove();
    const art = document.createElement('article');
    art.className = 'msg msg--assistant msg--error';
    if (turnId) art.dataset.turnId = turnId;
    let actions = '';
    if (retryable && turnId) {
        actions =
            '<div class="msg-error__actions">' +
            '<button type="button" class="btn-retry" data-action="retry" data-turn-id="' +
            escapeHtml(turnId) + '">' +
            '<i class="bi bi-arrow-clockwise"></i> Retry</button></div>';
    }
    const trace =
        traceId
            ? '<div class="msg-error__trace">trace: ' + escapeHtml(String(traceId)) + '</div>'
            : '';
    art.innerHTML =
        '<div class="msg__bubble"><div class="msg-error">' +
        '<div class="msg-error__head">' +
        '<i class="bi bi-exclamation-triangle-fill msg-error__icon"></i>' +
        '<div><div class="msg-error__title">Failed</div>' +
        '<div class="msg-error__detail">' + escapeHtml(detail || 'error') + '</div>' +
        trace +
        '</div></div>' +
        actions + '</div></div>';
    messagesEl.appendChild(art);
    finishActivity(messagesEl, shouldFollow);
    return art;
}

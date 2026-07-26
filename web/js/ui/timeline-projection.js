const EMPTY_MODEL_RESPONSE = '(empty model response)';
const HIDDEN_CONTROL_ITEMS = new Set([
    'thread.started',
    'turn.started',
    'turn.completed',
    'turn.resumed',
]);

function isEmptyModelResponse(item) {
    return item
        && item.type === 'agent_message'
        && String(item.text || '').trim() === EMPTY_MODEL_RESPONSE;
}

function isSuccessfulAgentMessage(item) {
    return item
        && item.type === 'agent_message'
        && String(item.text || '').trim() !== ''
        && !isEmptyModelResponse(item);
}

/**
 * Project durable transcript items into the historical timeline. Retry uses
 * the same turn_id, so terminal artifacts from an earlier attempt must not
 * reappear after hydration while useful reasoning and tool history remains.
 *
 * @param {Object[]} items
 * @returns {Object[]}
 */
export function projectHistoricalItems(items) {
    const lines = Array.isArray(items) ? items : [];
    const attemptByTurn = {};
    const annotated = lines.map(function (item, index) {
        const turnId = String((item && item.turn_id) || '');
        let attempt = turnId ? (attemptByTurn[turnId] || 0) : 0;
        if (turnId && item && item.type === 'turn.resumed') {
            attempt += 1;
            attemptByTurn[turnId] = attempt;
        }
        return { item: item, index: index, turnId: turnId, attempt: attempt };
    });

    const latestAttempt = Object.assign({}, attemptByTurn);
    const latestState = {};
    annotated.forEach(function (row) {
        if (!row.turnId || row.attempt !== (latestAttempt[row.turnId] || 0)) return;
        const state = latestState[row.turnId] || {
            failedIndex: -1,
            completedIndex: -1,
            successfulMessageIndex: -1,
        };
        const type = (row.item && row.item.type) || '';
        if (type === 'turn.failed') state.failedIndex = row.index;
        if (type === 'turn.completed') state.completedIndex = row.index;
        if (isSuccessfulAgentMessage(row.item)) state.successfulMessageIndex = row.index;
        latestState[row.turnId] = state;
    });

    return annotated.filter(function (row) {
        const item = row.item;
        const type = (item && item.type) || '';
        if (HIDDEN_CONTROL_ITEMS.has(type)) return false;
        if (!row.turnId) return true;

        const currentAttempt = latestAttempt[row.turnId] || 0;
        const state = latestState[row.turnId] || {};
        if (type === 'turn.failed') {
            if (row.attempt !== currentAttempt || row.index !== state.failedIndex) return false;
            return !(state.completedIndex > row.index || state.successfulMessageIndex > row.index);
        }
        if (isEmptyModelResponse(item)) {
            if (row.attempt !== currentAttempt) return false;
            return !(state.failedIndex > row.index || state.successfulMessageIndex > row.index);
        }
        return true;
    }).map(function (row) {
        return row.item;
    });
}

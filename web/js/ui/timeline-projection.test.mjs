import assert from 'node:assert/strict';
import test from 'node:test';

import { projectHistoricalItems } from './timeline-projection.js';

function visibleSummary(items) {
    return projectHistoricalItems(items).map(function (item) {
        return item.type + (item.text ? ':' + item.text : '');
    });
}

test('successful retry hides the previous empty-response error', function () {
    const items = [
        { type: 'user_message', turn_id: 'turn-1', text: 'hello' },
        { type: 'agent_message', turn_id: 'turn-1', text: '(empty model response)' },
        { type: 'turn.completed', turn_id: 'turn-1' },
        { type: 'turn.resumed', turn_id: 'turn-1' },
        { type: 'agent_message', turn_id: 'turn-1', text: 'Hello from retry' },
        { type: 'turn.completed', turn_id: 'turn-1' },
    ];

    assert.deepEqual(visibleSummary(items), [
        'user_message:hello',
        'agent_message:Hello from retry',
    ]);
});

test('failed retry shows only its latest failure', function () {
    const items = [
        { type: 'user_message', turn_id: 'turn-1', text: 'hello' },
        { type: 'agent_message', turn_id: 'turn-1', text: '(empty model response)' },
        { type: 'turn.completed', turn_id: 'turn-1' },
        { type: 'turn.resumed', turn_id: 'turn-1' },
        { type: 'turn.failed', turn_id: 'turn-1', error: { code: 'llm_error' } },
    ];

    assert.deepEqual(visibleSummary(items), [
        'user_message:hello',
        'turn.failed',
    ]);
});

test('empty response remains visible when it is the latest attempt result', function () {
    const items = [
        { type: 'user_message', turn_id: 'turn-1', text: 'hello' },
        { type: 'agent_message', turn_id: 'turn-1', text: '(empty model response)' },
        { type: 'turn.completed', turn_id: 'turn-1' },
    ];

    assert.deepEqual(visibleSummary(items), [
        'user_message:hello',
        'agent_message:(empty model response)',
    ]);
});

test('successful completion hides an earlier failure in the same attempt', function () {
    const items = [
        { type: 'user_message', turn_id: 'turn-1', text: 'hello' },
        { type: 'turn.failed', turn_id: 'turn-1', error: { code: 'llm_error' } },
        { type: 'agent_message', turn_id: 'turn-1', text: 'Recovered answer' },
        { type: 'turn.completed', turn_id: 'turn-1' },
    ];

    assert.deepEqual(visibleSummary(items), [
        'user_message:hello',
        'agent_message:Recovered answer',
    ]);
});

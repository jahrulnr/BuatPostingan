import assert from 'node:assert/strict';
import test from 'node:test';

import {
    durableSeq,
    isNearBottom,
    reconnectDelay,
} from './stream-reliability.js';

test('durableSeq reads only durable event cursors', function () {
    assert.equal(durableSeq({ seq: 12 }), 12);
    assert.equal(durableSeq({ item: { seq: 13 } }), 13);
    assert.equal(durableSeq({ type: 'item.delta', delta: 'hello' }), 0);
});

test('reconnectDelay applies capped exponential backoff and jitter', function () {
    assert.equal(reconnectDelay(0, function () { return 0.5; }), 400);
    assert.equal(reconnectDelay(3, function () { return 0.5; }), 3200);
    assert.equal(reconnectDelay(8, function () { return 0.5; }), 8000);
    assert.equal(reconnectDelay(1, function () { return 0; }), 600);
    assert.equal(reconnectDelay(1, function () { return 1; }), 1000);
});

test('isNearBottom tolerates a small reading gap', function () {
    assert.equal(isNearBottom({ scrollHeight: 1000, scrollTop: 550, clientHeight: 400 }), true);
    assert.equal(isNearBottom({ scrollHeight: 1000, scrollTop: 400, clientHeight: 400 }), false);
});

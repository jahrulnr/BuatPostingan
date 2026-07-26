import assert from 'node:assert/strict';
import test from 'node:test';

import { formatApiError } from './error.js';

test('formatApiError prefers code · message from body', function () {
    const err = new Error('upstream');
    err.body = { code: 'upstream', error: 'upstream', message: 'fetch models' };
    assert.equal(formatApiError(err), 'upstream · fetch models');
});

test('formatApiError falls back to message, then code, then Error.message', function () {
    assert.equal(formatApiError({ body: { message: 'only msg' } }), 'only msg');
    assert.equal(formatApiError({ body: { code: 'validation' } }), 'validation');
    assert.equal(formatApiError(new Error('HTTP 500'), 'fallback'), 'HTTP 500');
    assert.equal(formatApiError(null, 'fallback'), 'fallback');
});

import assert from 'node:assert/strict';
import test from 'node:test';

import { createSettingsStore } from './settings-store.js';

test('provider catalog exposes the six initial provider families', function () {
    const store = createSettingsStore();
    const catalog = store.listProviderCatalog().providers;
    assert.deepEqual(catalog.map(function (p) { return p.type; }), [
        'openrouter',
        'omniroute',
        '9router',
        'openai',
        'openai-compatible',
        'claude',
    ]);
    assert.equal(catalog.find(function (p) { return p.type === 'claude'; }).api, 'messages');
});

test('mock provider persistence keeps provider type and messages dialect', function () {
    const store = createSettingsStore();
    const created = store.createProvider({
        type: 'claude',
        id: 'CLAUDE',
        name: 'Claude API',
        api: 'messages',
        base_url: 'https://api.anthropic.com/v1',
        api_key: 'test-secret',
        model_id: 'claude-sonnet',
    });
    assert.equal(created.type, 'claude');
    assert.equal(created.api, 'messages');
    assert.equal(created.api_key_set, true);
    assert.notEqual(created.api_key_masked, 'test-secret');
});

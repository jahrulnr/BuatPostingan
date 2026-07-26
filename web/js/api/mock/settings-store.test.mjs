import assert from 'node:assert/strict';
import test from 'node:test';

import { createSettingsStore } from './settings-store.js';

test('provider catalog exposes the five initial provider families', function () {
    const store = createSettingsStore();
    const catalog = store.listProviderCatalog().providers;
    assert.deepEqual(catalog.map(function (p) { return p.type; }), [
        'openrouter',
        'omniroute',
        '9router',
        'openai',
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

test('mock settings API preserves imported model capability metadata', function () {
    const store = createSettingsStore();
    store.createProvider({
        type: 'openai',
        id: 'OPENAI',
        name: 'OpenAI',
        api: 'responses',
        base_url: 'https://api.openai.com/v1',
        api_key: 'test-secret',
        models: [{ id: 'gpt-image', output_modes: ['image'] }],
    });

    const provider = store.getProvider('OPENAI');
    assert.deepEqual(provider.models[0].output_modes, ['image']);
});

test('mock settings snapshot exposes product knobs and patch updates them', function () {
    const store = createSettingsStore();
    const snap = store.snapshot();
    assert.equal(snap.limits.max_tool_rounds, 8);
    assert.equal(snap.llm.stream, true);
    assert.equal(snap.context.compaction_enabled, true);
    assert.equal(snap.docs.app_id, 'buatpostingan');
    assert.equal(snap.mcp.enabled, true);
    assert.equal(snap.web_search.github_token_set, false);

    const next = store.patchConfig({
        limits: { max_tool_rounds: 11 },
        llm: { stream: false, vision: 'on' },
        web_search: { github_token: 'ghp_secret_token' },
        mcp: { enabled: false },
    });
    assert.equal(next.limits.max_tool_rounds, 11);
    assert.equal(next.llm.stream, false);
    assert.equal(next.llm.vision, 'on');
    assert.equal(next.mcp.enabled, false);
    assert.equal(next.web_search.github_token_set, true);
    assert.notEqual(next.web_search.github_token_masked, 'ghp_secret_token');
});

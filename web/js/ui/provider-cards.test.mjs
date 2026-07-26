import assert from 'node:assert/strict';
import test from 'node:test';

import {
    catalogCardProvider,
    isMultiInstanceProviderType,
    standaloneInstanceProviders,
} from './provider-cards.js';

test('openai-compatible is multi-instance; singleton families are not', function () {
    assert.equal(isMultiInstanceProviderType('openai-compatible'), true);
    assert.equal(isMultiInstanceProviderType('openrouter'), false);
    assert.equal(isMultiInstanceProviderType('openai'), false);
});

test('catalog card never claims an openai-compatible connection', function () {
    const byType = {
        'openai-compatible': [
            { id: 'SAMPLE1', type: 'openai-compatible', name: 'sample1' },
            { id: 'SAMPLE2', type: 'openai-compatible', name: 'sample2' },
        ],
        openrouter: [{ id: 'OPENROUTER', type: 'openrouter', name: 'OpenRouter' }],
    };
    assert.equal(catalogCardProvider('openai-compatible', byType), null);
    assert.equal(catalogCardProvider('openrouter', byType).id, 'OPENROUTER');
});

test('every custom openai-compatible connection is a standalone card', function () {
    const catalog = [
        { type: 'openrouter' },
        { type: 'openai' },
    ];
    const providers = [
        { id: 'OPENROUTER', type: 'openrouter', name: 'OpenRouter' },
        { id: 'SAMPLE1', type: 'openai-compatible', name: 'sample1', base_url: 'http://a' },
        { id: 'SAMPLE2', type: 'openai-compatible', name: 'sample2', base_url: 'http://b' },
    ];
    const standalone = standaloneInstanceProviders(providers, catalog);
    assert.deepEqual(
        standalone.map(function (p) { return p.id; }),
        ['SAMPLE1', 'SAMPLE2']
    );
});

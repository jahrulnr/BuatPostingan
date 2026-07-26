import assert from 'node:assert/strict';
import test from 'node:test';

import { modelPickerMetadata, modelPickerName } from './model-picker.js';

test('model picker name removes a duplicated provider suffix', function () {
    assert.equal(
        modelPickerName({
            id: 'openai/gpt-4o-mini',
            label: 'openai/gpt-4o-mini · OPENROUTER',
            provider: 'OPENROUTER',
        }),
        'openai/gpt-4o-mini',
    );
    assert.equal(
        modelPickerName({
            id: 'glm-4.5-air',
            label: 'GLM 4.5 Air',
            provider: 'GLM',
        }),
        'GLM 4.5 Air',
    );
});

test('model picker metadata keeps provider identity separate from capabilities', function () {
    assert.deepEqual(
        modelPickerMetadata({
            provider: 'GLM',
            supports_vision: true,
            output_modes: ['text', 'video'],
            supported_efforts: ['low', 'high'],
        }),
        {
            provider: 'GLM',
            capabilities: ['vision', 'video', 'reasoning'],
        },
    );
});

test('model picker metadata normalizes and deduplicates capability labels', function () {
    assert.deepEqual(
        modelPickerMetadata({
            provider: '  openrouter  ',
            supports_vision: true,
            output_modes: ['TEXT', 'Vision', 'vision', 'audio'],
        }),
        {
            provider: 'openrouter',
            capabilities: ['vision', 'audio'],
        },
    );
});

test('model picker metadata omits absent provider and text-only output', function () {
    assert.deepEqual(
        modelPickerMetadata({
            output_modes: ['text'],
            supported_efforts: [],
        }),
        {
            provider: '',
            capabilities: [],
        },
    );
});

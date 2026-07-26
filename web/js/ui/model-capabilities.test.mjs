import assert from 'node:assert/strict';
import test from 'node:test';

import { isChatSelectable, chatModels } from './model-capabilities.js';

test('chat filter keeps multimodal chat and legacy models but hides known non-chat models', function () {
    const models = [
        { id: 'text', output_modes: ['text'] },
        { id: 'gpt-realtime', output_modes: ['audio', 'text'] },
        { id: 'image-only', output_modes: ['image'] },
        { id: 'text-embedding-3-small' },
        { id: 'qwen3-embedding-8b' },
        { id: 'gpt-4o-mini-tts' },
        { id: 'whisper-1' },
        { id: 'gpt-4o-transcribe' },
        { id: 'gpt-image-1' },
        { id: 'dall-e-3' },
        { id: 'omni-moderation-latest' },
        { id: 'bge-reranker-v2-m3' },
        { id: 'sora-2' },
        { id: 'opaque-task-model', task: 'embedding' },
        { id: 'legacy' },
    ];

    assert.equal(isChatSelectable(models[0]), true);
    assert.equal(isChatSelectable(models[1]), true);
    assert.equal(isChatSelectable(models[2]), false);
    assert.deepEqual(
        chatModels(models).map(function (model) { return model.id; }),
        ['text', 'gpt-realtime', 'legacy'],
    );
});

test('chat filter normalizes metadata and does not hide chat audio models', function () {
    assert.equal(isChatSelectable({ id: 'model', output_modes: [' TEXT '] }), true);
    assert.equal(isChatSelectable({ id: 'model', output_modes: [] }), true);
    assert.equal(isChatSelectable({ id: 'model', output_modes: 'text' }), true);
    assert.equal(isChatSelectable({ id: 'gpt-audio' }), true);
    assert.equal(isChatSelectable({ id: 'gpt-realtime' }), true);
});

const NON_CHAT_TASKS = new Set([
    'embedding', 'embeddings',
    'text-to-speech', 'speech-to-text', 'tts', 'stt',
    'transcription', 'translation',
    'image-generation', 'video-generation',
    'moderation', 'rerank', 'reranking',
    'classification', 'ocr',
]);

const NON_CHAT_ID_MARKERS = [
    'embedding', 'embed-', '-embed', '/embed',
    'rerank', 're-rank', 'moderation',
    'transcribe', 'transcription', 'whisper',
    'text-to-speech', 'speech-to-text', '-tts', '/tts', 'tts-', '-stt', '/stt', 'stt-',
    'gpt-image', 'dall-e', 'image-generation', 'imagegen',
    'stable-diffusion', 'sdxl', 'seedream',
    'video-generation',
];

function hasTextOutput(model) {
    const modes = model && model.output_modes;
    if (!Array.isArray(modes) || modes.length === 0) return true;
    return modes.some(function (mode) {
        return String(mode || '').trim().toLowerCase() === 'text';
    });
}

function hasFamilyPrefix(id, family) {
    return id === family || id.startsWith(family + '-') || id.includes('/' + family + '-');
}

function isKnownNonChatId(modelId) {
    const id = String(modelId || '').trim().toLowerCase();
    if (!id) return false;
    if (NON_CHAT_ID_MARKERS.some(function (marker) { return id.includes(marker); })) return true;
    return ['sora', 'flux', 'imagen', 'veo'].some(function (family) {
        return hasFamilyPrefix(id, family);
    });
}

/**
 * Legacy models without capability metadata remain selectable. Explicit
 * non-chat tasks and well-known OpenAI-protocol model families stay stored
 * in settings but are hidden from chat-facing pickers.
 */
export function isChatSelectable(model) {
    if (!model || !hasTextOutput(model)) return false;
    const task = String(model.task || '').trim().toLowerCase();
    return !NON_CHAT_TASKS.has(task) && !isKnownNonChatId(model.id);
}

export function chatModels(models) {
    return Array.isArray(models) ? models.filter(isChatSelectable) : [];
}

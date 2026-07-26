/**
 * Format a thrown API error (from real/mock drivers) for UI copy.
 * Prefers body.message, then pairs code · message when both exist.
 */
export function formatApiError(err, fallback) {
    const fb = fallback || 'Request failed';
    if (!err) return fb;
    const body = err.body && typeof err.body === 'object' ? err.body : null;
    const message = body ? String(body.message || '').trim() : '';
    const codeRaw = body
        ? (typeof body.code === 'string' && body.code) || (typeof body.error === 'string' && body.error) || ''
        : '';
    const code = String(codeRaw).trim();
    if (message && code && code !== message) return code + ' · ' + message;
    if (message) return message;
    if (code) return code;
    const fromErr = err.message ? String(err.message).trim() : '';
    return fromErr || fb;
}

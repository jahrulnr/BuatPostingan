export function durableSeq(data) {
    const direct = Number(data?.seq);
    if (Number.isFinite(direct) && direct > 0) return direct;
    const nested = Number(data?.item?.seq);
    return Number.isFinite(nested) && nested > 0 ? nested : 0;
}

function browserRandom() {
    if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
        const value = new Uint32Array(1);
        crypto.getRandomValues(value);
        return value[0] / 0x100000000;
    }
    return (Date.now() % 1000) / 1000;
}

export function reconnectDelay(attempt, random) {
    const index = Math.max(0, Number(attempt) || 0);
    const base = Math.min(8000, 400 * Math.pow(2, index));
    const sample = typeof random === 'function' ? random() : browserRandom();
    const jitter = 0.75 + Math.max(0, Math.min(1, sample)) * 0.5;
    return Math.round(Math.min(8000, base * jitter));
}

export function isNearBottom(element, threshold) {
    if (!element) return true;
    const gap = Number(element.scrollHeight || 0)
        - Number(element.scrollTop || 0)
        - Number(element.clientHeight || 0);
    return gap <= (threshold == null ? 64 : Number(threshold));
}

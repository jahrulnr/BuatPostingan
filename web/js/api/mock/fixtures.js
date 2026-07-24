/** Sample fixtures for mock driver. */

export const docsIndexReady = {
    usable: true,
    status: 'ready',
    message: 'Docs index ready',
    document_count: 12,
};

export const docsIndexBuilding = {
    usable: false,
    status: 'building',
    message: 'Docs index sedang dibangun. AI sementara tidak tersedia.',
    document_count: 0,
};

/** Toggle via localStorage key `bp.mock.docs_locked=1` in console if needed. */
export function resolveInitialDocsIndex() {
    try {
        if (localStorage.getItem('bp.mock.docs_locked') === '1') {
            return { ...docsIndexBuilding };
        }
    } catch (e) { /* ignore */ }
    return { ...docsIndexReady };
}

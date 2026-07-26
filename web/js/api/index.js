import * as mock from './mock/driver.js';
import * as real from './real/driver.js';

const DEFAULT_API_BASE = '/api/webchat';
const DEFAULT_BE_ORIGIN = 'http://localhost:8080';

/**
 * Resolve mockMode:
 * 1. ?mock=0|1|true|false
 * 2. window.__BP_MOCK__
 * 3. localStorage bp.mockMode
 * 4. default false (real BE) — use ?mock=1 for mock driver
 */
export function resolveMockMode() {
    try {
        const params = new URLSearchParams(window.location.search);
        if (params.has('mock')) {
            const v = String(params.get('mock') || '').toLowerCase();
            return !(v === '0' || v === 'false' || v === 'no' || v === 'real');
        }
    } catch (e) { /* ignore */ }

    if (typeof window.__BP_MOCK__ === 'boolean') {
        return window.__BP_MOCK__;
    }

    try {
        const stored = localStorage.getItem('bp.mockMode');
        if (stored === '0' || stored === 'false') return false;
        if (stored === '1' || stored === 'true') return true;
    } catch (e) { /* ignore */ }

    return false;
}

function stripTrailingSlash(url) {
    return String(url || '').replace(/\/+$/, '') || DEFAULT_API_BASE;
}

/**
 * Resolve API base for the real driver:
 * 1. ?api=http://host:8080/api/webchat (or bare origin)
 * 2. absolute window.__BP_API_BASE__
 * 3. real mode on non-BE static host (make fe :5173) → http://localhost:8080/api/webchat
 *    (Python http.server returns 501 on POST to relative /api)
 * 4. relative /api/webchat (same-origin when FE is served by Go on :8080)
 */
export function resolveApiBase(mockMode) {
    try {
        const params = new URLSearchParams(window.location.search);
        if (params.has('api')) {
            const raw = String(params.get('api') || '').trim();
            if (raw) {
                if (/^https?:\/\//i.test(raw)) {
                    return stripTrailingSlash(raw.endsWith('/api/webchat') ? raw : raw.replace(/\/+$/, '') + '/api/webchat');
                }
                return stripTrailingSlash(raw);
            }
        }
    } catch (e) { /* ignore */ }

    const configured = typeof window.__BP_API_BASE__ === 'string' ? window.__BP_API_BASE__.trim() : '';
    if (/^https?:\/\//i.test(configured)) {
        return stripTrailingSlash(configured);
    }

    if (!mockMode) {
        try {
            // The Docker deployment mounts the admin UI at /admin/ behind the
            // same Nginx origin. Keep its API relative even though its public
            // port is intentionally not the local-development :8080.
            if (String(window.location.pathname || '').startsWith('/admin/')) {
                return stripTrailingSlash(DEFAULT_API_BASE);
            }
            const port = String(window.location.port || '');
            // Go default BP_HTTP_ADDR=:8080; make fe uses FE_PORT=5173 (python -m http.server).
            if (port && port !== '8080') {
                const host = window.location.hostname || 'localhost';
                return stripTrailingSlash(`${window.location.protocol}//${host}:8080${DEFAULT_API_BASE}`);
            }
        } catch (e) {
            return stripTrailingSlash(DEFAULT_BE_ORIGIN + DEFAULT_API_BASE);
        }
    }

    return stripTrailingSlash(configured || DEFAULT_API_BASE);
}

const mockMode = resolveMockMode();
const baseUrl = resolveApiBase(mockMode);

export const listConversations = mockMode
    ? mock.listConversationsMock
    : real.listConversationsImpl;

export const createThread = mockMode
    ? mock.createThreadMock
    : real.createThreadImpl;

export const getThread = mockMode
    ? mock.getThreadMock
    : real.getThreadImpl;

export const renameThread = mockMode
    ? mock.renameThreadMock
    : real.renameThreadImpl;

export const deleteThread = mockMode
    ? mock.deleteThreadMock
    : real.deleteThreadImpl;

export const startTurn = mockMode
    ? mock.startTurnMock
    : real.startTurnImpl;

export const uploadAttachment = mockMode
    ? mock.uploadAttachmentMock
    : real.uploadAttachmentImpl;

export const listAttachments = mockMode
    ? mock.listAttachmentsMock
    : real.listAttachmentsImpl;

export const listModels = mockMode
    ? mock.listModelsMock
    : real.listModelsImpl;

export const retryTurn = mockMode
    ? mock.retryTurnMock
    : real.retryTurnImpl;

export const interruptTurn = mockMode
    ? mock.interruptTurnMock
    : real.interruptTurnImpl;

export const subscribeEvents = mockMode
    ? mock.subscribeEventsMock
    : real.subscribeEventsImpl;

export const getSettingsSnapshot = mockMode
    ? mock.getSettingsSnapshotMock
    : real.getSettingsSnapshotImpl;

export const patchSettingsConfig = mockMode
    ? mock.patchSettingsConfigMock
    : real.patchSettingsConfigImpl;

export const listSettingsUsers = mockMode
    ? mock.listSettingsUsersMock
    : real.listSettingsUsersImpl;

export const createSettingsUser = mockMode
    ? mock.createSettingsUserMock
    : real.createSettingsUserImpl;

export const updateSettingsUser = mockMode
    ? mock.updateSettingsUserMock
    : real.updateSettingsUserImpl;

export const deleteSettingsUser = mockMode
    ? mock.deleteSettingsUserMock
    : real.deleteSettingsUserImpl;

export const listLLMProviders = mockMode
    ? mock.listLLMProvidersMock
    : real.listLLMProvidersImpl;

export const listLLMProviderCatalog = mockMode
    ? mock.listLLMProviderCatalogMock
    : real.listLLMProviderCatalogImpl;

export const getLLMProvider = mockMode
    ? mock.getLLMProviderMock
    : real.getLLMProviderImpl;

export const createLLMProvider = mockMode
    ? mock.createLLMProviderMock
    : real.createLLMProviderImpl;

export const updateLLMProvider = mockMode
    ? mock.updateLLMProviderMock
    : real.updateLLMProviderImpl;

export const deleteLLMProvider = mockMode
    ? mock.deleteLLMProviderMock
    : real.deleteLLMProviderImpl;

export const addLLMModel = mockMode
    ? mock.addLLMModelMock
    : real.addLLMModelImpl;

export const removeLLMModel = mockMode
    ? mock.removeLLMModelMock
    : real.removeLLMModelImpl;

export const importLLMModels = mockMode
    ? mock.importLLMModelsMock
    : real.importLLMModelsImpl;

export const browseDir = mockMode
    ? mock.browseDirMock
    : real.browseDirImpl;

export const pagePreviewURL = mockMode
    ? mock.pagePreviewURLMock
    : real.pagePreviewURLImpl;

export const listPages = mockMode
    ? mock.listPagesMock
    : real.listPagesImpl;

export const publishPage = mockMode
    ? mock.publishPageMock
    : real.publishPageImpl;

export const unpublishPage = mockMode
    ? mock.unpublishPageMock
    : real.unpublishPageImpl;

export const deletePage = mockMode
    ? mock.deletePageMock
    : real.deletePageImpl;

export const authMe = real.authMeImpl;
export const authLogin = real.authLoginImpl;
export const authLogout = real.authLogoutImpl;

/** @type {import('./types.js').ApiContext} */
export const api = {
    baseUrl: baseUrl,
    mockMode: mockMode,
    adminUserId: Number(window.__BP_ADMIN_USER_ID__ || 1),
    adminDisplayName: String(window.__BP_ADMIN_DISPLAY_NAME__ || 'Admin User'),
    csrf: window.__BP_CSRF__ || '',
};

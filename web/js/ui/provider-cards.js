/**
 * Provider catalog card binding rules for Settings → Models.
 */

/** @param {string} type */
export function isMultiInstanceProviderType(type) {
    return String(type || '') === 'openai-compatible';
}

/**
 * Which configured provider (if any) should overlay a catalog family card.
 * Multi-instance families never claim a connection onto the catalog card.
 *
 * @param {string} catalogType
 * @param {Record<string, object[]>} providersByType
 * @returns {object | null}
 */
export function catalogCardProvider(catalogType, providersByType) {
    if (isMultiInstanceProviderType(catalogType)) return null;
    const matches = (providersByType && providersByType[catalogType]) || [];
    return matches[0] || null;
}

/**
 * Providers that should render as standalone instance cards.
 *
 * @param {object[]} providers
 * @param {object[]} catalog
 * @returns {object[]}
 */
export function standaloneInstanceProviders(providers, catalog) {
    const byType = {};
    (providers || []).forEach(function (p) {
        const type = p.type || 'openai-compatible';
        if (!byType[type]) byType[type] = [];
        byType[type].push(p);
    });
    const seen = {};
    (catalog || []).forEach(function (definition) {
        const claimed = catalogCardProvider(definition.type, byType);
        if (claimed) seen[claimed.id] = true;
    });
    return (providers || []).filter(function (p) {
        return !seen[p.id];
    });
}

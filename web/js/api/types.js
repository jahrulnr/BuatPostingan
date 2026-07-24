/**
 * @typedef {Object} ApiContext
 * @property {string} baseUrl
 * @property {boolean} mockMode
 * @property {number} [adminUserId]
 * @property {string} [adminDisplayName]
 * @property {string} [csrf]
 */

/**
 * @typedef {Object} DocsIndexGate
 * @property {boolean} usable
 * @property {string} status
 * @property {string} [message]
 * @property {number} [document_count]
 */

/**
 * @typedef {Object} Conversation
 * @property {string} thread_id
 * @property {string|null} title
 * @property {string} title_source
 * @property {string} status
 * @property {number} created_by_admin_user_id
 * @property {string|number} updated_at
 * @property {string|number} last_activity_at
 * @property {number|null} floor_holder_admin_id
 * @property {number} floor_remaining_sec
 */

/**
 * @typedef {Object} ListConversationsResponse
 * @property {Conversation[]} conversations
 * @property {DocsIndexGate} docs_index
 */

/**
 * @typedef {Object} CreateThreadResponse
 * @property {string} thread_id
 * @property {number} seq_head
 * @property {number} created_by_admin_user_id
 * @property {string} [created_at]
 */

/**
 * @typedef {Object} ThreadSnapshot
 * @property {string} thread_id
 * @property {number} seq_head
 * @property {boolean} busy
 * @property {number|null} floor_holder_admin_id
 * @property {number} floor_remaining_sec
 * @property {string|null} active_turn_id
 * @property {number|null} active_turn_initiator_admin_id
 * @property {Object[]} items
 */

/**
 * @typedef {Object} StartTurnResponse
 * @property {string} thread_id
 * @property {string} turn_id
 * @property {number} seq_head
 * @property {string} status
 * @property {number|null} [floor_holder_admin_id]
 * @property {number} [floor_remaining_sec]
 */

/**
 * @typedef {Object} SubscribeEventsRequest
 * @property {string} threadId
 * @property {number} afterSeq
 * @property {(eventName: string, data: Object) => void} onEvent
 * @property {(err?: Error) => void} [onError]
 */

/**
 * @typedef {Object} EventSubscription
 * @property {() => void} close
 */

export {};

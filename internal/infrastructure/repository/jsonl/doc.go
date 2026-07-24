// Package jsonl implements durable webchat persistence (AIPedia WebchatJsonlStore).
//
// Layout under root:
//
//	session_index.jsonl
//	threads/{id}.jsonl, {id}.seq, {id}.lock
//	interrupt/{thread}/{turn}.flag
package jsonl

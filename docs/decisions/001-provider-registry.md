# ADR-001: Inject a typed LLM provider registry

## Status

Accepted

## Date

2026-07-26

## Context

Provider settings previously treated every upstream as one generic
OpenAI-compatible row. That kept the initial implementation small, but mixed
vendor defaults, authentication rules, and UI presentation in one form. Adding
local gateways or Anthropic Messages would otherwise grow conditionals across
settings, transport, and frontend code.

## Decision

Use a provider registry whose concrete adapters live under
`internal/infrastructure/provider/<type>/`. `cmd/app` constructs the registry
and injects the `domain/service.ProviderRegistry` interface into the settings
use case.

The registry owns provider metadata, matching, defaults, and connection
normalization. The LLM transport still selects behavior from the normalized API
dialect (`chat`, `responses`, or `messages`), so adapters do not duplicate the
router or HTTP retry machinery.

The persisted `type` field is optional. Existing entries are inferred by ID or
base URL; unknown entries map to `openai-compatible`.

## Alternatives Considered

### One switch in settings and the LLM client

Rejected because each new provider would require edits in multiple unrelated
layers and make provider behavior difficult to test independently.

### One complete HTTP client per provider

Rejected because OpenRouter, OmniRoute, 9Router, OpenAI, and custom gateways
share compatible wire protocols. Duplicating timeout, retry, streaming, and
parsing logic would create drift.

## Consequences

- Adding a provider starts with one adapter and one DI registration.
- Provider cards consume the same credential-free metadata as backend defaults.
- Existing config remains readable without a migration.
- Claude Messages has a dedicated request/response path; its distinct SSE
  protocol remains non-streaming until a native parser is added.

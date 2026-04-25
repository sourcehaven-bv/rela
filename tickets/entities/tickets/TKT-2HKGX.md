---
id: TKT-2HKGX
type: ticket
title: Lua HTTP API support
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Add HTTP client capabilities to the Lua scripting sandbox, allowing Lua scripts
to make HTTP(S) requests to external APIs and services.

## Motivation

Lua scripts currently have access to the AI provider for LLM calls, but cannot
make arbitrary HTTP requests. This limits script extensibility — users cannot
integrate with external services, fetch data from APIs, or post webhooks from
their scripts.

## Acceptance Criteria

- Lua scripts can make HTTP GET, POST, PUT, PATCH, DELETE requests
- Request headers, body, and query parameters are configurable
- Response includes status code, headers, and body
- JSON request/response helpers for convenience
- Timeouts are enforced (per-request and inherited from runtime)
- TLS is supported (HTTPS)
- Error handling follows the ai.chat pattern: (nil, err_table) for network/runtime
  errors, raise for programming errors
- Request body size not needed; response body size limits prevent OOM (10 MiB cap)
- Redirects are NOT followed automatically; the 3xx is returned directly

## Design

The `http.*` module exposes:

- `http.request(opts)` — full-form request
- `http.get / post / put / patch / delete` — convenience methods
- `http.json_encode / json_decode` — JSON helpers

Error shape mirrors `ai.Error` (kind, status, message, retry_after, details)
so scripts switching between `ai.chat` and `http.request` see the same layout.

## Out of scope

- **SSRF filtering.** Private-IP range and localhost are reachable; Lua scripts
  are already treated as trusted code (see `rela.write_file`, `ai.chat`).
- **Multi-value response headers.** First-value-wins; the API-calling use case
  this module targets rarely needs full header arrays.
- **Automatic redirect following.** Returned directly so scripts can inspect
  `Location` and decide.

## History

This ticket supersedes TKT-5Z863 / PR #388, which accumulated on a pre-refactor
base branch and could not be merged after `internal/graph`, `internal/model`,
and the `lua.Services` architecture were replaced. The HTTP module is re-ported
here against the new `ReadDeps` / `WriteDeps` structure, with the review
findings from #388 (JSON cycle protection, error-shape parity with `ai`,
method validation, canceled-kind classifier test) applied from the start.

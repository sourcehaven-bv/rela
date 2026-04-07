---
id: TKT-YBKB
type: ticket
title: Add OpenAI-compatible AI client config and Lua bindings
kind: enhancement
priority: medium
effort: l
status: review
---

## Goal

Land the foundation for AI integration in rela: a config loader for
`.rela/ai.yaml`, an `internal/ai` package with an OpenAI-compatible chat client,
and Lua bindings that expose `ai.chat()` and `ai.complete()` to scripts.

This is the first slice of `FEAT-ER3Y` (AI integration via OpenAI-compatible
API). It deliberately ships only the chat primitive — embeddings, CLI commands,
MCP tools, caching, and UI integration are explicitly out of scope and will
follow as separate tickets once this foundation proves out.

## In Scope

- **Config**: load `.rela/ai.yaml` with `provider`, `base_url`, `model`, `api_key_env` fields. Gitignored. Missing config = AI disabled (Lua functions error clearly).
- **Package**: `internal/ai` with a `Client` interface and one OpenAI-compatible chat completions implementation. HTTP-based, no SDK dependency.
- **Lua bindings**: top-level `ai` table in the Lua sandbox exposing:
  - `ai.chat({messages, model?, temperature?, max_tokens?})` → `(result_table, nil)` on success, `(nil, err_string)` on failure. Result table has flat fields: `content`, `model`, `finish_reason`, `usage = {prompt_tokens, completion_tokens, total_tokens}`.
  - `ai.complete(string)` → `(string, nil)` / `(nil, err)`. Convenience that wraps `ai.chat` with a single user message and returns just the content.
- **Errors**: surface clearly to Lua via `nil, err` pattern (no silent failures, no panics).
- **Tests**: unit tests with a fake HTTP server (httptest); one Lua-level integration test that exercises both functions through the sandbox; tests for config loading edge cases.

## Out of Scope

- Embeddings API (`ai.embed`)
- Caching layer
- CLI commands (`rela ai ...`)
- MCP tool wrappers
- Web UI integration
- Streaming responses
- Tool use / function calling
- Multi-provider abstraction beyond OpenAI-compat
- Pre-built AI-powered example scripts (suggest-links, validate-with-ai, etc.)
- Logging / audit trail of AI calls (deferred to a dedicated ticket)

## Acceptance Criteria

1. A user can create `.rela/ai.yaml` pointing at any OpenAI-compatible endpoint (OpenAI, Ollama, LM Studio, Groq, etc.) and the config loads without error.
2. With config present and API key set, a Lua script calling `ai.chat({messages={{role="user", content="hi"}}})` returns a non-empty result table with `.content` populated.
3. With config present, `ai.complete("hi")` returns a non-empty string.
4. With config absent or invalid, both functions return `nil, err` with a clear, actionable error message — no panic, no silent failure.
5. With a network/HTTP error, both functions return `nil, err` with the underlying error surfaced.
6. The Lua sandbox does not regress: existing scripts and tests continue to pass.
7. Coverage ratchet passes — new code in `internal/ai` is unit-tested with a fake HTTP server.

## Notes

- Config file is gitignored (consistent with `.rela/user-defaults.yaml`).
- API key must come from an env var, not the config file, so config can be safely shared.
- Use Go's `net/http` directly; do not pull in an OpenAI SDK.
- The `ai` Lua global must not collide with existing sandbox names — verify during planning.

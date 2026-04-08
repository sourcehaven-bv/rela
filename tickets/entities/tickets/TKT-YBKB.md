---
id: TKT-YBKB
type: ticket
title: Add OpenAI-compatible AI client config and Lua bindings
kind: enhancement
priority: medium
effort: l
status: done
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

- **Config**: load `.rela/ai.yaml` with `provider`, `base_url`, `model`, `api_key_env?`, `timeout_seconds?` fields. Gitignored. `api_key_env` is optional — when absent, no `Authorization` header is sent (supports local providers like ollama, apfel, LM Studio). Missing config = AI disabled (Lua functions return a typed `not_configured` error table).
- **Package**: `internal/ai` with a `Provider` interface (named `Provider` so embeddings can be added later without parallel wiring) and one OpenAI-compatible chat completions implementation. HTTP-based, no SDK dependency.
- **Typed errors**: stable `ErrKind` enum (`not_configured`, `auth`, `bad_request`, `rate_limited`, `server_error`, `timeout`, `network`, `bad_response`, `streaming_unsupported`) classified by upstream `error.type` first with HTTP status fallback. `RetryAfter` populated for rate-limit responses.
- **Lua bindings**: top-level `ai` table in the Lua sandbox exposing:
  - `ai.chat({messages, model?, temperature?, max_tokens?})` → `(result_table, nil)` on success, `(nil, err_table)` on failure. Result table has flat fields: `content`, `model`, `finish_reason`, `usage = {prompt_tokens, completion_tokens, total_tokens}`. Error table has `kind`, `status`, `message`, `retry_after`, `details`.
  - `ai.complete(string)` → `(string, nil)` / `(nil, err_table)`. Convenience that wraps `ai.chat` with a single user message and returns just the content.
- **Optional pointer types** for `temperature` and `max_tokens` so 0 round-trips distinctly from "unset".
- **Operational logging** via `log/slog`: `slog.Debug` on request start, `slog.Info` on success with token counts, `slog.Warn` on any failure. Zero secrets, zero PII, zero message content.
- **API key safety**: read at `Chat()` call time (not construction), `redactKey` defense-in-depth at every error and log site, table-driven sentinel test that asserts the key never appears in any error or log line across success path, every error path, and the network-error path.
- **Provider divergence handling**: tolerate missing `usage`, `finish_reason`, `model`. Decode `content` as either a string or an array of `{type, text}` parts. Reject `text/event-stream` with `ErrStreamingUnsupported`. Validate `Content-Type` before JSON decoding so HTML proxy errors produce `ErrBadResponse` with a body snippet, not a confusing JSON parse error.
- **Body cap**: 10 MiB via `io.LimitReader`; exceeding it returns `ErrBadResponse` (not `ErrNetwork`) via the `errBodyTooLarge` sentinel.
- **HTTP redirect policy**: `CheckRedirect: ErrUseLastResponse` so the client refuses to follow upstream redirects.
- **`base_url` validation**: must include scheme, must not contain credentials, must not contain a query string or fragment (so query-string `api_key=...` parameters can never end up in logs).
- **`LoadProvider` returns `(Provider, error)`** so each entry point picks its own policy: interactive commands surface errors immediately, background contexts soft-fail with a log warning.
- **Wired into 4 of 5 Lua entry points**: `internal/cli/script.go`, `internal/cli/flow.go`, `internal/script/executor.go`, `internal/mcp/tools_lua.go`. Validation rules (`internal/validation/lua.go`) deliberately do NOT get AI access — see Out of Scope.
- **Tests**: unit tests for `internal/ai` using `httptest.Server` covering all error paths, the success path, content-shape variants, redirect rejection, body-cap, sentinel leak (across success + error + network paths). Lua-level integration tests through the sandbox. Coverage 93%+.
- **Documentation**: `CLAUDE.md` AI Integration section (config, Lua API, error taxonomy, network egress + script-level exfiltration security note). `ai-integration` concept entity. Top-of-file comment in `internal/lua/ai.go` documenting the `(nil, err_table)` convention split, the programming-error vs runtime-error taxonomy, and the LState concurrency invariant.

## Out of Scope

- Embeddings API (`ai.embed`)
- Caching layer
- CLI commands (`rela ai ...`)
- MCP tool wrappers (other than the existing `lua_eval` / `lua_run` carrying AI through Lua)
- Web UI integration
- Streaming responses
- Tool use / function calling
- Multi-provider abstraction beyond OpenAI-compat
- Pre-built AI-powered example scripts (suggest-links, validate-with-ai, etc.)
- **AI in validation rules**: a validation rule that calls `ai.chat` would hit the provider on every entity on every analyze run with no quota, no kill switch, and no cost warning. The 5-second validation timeout would also silently clip slow calls. This needs its own design (per-rule opt-in, cost guardrails, longer per-rule budget) and is a follow-up ticket.
- **Audit logging** (storing every prompt/response for compliance) — operational logging via slog ships with this ticket; full audit trail is a separate ticket.
- HTTP retries with backoff — the typed `rate_limited` error with `retry_after` gives scripts what they need to implement their own retry policy.
- Per-call timeout argument (`ai.chat({..., timeout=60})`).
- Logger dependency injection — currently uses `slog.Default()`, which means tests that capture log output must serialize through a global mutex. Tracked as a follow-up.

## Acceptance Criteria

1. A user can create `.rela/ai.yaml` pointing at any OpenAI-compatible endpoint (OpenAI, Ollama, LM Studio, apfel, Groq, etc.) and the config loads without error. `api_key_env` is optional.
2. With config present and API key set (or absent for no-auth providers), a Lua script calling `ai.chat({messages={{role="user", content="hi"}}})` returns a non-empty result table with `.content` populated.
3. With config present, `ai.complete("hi")` returns a non-empty string.
4. With config absent, both functions return `(nil, err_table)` with `err.kind == "not_configured"`.
5. With config present but malformed, `rela script` and `rela flow` fail at startup with the parse error in the message; background contexts (executor, MCP) log a warning and continue without AI.
6. With a network/HTTP error, both functions return `(nil, err_table)` with the appropriate `err.kind` (`network`, `timeout`, `auth`, `rate_limited`, `server_error`, `bad_request`, `bad_response`, `streaming_unsupported`).
7. `temperature=0` is sent distinctly from `temperature` absent (verified by request-body capture).
8. The Lua sandbox does not regress: existing scripts and tests continue to pass.
9. Coverage ratchet passes — `internal/ai` is at 93%+ with `httptest.Server`-based unit tests.
10. Smoke-tested end-to-end against real ollama `gemma3:12b` and against apfel for error-path divergences.

## Notes

- Config file is gitignored (consistent with `.rela/user-defaults.yaml`).
- API key must come from an env var, not the config file, so config can be safely shared.
- Use Go's `net/http` directly; no OpenAI SDK dependency.
- The `ai` Lua global is registered unconditionally at the top level (verified not to collide with the existing `rela` global or any other global).
- The `ai.*` Lua bindings deliberately use `(nil, err_table)` for runtime failures while every other rela Lua binding raises via `RaiseError`. This convention split is documented in `internal/lua/ai.go` top-of-file.

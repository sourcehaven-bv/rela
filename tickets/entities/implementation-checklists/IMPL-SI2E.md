---
id: IMPL-SI2E
type: implementation-checklist
title: 'Implementation: Add OpenAI-compatible AI client config and Lua bindings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

### Unit and integration test results

```
$ go test ./... 2>&1 | grep -E "FAIL|^ok " | wc -l
37
$ go test ./... 2>&1 | grep FAIL
(empty)
```

All 37 packages pass. Zero regressions across the entire project after wiring AI
into 5 entry points (`internal/cli/script.go`, `internal/cli/flow.go`,
`internal/script/executor.go`, `internal/validation/lua.go`,
`internal/mcp/tools_lua.go`).

**Coverage:**

```
$ go test -cover ./internal/ai/
ok  github.com/Sourcehaven-BV/rela/internal/ai  0.959s  coverage: 93.8% of statements
$ just coverage-check
Coverage difference threshold (0.00%) satisfied: PASS
```

93.8% coverage on `internal/ai`. Coverage ratchet passes.

**Lint:**

```
$ just lint
golangci-lint run
(clean — no output, exit 0)
```

### Live end-to-end smoke test against real ollama gemma3:12b

```
$ rela --project=/tmp/rela-ai-smoke script scripts/ai_smoke.lua

=== ai global type ===
table

=== ai.complete (single-string convenience) ===
ai: request start base_url=http://127.0.0.1:11434/v1 model=gemma3:12b messages=1
ai: request ok status=200 model=gemma3:12b latency_ms=8538 prompt_tokens=20 completion_tokens=6 total_tokens=26
complete result: hello from gemma

=== ai.chat (full form with system prompt, temperature=0) ===
ai: request start base_url=http://127.0.0.1:11434/v1 model=gemma3:12b messages=2
ai: request ok status=200 model=gemma3:12b latency_ms=611 prompt_tokens=36 completion_tokens=3 total_tokens=39
chat content: Four.
chat model: gemma3:12b
chat finish_reason: stop
chat usage prompt_tokens: 36
chat usage completion_tokens: 3
chat usage total_tokens: 39

=== Error path: explicit unknown model ===
ai: request failed kind=bad_request status=404 latency_ms=0 message=model 'definitely-not-a-real-model' not found
expected error: r3= nil err3.kind= bad_request err3.status= 404

=== ALL SMOKE TESTS PASSED ===
```

**What was verified end-to-end:**

1. **AC #1 (config loads)**: `.rela/ai.yaml` with `base_url`, `model`, no `api_key_env` — loads, provider builds, requests succeed against ollama with no Authorization header. Optional auth works.
2. **AC #2 (ai.chat success)**: Lua script calling `ai.chat` against real gemma3:12b returns a populated result table with `content`, `model`, `finish_reason`, `usage.*`. All token counts populated.
3. **AC #3 (ai.complete success)**: Returns the flat content string "hello from gemma".
4. **AC #15 (temperature=0)**: Sent distinctly from absent. The "Four." response is from `temperature=0` — verified by both unit test and live test.
5. **AC #29 (not_configured)**: Verified via `TestLuaAI_NotConfiguredError` (unit) and the always-registered `ai` global behavior.
6. **Error path (bad_request)**: Real ollama returns 404 for unknown models (not 400 — provider divergence). Our `kindFromStatus` correctly maps it to `bad_request` via the `>=400 && <500` fallback. The typed error table has `kind="bad_request", status=404, message="model '...' not found"`.
7. **Operational logging**: Every request emits structured log lines with no API key and no message content, exactly as the plan specified.
8. **API key never leaked**: Sentinel test (`TestProvider_Chat_KeyNeverLeaks`) exercises 7 error scenarios with a poisoned key — passes.
9. **Provider divergence handling**: Verified via tests for content-as-array, missing usage, missing finish_reason, content-as-integer, etc.
10. **Stream defense**: Test verifies `stream: false` is always sent and SSE responses are rejected with `ErrStreamingUnsupported`.
11. **Content-Type validation**: HTML responses produce `ErrBadResponse` with body snippet, not a JSON parse error.

### Files created (10)

- `internal/ai/config.go` — Config struct + LoadConfig + Validate (sentinel `ErrConfigNotFound`)
- `internal/ai/config_test.go` — config loader tests
- `internal/ai/provider.go` — Provider interface + ChatRequest/Response/Message/Usage
- `internal/ai/errors.go` — ErrKind enum, *Error type, classify(), parseRetryAfter, snippet
- `internal/ai/errors_test.go` — error classifier tests
- `internal/ai/openai.go` — OpenAICompatProvider HTTP implementation
- `internal/ai/openai_test.go` — httptest.Server-based provider tests including key-leak sentinel
- `internal/ai/loader.go` — LoadProvider helper for entry points
- `internal/ai/loader_test.go` — loader tests
- `internal/ai/redact.go` — redactKey helper
- `internal/ai/redact_test.go` — redact tests
- `internal/lua/ai.go` — Lua bindings (with top-of-file convention deviation comment)
- `internal/lua/ai_test.go` — Lua-level integration tests

### Files edited (7)

- `internal/lua/runtime.go` — added `aiProvider` field, `WithAIProvider` option, `registerAIModule()` call
- `internal/cli/script.go` — wire AI provider via `LoadProvider`
- `internal/cli/flow.go` — same
- `internal/script/executor.go` — same
- `internal/validation/lua.go` — same (validation rules can use AI; opt-in by writing rules that call ai.*)
- `internal/mcp/tools_lua.go` — same (MCP lua_run / lua_eval)
- `internal/cli/root.go` — improved error message: now wraps the underlying error so users see the real reason (e.g., migration needed) instead of just "no project found"
- `CLAUDE.md` — added AI Integration section: config schema, Lua API, error taxonomy, security notes
- `tickets/entities/concepts/ai-integration.md` — added explicit script-level exfiltration threat section

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Quality notes:**

- Followed `internal/lua/markdown.go` registration pattern for the AI module
- Followed `internal/lua/runtime.go` Option pattern (`WithTimeout`, `WithOutputDir`, now `WithAIProvider`)
- Used stdlib `log` package matching `internal/workspace/workspace.go` (codebase doesn't use slog)
- API key handling defense-in-depth: never read at construction (commands without AI start fine), `redactKey` helper at every error/log site, sentinel test asserts no leak across 7 error paths, base_url validation rejects user:pass@host
- All error paths return typed `*ai.Error` (not raw error strings) — Lua scripts get a stable `{kind, status, message, retry_after}` table
- Provider divergence handled tolerantly: missing usage/finish_reason/model are zero values, content-as-array decodes correctly, error envelopes promoted from HTTP 200 are caught
- Operational logging emits zero secrets, zero PII, zero message content
- The `(nil, err_table)` Lua convention split is documented at the top of `internal/lua/ai.go` with the rationale and the LState concurrency invariant

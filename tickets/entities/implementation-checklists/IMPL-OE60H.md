---
id: IMPL-OE60H
type: implementation-checklist
title: 'Implementation: Inject *slog.Logger into ai.Provider for parallel-test-safe log capture'
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

### Test suite

```
$ go test -race -count=10 ./internal/ai/
ok  github.com/Sourcehaven-BV/rela/internal/ai  8.084s

$ go test -race ./...
(37 packages, all pass)

$ just lint
(clean)
```

### AC checklist

| AC | Status | Evidence |
|----|--------|----------|
| 1. `logger *slog.Logger` field defaults to `slog.Default()` | PASS | `NewOpenAICompatProvider` initializes `logger: slog.Default()` before applying options |
| 2. `WithLogger(*slog.Logger) Option` lets callers inject | PASS | Added `WithLogger` in openai.go, used by 5 log-capture tests |
| 3. `logRequest*` are methods on provider | PASS | Converted `logRequestStart`, `logRequestSuccess`, `logRequestFailure` to methods; 9 call sites in `Chat`/`parseResponse` updated with `p.` prefix |
| 4. `captureLog` removed | PASS | Deleted; replaced with `newCapturedLogger() (*slog.Logger, *bytes.Buffer)` that returns a fresh per-test pair. No global mutation. No `logMu`. `sync` import dropped. |
| 5. At least one leak test runs under `t.Parallel()` | PASS | Opted 5 tests into parallel: `KeyNeverLeaks` (parent + subtests), `KeyNeverLeaks_SuccessPath`, `KeyNeverLeaks_NetworkError`, `LogsSuccessAndFailure`, `LogsFailure` |
| 6. `go test -race ./internal/ai/...` passes | PASS | Verified with `-count=10 -race` (8 seconds, clean) |
| 7. No production behavior change | PASS | CLI/MCP entry points still construct providers without `WithLogger`; logs continue to route to `slog.Default()`. Live smoke test against ollama gemma3:12b confirms identical output format |

### Live smoke test

```
$ rela --project=/tmp/rela-ai-smoke script scripts/ai_smoke.lua
time=2026-04-08T... level=INFO msg="ai request ok" status=200 model=gemma3:12b latency_ms=... prompt_tokens=20 completion_tokens=5 total_tokens=25
complete result: hello from gemma
chat content: Four.
time=... level=WARN msg="ai request failed" kind=bad_request status=404 ...
=== ALL SMOKE TESTS PASSED ===
```

### Files changed

- `internal/ai/openai.go` — added `logger` field, `Option` type, `WithLogger` option, converted 3 log functions to methods, updated 9 call sites. +47 / -21 lines.
- `internal/ai/openai_test.go` — added `TestMain` to pre-set env vars, replaced `captureLog` with `newCapturedLogger`, updated 5 log-capture tests to use per-test loggers, added `t.Parallel()` to 5 leak/log tests, updated `newTestProvider` to accept variadic options, removed `sync` import, added `bytes` and `os` imports. +28 / -43 lines.

### Notable decisions

1. **`WithLogger(nil)` is a no-op** rather than a panic. Matches how `http.Client.Timeout == 0` means "no timeout". Simpler for callers and avoids the common zero-value-panic antipattern.
2. **`TestMain` sets env vars once at binary startup** rather than per-test. This was the only way to make leak tests parallel-safe because `t.Setenv` panics when called after `t.Parallel()`. All leak tests share the same `sentinelKey`, so sharing env vars across tests is safe.
3. **Parallel tests use their own `*bytes.Buffer`** — `newCapturedLogger` returns a fresh buffer on each call, so there's no shared state between parallel test instances.
4. **`bytes.Buffer` concurrent-read during assertion is safe** because the test goroutine waits for `Chat` to return (all logger writes happen synchronously in the request goroutine) before reading `logBuf.String()`. Documented inline in `newCapturedLogger`'s comment.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Quality notes:**

- Followed the existing `Option` pattern from `internal/lua/runtime.go` (`WithTimeout`, `WithOutputDir`, `WithAIProvider`, `WithContext`). `WithLogger` slots in cleanly alongside these.
- The 3 log methods are trivial wrappers around the same structured-field calls that used to use `slog.Default()`. Behavior unchanged; just the dispatch target changed from package-global to per-provider.
- Zero new security surface: the logger is an opaque `*slog.Logger`, no serialization boundary, no user input. `redactKey` calls still run before the logger receives the message.
- No production config change: default logger is `slog.Default()`, which the CLI/MCP entry points already configure via `--verbose`/`--quiet` flags (from FEAT-VH7Z / #328).

---
id: PLAN-0SZ5W
type: planning-checklist
title: 'Planning: Inject *slog.Logger into ai.Provider for parallel-test-safe log capture'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `internal/ai` uses `slog.Default()` for operational logging. Tests
that capture log output (`TestProvider_Chat_Logs*`,
`TestProvider_Chat_KeyNeverLeaks*`) must swap the process-global default via
`slog.SetDefault`, which serializes them through a global mutex `logMu` and
blocks `t.Parallel()`.

**Scope (in):**

- Add `logger *slog.Logger` field to `openAICompatProvider`
- Add `WithLogger(*slog.Logger) Option`
- Convert `logRequestStart`/`logRequestSuccess`/`logRequestFailure` from package-level funcs to methods on `*openAICompatProvider` using `p.logger`
- Rewrite `captureLog` test helper to build a per-test `*slog.Logger` and pass it via `WithLogger`; drop the `logMu` global
- Opt at least one leak test into `t.Parallel()` to prove the race is gone

**Scope (out):**

- Changing log line format
- Exposing `WithLogger` to CLI entry points (they keep `slog.Default()`)
- Any change to `ai.LoadProvider` signature or call sites

**Acceptance Criteria:** reuses the 7 criteria from the ticket body. Summary:

1. `logger` field with `slog.Default()` default — Test: construct provider without `WithLogger`, call `Chat`, assert logs still land in `slog.Default()` (unchanged production behavior).
2. `WithLogger` option — Test: construct provider with `WithLogger(customLogger)`, call `Chat`, assert logs land in the custom logger's output buffer and NOT in `slog.Default()`.
3. `logRequest*` as methods — verified by the logs-go-to-custom-logger test.
4. `captureLog` rewritten — verified by reading the test helper source; no `slog.SetDefault` call, no `logMu`.
5. `t.Parallel()` in at least one leak test — verified by reading test source.
6. `go test -race ./internal/ai/...` passes — CI.
7. No production behavior change — CLI/MCP entry points still construct providers without `WithLogger`; existing log output is identical.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing solutions:** stdlib `log/slog` is already used. No new dependency
needed. `slog.New(slog.NewTextHandler(buf, nil))` is the canonical pattern for a
per-test logger writing to a buffer.

**Codebase patterns:** the `Option` pattern (`WithTimeout`, `WithAIProvider`,
`WithContext`) is already established in `internal/lua/runtime.go` and
`internal/ai/openai.go`. Mirror it exactly.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Approach:**

1. Add `logger *slog.Logger` to `openAICompatProvider` struct.
2. Add `WithLogger(l *slog.Logger) Option` that sets the field.
3. In `NewOpenAICompatProvider`, default `logger = slog.Default()` if unset after applying options.
4. Convert the three `logRequest*` functions to methods on `*openAICompatProvider` and replace `slog.Debug/Info/Warn` with `p.logger.Debug/Info/Warn`.
5. Update call sites in `Chat` from `logRequestStart(...)` to `p.logRequestStart(...)`.
6. Rewrite `captureLog` to return a `*slog.Logger` rather than mutate `slog.Default()`. Tests that use it pass the returned logger to `newTestProvider` via a new `WithLogger` argument.
7. Update `newTestProvider` to accept an optional `*slog.Logger` (variadic or options pattern).
8. Opt `TestProvider_Chat_KeyNeverLeaks_SuccessPath` into `t.Parallel()` as a smoke test of the fix.

**Alternatives considered:**

- Option signature: `WithLogger(l *slog.Logger)` vs `Logger *slog.Logger` on `Config`. Picked the Option pattern for consistency with existing `WithTimeout`, `WithAIProvider`, `WithContext`.
- Package-level `Logger` var: process-global again. Rejected — defeats the point.
- `log/slog.Logger` on `ai.Provider` interface: leaks test concern into the interface. Rejected — lives on the concrete `openAICompatProvider`.

**Files to modify:**

| File | Change |
|---|---|
| `internal/ai/openai.go` | Add `logger` field; add `WithLogger` option; convert 3 funcs to methods; default to `slog.Default()` |
| `internal/ai/openai_test.go` | Rewrite `captureLog` to return per-test logger; update call sites; drop `logMu` and `sync` import if unused; add `t.Parallel()` to at least one test |

**Dependencies:** stdlib `log/slog` (already used). No new deps.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources:** None new. The `*slog.Logger` is an opaque Go object passed by
consumers; it has no serialization boundary.

**Security-sensitive operations:** None introduced. The existing `redactKey`
calls in `logRequest*` continue to work because they run before
`p.logger.X(...)`; changing from package-global to per-provider logger doesn't
affect the redaction pipeline.

**Key leak guard unchanged:** `TestProvider_Chat_KeyNeverLeaks_*` still runs
against a buffer and asserts the sentinel never appears. With per-test loggers,
the race condition goes away *and* the test becomes stricter: in parallel mode,
contamination from other tests is impossible by construction.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

1. **Default logger** — construct provider without `WithLogger`; assert the unset field defaults to `slog.Default()`. Covered by existing tests (which stop swapping the global and expect production logging to land in stderr).
2. **Custom logger** — construct provider with `WithLogger(customLogger)` where `customLogger` writes to a `*bytes.Buffer`. Call `Chat` with a success fake server. Assert the buffer contains `"ai request ok"`.
3. **Custom logger isolation** — same as above but also confirm `slog.Default()` output (redirected to a separate buffer just for this test) is empty. This verifies there's no fallback to the global.
4. **Parallel leak test** — `TestProvider_Chat_KeyNeverLeaks_SuccessPath` with `t.Parallel()`. Run under `-race -count=10` to shake out any data race.
5. **All existing tests** — regression-free. `go test -race ./internal/ai/...` green.

**Edge cases:**

- `WithLogger(nil)`: explicitly document behavior. Simplest: treat nil the same as not setting it (default to `slog.Default()` in the constructor). Alternative: panic. Go with the lenient default — matches how `http.Client.Timeout == 0` means "no timeout".
- Multiple `WithLogger` options applied: last wins. Standard Options pattern behavior.

**Negative tests:**

- `nil` logger handled gracefully (becomes `slog.Default()`).

**Integration test:** the rewritten `captureLog` helper + `t.Parallel()` leak
test is itself the integration test for the whole fix.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Breaking test assertions due to format drift | Low | Low | Keep `slog.TextHandler` format identical to today |
| Production output regression (CLI user sees different logs) | Low | Low | Default to `slog.Default()` in the constructor; CLI behavior unchanged |
| Test file gets harder to read due to per-test logger plumbing | Medium | Low | Keep the `captureLog` helper, just have it return a logger instead of mutating global |
| `nil` logger from a forgetful caller | Low | Low | Default to `slog.Default()` on nil, matching stdlib patterns |

**Effort:** **s** (matches ticket). ~100 lines of Go, one file, plus test
updates. Half an hour of focused work.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: internal refactor, no user-visible change)
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: internal package internals, not architectural guidance)
- [x] ~~README.md~~ (N/A: internal refactor)
- [x] ~~API docs~~ (N/A: internal package, no public API)
- [x] ~~Docs-checklist~~ (N/A: refactor, no docs to update)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the refactor is small and mechanical; the AC list IS the design, and the cranky-reviewer's original F10 writeup already covered the trade-offs. Running a full design-review pass on this would be ceremony for its own sake.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — /design-review skipped by design (see above).

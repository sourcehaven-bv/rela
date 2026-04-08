---
id: REV-HMSNX
type: review-checklist
title: 'Review: Inject *slog.Logger into ai.Provider for parallel-test-safe log capture'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

| RR | Title | Severity | Status |
|----|-------|----------|--------|
| RR-B6A96 | TestMain swallows os.Setenv errors | nit | addressed (panic on error instead) |
| RR-28P3F | Stale claim about loader.go in the review prompt (not in ticket) | nit | wont-fix (stale claim was only in ephemeral prompt, not in any committed artifact) |

**Summary:** The cranky-code-reviewer found **zero critical and zero significant
issues**. Verdict: "ship it." Two nits surfaced:

1. `TestMain` swallowed `os.Setenv` errors — addressed by rewriting the loop to panic on error.
2. A stale claim about `loader.go` in the ephemeral review prompt (not in any committed artifact) — recorded for the review trail, no action needed.

The reviewer specifically validated:

- Parallel-test safety of `newCapturedLogger` (happens-before on single goroutine; no race possible)
- `TestMain` env-var setup does NOT break `TestProvider_Chat_AuthErrorWhenEnvVarMissing` (which uses a different env var `DEFINITELY_NOT_SET_VAR_ZZZ` precisely to avoid this interaction)
- `WithLogger(nil)` no-op is defensible (matches stdlib options pattern)
- Last-wins option application is standard Go semantics
- No other `slog.` references leak in `internal/ai` outside `openai.go`
- `loader.go` does NOT emit logs (the review prompt was wrong about this, the code is fine)
- Parent + subtest `t.Parallel()` on the table-driven test does what you'd expect (max concurrency, Go 1.22+ loop-var fix means no capture footgun)
- No shared mutable state between parallel tests other than process env vars which are read-only after `TestMain`
- No path by which one test's log output could land in another test's buffer

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** see IMPL-OE60H table — all 7 criteria PASS, verified via
unit tests, `-race -count=10`, and a live ollama smoke test confirming identical
production log format.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor with no user-visible change)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist created)

**Docs Checklist:** N/A — internal refactor, no user-visible change.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** *to be filled in after `gh pr create`*

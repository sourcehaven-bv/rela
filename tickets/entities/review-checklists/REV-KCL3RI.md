---
id: REV-KCL3RI
type: review-checklist
title: 'Review: API error messages discarded at 22 call sites (interceptor rejects plain objects)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (987 unit tests, 59 files; E2E forms+kanban+document-edit-button: 37 specs against the built rela-server)
- [x] Lint clean (0 errors, 77-warning baseline)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: frontend coverage ratchet removed in PR #944)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer over the stack diff with extra focus on this changeset: 0 critical, 0 significant, 5 minor)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-39JY28 (addressed), RR-VF69XT (addressed), RR-S76E6N
(addressed), RR-K4AU69 (minor, deferred to the A7 field-surfacing work with
reason)

## Acceptance Verification

- [x] Each acceptance criterion tested (contract tests pin all four failure shapes incl. name-based cancellations, correlation_id, status fallback; grep verifies zero `instanceof Error` API catch sites remain; script-error routing verified by updated useListActions tests + document E2E)
- [x] Test evidence documented in implementation checklist (IMPL-9HVADX Verification Evidence section)

**Acceptance Status:** PASS — interceptor rejects only ApiError; getErrorMessage
at all 22 former sites; four divergent parsers deleted;
isCancelledFetch/getScriptError delegate to the typed error; reviewer confirmed
all semantic changes are improvements (server messages now surface) with no
regressions.

## Documentation (enhancements only)

- [x] ~~Docs section~~ (N/A: bug fix, no user-facing docs; catch-site conventions documented in errors.ts header)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (errors.ts header documents the catch-site conventions: getErrorMessage / getScriptError / isCancelledFetch / validationErrors)

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (verified after push)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/960 (stacked on
https://github.com/sourcehaven-bv/rela/pull/953)

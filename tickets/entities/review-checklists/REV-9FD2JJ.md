---
id: REV-9FD2JJ
type: review-checklist
title: 'Review: Extract the webhook find/create/then mapping core out of dataentry'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: load average 60 to 90; touched packages pass locally, including the postgres webhook tests; the full suite runs in CI, which gates the merge)
- [x] ~~Lint clean (`just lint`)~~ (N/A locally: run in CI per user instruction "push to ci & check there"; CI gates the merge)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: CI runs it; ingest is at 96.8% against a new 90% floor)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-R2J9KD, RR-L3OM2E, RR-8IOWLV, RR-W2TNPQ, RR-DF75M4,
RR-EEBROW, RR-CEFO56, RR-OPGGI3, RR-KU4DVF, RR-HEF5VV, RR-37NI70, RR-XIPLXE

Both reviewers (quality and security) found no critical issues. The three
significant findings are fixed. One minor finding is deferred to TKT-RUO1GY and
one is wont-fix with a reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: existing webhook tests pass. Assertions are unchanged; three calls in two seam tests now go through `ingest.Interpolate` because the wrapper they called was removed.
2. PASS: `internal/ingest` unit tests, 96.8% coverage.
3. PASS: `TestRun_FlattensEveryTemplatedField` and `TestRun_DefaultMatchValueIsFlattened`.
4. PASS: `just arch-lint` reports no warnings.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

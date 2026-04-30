---
id: REV-ZHEXO
type: review-checklist
title: 'Review: Extract stubEntityManager to shared test helper package'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test` — full `go test -race ./...` clean)
- [x] Lint clean (`just lint` — 0 issues)
- [x] Coverage maintained (`just coverage-check` — 74.2% total, all package floors satisfied)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: trivial mechanical extraction of a duplicated test stub — 7 panic methods + a compile-time interface assertion. No reviewable logic.)
- [x] ~~All critical review-responses addressed~~ (N/A: none generated)
- [x] ~~All significant review-responses addressed~~ (N/A: none generated)
- [x] Self-reviewed the diff for unrelated changes — diff is exactly 4 files: new helper, two test files updated, arch-lint exclude line.

**Review Responses:** None.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. New package compiles and is consumed by both test files — **PASS** (`go build ./...` clean; both consumers compile and test green).
2. Existing tests still pass unchanged in behaviour — **PASS** (`go test -race ./internal/dataentry ./internal/script` ok).
3. `just arch-lint` passes — **PASS** for this change (only pre-existing `.ignored/` notices remain on develop, unrelated).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor, no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (compile-time interface assertion guides future maintainers when the interface evolves)

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: user requested implementation only; PR creation will happen when user runs `/pr`.)
- [x] ~~All CI checks pass~~ (Deferred: depends on PR creation.)
- [x] ~~PR URL documented below~~ (Deferred: depends on PR creation.)

**PR:** Pending — user has not requested `/pr` yet.

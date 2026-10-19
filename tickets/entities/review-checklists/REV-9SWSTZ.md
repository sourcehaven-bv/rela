---
id: REV-9SWSTZ
type: review-checklist
title: 'Review: Fuzz sweep: install Wails Linux deps for the build gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (PR CI)
- [x] Lint clean (PR CI)
- [x] Arch-lint clean (PR CI)
- [x] ~~Coverage maintained~~ (N/A: no Go code changed)

## Code Review

- [x] Self-reviewed the diff for unrelated changes
- [x] ~~All critical review-responses addressed~~ (N/A: no review-responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review-responses)

**Review Responses:** None.

## Acceptance Verification

- [x] Each acceptance criterion tested

**Acceptance Status:**

- Fuzz sweep build gate passes on ubuntu-latest: PASS (run 39215430631, 55 targets).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors. This is a `chore`-kind
ticket; no docs change required.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1931

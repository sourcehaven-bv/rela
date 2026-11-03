---
id: REV-TPNPPA
type: review-checklist
title: 'Review: Section hide_empty'
status: done
---

## Automated Checks

- [x] All tests pass — `just coverage-check` ran the full suite. One failure,
`cmd/rela-desktop TestChromeStyle_TargetsShippedClasses`, needs a built SPA (`no
stylesheets in the embedded SPA`) and is unrelated. After the review fixes, `go
test -race ./internal/dataentry/... ./internal/dataentryconfig/...` passes.
- [x] Lint clean — `golangci-lint run` for `maildemo,mailmanual`, `sqlite` and
`postgres`: 0 issues each.
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained — `go-test-coverage`: package and total thresholds
pass, total 83.1%.

## Code Review

- [x] Ran the cranky-code-reviewer agent on the diff
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — RR-B7T9Z7, RR-E2L8B3.
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-B7T9Z7, RR-E2L8B3, RR-S6LE5Q, RR-V7ROMN, RR-8B1CUY
(deferred to FEAT-KQ45P).

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist (IMPL-VMPJRF)

**Acceptance Status:**
- PASS: empty opted-in section absent — `TestHideEmpty_DropsOnlyEmptyOptedInSections`, manual `_views` check.
- PASS: empty section without the key still renders — same test.
- PASS: create on a later section unaffected — `TestHideEmpty_KeepsCreateOnLaterSection`.
- PASS: refused combinations fail load — `TestSectionHideEmpty_RefusesContradictions`, `TestSectionHideEmpty_RefusedInSidePanel`.
- PASS: docs — `docs/data-entry.md` regenerated via `scripts/generate-docs.sh`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-91BYY5

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR deliberately not opened: the requester asked for a local commit only.

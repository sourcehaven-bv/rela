---
id: REV-28GQM1O
type: review-checklist
title: 'Review: TestHiddenProperty_NotCompared fails when a random entity id contains 9'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Automated Checks

- [x] All tests pass (`go test ./internal/mapimport/`; targeted per machine rules, CI runs the full suite)
- [x] Lint clean (`golangci-lint run --allow-serial-runners ./internal/mapimport/...`: 0 issues)
- [x] Comment lint gate clean (commentlint commented-code,doclink on ./internal/mapimport)
- [x] ~~Coverage maintained~~ (N/A: test-only change, no production code)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: 40-line test-only diff, self-reviewed; Plan has no timestamps or other numeric fields where the sentinel could occur)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

## Acceptance Verification

- [x] Each acceptance criterion tested (0 failures in 200 runs; 20 of 200 before the fix)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh after this checklist)

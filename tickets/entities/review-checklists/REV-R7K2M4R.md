---
id: REV-R7K2M4R
type: review-checklist
title: 'Review: TestSSEACL_Debounce flakes on a loaded runner'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Automated Checks

- [x] TestSSEACL_Debounce passes 20 times under -race; CI runs the rest
- [x] ~~Lint clean (`just lint`)~~ (N/A: test-only change, CI runs it)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: CI runs it)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: test-only change)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: one test helper)
- [x] ~~All critical review-responses addressed~~ (N/A: none)
- [x] ~~All significant review-responses addressed~~ (N/A: none)
- [x] Self-reviewed the diff for unrelated changes

## Acceptance Verification

- [x] Each acceptance criterion tested: the burst still yields exactly one frame
- [x] Test evidence documented: sse_acl_test.go

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

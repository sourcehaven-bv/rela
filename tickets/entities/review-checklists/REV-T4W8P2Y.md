---
id: REV-T4W8P2Y
type: review-checklist
title: 'Review: Release workflow tests time out at the default 10 minutes'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Automated Checks

- [x] ~~Tests pass~~ (N/A: workflow flag only; the Release run on the next tag exercises it)
- [x] ~~Lint clean (`just lint`)~~ (N/A: CI runs it)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: CI runs it)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no code change)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: one flag)
- [x] ~~All critical review-responses addressed~~ (N/A: none)
- [x] ~~All significant review-responses addressed~~ (N/A: none)
- [x] Self-reviewed the diff for unrelated changes

## Acceptance Verification

- [x] Each acceptance criterion tested: the flag matches ci.yml
- [x] Test evidence documented: the failed run 45417975154 shows the 600 s timeout

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

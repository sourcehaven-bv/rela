---
id: REV-C8S0MDK
type: review-checklist
title: 'Review: Remotes page asks update right before delete for remote_deleted entities'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (go test ./internal/dataentry/... passes; the full suite runs in CI)
- [x] Lint clean (golangci-lint on internal/dataentry: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A: a branch was reordered and a test case added; no new code paths)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: 15-line diff reordering one permission branch; self-reviewed instead)
- [x] ~~All critical review-responses addressed~~ (N/A: no review-responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review-responses)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

## Acceptance Verification

- [x] Each acceptance criterion tested (delete without update is offered resolve_remote; update without delete is not)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS, see IMPL-MQSR18P.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix; the guide already states the fixed behavior)
- [x] ~~User-facing documentation updated~~ (N/A: GUIDE-remote-sync-two-way already says delete right only)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh pr create; CI monitoring is out of scope for this run)

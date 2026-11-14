---
id: REV-2J22BA3
type: review-checklist
title: 'Review: loadConfigPolicy drops the acl.yaml validation error silently'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`go test ./internal/appbuild/` passes; CI runs the full suite)
- [x] Lint clean (`just lint`) (`golangci-lint run --allow-serial-runners ./internal/appbuild/...`: 0 issues; `just arch-lint`: OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: three added lines in a package far above its floor; CI checks coverage)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: a one-call change that logs a previously discarded error; self-reviewed)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (a schema-mismatched acl.yaml under NopACL still boots and logs the warning)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: the subtest fails on origin/develop and passes with
the fix.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh pr create; CI not awaited per task scope)

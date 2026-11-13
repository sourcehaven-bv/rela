---
id: REV-HK9SQO3
type: review-checklist
title: 'Review: rela mcp fails to start when flows.yaml names roles'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/appbuild/... ./internal/cli/...`; full suite runs in CI)
- [x] Lint clean (`golangci-lint run ./internal/appbuild/...`: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A: floors only; the change adds tests to internal/appbuild and removes none)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer agent on the diff)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-GT629C0)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-GT629C0, RR-H3YG95N, RR-Q43Z4IH, RR-IE446G1

## Acceptance Verification

- [x] Each acceptance criterion tested (rela mcp starts on a flows project with roles; older run versions compile; missing acl.yaml is named)
- [x] Test evidence documented in implementation checklist (IMPL-RXQ0PI1)

**Acceptance Status:** PASS: rela mcp answers initialize on the flowcfg testdata
project; regression tests fail without the fix.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened after this checklist)

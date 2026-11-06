---
id: REV-SMS4ER6
type: review-checklist
title: 'Review: File-store conflict-block refusal answers HTTP clients with a generic error'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test on store/..., entitymanager/..., dataentry/..., appbuild/..., mcp/..., affordances/...: all ok)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages: 0 issues; `just arch-lint` OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no new package; the change adds tests to existing packages)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-06ZBS0B, RR-G0IRZ28, RR-XXN6OK0, RR-ZMJBW0F

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
PASS: entity create, entity PATCH, create with relation content and relation
content in a unified PATCH answer 422 `conflict_block` over a real file store,
and the body is not echoed (TestWrite_ConflictBlockBodyIs422). PASS: a 500 path
answers the refusal as 422 (TestWriteInternalError_ConflictBlockIs422). PASS:
the CLI prints the sentinel message for `rela create` and `rela update`; MCP
returns the same error text.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened by hand after done and CI is not polled, per the task instructions)

---
id: REV-NC5U1V
type: review-checklist
title: 'Review: Copy option to keep named fields on the target face'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./... passes)
- [x] Lint clean (`just lint`) (golangci-lint on touched packages: 0 issues; plimsoll and arch-lint clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new code is covered by unit and integration tests; CI runs the floor check)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-50VQN3, RR-XBIU0V (both addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a `fields: all` copy with `keep: [spec]` leaves the
target face's upload and bytes in place (TestFaced_CopyKeepLeavesTargetUpload).
PASS: other fields are still copied, and a new target face stays without the
kept value (TestCopyKeep). PASS: keep without `fields: all`, undeclared names
and duplicates fail the load (TestValidateCopies).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-content-states, GUIDE-metamodel, regenerated docs/)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-JTZOOO

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened after the ticket is done, per TKT-UFV01M)

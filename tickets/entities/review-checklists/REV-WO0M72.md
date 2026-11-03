---
id: REV-WO0M72
type: review-checklist
title: 'Review: Per-face fields: policy'
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
- [x] All significant review-responses addressed (RR-0T4NIZ)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0T4NIZ, RR-MCP1CO, RR-MMLE1A, RR-ORY0QX (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a `fields:` block keyed `ticket@published` lets a
role upload one file field on the published face
(TestFacedAttachment_FaceFieldsBlock, 200). PASS: other fields of that face stay
read-only (PATCH title, 403). PASS: other faces keep the role's normal rights
(PATCH draft, 200; TestFieldVerdicts_FaceBlocks). PASS: invalid face keys fail
the load and the audit (TestNew_FaceBlockKeyErrors, TestAudit_B4B5_FaceKeys).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-acl-overview, docs/acl-overview.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-PYHHQI

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (per-face visible: is tracked as TKT-GYGUJO)
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened after the ticket is done, per TKT-UFV01M)

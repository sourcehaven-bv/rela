---
id: REV-HT3S30
type: review-checklist
title: 'Review: List group_by on a single-valued relation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just ci`)
- [x] Lint clean (`golangci-lint` 0 issues on changed packages; `vue-tsc` clean)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (`just coverage-check`, part of `just ci`)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer, two rounds: origin/develop...6f21af1dc and 6f21af1dc..4884f92a7; plus an automated security review of 93f44f514)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None filed on this ticket. All code-review findings of the branch are on TKT-65LVAK, TKT-KJ3Q07 and BUG-022MB1.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 PASS: list sections and board columns come from the same useRelationColumns; grouped list by relation checked on the demo.
- AC2 PASS: the section create prefill sets the relation (useListGrouping, InlineCreateFormModal).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-UVV3TV)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-UVV3TV

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (the coordinator opens the PR for the whole branch)

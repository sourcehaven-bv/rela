---
id: REV-D2Y1A8J
type: review-checklist
title: 'Review: Coloured cells on a kanban matrix'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: the machine is shared, so only targeted suites ran: go test for dataentryconfig, configedit and the dataentry kanban/matrix/direction tests; 126 Vitest tests; 22 kanban e2e tests. CI runs the full suite.)
- [x] Lint clean (`just lint`): golangci-lint on dataentryconfig, dataentry and configedit reports 0 issues; eslint has only the existing KanbanView max-lines warning; `vue-tsc -b` clean; `just arch-lint` clean.
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the brief forbids it on the shared machine; CI enforces the floors, and the new code has its own tests.)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-JB0VIHD (critical, addressed), RR-US2E48I, RR-BV43DJF,
RR-W3F0I47, RR-558V3WQ (minor, addressed), RR-8H0ODM7, RR-U9VSHBI (nit,
addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: the e2e board paints the error cell and shows "Critical"
(kanban-cell-colors.spec.ts, KanbanView.cellColors.test.ts).
2. PASS: unknown column, lane and colour fail load naming the board
(TestValidateConfig_KanbanCellColors).
3. PASS: no swimlane_property and columns_from both fail (same test).
4. PASS: a non-tone colour without a label fails (same test).
5. PASS: the served config carries the class per cell
(relation_direction_test.go, TestResolveKanbanCellClasses).
6. PASS: cards keep their own background; checked on light and dark
screenshots, and plain and toned cells share padding.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-V0V830F

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

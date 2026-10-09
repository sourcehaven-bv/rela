---
id: REV-0NNT4H
type: review-checklist
title: 'Review: Collapsible kanban columns'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend vitest 3864 + new tests; rela-components vitest 459; go test dataentryconfig and dataentry; e2e kanban + kanban-collapse 21/21; full suites run again in CI)
- [x] Lint clean (`just lint`) (eslint 0 errors, vue-tsc clean for app and library, rela-components `npm run check` passes, golangci-lint on dataentryconfig 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go change is one struct field with a test; frontend has no coverage enforcement)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-IH9UDU, RR-1MB0W0, RR-WND239, RR-5B3SWG)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** addressed: RR-IH9UDU, RR-1MB0W0, RR-WND239, RR-5B3SWG,
RR-7CDQ2D, RR-B8X3F6, RR-6C8L7R, RR-MMKK5W, RR-7N2BP1. Deferred: RR-Z9VS5V,
RR-IWWJRD. Won't fix: RR-W0SOYZ, RR-2NXJE2.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Collapse/expand on plain and swimlane boards: PASS (e2e kanban-collapse.spec.ts, story play tests, KanbanView.collapse.test.ts).
2. Collapsed column shows title and count: PASS (e2e expectColumnCollapsed asserts the rail count).
3. Survives reload per board: PASS (e2e reload; composable tests for per-board keys and id switch).
4. collapsed: true default, reader can expand and it is remembered: PASS (Go config test; e2e bug-lanes board; KanbanView test).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-PJVZML

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

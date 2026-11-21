---
id: REV-K63OXV9
type: review-checklist
title: 'Review: View section tables on phones, list band and title link colour'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (focused vitest, vue-tsc, go test for dataentry and dataentryconfig, the e2e mobile-layout and table-inline-edit specs; the full suite runs in CI)
- [x] Lint clean (eslint and gofmt on the changed files)
- [x] ~~Comment lint gate clean~~ (N/A: Go comments only reference the bug)
- [x] ~~Coverage maintained~~ (N/A locally: new code has unit tests; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (RR-VIEVNFK, RR-U48C8TG)
- [x] All significant review-responses addressed (RR-WH6BUIV)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-VIEVNFK, RR-U48C8TG, RR-WH6BUIV, RR-HMVUQ2R, RR-WHWUGZ4, RR-ZSAMUPN, RR-R5IFSW3, RR-ME9HJPG, RR-F5QCAJ4, RR-XUBR5M0, RR-88GM9QF, RR-1VFX0Q2

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Table section stacks on a phone: PASS (e2e mobile-layout, SectionTableRowCells.test.ts)
- primary on table sections only: PASS (TestValidateConfig_PrimaryColumnPlacement, TestViewColumnsToV1_Primary)
- Empty and editable cells: PASS (sectionTable.test.ts, EditableRowCells.test.ts)
- List table under the toolbar: PASS (e2e mobile-layout)
- Title link colour: PASS (EntityList.titleLink.test.ts, browser check)
- Dashboard long values wrap: PASS (browser check)
- Documented: PASS (docs/data-entry.md and GUIDE-data-entry)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/data-entry.md and GUIDE-data-entry)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there

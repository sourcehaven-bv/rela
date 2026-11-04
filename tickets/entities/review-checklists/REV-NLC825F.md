---
id: REV-NLC825F
type: review-checklist
title: 'Review: Edit owned and linked items inline on the parent page'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

## Automated Checks

- [x] All tests pass (`just test` exit 0; vitest 374 files / 5608 tests; e2e table-inline-edit + settings 21 passed)
- [x] Lint clean (`just lint`: 0 issues for every build tag; `just arch-lint` and `just plimsoll` exit 0)
- [x] Comment lint gate clean (`just comment-lint` exit 0)
- [x] Coverage maintained (`just coverage-check`: total 85.6%, all floors pass)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer: 15 findings, recorded as RRs)
- [x] All critical review-responses addressed (none open)
- [x] All significant review-responses addressed (handle-cell CSS and file-column refusal fixed)
- [x] Self-reviewed the diff for unrelated changes (the worktree also holds the decimal type and rela-components changes; Jeroen decides how to split them into commits)

**Review Responses:** RR-3LDEAK1, RR-3O13G5Z, RR-3XLCZ00, RR-4TFSF7D,
RR-55WMF2Q, RR-6YIJRH1, RR-7D0Q2T7, RR-7N09FZQ, RR-8669P8G, RR-8I32H8V,
RR-9P7F43A, RR-AFX1C8X, RR-ARK8S1Y, RR-CJ7ODEZ, RR-CPB5PXW, RR-CQ6QJJX,
RR-F35K7DL, RR-H7OWQ3H, RR-IRWA2HI, RR-JXTTP5L, RR-KYILNY1, RR-KYJGY4M,
RR-LY9SC4O, RR-N3ZC0DR, RR-NFQ2D2G, RR-OQF1IDT, RR-PA3KBWR, RR-PABTYTY,
RR-PIG9KY8, RR-QSTQJTW, RR-RZM49CQ, RR-T7DG96E, RR-VIIGYMT

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-8YARL46)

**Acceptance Status:**

1. Editable table cells: PASS. `render: input` on `columns` / `columns_by_type` edits through InlinePropertyValue, one autosave per row (EditableRowCells.test.ts, table-inline-edit.spec.ts); section-level `render:` stays inert (dataentryconfig tests).
2. Shared autosave host: PASS. `useSectionAutoSave` serves SectionEditForm and table rows; apply path covers `rows` and `groups[].rows` (sectionTableRows.test.ts).
3. Section totals: PASS. `totals:` reads entry properties; a row save reloads the view and the total moves (e2e, and manual check: qty 3 to 10 moved the total from 5 to 12 without a page reload).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-data-entry, docs/data-entry.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-1SNL8N9

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: not committed yet; Jeroen decides the commit split)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR follows done and the commit split, see TKT-UFV01M)

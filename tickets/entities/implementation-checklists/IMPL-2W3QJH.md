---
id: IMPL-2W3QJH
type: implementation-checklist
title: 'Implementation: File properties show the file name in table and list cells'
started: "2026-10-16"
completed: "2026-10-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
  - `widgets/FileCellWidget.test.ts`: name, icon, title and download attribute; single string value; escaped name; no link without an owning entity or plural; "N files" summary; empty value; click does not reach an enclosing row link.
  - `utils/fileRefs.test.ts`: keyed path, legacy name, backslashes, trailing slash, empty, `..`, non-string, sort and dedupe.
  - `utils/format.test.ts`: `formatCellValue` shows file names.
  - `widgets/viewRouting.test.ts`: `file` (also `list: true`) routes to `FileCellWidget`; no dense hint resolves to FileWidget.
- [x] Integration tests written (test full flow, not just units)
  - `components/entity/EntityDetail.table.test.ts`: a mounted `display: table` section with one file, several files, an empty cell, and a row served in a face.
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed): an unknown plural renders the name without a link instead of throwing in render.

## Test Quality

- [x] Using fixture builders or factories for test data (`cells()`, `render()`, `path()`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
On a local atlas demo (postgres build of this branch), a `bijlagen` column on
the procedure page's "Uitgevoerde controles" table shows a paperclip and
`hersteltest-q2.pdf` instead of the storage path. The link
`/api/v1/taken/TASK-ZBGVX/_attachments/bijlagen/hersteltest-q2.pdf` returns
200 with `application/pdf`; empty cells stay blank.

## Quality

- [x] Code follows project patterns (check similar code): dense routing hint, widget per type, no per-type `v-if` in views.
- [x] Checked for DRY opportunities: `attachmentFileUrl` reuses `propertyUrl`; `fileRefNames` serves the widget and `formatCellValue`.
- [x] No security issues introduced (the server gates every download)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

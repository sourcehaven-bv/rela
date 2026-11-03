---
id: IMPL-8P7VVX
type: implementation-checklist
title: 'Implementation: Busy column on list rows'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestValidateConfig_ListBusyColumn`, `TestValidateConfig_BusyColumnOnlyOnLists`, `TestValidateChannels_BusyListColumn`, `TestCheckEdit_DataEntry` busy cases, `tableColumns.test.ts`, `utils/busy.test.ts`, `models.test.ts`, `changes.test.ts`
- [x] Integration tests written (test full flow, not just units): `TestValidate_BusyListColumnWithoutChannels`, `TestConfigure_BusyFieldAgainstRealtime`, `TestSetRealtime_BusyListColumn`, `TestReloadConfig_ChecksBusyField`, `TestLive_BusyListPagedWake`, `TestListBusy_ListIDScoped`, `TestExportList_SkipsBusyColumn`, `EntityList.busy.test.ts`, `ListView.test.ts`
- [x] Happy path implemented: `- busy: <type>` in a list's columns sends `?busy=` and renders `RlBusyIndicator` in a 160 px primary column
- [x] Edge cases from planning handled: refused at column 0, with sortable or other source keys, twice per list, on dashboards and view sections; grouped by the first column keeps the busy column; searched and paged lists; the export skips it; Configure refuses a move to the top with an announced reason
- [x] Error handling in place (errors surfaced, not swallowed): load errors name the list and column; a channel type missing from realtime.yaml fails on every check path

## Test Quality

- [x] Using fixture builders or factories for test data: existing config and live env builders, `noChannelTypes` fake
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: Storybook play functions for `BusyColumn` and `RefusedMove` run in the browser; server paths are covered by tests)
- [x] Each acceptance criterion verified with test scenario from planning: AC table in the notes maps 1 to 14 to tests
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build on four tags; vet; `just lint` (0 issues on all
tag sets), arch-lint, comment-lint; `go test -race` on dataentry, realtimecfg,
configedit and the other touched packages; 5410 frontend tests, type check,
lint, `just build-frontend`; rela-components stories; `just docs`; markdownlint;
archplan ci; coverage check.

**Deviations:** allowlist cases live in `TestCheckEdit_DataEntry`; AC 9 narrows
with `condition:` because `list_id` does not apply static `filters:`;
`RlSortableList` gained a `refuseMove` prop because it cannot lock one row.

## Quality

- [x] Code follows project patterns (check similar code): mirrors the face column and the kanban `busy:` field; one `busyFields` walker feeds every check path
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): shared `_busy` to indicator helper for
board and list; `columnModel` and `columnHeader` reused in Configure
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

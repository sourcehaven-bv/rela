---
id: IMPL-H8Y3LMR
type: implementation-checklist
title: 'Implementation: Settings: search picker for relation defaults'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: not clicked through in a browser in this session; the handler is covered through httptest and the golden corpus, the picker through component tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `TestQueryBudget_SettingsReadsNoEntities` passes; against the old handler it
fails with 10 calls and 2043 rows over the padded store, against 2 calls and 1
row for binding alone.
- `TestSettings_RelationDefsCarryTargetTypesOnly` and `TestGoldenParity`
(regenerated settings goldens) pass; `go test -race ./internal/dataentry/
./internal/archguard/` passes.
- `RelationDefaultPicker.test.ts` (13 cases: empty, one read, prefix order,
404 fallthrough, failure, superseded read, unmount, pick without read,
accessible names, no target types, clear) and the updated
`EntityPickerModal.test.ts` pass, as do the editor, channel, view and i18n
suites; `npm run typecheck` and `npm run build` pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

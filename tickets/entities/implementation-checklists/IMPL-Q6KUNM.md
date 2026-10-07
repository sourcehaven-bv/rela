---
id: IMPL-Q6KUNM
type: implementation-checklist
title: 'Implementation: Owned items: relation targets that live only on the parent page'
started: "2026-10-05"
completed: "2026-10-06"
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Schema: `internal/metamodel/owning_test.go` covers `owning: true` loading
and the refusals (symmetric, `max_incoming` above 1, faced types).
- Write rules: `TestCreateRelation_OwningRules` (second owner, nested, self)
and `TestAutomationCreateRelation_OwningRules`. A mutation check confirmed the
automation test fails when the cascadehost check is removed.
- Delete: `TestDeleteEntity_TakesOwnedEntities`, `_OwnedWithoutCascade`,
`_OwnedDenialDeletesNothing`, `_OwnedEntityAlone`; soft delete and restore in
`TestSoftDelete_OwnedEntitiesRoundTrip` and `_OwnedDenialMarksNothing`. These
run on every backend in `concBackends` and pass on sqlite (`-tags sqlite`).
Postgres was not run locally (`RELA_TEST_DATABASE_URL` unset); CI runs it.
- Read side: `TestOwner_ListAndSearchRows`, `TestOwner_Irregular` (hidden
owner neither shown nor leaked, ambiguous owners, nested, mutual, self),
`TestOwner_ReadBudget` (same reads at 10 and 50 rows) and
`TestOwner_ViewCarriesOwnerAndRelatedSection` (the view entry and the rows of a
`related` section carry `_owner`; no body is loaded).
- Analyze: `internal/analysis/owning_test.go` for `rela analyze owning`.
- Config: `TestValidateConfig_ViewSectionRelatedDisplay`.
- SPA unit tests: `EntityDetail.owner.test.ts` (redirect to `owner#child`
with the query kept, no redirect in a preview, "Part of" link, row anchors,
`related` rows render; the last was mutation-checked by removing the import),
`RelatedSectionRows.test.ts`, and `ownedEntityHref` in `entityRoute.test.ts`.
- End to end: `e2e/tests/owned-items.spec.ts` against the real server and SPA.
Opening `/entity/step/STEP-001` lands on `/entity/plan/PLAN-001#STEP-001` with
the row and its assignee shown. A search for the step shows "in Launch plan" and
Enter opens the plan anchored at the step. Both pass. This run caught a missing
component import that the unit tests had missed.
- Full e2e suite: 415 passed. The failures were the settings type and relation
counts (updated for the two new fixture types) and specs that time out or lose
`bin/rela-server` under a machine load average of about 85. Rerun with one
worker, they pass except one that failed on the missing binary.
- Gates: `go test` for every touched package passes; `just arch-lint` and
`just comment-lint` are clean; frontend `typecheck`, `lint` (0 errors) and
`test:run` pass; the vendored component library's `check` and tests pass.

Deviations from the plan:

- The owner rides the shared entity wire type as `_owner` (list rows, search
hits, the view entry and `related` section rows), not a separate
`ViewResponse.Owner` or `LinkRow.Parent` field.
- The views handler still builds sections when the entry has an owner. The
SPA redirects before rendering, so the cost is one discarded response.
- A `related` section uses `RlRelatedRow` for its rows. `RlRelatedList` would
draw a second heading under the section's own heading and create button.
- `RlRelatedRow` dropped the space after its screen-reader labels, because
the template compiler trims a trailing space before a closing tag. Fixed in both
the vendored copy and `../rela-components`.
- MCP `analyze` has no owning check: arch-lint forbids `internal/mcp` from
importing `internal/analysis`. The CLI check is the one the plan required.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

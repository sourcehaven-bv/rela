---
id: IMPL-U39FWE
type: implementation-checklist
title: 'Implementation: lua: reject non-string filter options in the gated rela.get_relations'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: the change is one argument-parsing function on an existing binding; the end-to-end path is covered by running the live 120-rule validation suite and both example scripts against the real ticket graph, documented below)
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

Bug reproduced BEFORE the fix (characterization test against unmodified code):
all four of `{from = 12345}`, `{type = true}`, `{to = {}}`, `{to = 0}` returned
the unfiltered result, failing with "a non-string filter was silently DROPPED".

The fixture is deliberately two-relation, not the default single-relation
`newMockWorkspace` — with one relation, "filtered to nothing" and "returned
everything" are indistinguishable, which is the exact confusion under test.
Confirmed load-bearing by mutation 3 below.

Mutation testing (production code mutated, test confirmed failing, reverted):

1. Silent-skip instead of reject → `TestGetRelations_RejectsNonStringFilter`
fails on all 4 subtests. Catches the original bug.
2. Drop the `*lua.LNilType` case (absent becomes invalid) →
`TestGetRelations_AcceptsAbsentAndStringFilters` fails on 5 subtests. Catches
over-rejection, i.e. a fix that swings too far.
3. Drop the `from` field from the query builder →
`TestGetRelations_AcceptsAbsentAndStringFilters` fails on from_filter (1 vs 2)
and no_match (0 vs 2). Proves the fixture discriminates.

Live end-to-end against the real ticket graph (2246 relations):

- `rela analyze validations` → all 120 rules pass, including
`require-relation-count.lua`, the real in-tree caller passing `entity.id`.
- Confirmed every id construction site uses `lua.LString` (`runtime.go:832`,
`:1112`, `:1218`, `:2005`), so no in-tree caller can regress.
- `examples/view-deps.lua` runs and returns results.
- `examples/view-affected.lua` fails on its own arg convention — verified
IDENTICAL on a stashed clean tree, so pre-existing and unrelated.

Found and fixed a live bug in our own docs while verifying: the orphan-report
example in `GUIDE-scheduled-tasks.md` called `rela.get_relations(e.id)`.
Measured: 2246 relations returned vs 2 for `{from = e.id}` — its `#rels == 0`
check could never fire, so the documented scheduled task would report "No
orphaned entities found" forever.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the fix REMOVED duplication rather than
adding it: `elevatedRelationQuery` was already generic, so it was renamed
`relationQuery` and shared by both bindings. This is the point of the change,
not a side effect — two copies of "what a filter means" is what let them drift
in the first place.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

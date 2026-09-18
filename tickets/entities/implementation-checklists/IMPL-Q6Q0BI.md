---
id: IMPL-Q6Q0BI
type: implementation-checklist
title: 'Implementation: Relation edits silently dropped when a linked entity is outside the picker''s first-100 candidate window'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (6 picker cases + 10 helper cases; the bug cases and the post-mount regression were each confirmed failing before their fix)
- [x] Integration tests written (test full flow, not just units) — the tests drive the mounted `RelationPicker` through load → emit → user selection, asserting the `update:types` payload that `reshapeLegacyToModern` consumes
- [x] Happy path implemented
- [x] Edge cases from planning handled: multi- and single-select, addition after load, lookup failure, no-op when nothing is missing, skipped entirely on create (no `entityId`) and on incoming pickers
- [x] Error handling in place: a failed resolve logs and degrades to candidate-page types; cancelled fetches are suppressed via `isCancelledFetch` like the sibling loaders

## Test Quality

- [x] Using fixture builders or factories for test data (reuses the file's `entity()` / `seedSchema` / `seedCandidates` helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions compare ids/types deliberately written as literals so the test states the expected contract rather than restating the fixture)

## Manual Verification

- [x] Feature manually tested end-to-end (live `rela-server`, see evidence)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Tests first, confirmed failing:* 3 of 4 new cases failed with `expected
undefined to be 'ticket'` and `expected +0 to be 1` — the precise symptom (no
resolved type, no rendered chip). Reverting only `buildOutgoingTypes` to its old
candidate-scan re-fails 2 of them, confirming the tests bind to the fix rather
than passing incidentally.

*Automated, after the fix and the review round:* `RelationPicker.test.ts` 31/31,
`outOfPageLinks.test.ts` 10/10, `src/components/forms/` 282/282, full suite
2473/2473 (152 files). `npm run typecheck` clean; `npm run lint` 0 errors and
0 warnings on all four changed files; `just arch-lint` OK.

*End-to-end against a live server:* built the SPA and `rela-server`, ran a
scratch copy of the data-entry prototype with 126 labels (so the linked one is
off page 1) and the edit form's `tagged` relation switched from `cards` to
`multi-select` (the `cards` widget is immune). `TKT-001` linked to
`zz-label-120`, which sorts last.

- The entity GET returns bare ids, no types: `tagged: ["bug","urgent","zz-label-120"]`.
- The candidate page (`/api/v1/labels?per_page=100`, 100 of 126) resolves `bug` and `urgent` but **not** `zz-label-120` — the exact save-abort condition, confirmed on real data rather than a mock.
- The relations endpoint (`/api/v1/tickets/TKT-001/relations/tagged`), which the fix now queries, returns `{"id":"zz-label-120","type":"label"}`.
- The PATCH the fix makes buildable — all four identifiers carrying `type` — returns **HTTP 200**, adding the new label while preserving `zz-label-120`. Before the fix the SPA could not construct this body at all and aborted with the toast.

Re-run end-to-end against the FINAL code after the review round (the fix changed
substantially): same 126-label setup, same result — candidate page cannot
resolve the link, relations endpoint returns its type, PATCH returns HTTP 200
preserving it. Scratch project removed; nothing left in the working tree.

*Not covered:* the browser extension was unavailable, so the rendered chip was
verified by unit test (`renders a pre-existing out-of-page link as a selected
chip`) rather than visually.

## Quality

- [x] Code follows project patterns — `resolveOutOfPageLinks` mirrors the existing `loadIncomingValue` (same error handling, same `isCancelledFetch` guard, same onMounted sequencing), and uses the edge-carried `type` that `RelationCards` already relies on
- [x] Checked for DRY opportunities — the shared `knownById` lookup now backs both `selectedEntities` and `buildOutgoingTypes`, removing the duplicated candidate scan that caused the two symptoms to diverge
- [x] No security issues introduced — the resolve goes through the existing ACL-gated relations endpoint; no new surface
- [x] No silent failures — a failed resolve is logged; an id that stays unresolved still hits `reshapeLegacyToModern`'s refusal rather than emitting a malformed identifier
- [x] No debug code left behind

**Known limitation (not a regression):** a resolved out-of-page chip shows the
bare id, because the relations endpoint carries `id` and `type` but no title.
Today that link renders as nothing at all, so this is strictly better; a title
would cost one fetch per link.

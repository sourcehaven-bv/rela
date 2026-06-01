---
id: IMPL-ASK7
type: implementation-checklist
title: 'Implementation: Reorderable relations via metamodel-declared ordering property'
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

Built `rela-server` against a hand-rolled fixture project (`recipe`/`step` with
`orderable: outgoing`) and exercised the full flow end-to-end via puppeteer +
curl. Confirmed (with the actual binary, not just unit tests):

- **AC1 (schema acceptance)** — `metamodel.yaml` with `orderable: outgoing` loads. `GET /api/v1/_schema` returns `"orderable":{"outgoing":true}` on the relation type. Verified `orderable: yes` is rejected by the loader via unit tests.
- **AC2a (outgoing sort)** — Both `/api/v1/{plural}/{id}/relations` (grouped) and `/api/v1/{plural}/{id}/relations/{relType}` (per-type, the path used by the SPA) sort outgoing relations ascending by `_order_out`. Fixtures `[3, 1, missing, 2]` returned `[1, 2, 3, missing]`.
- **AC3 (drag-to-reorder persists)** — In the SPA, dispatched native HTML5 DnD events to move the third card to position 1. Composable computed `_order_out = 0` (one less than the new top neighbour's `1`). Saving issued a `PATCH /relations/has-step/STP-8MA7` with `meta._order_out=0`, persisted to disk, and the re-fetched list returned the new sort order. Only the moved relation's file was rewritten.
- **AC4 (tolerant rendering)** — A relation with `_order_out` missing appears last; covered by unit test `TestSortRelations_StableMissingLast` and verified end-to-end with one missing-order seeded fixture.
- **AC5 (analyze warnings)** — After hand-editing two `_order_out` values to be equal, `GET /api/v1/_analyze` returned exactly one warning with code `relation.order.duplicate` and severity `warning` (not `error`). Confirmed errors-count remained 0.
- **AC7 (auto-assign on create)** — `rela link` issued three times in a row produced `_order_out: 1`, `_order_out: 2`, `_order_out: 3` in creation order.
- **AC8 (renumber on collapse)** — PATCH'd a value to `1 + 1e-10` (below `OrderCollapseThreshold = 1e-9`). Backend transparently rewrote all three siblings to integer ordinals `1, 2, 3` preserving sort order.
- **Wire-format validation** — `PATCH .../relations/has-step/STP-X` with `meta._order_out: "abc"` returned 400 + `order_value_invalid`.

The drag handle visual (`⋮⋮`) shows on the left of each card, and dragging marks
exactly one card as `card-updated`. Screenshot taken at
`orderable-relation-cards`.

**Known limitations / follow-ups:**

- No Playwright e2e test added. The existing kanban DnD pattern is precedent, but writing a robust Playwright DnD spec for orderable relations is its own ticket (synthetic HTML5 DnD events are flaky across browsers). Tracked by the manual verification above for now.
- No CLI subcommand registered for `analyze relation-order`. The data-entry web UI's `analyze` aggregate surfaces it; the CLI exposes the other analyses individually. Adding a CLI subcommand is a follow-up.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

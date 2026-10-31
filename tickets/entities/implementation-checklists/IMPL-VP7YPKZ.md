---
id: IMPL-VP7YPKZ
type: implementation-checklist
title: 'Implementation: Card wall board with optional drag-to-group'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`validate_kanban_layout_test.go` 7 cases; `boardDropZone.test.ts` 9 cases for `zoneAt`; `useBoardFlip.test.ts` 2 cases)
- [x] Integration tests written (test full flow, not just units) (`KanbanView.wall.test.ts` 14 component tests over the API calls; e2e `kanban-wall.spec.ts` drags a real card onto another twice and reloads)
- [x] Happy path implemented
- [x] Edge cases from planning handled (drop on itself, same group, failed create, empty or unchanged rename, unknown group refetch, group type without a display property, reduced motion)
- [x] Error handling in place (errors surfaced, not swallowed) (every write failure shows the server error through `uiStore.error`, then the board refetches)

## Test Quality

- [x] Using fixture builders or factories for test data (`seed()` helper in the wall test; `api.createEntity` in e2e)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (e2e expectations built from `TITLES`)
- [x] Property comparisons use original object, not hardcoded strings (rename test compares with the label read before the edit)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Browser QA on a copy of the workshop project (`.ignored/workshop-qa`, port
8962), no console errors:

- AC1 plain wall: one wrapping grid of notes; edge drops reorder, and the order survives a reload.
- AC2 drop on a loose card: a new group appears, named after the target card.
- AC3 drop on a grouped card: the card joins that group.
- AC4 drop on the pile: the card leaves its group.
- AC5 group handle: groups reorder; the order survives a reload.
- AC6 groups render as tinted cards; notes are yellow and tilt only in the pile.
- AC7 rename in the heading saves; an empty name restores the old one.

Automated:

- `npx vitest run` in `frontend`: 361 files, 5449 tests pass.
- `npx playwright test tests/kanban-wall.spec.ts tests/kanban.spec.ts tests/relation-order.spec.ts`: all pass.
- `go test ./internal/dataentryconfig/ ./internal/dataentry/`: pass.
- `npx vue-tsc --noEmit` (app and rela-components), `npm run check` in rela-components, eslint on changed files, `just arch-lint`, `just comment-lint`, golangci-lint on `internal/dataentryconfig`: clean.

The e2e run found that a group type whose title property is not required has no
display property, so the group showed its ID. Fixed: the wall writes the title
only into the type's display property, shows a read-only heading without one,
and the docs say so.

## Quality

- [x] Code follows project patterns (check similar code) (drag targets follow `useDropTargetColumn`; writes use the existing PATCH, relation and move endpoints)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (`zoneAt` shared by card drop targets;
grouping writes in `useWallGrouping`)
- [x] No security issues introduced (no new endpoint; ACL and validation apply to every write)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind (PROTOTYPE markers removed)

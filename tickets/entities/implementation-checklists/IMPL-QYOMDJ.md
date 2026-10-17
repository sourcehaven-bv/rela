---
id: IMPL-QYOMDJ
type: implementation-checklist
title: 'Implementation: Traverse condition: predicate expressions on view traverse steps'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
  - `predicatefns/collection_test.go`: `entity.type`, `entity.id`, foreign property as nil, own declaration wins, plain `Compile` unchanged.
  - `conditionlint/traversecondition_test.go`: one program per collected type, typo, filter dialect, `related(...)`, entity type from the key.
  - `dataentryconfig/traverse_condition_test.go`: unparseable `where:` on a relation step, side panel `condition:` and `where:`, id and target types.
- [x] Integration tests written (test full flow, not just units)
  - `dataentry/views_traverse_condition_test.go`: `executeView` with the production compiler on a mixed-type traversal (compound, disjunction, negation, type only, ANDed with `where:`, `where:` alone); fail closed without a compiler or program; a `mentioned_by` step with a condition through the `_views` endpoint under an ACL.
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed): an evaluation error or a missing program fails the view; an unparseable `where:` no longer widens.

## Test Quality

- [x] Using fixture builders or factories for test data (`testViewAppWithMixedTypes`, `mentionedByApp`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
On a copy of the atlas config, `rela validate` passes with
`condition: "entity.type == 'project' or (entity.type == 'taak' and entity.status ~= 'gereed')"`
on the `topic` view's `bestaat_uit` step, refuses a typo with one line per
collected type (project, taak, terugkerend, werkitem), and refuses
`where: "status"` with `view "topic": traverse[0] has an invalid where`.

## Quality

- [x] Code follows project patterns (check similar code): list `condition:` seam, per-type keys as for action `when:`.
- [x] Checked for DRY opportunities: `collectionTypes` now calls `TraverseTargetTypes`; one `validateTraverseWhere` for views, `mentioned_by` and side panels.
- [x] No security issues introduced (condition reads the redacted entity)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

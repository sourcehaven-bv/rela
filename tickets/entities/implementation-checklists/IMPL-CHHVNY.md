---
id: IMPL-CHHVNY
type: implementation-checklist
title: 'Implementation: Entity references to faced types never resolve as mentions'
started: "2026-10-11"
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

Automated:
- `go test ./internal/dataentry/... ./internal/visibility/...`: pass.
- New `mentions_face_test.go` (router, real world middleware and
  `PolicyReader`): AC1 (ranked world, every face readable → published
  title), AC2 (draft-only → draft title), AC3 (published denied → draft;
  nothing readable → no mention), excluding world drops draft-only, AC5
  (hidden title → id). Both `/_views` and entity GET. Fails on
  origin/develop (4 of 5 subtests), passes now.
- AC4: existing `mentions_test.go`, `mentions_entity_get_test.go`,
  `acl_displaytitle_surfaces_test.go` pass with only the signature change.
- AC6: `TestCollectMentions_StoreErrorDegradesToNoMentions` passes.
- AC7: `markdown.test.ts` world / no-world cases; `src/components/entity`
  suite (279 tests) passes; `vue-tsc` clean.

Manual (rela-server built from this branch, Atlas `schema.yaml` and
`data-entry.yaml` in a temp project, procedure `PROCEDURE-XRJF` only at
`vastgesteld`, `PROCEDURE-WCTK` only at `concept`, source citing both):
- `GET /api/v1/_views/procedure/<src>` (bare, `@concept`, `?world=actueel`)
  and `GET /api/v1/procedures/<src>`: both ids in `mentions` with their
  titles.
- Headless Chromium on `/entity/procedure/<src>`: two links
  `/entity/procedure/PROCEDURE-XRJF?world=actueel` "Vastgestelde procedure"
  and `.../PROCEDURE-WCTK?world=actueel` "Alleen concept". Without
  `?world=` in the URL the links still carry `actueel`, the declared
  default world (same as neighbor links).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: a mention lookup fault is logged and drops the mentions by design, so a view never fails on them; pinned by `TestCollectMentions_StoreErrorDegradesToNoMentions`)
- [x] No debug code left behind

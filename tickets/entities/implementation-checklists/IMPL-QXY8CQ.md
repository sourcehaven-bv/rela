---
id: IMPL-QXY8CQ
type: implementation-checklist
title: 'Implementation: Direction inference for the remaining relation surfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

| File | Change |
| --- | --- |
| `internal/dataentryconfig/direction.go` | `CheckAmbiguousDirection` + `AmbiguousDirectionError`; `InferDirection` godoc generalized per surface |
| `internal/dataentryconfig/validate.go` | ambiguity guard on list columns, list filter controls, kanban card fields, kanban filter controls; form site now uses the shared error builder |
| `internal/dataentryconfig/validate_caldav.go` | ambiguity guard on dynamic collections, anchored on the member type |
| `internal/dataentry/api_v1.go` | `resolveConfigDirection` + `resolveListDirections` / `resolveKanbanDirections` / `resolveFilterControlDirections`; `resolveFormRelations` collapsed onto the shared resolver |
| `internal/dataentry/sections.go` | view-section relation columns resolve per row entity type |
| `internal/dataentry/caldav_backend.go` | `dynamicMembers` resolves an absent direction from the member type |
| `internal/migration/relation_direction.go` | renamed from `form_relation_direction.go`; extended to all surfaces; Detect/Apply share one `bindings()` traversal |
| `docs-project/entities/guides/GUIDE-data-entry.md` | generalized docs (+ regenerated `docs/`) |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Table-driven subtests throughout, reusing the existing `inverseTestMetamodel`,
`caldavMetamodel` and `directionTestMetamodel` fixtures rather than adding new
ones. `inverseTestMetamodel` gained an enum `status` property because kanban
validation requires an enum column property.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario
- [x] Edge cases manually verified

**Verification Evidence:**

Scratch project exercising all five surfaces:

```
$ rela migrate
  ✓ relation-direction: Write explicit direction: on relation bindings that omit it
# list column, list filter_control, kanban card field -> direction: incoming
# caldav tasks-per-project (member=task, from-side)   -> direction: outgoing
# both self-referencing depends-on bindings           -> untouched

$ rela validate
  ✗ caldav dynamic "blockers" needs an explicit `direction:` — entity type "task"
    is both a from and a to of relation "depends-on" ...
  ✗ list "tasks": column[0] needs an explicit `direction:` — ...
```

After hand-resolving both, validate is clean and a second migrate reports
"No migrations needed" (idempotent).

In-repo configs: `tickets/` and `prototypes/data-entry/project` validate clean;
migrate is a no-op on both, as expected — every relation binding in-repo is a
form relation, so this change is exercised by tests rather than by repo configs.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — one shared ambiguity guard and one shared
resolver instead of six and five copies respectively; the migration's Detect and
Apply share a single traversal
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned) — the resolver logs a
warning if an ambiguous binding somehow reaches the config handler, which
validation should have prevented
- [x] No debug code left behind

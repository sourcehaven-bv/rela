---
id: IMPL-5DDLS3
type: implementation-checklist
title: 'Implementation: Per-type columns and parent columns for the nested view section'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written
- [x] Happy path implemented
- [x] Edge cases handled
- [x] Error handling in place

**What was built:**

- `internal/dataentryconfig/config.go` — `ViewSection.ParentColumns` /
`.ChildColumns`, both `map[string][]ListColumn`.
- `internal/dataentryconfig/validate.go` — `collectionTypes` (returns the FULL
set of types a collection may hold, where `determineTargetType` returns one or
none), `validateLevelColumns`, and the `columns:`-on-a-nested-section refusal.
Removed `nestedChildDef`, the single-type workaround this replaces.
- `internal/dataentry/sections_nested.go` — per-level column selection,
`nestedColumnUnion` / `indexed` so relation columns still resolve in ONE query
across both levels rather than once per (level, type).
- `internal/dataentry/sections.go`, `internal/apiwire/v1/responses.go` —
`Columns` moved onto the NODE, since a nested section has no single column list.
- `frontend/` — `ViewTreeNode.columns`, and `nestedCellsFor` reading columns from
the node rather than the section.

## Test Quality

- [x] Fixture builders used
- [x] No hardcoded values where the object is in scope
- [x] Only values that matter are specified

13 validation cases in `TestValidateConfig_NestedSection`, including the two
that motivate the design: a heterogeneous relation with per-type columns, and
the SAME type at both levels taking different columns. Added a `relates-to`
relation (`ticket → [ticket, category]`) to the test metamodel so the
heterogeneous case is covered by a real relation rather than a contrived one.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Evidence:** the demo project was given a genuinely heterogeneous relation
(`has-task: {to: [task, bug]}`) plus a `bug` type with a `severity` property the
`task` type does not have.

Before: config load FAILED with `✗ view "project": section[1] column[2] property
"due" not in entity "epic"`.

After: config validates, and the rendered tree shows `BUG-1` with `Doing |
Critical` beside sibling tasks showing `status | owner | due` — different
columns, same list, same section. The epic parent shows `status | owner` only,
with no `due`.

Per AC: (1) heterogeneous loads and renders ✓ (2) per-type child columns ✓ (3)
parent columns independent ✓ (4) absent type renders title/id only ✓ (5) bad
property is a load error ✓ (6) same type both levels differs ✓ (7) `columns:`
refused, unchanged elsewhere ✓

## Quality

- [x] Follows project patterns
- [x] DRY checked
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code

Follows `Gantt.Sources` for per-type config and its "absent means default, not
error" rule. The ACL path is untouched — children still resolve against the
already-gated collection.

One cost note: relation columns resolve once over the UNION of both levels'
columns, then re-key per row (`indexed`). Resolving per (level, type) would have
reissued the query per group, undoing the batching that keeps this constant in
row count.

**Gates:** `golangci-lint` 0 issues · `arch-lint` · `comment-lint` · Go tests
green · `vue-tsc` · eslint 0 errors · 2456 frontend tests.

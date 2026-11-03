---
id: BUGA-E6TJSN
type: bug-analysis-checklist
title: 'Analysis: List filter and sort evaluate hidden fields (value disclosure)'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally. `internal/dataentry/list_field_gate_test.go` fails
on develop (b9f2b42bf): every filter operator (`eq`, `ne`, `contains`, `in`, the
`[]` array form, `lt`, `lte`, `gt`, `gte`) and `sort` (ascending, descending,
secondary key) on a property hidden by `visible:` answers 200 and is evaluated
on the raw value. The golden corpus records the same leak in
`BUG-WJSYQT-list-filter-hidden*` and `BUG-WJSYQT-list-sort-hidden`
(`?filter[secret][contains]=brav` returns TKT-2 to the viewer, whose `secret` is
hidden on that row).
- [x] Minimal reproduction steps documented. Policy: role `viewer` with
`read: [ticket]` and `visible: {ticket: [title]}`. Seed TKT-001 with `status:
bravo-secret`. As the viewer, `GET
/api/v1/tickets?filter[status][contains]=brav` returns TKT-001 and `...=bx`
returns nothing, so a binary search with `contains`, `gt` and `lt` recovers the
value. `?sort=status` orders rows by it.
- [x] Environment/conditions noted. Every backend. The Go path
(`applyV1Filters`, `applyV1Sorting`) evaluates the raw store rows, and the store
pushdown (`planListPushdown`) sends `eq`/`ne` and `ORDER BY` on a string-shaped
hidden property to the store. The planner is backend-agnostic, so pgstore and
sqlitestore receive the same predicate and ordering: the leak is the same there.
Only field-level `visible:` is involved; row gating is correct.

## Root Cause

- [x] Immediate cause identified (why1): `applyV1Filters`, `applyV1Sorting`
and `planListPushdown` read `e.Properties[name]` / push `PropPredicate` and
`OrderSpec` for any property the caller names. None of them asks whether the
principal may read that property.
- [x] Contributing factors found (why2-3): field redaction (`visible:`) is
applied only when a row is serialized (`stripHiddenProperties`,
`visibility.Redact`). Everything computed from the raw row before that point,
such as membership in a filter and position in a sort, sees the hidden value.
The list pipeline was written before field-level `visible:` existed, and when
`visible:` was added (and the search oracle closed in TKT-GGQ0JT) the list
filter and sort were not revisited.
- [x] Systemic cause explored (why4-5): redaction is a property of the
response body, not of the data a request may compute over. Each derived
computation (search match, ETag, gantt roll-up, filter, sort) has to remember to
respect it, and no shared seam or test forces that. The same gap exists in
`/_search` structured property filters and `sort:` (verified:
`/_search?q=status:bravo-secret` returns TKT-001 to the viewer); that is a
separate path and is reported for a follow-up bug.

## Fix Planning

- [x] Fix approach determined.
  1. `acl.Policy.FieldVisibleOnEveryRow(type, property, globalRoles,
ceiling)`: true only when no row can hide the property from this principal.
Hidden on a row means: some role applying to the row declares a `visible:` block
for the type, and no applying role grants the property on that row. The roles
that can apply are the principal's global roles plus every role a
`role_relations` entry can confer. The property is visible on every row when the
client ceiling does not hide it AND (no such role can hide it, i.e. each either
declares no `visible:` block for the type or grants the property without
`when:`; OR a global role grants it without `when:`).
  2. `affordances.PolicyResolver.FieldVisibleOnEveryRow(ctx, type, property)`
resolves the request's global roles and ceiling the same way `FieldVerdicts`
does, and answers true for a property the type does not declare (redaction never
hides one).
  3. `dataentry.FieldVerdictResolver` gains `FieldVisibleOnEveryRow`, so every
resolver must answer at compile time (Nop: true; Demo: its fixture).
  4. New `internal/dataentry/listfieldgate.go`: a `gatedListQuery` type that
only `gateListQuery` can build. It checks every property a property filter key
names (relation-control keys excluded) and every sort key except the `id` and
`modified` pseudo-properties. `scopedSortedEntitiesScoped` and
`listNarrowing.pushdownPlan` take `gatedListQuery` instead of the raw map, so
the Go filter, the Go sort and the store pushdown cannot be reached with an
unchecked query. `listPage`, `scopedSortedEntities`, `navStatusRows` and
`storePosition` build it.
  5. `writeListPipelineError` maps the error to 400 `unreadable_property`
whose detail names the property and type. Property names are config, not secrets
(root CLAUDE.md).
  6. `when:` decision: **reject**. A property visible on some rows only is
refused like a hidden one. Evaluating on redacted rows would need
`FieldVerdicts` for every row of the type before the page is cut (the pushdown
must decline for every such request), would filter on a redacted copy while
returning raw rows (redaction is non-composable, so the two would have to be
kept in step by hand), and gives a confusing result (a hidden value sorts as
absent). Rejecting is one decision per (principal, type, property), costs no row
reads, is identical on the Go path and the pushdown, and fails closed. A role
that should filter on the field gets an unconditional grant, which the rule
honors.
- [x] Regression test planned: `list_field_gate_test.go` covers each operator,
the array form, sort in both directions and as a secondary key, the `when:`
case, an unconditional grant beside a hiding role (allowed), the
`id`/`modified`/undeclared controls, and the shared pipeline
(`scopedSortedEntities`). Unit tests for `acl.Policy.FieldVisibleOnEveryRow`
(table) and the resolver. The golden corpus cases are renamed and re-recorded as
400s. Pins MEAS-LIST-FILTER-SORT-GATED.
- [x] Related areas checked for similar issues: `/_search` structured filters
and `sort:` (queryservice.executeQueryPrefiltered) leak the same way (to be
filed separately); view `condition:` already evaluates on redacted rows
(`redactedForSuggestion`); relation filters match neighbour titles, which are
redacted by the display-property fallback and are out of scope here.
Config-authored list `filters:`/`sort:` reach the same gate through
`navStatusRows` / `navItemsListQuery`, so a configured filter on a field a
principal cannot read on every row now fails for that principal instead of
counting over hidden values.

## Revision after design review

The go-architect and rela-security-reviewer findings (RR-5X1GV2 to RR-YF00X7)
changed the plan as follows:

- The rule moved from `acl.Policy` to `affordances.PolicyResolver`, over the
same compiled grants `FieldVerdicts` reads, plus a new
`acl.FieldCeiling.Hides`. A differential test keeps the two in step.
- `gatedListQuery` carries the gate's relation-key classifier, and the filter,
sort, relation pass and pushdown take it. One key parser (`parseFilterKey`)
serves all of them. A guard test allows the literal only in `listfieldgate.go`.
- The pushdown declines a virtual sort key, so `sort=modified` on a type that
declares a `modified` property cannot order by its hidden value.
- List `?q=` searched with the unredacted searcher, the same oracle. It now
goes through the field-provenance search `/_search` uses.
- The error code is `property_not_visible`.

## Revision after the role-union fix

Review finding RR-FFXKEK showed administrators refused filter and sort on a
field a conferrable local role hides. The cause was a non-monotonic role
union in `FieldVerdicts`, fixed separately in BUG-Y5UF3K: a role without a
`visible:` block now shows every field when it reads the row's type and face,
and adding a role never removes a field.

`FieldVisibleOnEveryRow` was rewritten on that rule. When a global role reads
the type, the field must be shown on each face a global role reads, by a
global role that reads that face and declares no block, or by a global block
that grants it without `when:`. Only when no global role reads the type does
it look at the roles `role_relations` confer, because those are then the only
way a row is admitted. `acl.Policy.ConferrableRoles` was removed. Admin and
editor may filter and sort on `secret` again; the viewer is still refused.

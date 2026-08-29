---
id: TKT-SJ0LRS
type: ticket
title: Extract dataentry query/search leaf off App (92 → ~87), de-risking the read-pipeline steps
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of the [[TKT-R68TV8]] `dataentry.App` decomposition arc, following
[[TKT-8AJ1PM]] (104 → 92).

The App survey ranked this as the step to do **before** next-action, scope and
the read pipeline: those three clusters all *borrow* these helpers, so
extracting the leaf first turns three later closure-threading problems into one
shared collaborator.

## What

**Pure structural extraction, no behavior change, no ACL change.**

Move the search/query/sort helpers from `helpers.go` onto a focused
`queryService` leaf (the `entityReader`/`visibleReader` value-type shape):

- `executeQuery`, `runVisibleFreeTextSearch`, `freeTextIDsForType`,
`sortEntitiesMulti`, `matchesPropertyFilters`
- **Delete `isRelationLinked`** — the survey found zero callers. Verify
repo-wide before deleting.

Deps: `visibleSearcher` (the ACL-scoped searcher — it stays the ACL-scoped one;
this extraction must not introduce an ungated search path), plus the affordance
service and the schema/Services closures the helpers already use.

`executeQuery` has callers in `nextaction.go` and `scope.go` as well as the read
pipeline — they become `a.queries.executeQuery(...)` (or take the leaf), which
is exactly the point: one collaborator instead of three closures later.

Ratchet `//plimsoll:max-methods` on App 92 → the real count (~87 after the 5
moves plus the dead-method deletion).

## Done when

plimsoll with the lowered directive; full suite + `-race` on dataentry green;
the ACL search tests (the `search.VisibleSearcher` contract) pass unchanged;
coverage floors hold; arch-lint/comment-lint/lint clean.
